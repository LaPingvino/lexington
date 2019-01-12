// The PDF package of Lexington creates a Screenplay PDF out of the Lex screenplay parsetree. This can be generated with the several other packages, e.g. the fountain package that parses fountain to lex in preparation.
package pdf

import (
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/rules"

	"github.com/jung-kurt/gofpdf"
)

var tr func(string) string

type Tree struct {
	PDF   *gofpdf.Fpdf
	Rules rules.Set
	F     lex.Screenplay
}

func (t Tree) pr(a string, text string) {
	line(t.PDF, t.Rules.Get(a), t.Rules.Get(a).Prefix+text+t.Rules.Get(a).Postfix)
}

func (t Tree) Render() {
	var block string
	for _, row := range t.F {
		switch row.Type {
		case "newpage":
			block = ""
			t.PDF.AddPage()
			continue
		case "titlepage":
			block = "title"
			t.PDF.SetY(4)
		case "metasection":
			block = "meta"
			t.PDF.SetY(-2)
		}
		if t.Rules.Get(row.Type).Hide && block == "" {
			continue
		}
		if block != "" {
			row.Type = block
		}
		t.pr(row.Type, row.Contents)
	}
}

func line(pdf *gofpdf.Fpdf, format rules.Format, text string) {
	pdf.SetFont(format.Font, format.Style, format.Size)
	pdf.SetX(format.Left)
	pdf.MultiCell(format.Width, 0.19, tr(text), "", format.Align, false)
}

func Create(file string, format rules.Set, contents lex.Screenplay) {
	pdf := gofpdf.New("P", "in", "Letter", "")
	tr = pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetMargins(1, 1, 1)
	pdf.SetXY(1, 1)
	f := Tree{
		PDF:   pdf,
		Rules: format,
		F:     contents,
	}
	f.Render()
	err := pdf.OutputFileAndClose(file)
	if err != nil {
		panic(err)
	}
}
