package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

// needSystemCJK skips a test when no font on the computer has Japanese.
func needSystemCJK(t *testing.T) {
	t.Helper()
	if len(systemGlyphs([]rune("パ症"), false)) < 2 {
		t.Skip("no font on this computer has Japanese")
	}
}

const cjkScript = "Title: パリ症候群\nAuthor: Someone\n\nINT. BARN - DAY\n\nRain. 東京 here.\n"

func drawCJK(t *testing.T) (Drawing, []byte) {
	t.Helper()
	var d Drawing
	data, err := (&PDFWriter{Elements: rules.Default, Record: &d}).Render(fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(cjkScript)))
	if err != nil {
		t.Fatal(err)
	}
	return d, data
}

func textMark(d Drawing, s string) (Text, bool) {
	for _, p := range d.Pages {
		for _, m := range p.Marks {
			if tx, ok := m.(Text); ok && strings.Contains(tx.Text, s) {
				return tx, true
			}
		}
	}
	return Text{}, false
}

// A character Courier Badi lacks takes the cells the Lekhan width gives
// it (CJK two), found on the computer or not: the title is centred by them.
func TestFallbackCells(t *testing.T) {
	d, _ := drawCJK(t)
	title, ok := textMark(d, "パリ症候群")
	if !ok {
		t.Fatal("no mark of the title")
	}
	if want := 10 * 7.2; title.Width < want-0.1 || title.Width > want+0.1 {
		t.Errorf("the title is %.2fpt wide, want %.2f (ten cells)", title.Width, want)
	}
	author, _ := textMark(d, "Someone")
	if mid, want := title.X+title.Width/2, author.X+author.Width/2; mid < want-0.5 || mid > want+0.5 {
		t.Errorf("the title's middle is at %.1fpt, the author's at %.1f", mid, want)
	}
	line, ok := textMark(d, "東京")
	if !ok {
		t.Fatal("no mark of the action")
	}
	if want := 16 * 7.2; line.Width < want-0.2 || line.Width > want+0.2 { // "Rain. 東京 here.": 12 narrow, 2 wide
		t.Errorf("the action line is %.2fpt wide, want %.2f", line.Width, want)
	}
}

// With a font on the computer that has them, they print: the Drawing's
// font has their glyphs, and its pages paint them.
func TestFallbackFromSystem(t *testing.T) {
	needSystemCJK(t)
	d, data := drawCJK(t)
	title, _ := textMark(d, "パリ症候群")
	has := glyphTest(d.Fonts[title.Font])
	for _, r := range "パリ症候群東京" {
		if !has(r) {
			t.Errorf("the drawing's font has no %c", r)
		}
	}
	page := d.Image(0, 2)
	ink := 0
	x0, x1 := int(title.X*2), int((title.X+title.Width)*2)
	y0, y1 := int((title.Y-12)*2), int((title.Y+3)*2)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if c := page.RGBAAt(x, y); c.R < 128 {
				ink++
			}
		}
	}
	if ink < 100 {
		t.Errorf("the title paints %d dark pixels: not drawn", ink)
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		file := filepath.Join(t.TempDir(), "cjk.pdf")
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command("pdftotext", file, "-").Output()
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []string{"パリ症候群", "東京"} {
			if !bytes.Contains(out, []byte(s)) {
				t.Errorf("the PDF's text has no %s:\n%s", s, out)
			}
		}
	}
	if m, _ := Unsupported(fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(cjkScript))); len(m) != 0 {
		t.Errorf("missing %q, though a font on the computer has them", string(m))
	}
}

// A character no font has gets an empty glyph of its cells; a font that
// lacks nothing of the script is the font itself, byte for byte.
func TestMergeGlyphs(t *testing.T) {
	base := font.GetFont(font.CourierPrimeName, "")
	if out := withFallback(base, "", []rune("Ĉu Русский")); !bytes.Equal(out, base) {
		t.Error("a script Courier Badi has all of changed its font")
	}
	out := mergeGlyphs(base, []rune("パ"), nil)
	if out == nil {
		t.Fatal("not merged")
	}
	f := NewFpdf("P", "pt", "Letter")
	f.AddUTF8FontFromBytes("m", "", out)
	f.SetFont("m", "", 12)
	if w := f.GetStringWidth("パ"); w < 14.35 || w > 14.45 {
		t.Errorf("an empty glyph is %.2fpt wide, want two cells", w)
	}
	if w := f.GetStringWidth("A"); w < 7.19 || w > 7.21 {
		t.Errorf("A is %.2fpt wide after merging", w)
	}
}
