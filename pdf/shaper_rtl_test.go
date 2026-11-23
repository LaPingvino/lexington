package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/lex"
)

// A program laying out its own text prints right-to-left text through
// Shaper: composed (Arabic joined, Hebrew's forms), its lines in visual
// order (VisualRuns), each right-to-left run turned around with its marks
// placed (CellRTL); pdftotext reads it back as written.
func TestShaperRightToLeft(t *testing.T) {
	data := font.GetFont(font.CourierPrimeName, "")
	s := ShaperOf(data)
	if got := s.Compose("سلام"); got != "\uFEB3\uFEFC\u0645" { // seen initial, lam-alef final, meem alone
		t.Errorf("سلام composed %+q, want its joined forms", got)
	}
	if got := s.Compose("لا"); got != "\uFEFB" {
		t.Errorf("لا composed %+q, want lam-alef", got)
	}
	if got := s.Compose("וּ"); got != "\uFB35" {
		t.Errorf("וּ composed %+q, want U+FB35", got)
	}
	if got := s.Compose("abc क"); got != "abc क" {
		t.Errorf("composed %q", got)
	}
	var nilShaper *Shaper
	if nilShaper.Compose("سلام") != "سلام" {
		t.Error("a nil shaper composes")
	}

	// a Hebrew paragraph with a Latin word and a number: right to left,
	// the Latin and the number left to right inside it
	para := []rune("שלום world 12 (עולם).")
	lv := Levels(para, lex.LineRTL(string(para), false))
	runs := VisualRuns(lv, 0, len(para))
	var seen []string
	for _, r := range runs {
		seen = append(seen, string(para[r.Start:r.End]))
	}
	// the Latin word and the number one run (level 2), the rest right to
	// left around it, its brackets and period with it
	if want := []string{" (עולם).", "world 12", "שלום "}; !slices.Equal(seen, want) ||
		!runs[0].RTL || runs[1].RTL || !runs[2].RTL {
		t.Errorf("runs %q (%+v), want %q", seen, runs, want)
	}
	covered := 0
	for _, r := range runs {
		covered += r.End - r.Start
	}
	if covered != len(para) {
		t.Errorf("the runs cover %d of %d characters", covered, len(para))
	}

	d := NewFpdf("P", "pt", "A4")
	AddFont(d, "m", "", data)
	d.SetAutoPageBreak(false, 0)
	d.SetCellMargin(0)
	d.AddPage()
	d.SetFont("m", "", 12)
	print := func(y float64, text string, rtl bool) {
		rs := []rune(text)
		lv := Levels(rs, rtl)
		x := 40.0
		for _, r := range VisualRuns(lv, 0, len(rs)) {
			part := string(rs[r.Start:r.End])
			w := s.Width(d, part)
			d.SetXY(x, y)
			if r.RTL {
				s.CellRTL(d, "m", "", 12, w, 20, part, "L")
			} else {
				s.Cell(d, "m", "", 12, w, 20, part, "L")
			}
			if got := d.GetX(); abs(got-(x+w)) > 1e-6 {
				t.Errorf("after %q at %.2f, want %.2f", part, got, x+w)
			}
			x += w
		}
	}
	// (no brackets: pdftotext turns the text around but not its
	// brackets; nor a Latin word: on a page mostly left to right it keeps
	// a line's left-to-right runs where they are seen)
	hebrew := "שָׁלוֹם וּבְשׁוּבוֹ עולם"
	arabic := "أَعُودَ حَتَّى لا سلام"
	mixed := "שלום עולם, world 12"
	print(60, s.Compose(hebrew), true)
	print(100, s.Compose(arabic), true)
	print(120, mixed, true)
	print(140, "Latin text first, then a few words", false)
	print(180, "and more Latin to make the page read left to right", false)
	var b bytes.Buffer
	if err := d.Output(&b); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "rtl.pdf")
	if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("no pdftotext")
	}
	out, err := exec.Command("pdftotext", file, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	clean := strings.NewReplacer("\u202a", "", "\u202b", "", "\u202c", "").Replace(string(out))
	for _, want := range []string{hebrew, arabic, "שלום עולם", "world 12"} {
		if !strings.Contains(norm.NFKD.String(clean), norm.NFKD.String(want)) {
			t.Errorf("pdftotext has no %q:\n%s", want, clean)
		}
	}
}
