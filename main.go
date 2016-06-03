package main

import "github.com/jung-kurt/gofpdf"

type Action int

var action = map[string]struct {
	Left, Width float64
}{
	"action":        {1.5, 6},
	"speaker":       {4.2, 3.3},
	"dialog":        {2.9, 3.3},
	"scene":         {1.5, 6},
	"parenthetical": {3.6, 2},
	"trans":         {6, 1.5},
	"note":          {1.5, 6},
	"allcaps":       {1.5, 6},
	"parens":        {1.5, 6},
	"empty":         {1.5, 6},
}

var tr func(string) string

type Tree struct {
	PDF *gofpdf.Fpdf
	F   []struct {
		Format string
		Text   string
	}
}

func (t Tree) pr(a string, text string) {
	line(t.PDF, action[a].Left, action[a].Width, text)
}

func (t Tree) Render() {
	for _, row := range t.F {
		t.pr(row.Format, row.Text)
	}
}

func line(pdf *gofpdf.Fpdf, jump, width float64, text string) {
	pdf.SetX(jump)
	pdf.MultiCell(width, 0.19, tr(text), "", "aligned", false)
}

func main() {
	pdf := gofpdf.New("P", "in", "Letter", "")
	tr = pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("courier", "", 12)
	pdf.SetMargins(1, 1, 1)
	pdf.SetXY(1, 1)
	f := Tree{PDF: pdf}
	f.F = []struct{ Format, Text string }{
		{"scene", "INT. HOUSE - DAY"},
		{"empty", ""},
		{"speaker", "MARY"},
		{"dialog", "I can't believe how easy it is to write in Fountain."},
		{"empty", ""},
		{"speaker", "TOM"},
		{"parenthetical", "(typing)"},
		{"dialog", "Look! I just made a parenthetical!"},
	}
	f.Render()
	err := pdf.OutputFileAndClose("fountain.pdf")
	if err != nil {
		panic(err)
	}
}
