package pdf

import (
	"math"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/rules"
)

// The pagination counts the lines a row prints as printing it does:
// plain text (MultiCell), styled (Write), another font, words longer
// than the line.
func TestLinesAsPrinted(t *testing.T) {
	pdf := gofpdf.New("P", "in", "Letter", "")
	for _, s := range []string{"", "B", "I", "BI"} {
		pdf.AddUTF8FontFromBytes("CourierPrime", s, font.GetFont("CourierPrime", s))
	}
	pdf.SetMargins(1, 1, 1)
	pdf.SetAutoPageBreak(false, 0)
	tree := &Tree{PDF: pdf, Rules: rules.Default, Fonts: newFonts(pdf)}
	words := strings.Fields("I never thought it would come to this. We had plans, you know. Big ones. " +
		"The house by the sea, the boat, the dog, and a supercalifragilisticexpialidociouslylongword too.")
	var texts []string
	for n := 1; n <= len(words); n++ {
		texts = append(texts, strings.Join(words[:n], " "))
	}
	texts = append(texts, "He says *hello* to **everyone** in _the room_, and then *some more words* go on and on and on.",
		"One line\nand another\n\nand one after a blank", strings.Repeat("x", 150), "Ça va? Schöne Grüße, señor.")
	for _, key := range []string{"action", "dialog", "paren", "lyrics", "trans"} {
		f := rules.Default.Get(key)
		for _, text := range texts {
			pdf.AddPage()
			y := pdf.GetY()
			tree.linePrint(f, text)
			got := int(math.Round((pdf.GetY() - y) / lineHeight))
			if want := tree.height(key, text); got != want {
				t.Errorf("%s %q: prints %d lines, counted %d", key, text, got, want)
			}
		}
	}
}
