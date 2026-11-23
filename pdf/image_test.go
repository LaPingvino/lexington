package pdf_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LaPingvino/lexington/pdf"
)

// picture is a test picture: blocks of colour, w × h pixels (their edges
// on a JPEG's 16-pixel blocks for w and h multiples of 32).
func picture(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{200, 40, 40, 255}
			if x >= w/2 {
				c = color.NRGBA{40, 60, 200, 255}
			}
			if y >= h/2 {
				c.G = 180
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func encoded(t *testing.T, format string, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&b, img)
	case "png16":
		img16 := image.NewNRGBA64(img.Bounds())
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				img16.Set(x, y, img.At(x, y))
			}
		}
		err = png.Encode(&b, img16)
	case "jpeg":
		err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 95})
	case "gif":
		err = gif.Encode(&b, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// imagePDF is a page with a picture on it.
func imagePDF(t *testing.T, data []byte, record bool) ([]byte, pdf.Drawing) {
	t.Helper()
	f := pdf.NewFpdf("P", "pt", "A5")
	var d pdf.Drawer = f
	var r *pdf.Recorder
	if record {
		r = pdf.NewRecorder(f)
		d = r
	}
	d.AddPage()
	w, h, err := pdf.AddImage(d, "pic", data)
	if err != nil {
		t.Fatal(err)
	}
	if w != 2*h {
		t.Fatalf("size %d×%d", w, h)
	}
	pdf.DrawImage(d, "pic", 40, 70, 240, 120)
	var b bytes.Buffer
	if err := d.Output(&b); err != nil {
		t.Fatal(err)
	}
	if r == nil {
		return b.Bytes(), pdf.Drawing{}
	}
	return b.Bytes(), r.Drawing()
}

// A picture in each format is drawn into the PDF, the same PDF recorded
// or not; the drawing has it where it was put, and paints it as pdftoppm
// paints the PDF (enlarged, made smaller, enlarged more than four times).
func TestImages(t *testing.T) {
	pdf.FixDate()
	for _, c := range []struct {
		format string
		w, h   int
	}{{"png", 128, 64}, {"png16", 128, 64}, {"jpeg", 128, 64}, {"gif", 128, 64}, {"png", 64, 32}, {"jpeg", 512, 256}} {
		format := c.format
		t.Run(fmt.Sprintf("%s-%d", format, c.w), func(t *testing.T) {
			data := encoded(t, format, picture(c.w, c.h))
			plain, _ := imagePDF(t, data, false)
			recorded, d := imagePDF(t, data, true)
			if !bytes.Equal(plain, recorded) {
				t.Error("the PDF differs when recorded")
			}
			var got []pdf.Image
			for _, m := range d.Pages[0].Marks {
				if im, ok := m.(pdf.Image); ok {
					got = append(got, im)
				}
			}
			if len(got) != 1 || got[0].X != 40 || got[0].Y != 70 || got[0].W != 240 || got[0].H != 120 || d.Images["pic"] == nil {
				t.Fatalf("recorded %+v", got)
			}
			if !d.Pages[0].Equal(d.Pages[0]) {
				t.Error("a page with a picture is not equal to itself")
			}
			if _, err := exec.LookPath("pdftoppm"); err != nil {
				return
			}
			if dir := os.Getenv("LEXINGTON_FIDELITY"); dir != "" {
				os.WriteFile(filepath.Join(dir, "image-"+format+".pdf"), recorded, 0o644)
			}
			shots := pdftoppm(t, recorded)
			painted := pdf.NewRasterizer(&d).Page(0, fidelityDPI/72.0)
			// (Poppler smooths an enlarged picture's edges over a pixel or
			// two more than x/image's bilinear scaling)
			if m := compare(painted, shots[0]); m.size != "" || m.score() < 0.99 {
				saveFailure(t, "image-"+format, 0, painted, shots[0])
				t.Errorf("painted unlike the PDF: %v", m)
			}
		})
	}
}

// What is not a picture is AddImage's error, and the PDF is still made.
func TestImageNotAPicture(t *testing.T) {
	r := pdf.NewRecorder(pdf.NewFpdf("P", "pt", "A5"))
	r.AddPage()
	if _, _, err := pdf.AddImage(r, "x", []byte("<svg/>")); err == nil {
		t.Error("an SVG read as a picture")
	}
	var b bytes.Buffer
	if err := r.Output(&b); err != nil {
		t.Errorf("the PDF failed: %v", err)
	}
}
