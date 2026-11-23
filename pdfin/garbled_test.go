package pdfin

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

// Words of a printed script look like words; a scan's stray marks, glued
// punctuation, mixed case and spread letters do not.
func TestTextQualityWords(t *testing.T) {
	for _, c := range []struct {
		text string
		odd  int
	}{
		{"INT. HOUSE - DAY", 0},
		{"He's well-known in the U.S.A. -- since 1,000 B.C. at 3:15, the 21st.", 0},
		{"McDONALD (O.S.) and/or MacLeod... scene 12A, 4-A, 80s", 0},
		{"Tink ! « Oui ! » Je te tiens ? ... Bram", 0},
		{"Love you ♪ & 20% off — fine {{12}}", 0},
		{"שלום, עולם! צה״ל", 0},
		{"Привет, мир. Ἐν ἀρχῇ ἦν ὁ λόγος", 0},
		{"你好，世界。「こんにちは」", 0},
		{"_ Marta (softly) |", 2},
		{"door. }", 1},
		{"Ivo_(pointing l0ve tHe", 3},
		{"THE OLD“MILL”", 1},
		{"T H R E E", 5}, // every letter of the spread word
		{"Рrivet", 1},    // a Cyrillic Р before Latin letters
	} {
		var q TextQuality
		q.add(c.text)
		if q.Odd != c.odd {
			t.Errorf("%q: %d odd, want %d", c.text, q.Odd, c.odd)
		}
	}
}

// Every page Lexington prints scores 1, in every script it prints, and
// none counts as garbled.
func TestCleanLayersScoreOne(t *testing.T) {
	files, _ := filepath.Glob("../examples/*.fountain")
	scripts := map[string]string{}
	for _, f := range files {
		src, _ := os.ReadFile(f)
		scripts[filepath.Base(f)] = string(src)
	}
	scripts["languages"] = `INT. КУХНЯ - ДЕНЬ

Анна ставит чайник. "Hello," she says — to nobody.

ANNA
Доброе утро! Сколько стоит 1,50 €?

EXT. ΑΓΟΡΑ - NIGHT

Ἐν ἀρχῇ ἦν ὁ λόγος. Rue de l'Église, n° 12 : « Tiens ! »

ДАВИД
בראשית ברא אלהים את השמים ואת הארץ.

Über Straßen, señor, ¿qué? Ĉu vi ŝatas ĝin?
`
	for name, src := range scripts {
		file := filepath.Join(t.TempDir(), "x.pdf")
		if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, fountain.Parse(scenes, strings.NewReader(src))); err != nil {
			t.Fatal(err)
		}
		layers, err := TextLayers(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, pt := range layers {
			if pt.Quality.Score() != 1 || pt.Scan {
				t.Errorf("%s page %d: %+v scores %.3f", name, pt.Page, pt, pt.Quality.Score())
			}
		}
		if g, _, err := GarbledPages(file); g != 0 || err != nil {
			t.Errorf("%s: %d garbled (%v)", name, g, err)
		}
	}
}

// garbledLines are a scan's text layer as a poor OCR made it: stray
// marks of the scan at the lines' ends, a word broken, glued punctuation.
var garbledLines = []string{
	"_ MARTA (softly) ol ruen",
	"The boat is late again. We wait by the pier. |",
	"(she folds the map ond puts it away) }",
	"CUT TO: -",
	"a THE OLD“MILL” AT DUSK :",
	"Rain on the roof. Ivo and the dog come ~",
	"in from the yard, shaking off the water, |",
	"and sit down by the stove. _",
	"Ivo_(pointing at the clock on the wall)",
	"Half past six and still no word from her —",
	"Maybe the road is closed, maybe she is |",
	"still at the mar- | ket in the town.",
}

var cleanLines = []string{
	"MARTA (softly)",
	"The boat is late again. We wait by the pier.",
	"(she folds the map and puts it away)",
	"CUT TO:",
	"THE OLD MILL AT DUSK",
	"Rain on the roof. Ivo and the dog come",
	"in from the yard, shaking off the water,",
	"and sit down by the stove.",
	"IVO (pointing at the clock on the wall)",
	"Half past six and still no word from her.",
	"Maybe the road is closed, maybe she is",
	"still at the market in the town.",
}

// scanPDF is a one-page PDF as a scanner makes it: a page-sized image
// (blank, 100 dots an inch) with lines over it as its text layer, drawn
// invisible (an OCR's).
func scanPDF(lines ...string) []byte {
	const w, h = 850, 1100
	var pix bytes.Buffer
	z := zlib.NewWriter(&pix)
	z.Write(bytes.Repeat([]byte{0xff}, w*h))
	z.Close()
	var content strings.Builder
	content.WriteString("q 612 0 0 792 0 0 cm /Im0 Do Q\nBT 3 Tr /F1 12 Tf 14 TL 72 700 Td\n")
	for _, l := range lines {
		l = strings.NewReplacer(`\`, `\\`, "(", `\(`, ")", `\)`).Replace(l)
		fmt.Fprintf(&content, "(%s) Tj T*\n", l)
	}
	content.WriteString("ET")
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> /XObject << /Im0 6 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /FirstChar 32 /LastChar 126 /Widths [" +
			strings.Repeat("600 ", 95) + "] >>",
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceGray /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream",
			w, h, pix.Len(), pix.String()),
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// wordsOCR reads every scan as one line of words.
type wordsOCR string

func (o wordsOCR) HOCR(context.Context, image.Image) (string, error) {
	return `<div class='ocr_page'><span class='ocr_line' title="bbox 100 100 600 112"><span class='ocrx_word' title='bbox 100 100 600 112'>` +
		string(o) + `</span></span></div>`, nil
}

// A scan whose text layer is garbled is counted, and with OCRText OCR
// reads it in place of the layer; a scan with a good layer is not
// counted, and without OCRText the layer is read.
func TestGarbledScan(t *testing.T) {
	dir := t.TempDir()
	garbled, clean := filepath.Join(dir, "garbled.pdf"), filepath.Join(dir, "clean.pdf")
	os.WriteFile(garbled, scanPDF(garbledLines...), 0o600)
	os.WriteFile(clean, scanPDF(cleanLines...), 0o600)
	if g, pages, err := GarbledPages(garbled); g != 1 || pages != 1 || err != nil {
		layers, _ := TextLayers(garbled)
		t.Errorf("garbled: %d of %d (%v) %+v", g, pages, err, layers)
	}
	if g, _, err := GarbledPages(clean); g != 0 || err != nil {
		layers, _ := TextLayers(clean)
		t.Errorf("clean: %d garbled (%v) %+v", g, err, layers)
	}
	if s, _, _ := ScannedPages(garbled); s != 0 {
		t.Errorf("a scan with a text layer is no scan to ScannedPages: %d", s)
	}
	read := func(opts Options) string {
		got, err := ReadFileWith(garbled, opts)
		if err != nil {
			t.Fatal(err)
		}
		return kinds(got)
	}
	if got := read(Options{OCR: wordsOCR("READ BY OCR"), OCRText: true}); !strings.Contains(got, "READ BY OCR") || strings.Contains(got, "MARTA") {
		t.Errorf("OCRText read:\n%s", got)
	}
	if got := read(Options{OCR: wordsOCR("READ BY OCR")}); strings.Contains(got, "READ BY OCR") || !strings.Contains(got, "MARTA") {
		t.Errorf("without OCRText read:\n%s", got)
	}
}

// A line whose words sit at two heights (a skewed scan's layer) counts
// as split.
func TestSplitLines(t *testing.T) {
	var ls []line
	for i, text := range []string{"one line here", "the next one", "and its", "rest a little lower", "another line", "the last"} {
		y := 1 + float64(i)/6
		if i == 3 {
			y = ls[2].Y + 0.04
		}
		if i > 3 {
			y = 1 + float64(i-1)/6
		}
		ls = append(ls, ln(1, y, at(1.5, text)))
	}
	if q := qualityOf(ls); q.Split != 1 || q.Odd != 0 {
		t.Errorf("%+v", q)
	}
}
