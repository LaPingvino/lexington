package pdf

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // the formats AddImage reads
	_ "image/jpeg"
	"image/png"
	"io"
	"math"

	"github.com/phpdave11/gofpdf"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Pictures in a PDF: AddImage reads a picture into the PDF (PNG, JPEG,
// GIF or WebP; what gofpdf cannot read as it is — a GIF, a WebP, an
// interlaced or 16-bit PNG — made an 8-bit PNG first), so a bad file is
// an error of its own, never gofpdf's (whose errors stick to the whole
// PDF). DrawImage places it. A Recorder records the picture (Image) and
// keeps it decoded in the Drawing's Images, which the Rasterizer paints.

// ErrImage is a picture AddImage cannot read.
var ErrImage = errors.New("not a picture PDF can show (PNG, JPEG, GIF or WebP)")

// ImageSize is a picture's size in pixels, read from its header.
func ImageSize(data []byte) (w, h int, err error) {
	c, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, ErrImage
	}
	return c.Width, c.Height, nil
}

// AddImage reads the picture data into d under name (once: a name added
// already is kept), and gives its size in pixels.
func AddImage(d Drawer, name string, data []byte) (w, h int, err error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, ErrImage
	}
	b := img.Bounds()
	tp := "png"
	switch {
	case format == "jpeg" && jpegFits(img):
		tp = "jpg"
	case format == "png" && pngFits(data):
	default: // as an 8-bit PNG, non-interlaced
		var buf bytes.Buffer
		if err := png.Encode(&buf, flat(img)); err != nil {
			return 0, 0, err
		}
		data = buf.Bytes()
	}
	if r, ok := d.(*Recorder); ok {
		r.drawing.Images[name] = img
	}
	d.RegisterImageOptionsReader(name, gofpdf.ImageOptions{ImageType: tp}, bytes.NewReader(data))
	return b.Dx(), b.Dy(), nil
}

// jpegFits: gofpdf takes the JPEG as it is (grey, RGB or CMYK).
func jpegFits(img image.Image) bool {
	switch img.(type) {
	case *image.Gray, *image.YCbCr, *image.CMYK:
		return true
	}
	return false
}

// pngFits: gofpdf reads the PNG as it is: 8 bits a channel (or fewer, in
// a palette or grey), not interlaced.
func pngFits(data []byte) bool {
	// the IHDR chunk: 8 bytes of signature, length, "IHDR", width,
	// height, then bit depth, colour type, compression, filter, interlace
	if len(data) < 29 || string(data[12:16]) != "IHDR" {
		return false
	}
	depth, interlace := data[24], data[28]
	return depth <= 8 && interlace == 0
}

// flat is img with 8 bits a channel: as it is if it has them.
func flat(img image.Image) image.Image {
	switch img.(type) {
	case *image.NRGBA, *image.RGBA, *image.Gray, *image.Paletted:
		return img
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}

// DrawImage places the picture added as name in the rectangle x, y, w, h
// (in the document's unit) on the page.
func DrawImage(d Drawer, name string, x, y, w, h float64) {
	d.ImageOptions(name, x, y, w, h, false, gofpdf.ImageOptions{}, 0, "")
}

// RegisterImageOptionsReader reads a picture into the PDF as gofpdf's,
// keeping it decoded for the drawing (AddImage does that too, and better:
// it reads what gofpdf cannot).
func (r *Recorder) RegisterImageOptionsReader(name string, options gofpdf.ImageOptions, rd io.Reader) *gofpdf.ImageInfoType {
	if _, ok := r.drawing.Images[name]; ok {
		return r.Fpdf.RegisterImageOptionsReader(name, options, rd)
	}
	data, err := io.ReadAll(rd)
	if err != nil {
		r.SetError(err)
		return nil
	}
	if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		r.drawing.Images[name] = img
	}
	return r.Fpdf.RegisterImageOptionsReader(name, options, bytes.NewReader(data))
}

// ImageOptions places a picture as gofpdf's, and records it.
func (r *Recorder) ImageOptions(name string, x, y, w, h float64, flow bool, options gofpdf.ImageOptions, link int, linkURL string) {
	r.Fpdf.ImageOptions(name, x, y, w, h, flow, options, link, linkURL)
	img := r.drawing.Images[name]
	if r.Err() || img == nil || w < 0 || h < 0 {
		return // (one placed by its dots per inch is not recorded)
	}
	px, py := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	k := r.GetConversionRatio()
	switch {
	case w == 0 && h == 0: // at 96 dpi, as gofpdf
		w, h = px*72/96/k, py*72/96/k
	case w == 0:
		w = h * px / py
	case h == 0:
		h = w * py / px
	}
	if flow {
		y = r.GetY() - h
		if x < 0 {
			x = r.GetX()
		}
	}
	r.add(Image{X: x * k, Y: y * k, W: w * k, H: h * k, Name: name, Paint: r.paint()})
	if link > 0 || linkURL != "" {
		r.link(x, y, w, h, link, linkURL)
	}
}

// image paints a picture, scaled into its rectangle.
func (r *Rasterizer) image(img *image.RGBA, m Image, scale float64) {
	src := r.d.Images[m.Name]
	if src == nil {
		return
	}
	// its edges on whole pixels, as a PDF viewer paints it
	rect := image.Rect(int(math.Floor(m.X*scale)), int(math.Floor(m.Y*scale)),
		int(math.Ceil((m.X+m.W)*scale)), int(math.Ceil((m.Y+m.H)*scale)))
	if rect.Empty() {
		return
	}
	var opts *xdraw.Options
	if a := m.Alpha; a > 0 && a < 1 {
		opts = &xdraw.Options{DstMask: image.NewUniform(color.Alpha{A: uint8(a * 255)})}
	}
	// as Poppler paints it: smoothly, but in blocks when made more than
	// four times as large
	var scaler xdraw.Scaler = xdraw.BiLinear
	if rect.Dx() < src.Bounds().Dx() {
		scaler = xdraw.CatmullRom
	} else if rect.Dx() > 4*src.Bounds().Dx() {
		scaler = xdraw.NearestNeighbor
	}
	scaler.Scale(img, rect, src, src.Bounds(), draw.Over, opts)
}

// mergeImages is a drawing's pictures with o's (a drawing made again from
// a page has those it drew only).
func mergeImages(a, b map[string]image.Image) map[string]image.Image {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	out := make(map[string]image.Image, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
