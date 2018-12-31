package pdf

import (
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/rules"

	"github.com/jung-kurt/gofpdf"
)

var tr func(string) string

type Tree struct {
	PDF *gofpdf.Fpdf
	Rules rules.Set
	F   lex.Screenplay
}

func (t Tree) pr(a string, text string) {
	line(t.PDF, t.Rules.Get(a), text)
}

func (t Tree) Render() {
	for _, row := range t.F {
		switch row.Type {
		case "newpage":
			t.PDF.AddPage()
			continue
		case "titlepage":
			t.PDF.SetY(4)
		case "metasection":
			t.PDF.SetY(-2)
		}
		if t.Rules.Get(row.Type).Hide {
			continue
		}
		t.pr(row.Type, row.Contents)
	}
}

func line(pdf *gofpdf.Fpdf, format rules.Format, text string) {
	pdf.SetFont(format.Font, format.Style, format.Size)
	pdf.SetX(format.Left)
	pdf.MultiCell(format.Width, 0.19, tr(text), "", "aligned", false)
}

func Create(file string, format rules.Set, contents lex.Screenplay) {
	pdf := gofpdf.New("P", "in", "Letter", "")
	tr = pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetMargins(1, 1, 1)
	pdf.SetXY(1, 1)
	f := Tree{
		PDF: pdf,
		Rules: format,
		F: contents,
	}
	f.Render()
	err := pdf.OutputFileAndClose(file)
	if err != nil {
		panic(err)
	}
}
