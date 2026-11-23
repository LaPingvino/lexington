package pdf_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

// drawnMargins are each page's texts drawn: those in the top and bottom
// margins (above 0.9in, below 10.2in) as "top:" and "foot:", with where
// they are ("L" from the inch's margin, "C" centred, "R" ending an inch
// from the right).
func drawnMargins(t *testing.T, src string) []string {
	t.Helper()
	var d pdf.Drawing
	s := fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(src))
	if err := (&pdf.PDFWriter{OutputFile: filepath.Join(t.TempDir(), "h.pdf"), Elements: rules.Default, Record: &d}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range d.Pages {
		var line []string
		for _, m := range p.Marks {
			tx, ok := m.(pdf.Text)
			if !ok {
				continue
			}
			tx.X, tx.Y, tx.Width = tx.X/72, tx.Y/72, tx.Width/72 // (points)
			if tx.Y > 0.9 && tx.Y < 10.2 {
				continue
			}
			where := "top"
			if tx.Y >= 10.2 {
				where = "foot"
			}
			side := "C"
			switch {
			case tx.X > 0.99 && tx.X < 1.01:
				side = "L"
			case tx.X+tx.Width > 7.49 && tx.X+tx.Width < 7.51:
				side = "R"
			}
			line = append(line, fmt.Sprintf("%s %s %s", where, side, tx.Text))
		}
		out = append(out, fmt.Sprintf("%d: %s", p.Number, strings.Join(line, "; ")))
	}
	return out
}

func pages(n int, prefix string) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "%s %d.\n\n", prefix, i+1)
	}
	return b.String()
}

// The header on every page, the first too, in place of the page's
// number; the footer; %p, %pp and %#; {{PAGENUMBER}} renumbering the page
// it is on and those after.
func TestHeaderFooterPrinted(t *testing.T) {
	src := "{{HEADER: %# | %p of %pp}}\n{{FOOTER: Left | Centre | Right}}\n\n# Act One\n\nINT. ROOM - DAY\n\n" +
		pages(40, "Filler") + "# Act Two\n\n===\n\n{{PAGENUMBER: 10}}\n\nINT. GARDEN - NIGHT\n\nThe end.\n"
	got := drawnMargins(t, src)
	want := []string{
		"1: foot L Left; foot C Centre; foot R Right; top L Act One; top R 1 of 10",
		"2: foot L Left; foot C Centre; foot R Right; top L Act One; top R 2 of 10",
		"3: foot L Left; foot C Centre; foot R Right; top L Act Two; top R 10 of 10",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// %none: no header, nor the page's number
	got = drawnMargins(t, "{{HEADER: %none}}\n\n"+pages(40, "Filler"))
	if len(got) < 2 || got[1] != "2: " {
		t.Errorf("%%none: %q", got)
	}
	// without: "2." from the second page, as before
	got = drawnMargins(t, pages(40, "Filler"))
	if len(got) < 2 || got[0] != "1: " || got[1] != "2: top C 2." {
		t.Errorf("no header: %q", got)
	}
}

// A {{TOC}} lists the sections with the pages they print on, as
// numbered; {{SERIES}} numbers the chapters.
func TestTOCPrinted(t *testing.T) {
	var d pdf.Drawing
	src := "{{TOC}}\n\n# Chapter {{SERIES}}\n\nINT. ROOM - DAY\n\nText.\n\n===\n\n# Chapter {{SERIES}}\n\n{{PAGENUMBER: 10}}\n\nINT. HALL - DAY\n\nMore.\n"
	s := fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(src))
	if err := (&pdf.PDFWriter{OutputFile: filepath.Join(t.TempDir(), "toc.pdf"), Elements: rules.Default, Record: &d}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, m := range d.Pages[0].Marks {
		if tx, ok := m.(pdf.Text); ok && tx.Y < 1.6*72 {
			texts = append(texts, tx.Text)
		}
	}
	if got := strings.Join(texts, "|"); got != "Chapter 1|1|Chapter 2|10" {
		t.Errorf("TOC %q", got)
	}
}
