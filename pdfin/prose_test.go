package pdfin

import (
	"bytes"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"
)

// prosePDF is a short story printed as a book is: a title and chapter
// headings in larger type, paragraphs wrapped by the page with indented
// first lines, a word broken over two lines, a list, a running header
// and page numbers.
func prosePDF(t *testing.T) []byte {
	t.Helper()
	f := gofpdf.New("P", "pt", "A4", "")
	f.SetMargins(72, 72, 72)
	f.SetAutoPageBreak(true, 72)
	f.SetHeaderFunc(func() {
		if f.PageNo() > 1 {
			f.SetFont("Times", "I", 9)
			f.SetXY(72, 36)
			f.CellFormat(0, 10, "The Lighthouse Keeper", "", 0, "C", false, 0, "")
		}
	})
	f.SetFooterFunc(func() {
		f.SetFont("Times", "", 9)
		f.SetXY(72, 842-48)
		f.CellFormat(0, 10, "- "+itoa(f.PageNo())+" -", "", 0, "C", false, 0, "")
	})
	f.AddPage()
	f.SetFont("Times", "B", 22)
	f.MultiCell(0, 28, "The Lighthouse Keeper", "", "L", false)
	f.Ln(10)
	tr := f.UnicodeTranslatorFromDescriptor("")
	// a paragraph, its first line indented
	para := func(text string) {
		f.SetFont("Times", "", 12)
		for i, l := range f.SplitText(text, 451-18) {
			x := 72.0
			if i == 0 {
				x += 18
			}
			f.SetX(x)
			f.CellFormat(0, 15, l, "", 1, "L", false, 0, "")
		}
		f.Ln(6)
	}
	chapter := func(title string) {
		f.Ln(8)
		f.SetFont("Times", "B", 16)
		f.MultiCell(0, 20, title, "", "L", false)
		f.Ln(4)
	}
	chapter("1. The Storm")
	para("The wind came in from the west and the rain followed it, hard and grey, across the bay where the boats lay tied at the harbour wall. " +
		"It rattled the shutters of every house on the hill and blew the smoke flat over the roofs. Nobody went out that night.")
	para("Old Tomas kept the lamp burning and wrote in his book what he saw: the waves, the clouds and the light of a ship far out, " +
		"which came and went behind the rain until he could not tell whether it was there at all.")
	// a word broken by hand over two lines
	f.SetFont("Times", "", 12)
	f.SetX(90)
	f.CellFormat(0, 15, "In the morning the sea was calm, and the whole harbour came out to see the light-", "", 1, "L", false, 0, "")
	f.CellFormat(0, 15, "house still standing on its rock.", "", 1, "L", false, 0, "")
	f.Ln(6)
	chapter("2. What He Kept")
	para("He kept three things in the drawer of his table:")
	f.SetFont("Times", "", 12)
	for _, item := range []string{"a brass compass", "a letter from his sister", "a key to a door he never found"} {
		f.SetX(90)
		f.CellFormat(0, 15, tr("•  "+item), "", 1, "L", false, 0, "")
	}
	f.Ln(6)
	words := strings.Fields("the years went by like ships one after another each with its own light and sound in fog " +
		"while the keeper grew older and slower on the stairs but never once let the lamp go out")
	n := 0
	for range 20 { // varied, so no line comes back on every page
		var p []string
		for range 110 {
			n = (n*31 + 17) % 1009
			p = append(p, words[n%len(words)])
		}
		para("The years went by: " + strings.Join(p, " ") + ".")
	}
	var b bytes.Buffer
	if err := f.Output(&b); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func TestReadProse(t *testing.T) {
	pdf := prosePDF(t)
	p, err := ReadProse(bytes.NewReader(pdf), int64(len(pdf)), Options{})
	if err != nil {
		t.Fatal(err)
	}
	md := p.Markdown()
	t.Log(md)
	want := []string{
		"# The Lighthouse Keeper\n\n## 1. The Storm\n\nThe wind came in from the west",
		"Nobody went out that night.\n\nOld Tomas kept the lamp",
		"whether it was there at all.\n\nIn the morning",
		"the whole harbour came out to see the lighthouse still standing on its rock.",
		"## 2. What He Kept",
		"He kept three things in the drawer of his table:\n\n- a brass compass\n- a letter from his sister\n- a key to a door he never found\n\nThe years went by",
	}
	for _, w := range want {
		if !strings.Contains(md, w) {
			t.Errorf("missing %q", w)
		}
	}
	for _, not := range []string{"- 1 -", "- 2 -", "The Lighthouse Keeper\n\nThe Lighthouse Keeper"} {
		if strings.Contains(md, not) {
			t.Errorf("kept %q", not)
		}
	}
	if n := strings.Count(md, "The Lighthouse Keeper"); n != 1 {
		t.Errorf("the title %d times: the running header was kept", n)
	}
	// the paragraphs over the page breaks are each one
	if n := strings.Count(md, "\n\nThe years went by: "); n != 20 || strings.Count(md, "The years went by") != 20 {
		t.Errorf("%d paragraphs of the years, want 20", n)
	}
}

func TestJoinLine(t *testing.T) {
	for _, c := range []struct{ a, b, want string }{
		{"the light-", "house", "the lighthouse"},
		{"a well-", "Known", "a well- Known"},
		{"broken­", "word", "brokenword"},
		{"a", "b", "a b"},
	} {
		if got := joinLine(c.a, c.b); got != c.want {
			t.Errorf("joinLine(%q, %q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestProseMarkdownEscapes(t *testing.T) {
	p := Prose{{Text: "# not a heading"}, {Text: "1. not a list, *nor* this"}, {List: "3)", Text: "an item"}}
	want := "\\# not a heading\n\n1\\. not a list, \\*nor\\* this\n\n3. an item\n"
	if got := p.Markdown(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Lines without sizes (as OCR may give them) still make paragraphs: a
// short last line ends one, a page break does not.
func TestProseWithoutSizes(t *testing.T) {
	full := func(page int, y float64, text string) line {
		return line{Page: page, Y: y, Runs: []run{{X: 1, End: 7.4, Text: text}}}
	}
	short := func(page int, y float64, text string) line {
		return line{Page: page, Y: y, Runs: []run{{X: 1, End: 3, Text: text}}}
	}
	pages := [][]line{
		{full(1, 1, "One two"), full(1, 1.2, "three four"), short(1, 1.4, "five."), full(1, 1.6, "Six seven"), full(1, 1.8, "eight"), short(1, 10.5, "1")},
		{full(2, 1, "nine ten"), short(2, 1.2, "eleven."), short(2, 10.5, "2")},
	}
	got := proseOf(pages).Markdown()
	want := "One two three four five.\n\nSix seven eight nine ten eleven.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// a chapter's number in large type at the top of a page is kept
	big := func(page int, y float64, text string) line {
		return line{Page: page, Y: y, Runs: []run{{X: 1, End: 1.5, Text: text, Size: 24}}}
	}
	sized := func(l line) line { l.Runs[0].Size = 12; return l }
	pages = [][]line{
		{big(1, 1, "IV"), sized(full(1, 1.6, "One two")), sized(short(1, 1.8, "three.")), sized(short(1, 10.5, "12"))},
	}
	got = proseOf(pages).Markdown()
	want = "# IV\n\nOne two three.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// OCR's lines get their size from Tesseract's x_size, the line's height,
// so a heading stands out from the text.
func TestHOCRSizes(t *testing.T) {
	hocr := `<span class='ocr_line' title="bbox 100 100 900 160; baseline 0 -10; x_size 60; x_descenders 12; x_ascenders 15">` +
		`<span class='ocrx_word' title='bbox 100 100 400 160; x_wconf 95'>Chapter</span></span>` +
		`<span class='ocr_line' title="bbox 100 300 900 330; baseline 0 -5; x_size 30">` +
		`<span class='ocrx_word' title='bbox 100 300 300 330; x_wconf 95'>Some</span>` +
		`<span class='ocrx_word' title='bbox 310 300 400 330; x_wconf 95'>text.</span></span>`
	ls := hocrLines(1, hocr, 300)
	if len(ls) != 2 || lineSize(ls[0]) != 14.4 || lineSize(ls[1]) != 7.2 {
		for _, l := range ls {
			t.Logf("%q %.1f", l.text(), lineSize(l))
		}
		t.Fatal("sizes not read")
	}
}
