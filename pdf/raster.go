package pdf

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Rasterizer paints a Drawing's pages as images: its text in the fonts
// the PDF embeds, a glyph at a time at the advances gofpdf gave them (no
// kerning, no hinting, as in the PDF). The standard fonts, which the PDF
// does not embed, are painted in the Go fonts (Go Mono for Courier, Go
// for Helvetica and Times), each run stretched to the width it has in
// the PDF. A Rasterizer keeps the faces and glyphs it made; it is safe
// for use by several goroutines.
type Rasterizer struct {
	// Fallback, when set, draws a character the drawing's font has no
	// outline for (in the browser, where the PDF writer has no fonts of
	// the computer's: fallback.go), size pixels to the em in the advance
	// w the font gives it (its cells): a mask whose bounds are from the
	// pen on the baseline (y down), nil for none. Set it before painting.
	Fallback func(s string, size, w float64) *image.Alpha
	d        *Drawing
	mu       sync.Mutex
	fonts    map[string]*opentype.Font
	faces    map[faceKey]font.Face
	glyphs   map[glyphKey]*glyph
}

type faceKey struct {
	font string
	size float64 // in pixels
}

type glyphKey struct {
	face   faceKey
	r      rune
	fx, fy uint8 // the pen's position within its pixel, in quarters
}

type glyph struct {
	mask *image.Alpha
	at   image.Point // the mask's top left from the pen's pixel
	adv  fixed.Int26_6
}

// NewRasterizer paints d's pages.
func NewRasterizer(d *Drawing) *Rasterizer {
	return &Rasterizer{d: d, fonts: map[string]*opentype.Font{}, faces: map[faceKey]font.Face{},
		glyphs: map[glyphKey]*glyph{}}
}

// Image is page i (from 0) of the drawing at scale pixels a point.
func (d *Drawing) Image(i int, scale float64) *image.RGBA { return NewRasterizer(d).Page(i, scale) }

// standard are the Go fonts painting the standard fonts.
var standard = map[string][]byte{
	"courier": gomono.TTF, "courierB": gomonobold.TTF, "courierI": gomonoitalic.TTF, "courierBI": gomonobolditalic.TTF,
	"helvetica": goregular.TTF, "helveticaB": gobold.TTF, "helveticaI": goitalic.TTF, "helveticaBI": gobolditalic.TTF,
	"times": goregular.TTF, "timesB": gobold.TTF, "timesI": goitalic.TTF, "timesBI": gobolditalic.TTF,
}

// Page is page i (from 0) at scale pixels a point: white paper, what is
// drawn on it in its order.
func (r *Rasterizer) Page(i int, scale float64) *image.RGBA { return r.Paint(r.d.Pages[i], scale) }

// Paint paints a page in the drawing's fonts (a page of another drawing
// with those fonts too: one made again from a page, Reflow), as Page.
func (r *Rasterizer) Paint(p DrawnPage, scale float64) *image.RGBA {
	w, h := int(math.Ceil(p.Width*scale)), int(math.Ceil(p.Height*scale))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for k := range img.Pix {
		img.Pix[k] = 255
	}
	for _, m := range p.Marks {
		switch m := m.(type) {
		case Box:
			r.box(img, m, scale)
		case Text:
			r.text(img, m, scale)
		case Image:
			r.image(img, m, scale)
		}
	}
	return img
}

func (r *Rasterizer) box(img *image.RGBA, b Box, scale float64) {
	if b.Fill {
		fillRect(img, b.X*scale, b.Y*scale, (b.X+b.W)*scale, (b.Y+b.H)*scale, b.FillColor, b.Paint)
	}
	if b.Stroke {
		x0, y0, x1, y1, lw := b.X*scale, b.Y*scale, (b.X+b.W)*scale, (b.Y+b.H)*scale, b.LineWidth*scale
		c := b.StrokeColor
		fillRect(img, x0-lw/2, y0-lw/2, x1+lw/2, y0+lw/2, c, b.Paint)
		fillRect(img, x0-lw/2, y1-lw/2, x1+lw/2, y1+lw/2, c, b.Paint)
		fillRect(img, x0-lw/2, y0+lw/2, x0+lw/2, y1-lw/2, c, b.Paint)
		fillRect(img, x1-lw/2, y0+lw/2, x1+lw/2, y1-lw/2, c, b.Paint)
	}
}

// fillRect paints the rectangle from (x0, y0) to (x1, y1), its edges
// on whole pixels as PDF viewers paint them (crisp edges), at least a
// pixel wide (a hairline shows).
func fillRect(img *image.RGBA, x0, y0, x1, y1 float64, c color.RGBA, p Paint) {
	x0, x1 = hairline(x0, x1)
	y0, y1 = hairline(y0, y1)
	b := img.Bounds()
	for y := max(int(math.Floor(y0)), b.Min.Y); y < min(int(math.Ceil(y1)), b.Max.Y); y++ {
		cy := math.Min(y1, float64(y+1)) - math.Max(y0, float64(y))
		for x := max(int(math.Floor(x0)), b.Min.X); x < min(int(math.Ceil(x1)), b.Max.X); x++ {
			cx := math.Min(x1, float64(x+1)) - math.Max(x0, float64(x))
			if cov := cx * cy; cov > 0 {
				blend(img, x, y, c, p, cov)
			}
		}
	}
}

// hairline is the pixels from a to b: their edges rounded, at least a
// pixel wide (a line of width 0 is the thinnest there is).
func hairline(a, b float64) (float64, float64) {
	if b < a {
		return a, a
	}
	a, b = math.Round(a), math.Round(b)
	if b == a {
		b = a + 1
	}
	return a, b
}

// blend paints c over the pixel at (x, y), covering cov of it.
func blend(img *image.RGBA, x, y int, c color.RGBA, p Paint, cov float64) {
	a := cov * p.Alpha
	if a <= 0 {
		return
	}
	o := img.PixOffset(x, y)
	px := img.Pix[o : o+3 : o+3]
	src := [3]float64{float64(c.R), float64(c.G), float64(c.B)}
	for k := range 3 {
		d := float64(px[k])
		s := src[k]
		if strings.EqualFold(p.Blend, "Multiply") {
			s = d * s / 255
		}
		px[k] = uint8(math.Round(d*(1-a) + s*a))
	}
}

func (r *Rasterizer) face(key string, px float64) (faceKey, font.Face) {
	fk := faceKey{key, px}
	if f, ok := r.faces[fk]; ok {
		return fk, f
	}
	f, ok := r.fonts[key]
	if !ok {
		data := r.d.Fonts[key]
		if data == nil {
			data = standard[key]
		}
		if data == nil {
			data = gomono.TTF
		}
		f, _ = opentype.Parse(data)
		r.fonts[key] = f
	}
	var face font.Face
	if f != nil {
		face, _ = opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingNone})
	}
	r.faces[fk] = face
	return fk, face
}

// lacks tells whether the font key has no outline for c that prints:
// no glyph, or an empty one (fallback.go's for a character no font on
// the computer has), for a character that takes a cell.
func (r *Rasterizer) lacks(key string, c rune) bool {
	if unicode.IsSpace(c) || unicode.IsControl(c) || zeroCell(c) {
		return false
	}
	f := r.fonts[key]
	if f == nil {
		return false
	}
	var buf sfnt.Buffer
	g, err := f.GlyphIndex(&buf, c)
	if err != nil {
		return false
	}
	if g == 0 {
		return true
	}
	segs, err := f.LoadGlyph(&buf, g, 64*64, nil)
	return err == nil && len(segs) == 0
}

func (r *Rasterizer) text(img *image.RGBA, t Text, scale float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fk, face := r.face(t.Font, t.Size*scale)
	if face == nil {
		return
	}
	runes := []rune(t.Text)
	natural := 0.0
	for _, c := range runes {
		if adv, ok := face.GlyphAdvance(c); ok {
			natural += float64(adv) / 64
		}
	}
	stretch := 1.0
	if natural > 0 && t.Width > 0 {
		stretch = t.Width * scale / natural
	}
	pen, base := t.X*scale, t.Y*scale
	for _, c := range runes {
		ix, iy := math.Floor(pen), math.Floor(base)
		fx, fy := uint8((pen-ix)*4), uint8((base-iy)*4)
		gk := glyphKey{fk, c, fx, fy}
		g, ok := r.glyphs[gk]
		if !ok {
			g = &glyph{}
			dot := fixed.Point26_6{X: fixed.Int26_6(fx) * 16, Y: fixed.Int26_6(fy) * 16}
			if dr, mask, mp, adv, ok := face.Glyph(dot, c); ok && (r.Fallback == nil || !r.lacks(fk.font, c)) {
				a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
				for y := 0; y < dr.Dy(); y++ {
					for x := 0; x < dr.Dx(); x++ {
						_, _, _, ma := mask.At(mp.X+x, mp.Y+y).RGBA()
						a.Pix[y*a.Stride+x] = uint8(ma >> 8)
					}
				}
				g.mask, g.at, g.adv = a, dr.Min, adv
			} else {
				g.adv, _ = face.GlyphAdvance(c)
			}
			if r.Fallback != nil && (g.mask == nil || g.mask.Rect.Empty()) && r.lacks(fk.font, c) {
				if m := r.Fallback(string(c), t.Size*scale, float64(g.adv)/64); m != nil && !m.Rect.Empty() {
					a := image.NewAlpha(image.Rect(0, 0, m.Rect.Dx(), m.Rect.Dy()))
					for y := 0; y < m.Rect.Dy(); y++ {
						copy(a.Pix[y*a.Stride:y*a.Stride+m.Rect.Dx()], m.Pix[m.PixOffset(m.Rect.Min.X, m.Rect.Min.Y+y):])
					}
					g.mask, g.at = a, m.Rect.Min
				}
			}
			r.glyphs[gk] = g
		}
		if g.mask != nil {
			ox, oy := int(ix)+g.at.X, int(iy)+g.at.Y
			b := img.Bounds()
			for y := 0; y < g.mask.Rect.Dy(); y++ {
				for x := 0; x < g.mask.Rect.Dx(); x++ {
					if m := g.mask.Pix[y*g.mask.Stride+x]; m != 0 {
						if p := (image.Point{ox + x, oy + y}); p.In(b) {
							blend(img, p.X, p.Y, t.Color, t.Paint, float64(m)/255)
						}
					}
				}
			}
		}
		pen += float64(g.adv) / 64 * stretch
	}
}
