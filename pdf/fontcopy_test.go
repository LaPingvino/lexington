package pdf_test

import (
	"crypto/sha256"
	"testing"

	"github.com/LaPingvino/lexington/font"
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
