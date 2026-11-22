package pdf_test

import (
	"crypto/sha256"
	"testing"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/phpdave11/gofpdf"
)

// Making PDFs leaves the embedded fonts as they were: gofpdf writes into
// the bytes it is given (pdf.AddFont), so it must never get the program's own.
func TestPDFLeavesTheFontsAlone(t *testing.T) {
	faces := map[string][]byte{
		"Regular": font.CourierBadiRegular, "Bold": font.CourierBadiBold,
		"Italic": font.CourierBadiItalic, "BoldItalic": font.CourierBadiBoldItalic,
	}
	before := map[string][32]byte{}
	for n, b := range faces {
		before[n] = sha256.Sum256(b[:cap(b)])
	}
	for range 2 {
		printRTL(t, hebrewScript, true)
		printRTL(t, hebrewScript, false)
	}
	for n, b := range faces {
		if sha256.Sum256(b[:cap(b)]) != before[n] {
			t.Errorf("CourierBadi-%s changed while making PDFs", n)
		}
	}
}

// Through a Recorder a font keeps its identity: the drawing holds the
// bytes it was given (the same slice), and gofpdf still gets a copy.
func TestAddFontKeepsIdentityInARecorder(t *testing.T) {
	r := pdf.NewRecorder(gofpdf.New("P", "pt", "Letter", ""))
	b := font.CourierBadiRegular
	pdf.AddFont(r, "CourierBadi", "", b)
	got := r.Drawing().Fonts["courierbadi"]
	if len(got) == 0 || &got[0] != &b[0] {
		t.Error("the drawing's font is not the bytes it was given")
	}
}
