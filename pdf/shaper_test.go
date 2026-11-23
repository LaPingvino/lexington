package pdf

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/pdfin"
)

// A program laying out its own text (Accolade's prose) prints Devanagari
// shaped through Shaper: Width is what Cell prints, the shaper is one a
// file, and the text reads back as written.
func TestShaperCell(t *testing.T) {
	data := font.GetFont(font.CourierPrimeName, "")
	s := ShaperOf(data)
	if s == nil {
		t.Fatal("no shaper for Courier Badi")
	}
	if ShaperOf(bytes.Clone(data)) != s {
		t.Error("a copy of the file has a shaper of its own")
	}
	if ShaperOf([]byte("not a font")) != nil || ShaperOf(nil) != nil {
		t.Error("a shaper for what is not a font")
	}
	d := NewFpdf("P", "pt", "A4")
	AddFont(d, "deva", "", data)
	d.SetAutoPageBreak(false, 0)
	d.SetCellMargin(0)
	d.AddPage()
	d.SetFont("deva", "", 14)
	line := "नमस्ते, " + strings.Join(devaWords, " ")
	if w, plain := s.Width(d, line), d.GetStringWidth(line); w >= plain {
		t.Errorf("shaped width %.2f, not less than gofpdf's %.2f (the joiner and the marks take no cell)", w, plain)
	}
	if w, want := s.Width(d, "abc"), d.GetStringWidth("abc"); w != want {
		t.Errorf("Latin is %.2f wide, gofpdf says %.2f", w, want)
	}
	d.SetXY(40, 100)
	w := s.Width(d, line)
	s.Cell(d, "deva", "", 14, w, 20, line, "L")
	if got := d.GetX(); abs(got-(40+w)) > 1e-6 {
		t.Errorf("after the cell at %.2f, want %.2f", got, 40+w)
	}
	var nilShaper *Shaper
	d.SetXY(40, 140)
	nilShaper.Cell(d, "deva", "", 14, 100, 20, "abc", "L") // as CellFormat
	if w := nilShaper.Width(d, "abc"); w != d.GetStringWidth("abc") {
		t.Errorf("a nil shaper measures %.2f", w)
	}
	file := filepath.Join(t.TempDir(), "shaper.pdf")
	var b bytes.Buffer
	if err := d.Output(&b); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, p := range pages {
		for _, l := range p {
			all = append(all, strings.TrimSpace(l.Text))
		}
	}
	if !strings.Contains(strings.Join(all, "\n"), line) {
		t.Errorf("read back without %q:\n%s", line, strings.Join(all, "\n"))
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command("pdftotext", file, "-").Output()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), line) {
			t.Errorf("pdftotext reads:\n%s", out)
		}
	}
	for _, c := range []struct {
		a, b rune
		join bool
	}{{'क', '्', true}, {'्', 'ष', true}, {'र', 0x200D, true}, {0x200D, 'य', true}, {'क', 'ि', true}, {'क', 'ख', false}, {' ', 'क', false}} {
		if JoinsBefore(c.a, c.b) != c.join {
			t.Errorf("JoinsBefore(%q, %q) is %v", c.a, c.b, !c.join)
		}
	}
	if rs := []rune("क्\u200dष, a"); DevanagariRunEnd(rs, 0) != 4 {
		t.Errorf("the run of %q ends at %d", string(rs), DevanagariRunEnd(rs, 0))
	}
	if !HasDevanagari("a क") || HasDevanagari("a\u200d") || !DevanagariRune(0x200D) {
		t.Error("HasDevanagari / DevanagariRune")
	}
}
