// The PDF package of Lexington creates a Screenplay PDF out of the Lex screenplay parsetree. This can be generated with the several other packages, e.g. the fountain package that parses fountain to lex in preparation.
package pdf

import (
	"github.com/lapingvino/lexington/font"
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/rules"

	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/phpdave11/gofpdf"
)

// PDFWriter implements the writer.Writer interface for PDF output.
type PDFWriter struct {
	OutputFile string
	Elements   rules.Set
}

type Tree struct {
	PDF   *gofpdf.Fpdf
	Rules rules.Set
	F     lex.Screenplay
	HTML  gofpdf.HTMLBasicType
}

func (t Tree) pr(a string, text string) {
	line(t.PDF, t.Rules.Get(a), t.HTML, t.Rules.Get(a).Prefix+text+t.Rules.Get(a).Postfix)
}

func (t Tree) Render() {
	var block string
	var lastsection int
	for _, row := range t.F {
		switch row.Type {
		case "newpage":
			block = ""
			t.PDF.AddPage()
			t.PDF.SetHeaderFuncMode(func() {
				t.PDF.SetFont("CourierBadi", "", 12)
				t.PDF.SetXY(-1, 0.5)
				t.PDF.Cell(0, 0, strconv.Itoa(t.PDF.PageNo()-1)+".")
			}, true)
			continue
		case "titlepage":
			block = "title"
			t.PDF.SetY(4)
		case "title", "Title":
			t.PDF.SetTitle(row.Contents, true)
		case "metasection":
			block = "meta"
			t.PDF.SetY(-2)
		}

		var contents string
		var level int
		if row.Type == "section" {
			contents = strings.TrimLeft(row.Contents, "#")
			level = len(row.Contents) - len(contents)
			contents = strings.TrimLeft(contents, " ")
			lastsection = level
		} else if row.Type == "scene" {
			level = lastsection + 1
			contents = row.Contents
		}
		if contents != "" {
			t.PDF.Bookmark(contents, level, -1)
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

var (
	bolditalic = regexp.MustCompile("\\*{3}([^\\*\n]+)\\*{3}")
	bold       = regexp.MustCompile("\\*{2}([^\\*\n]+)\\*{2}")
	italic     = regexp.MustCompile("\\*{1}([^\\*\n]+)\\*{1}")
	underline  = regexp.MustCompile("_{1}([^\\*\n]+)_{1}")
)

func line(pdf *gofpdf.Fpdf, format rules.Format, html gofpdf.HTMLBasicType, text string) {
	pdf.SetFont(format.Font, format.Style, format.Size)
	pdf.SetX(0)
	pdf.SetLeftMargin(format.Left)
	pdf.SetRightMargin(format.Right)

	text = strings.TrimRight(text, "\r\n")

	if strings.ContainsAny(text, "*_") {
		text = bolditalic.ReplaceAllString(text, "<b><i>$1</i></b>")
		text = bold.ReplaceAllString(text, "<b>$1</b>")
		text = italic.ReplaceAllString(text, "<i>$1</i>")
		text = underline.ReplaceAllString(text, "<u>$1</u>")

		if format.Align == "C" {
			text = "<center>" + text + "</center>"
		}
		html.Write(0.165, text)
		pdf.SetY(pdf.GetY() + 0.165)
		return
	}

	pdf.MultiCell(0, 0.165, text, "", format.Align, false)
}

// Write converts the internal lex.Screenplay format to a PDF file.
// It implements the writer.Writer interface.
// Note: For PDF, the 'w io.Writer' argument is currently ignored as gofpdf
// requires a file path for output. The output file path is taken from PDFWriter.OutputFile.
func (p *PDFWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	pdf := gofpdf.New("P", "in", "Letter", "")
	pdf.AddUTF8FontFromBytes("CourierPrime", "", font.MustAsset("CourierBadi-Regular.ttf"))
	pdf.AddUTF8FontFromBytes("CourierPrime", "B", font.MustAsset("CourierBadi-Regular.ttf"))
	pdf.AddUTF8FontFromBytes("CourierPrime", "I", font.MustAsset("CourierBadi-Italic.ttf"))
	pdf.AddUTF8FontFromBytes("CourierPrime", "BI", font.MustAsset("CourierBadi-Italic.ttf"))
	pdf.AddPage()
	pdf.SetMargins(1, 1, 1)
	pdf.SetXY(1, 1)
	f := Tree{
		PDF:   pdf,
		Rules: p.Elements, // Use the Elements from the PDFWriter struct
		F:     screenplay, // Use the screenplay passed to the Write method
		HTML:  pdf.HTMLBasicNew(),
	}
	f.Render()
	err := pdf.OutputFileAndClose(p.OutputFile) // Use the OutputFile from the PDFWriter struct
	if err != nil {
		return err // Return the error instead of panicking
	}
	return nil
}
