package pdfin

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"

	"golang.org/x/image/ccitt"

	"github.com/LaPingvino/lexington/internal/jbig2"

	"github.com/LaPingvino/lexington/internal/pdf"
)

// pageImage is the largest image on a page: the scan of a scanned page.
func pageImage(p pdf.Page) (image.Image, error) {
	xobjects := p.V.Key("Resources").Key("XObject")
	var best pdf.Value
	area := int64(0)
	for _, name := range xobjects.Keys() {
		x := xobjects.Key(name)
		if x.Key("Subtype").Name() != "Image" {
			continue
		}
		if a := x.Key("Width").Int64() * x.Key("Height").Int64(); a > area {
			best, area = x, a
		}
	}
	if area == 0 {
		return nil, fmt.Errorf("no text and no image on the page")
	}
	return decodeImage(best)
}

// decodeImage decodes a PDF image: JPEG, fax (CCITT) or plain pixels.
func decodeImage(x pdf.Value) (img image.Image, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("decoding the image: %v", p)
		}
	}()
	w, h := int(x.Key("Width").Int64()), int(x.Key("Height").Int64())
	filter := x.Key("Filter")
	params := x.Key("DecodeParms")
	if filter.Kind() == pdf.Array { // the last filter is the image's own
		if n := filter.Len(); n > 0 {
			filter, params = filter.Index(n-1), params.Index(n-1)
		}
	}
	switch filter.Name() {
	case "DCTDecode":
		return jpeg.Decode(x.RawReader())
	case "CCITTFaxDecode":
		sf := ccitt.Group3
		if params.Key("K").Int64() < 0 {
			sf = ccitt.Group4
		}
		if cols := params.Key("Columns").Int64(); cols > 0 {
			w = int(cols)
		}
		gray := image.NewGray(image.Rect(0, 0, w, h))
		// fax is white on black unless BlackIs1
		err := ccitt.DecodeIntoGray(gray, x.RawReader(), ccitt.MSB, sf,
			&ccitt.Options{Align: params.Key("EncodedByteAlign").Bool(), Invert: params.Key("BlackIs1").Bool()})
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		return gray, nil
	case "JBIG2Decode":
		data, err := io.ReadAll(x.RawReader())
		if err != nil {
			return nil, err
		}
		var globals []byte
		if g := params.Key("JBIG2Globals"); g.Kind() == pdf.Stream {
			if globals, err = io.ReadAll(g.Reader()); err != nil {
				return nil, err
			}
		}
		gray, err := jbig2.Decode(data, globals, w, h)
		if err != nil {
			return nil, err
		}
		// ink is black, unless the PDF turns its samples round: a Decode
		// of [1 0], or a palette whose first entry is light (some scanners
		// store the paper as 1s)
		if paperIsZero(x) {
			for i := range gray.Pix {
				gray.Pix[i] = 255 - gray.Pix[i]
			}
		}
		return gray, nil
	case "JPXDecode":
		return nil, fmt.Errorf("the scan is in %s, which cannot be read yet", filter.Name())
	}
	// plain pixels, perhaps Flate-compressed
	data, err := io.ReadAll(x.Reader())
	if err != nil {
		return nil, err
	}
	return rawImage(data, w, h, int(x.Key("BitsPerComponent").Int64()), components(x.Key("ColorSpace")))
}

func components(cs pdf.Value) int {
	name := cs.Name()
	if cs.Kind() == pdf.Array && cs.Len() > 0 {
		name = cs.Index(0).Name()
		if name == "ICCBased" {
			return int(cs.Index(1).Key("N").Int64())
		}
	}
	switch name {
	case "DeviceRGB", "CalRGB":
		return 3
	case "DeviceCMYK":
		return 4
	}
	return 1
}

// rawImage is uncompressed image data as a grey image.
func rawImage(data []byte, w, h, bits, comps int) (image.Image, error) {
	if bits == 0 {
		bits = 8
	}
	stride := (w*bits*comps + 7) / 8
	if len(data) < stride*h {
		return nil, fmt.Errorf("the image's data is short")
	}
	gray := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		row := data[y*stride : (y+1)*stride]
		for x := 0; x < w; x++ {
			var v uint8
			switch {
			case bits == 1:
				if row[x/8]&(0x80>>(x%8)) != 0 {
					v = 255
				}
			case bits == 8 && comps == 1:
				v = row[x]
			case bits == 8 && comps >= 3:
				r, g, b := row[x*comps], row[x*comps+1], row[x*comps+2]
				if comps == 4 { // CMYK: light where there is little ink
					v = uint8(255 - min(255, (int(r)+int(g)+int(b))/3+int(row[x*comps+3])))
				} else {
					v = color.GrayModel.Convert(color.RGBA{r, g, b, 255}).(color.Gray).Y
				}
			default:
				return nil, fmt.Errorf("images with %d bits per component cannot be read", bits)
			}
			gray.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return gray, nil
}

// pngOf is an image encoded for an OCR program.
func pngOf(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := encodePNG(&b, img)
	return b.Bytes(), err
}

// paperIsZero reports whether a 1-bit image's 0 samples are the paper
// (light) rather than the ink: through its Decode array or its palette.
func paperIsZero(x pdf.Value) bool {
	inverted := false
	if d := x.Key("Decode"); d.Len() == 2 && d.Index(0).Float64() > d.Index(1).Float64() {
		inverted = true
	}
	cs := x.Key("ColorSpace")
	if cs.Kind() != pdf.Array || cs.Len() < 4 || cs.Index(0).Name() != "Indexed" {
		return inverted
	}
	var lookup []byte
	switch l := cs.Index(3); l.Kind() {
	case pdf.String:
		lookup = []byte(l.RawString())
	case pdf.Stream:
		lookup, _ = io.ReadAll(l.Reader())
	}
	n := components(cs.Index(1)) // bytes per palette entry
	if n < 1 || len(lookup) < 2*n {
		return inverted
	}
	light := func(e []byte) int {
		sum := 0
		for _, b := range e {
			sum += int(b)
		}
		if n == 4 { // CMYK: ink makes it dark
			return 255 - sum/4
		}
		return sum / n
	}
	// sample 0 is the palette's first entry (after Decode)
	first, second := lookup[:n], lookup[n:2*n]
	if inverted {
		first, second = second, first
	}
	return light(first) > light(second)
}
