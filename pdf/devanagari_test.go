package pdf

import (
	"cmp"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	gofont "github.com/go-text/typesetting/font"
	"golang.org/x/image/font/sfnt"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
)

// devaWords are words that need every part of the shaping: conjuncts
// (क्ष, ज्ञ, त्र, द्ध), the i-matra before its consonant, reph and
// rakar, the eyelash ra (र्‍), the candrabindu.
var devaWords = []string{"क्षत्रिय", "कि", "की", "धर्म", "श्रद्धा", "ज्ञान", "र्\u200dय", "कृष्ण", "हिन्दी", "शर्मा", "माँ", "ट्रेन"}

// cells is how many cells a text takes as Accolade counts them.
func cells(s string) int {
	n := 0
	for _, r := range s {
		if !zeroCell(r) {
			n++
		}
	}
	return n
}

// Shaped, each word is as wide as its cells, in each style, with a glyph
// for each of its letters (no dotted circle, no glyph left out).
func TestDevanagariShapedCells(t *testing.T) {
	for _, style := range []string{"", "B", "I", "BI"} {
		s := newFontShaper(font.GetFont(font.CourierPrimeName, style))
		cell := float64(s.face.HorizontalAdvance(mustGlyph(t, s, 'क'))) / float64(s.face.Upem())
		dotted, _ := s.face.NominalGlyph(0x25CC)
		for _, w := range devaWords {
			runs, width := s.devanagari([]rune(w))
			if want := float64(cells(w)) * cell; abs(width-want) > 1e-6 {
				t.Errorf("%s %q: %.4f em wide, %d cells are %.4f", style, w, width, cells(w), want)
			}
			if len(runs) == 0 {
				t.Errorf("%s %q: no glyphs", style, w)
			}
			for _, r := range runs {
				for _, c := range r.Text {
					if g, ok := s.face.NominalGlyph(c); ok && g == dotted {
						t.Errorf("%s %q: a dotted circle", style, w)
					}
				}
			}
		}
	}
}

func mustGlyph(t *testing.T, s *fontShaper, r rune) gofont.GID {
	g, ok := s.face.NominalGlyph(r)
	if !ok {
		t.Fatalf("no glyph for %q", r)
	}
	return g
}

// The font the PDF embeds maps a character to every glyph, the glyphs'
// own characters as before, in the cmap that gofpdf reads (format 4) and
// the one other readers prefer (format 12).
func TestGlyphCodedFont(t *testing.T) {
	data := font.GetFont(font.CourierPrimeName, "")
	c := glyphCoded(data)
	if glyphCoded(data) != c || glyphCoded(c.data) != c {
		t.Error("coded again")
	}
	orig, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	coded, err := sfnt.Parse(c.data)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.code) != orig.NumGlyphs()-1 {
		t.Errorf("%d glyphs coded of %d", len(c.code), orig.NumGlyphs()-1)
	}
	face, err := gofont.ParseTTF(strings.NewReader(string(c.data)))
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	for g, r := range c.code {
		if got, err := coded.GlyphIndex(&buf, r); err != nil || gofont.GID(got) != g {
			t.Fatalf("U+%04X: glyph %d (%v), want %d", r, got, err, g)
		}
		if got, _ := face.NominalGlyph(r); got != g {
			t.Fatalf("U+%04X: go-text's glyph %d, want %d", r, got, g)
		}
	}
	for _, r := range []rune{'A', 'क', 'ש', 'ب', 0x11B00} {
		a, _ := orig.GlyphIndex(&buf, r)
		b, _ := coded.GlyphIndex(&buf, r)
		if a != b || a == 0 {
			t.Errorf("U+%04X: glyph %d, was %d", r, b, a)
		}
	}
	// gofpdf's own reading (format 4)
	f := NewFpdf("P", "in", "Letter")
	f.AddUTF8FontFromBytes("x", "", c.data)
	f.AddPage()
	f.SetFont("x", "", 12)
	if w := f.GetStringWidth(string(rune(0xE000))); w == 0 || f.Err() {
		t.Errorf("gofpdf: width %v of a coded glyph, %v", w, f.Error())
	}
}

const devaScript = `बाहर. गाँव - दिन

क्षत्रिय कि की धर्म श्रद्धा ज्ञान र्‍य कृष्ण हिन्दी शर्मा माँ ट्रेन

शर्मा
क्षत्रिय कि की धर्म श्रद्धा ज्ञान र्‍य कृष्ण हिन्दी शर्मा माँ ट्रेन। धर्म श्रद्धा ज्ञान।

कृष्ण
*क्षत्रिय कि की* **धर्म श्रद्धा** _ज्ञान र्‍य_
`

// printDeva prints the Devanagari script to a file.
func printDeva(t *testing.T) string {
	t.Helper()
	s := fountain.ParseIn(rules.DefaultConf().Language("hi-Deva"), strings.NewReader(devaScript))
	file := filepath.Join(t.TempDir(), "deva.pdf")
	if err := (&PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	return file
}

// The text read out of the PDF is the text as written, each line where
// it was (Lexington's import, and pdftotext when there is one), though
// what is printed are the shaped glyphs.
func TestDevanagariTextReadBack(t *testing.T) {
	file := printDeva(t)
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range pages {
		for _, l := range p {
			got = append(got, strings.TrimSpace(l.Text))
		}
	}
	all := strings.Join(got, "\n")
	words := strings.Join(devaWords, " ")
	for _, want := range []string{"बाहर. गाँव - दिन", words, "शर्मा", "कृष्ण", "श्रद्धा ज्ञान।", "क्षत्रिय कि की धर्म श्रद्धा ज्ञान र्\u200dय"} {
		if !strings.Contains(all, want) {
			t.Errorf("read back without %q:\n%s", want, all)
		}
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return
	}
	out, err := exec.Command("pdftotext", file, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), words) {
		t.Errorf("pdftotext reads:\n%s", out)
	}
}

// A line of Devanagari wraps where a line of as many Latin letters does:
// by its cells (a joiner, a virama, a mark none), and never inside a
// cluster.
func TestDevanagariWrapsByCells(t *testing.T) {
	pdf := NewFpdf("P", "in", "Letter")
	for _, s := range []string{"", "B", "I", "BI"} {
		AddFont(pdf, "CourierPrime", s, font.GetFont("CourierPrime", s))
	}
	pdf.SetMargins(1, 1, 1)
	pdf.SetAutoPageBreak(false, 0)
	tree := &Tree{PDF: pdf, Rules: rules.Default, Fonts: newFonts(pdf)}
	deva := strings.Join(devaWords, " ")
	latin := ""
	for _, w := range devaWords {
		latin += strings.Repeat("x", cells(w)) + " "
	}
	latin = strings.TrimSpace(latin)
	for _, n := range []int{1, 2, 3, 4} {
		for _, key := range []string{"action", "dialog", "paren"} {
			f := rules.Default.Get(key)
			d, l := strings.Repeat(deva+" ", n), strings.Repeat(latin+" ", n)
			width := pageWidthOf(tree) - f.Left - f.Right
			if a, b := tree.lines(f, d, width), tree.lines(f, l, width); a != b {
				t.Errorf("%s ×%d: Devanagari %d lines, Latin %d", key, n, a, b)
			}
			pdf.AddPage()
			y := pdf.GetY()
			tree.linePrint(f, d)
			if got := int((pdf.GetY()-y)/lineHeight + 0.5); got != tree.height(key, d) {
				t.Errorf("%s ×%d: prints %d lines, counted %d", key, n, got, tree.height(key, d))
			}
		}
	}
	// a word longer than the line breaks between its clusters
	f := rules.Default.Get("dialog")
	long := strings.Repeat("क्षत्रि", 40)
	_, starts := tree.wrap(f, long, pageWidthOf(tree)-f.Left-f.Right)
	for _, at := range starts[1:] {
		if rs := []rune(long[:at]); len(rs) > 0 && joinsBefore(rs[len(rs)-1], []rune(long[at:])[0]) {
			t.Errorf("breaks inside a cluster at %d", at)
		}
	}
}

func pageWidthOf(t *Tree) float64 {
	w, _ := t.PDF.GetPageSize()
	return w
}

// A page view draws what the PDF prints: the shaped glyphs, by
// characters the drawing's font has.
func TestDevanagariDrawn(t *testing.T) {
	s := fountain.ParseIn(rules.DefaultConf().Language("hi-Deva"), strings.NewReader(devaScript))
	var d Drawing
	if _, err := (&PDFWriter{Elements: rules.Default, Record: &d}).Render(s); err != nil {
		t.Fatal(err)
	}
	var err error
	fonts := map[string]*sfnt.Font{}
	var buf sfnt.Buffer
	shaped := false
	for _, p := range d.Pages {
		for _, m := range p.Marks {
			tm, ok := m.(Text)
			if !ok {
				continue
			}
			f := fonts[tm.Font]
			if f == nil {
				if f, err = sfnt.Parse(d.Fonts[tm.Font]); err != nil {
					t.Fatal(err)
				}
				fonts[tm.Font] = f
			}
			for _, r := range tm.Text {
				if r == 0x094D || r == 0x200D {
					t.Errorf("%q drawn as written (a virama, a joiner)", tm.Text)
				}
				if r >= 0xE000 && r <= 0xF8FF {
					shaped = true
				}
				if g, err := f.GlyphIndex(&buf, r); err != nil || g == 0 {
					t.Errorf("%q: U+%04X has no glyph in its font", tm.Text, r)
				}
			}
		}
	}
	if !shaped {
		t.Error("no shaped glyph drawn")
	}
	// _ज्ञान र्‍य_: underlined from end to end in the drawing too, its
	// cells (4, a space, 2) at 7.2 pt each
	type span struct{ a, b float64 }
	var lines []span
	for _, p := range d.Pages {
		for _, m := range p.Marks {
			if b, ok := m.(Box); ok && b.Fill && b.H < 2 && b.W > 0 {
				lines = append(lines, span{b.X, b.X + b.W})
			}
		}
	}
	slices.SortFunc(lines, func(x, y span) int { return cmp.Compare(x.a, y.a) })
	var merged []span
	for _, l := range lines {
		if n := len(merged); n > 0 && l.a <= merged[n-1].b+0.01 {
			merged[n-1].b = max(merged[n-1].b, l.b)
			continue
		}
		merged = append(merged, l)
	}
	if len(merged) != 1 || abs(merged[0].b-merged[0].a-7*7.2) > 0.5 {
		t.Errorf("the underline drawn: %v, want one 50.4 pt long", merged)
	}
}
