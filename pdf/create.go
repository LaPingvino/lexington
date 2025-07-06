// The PDF package of Lexington creates a Screenplay PDF out of the Lex screenplay parsetree.
// This can be generated with the several other packages, e.g. the fountain package that parses
// fountain to lex in preparation.
package pdf

import (
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"

	"github.com/phpdave11/gofpdf"
)

// PDFWriter implements the writer.Writer interface for PDF output.
type PDFWriter struct {
	OutputFile string
	Elements   rules.Set
}

type Tree struct {
	PDF          *gofpdf.Fpdf
	Rules        rules.Set
	F            lex.Screenplay
	HTML         gofpdf.HTMLBasicType
	DualDialogue bool       // Track if we're in dual dialogue mode
	DualColumn   int        // Track which column we're in (0 = left, 1 = right)
	DualBuffer   []lex.Line // Buffer for dual dialogue elements
}

func (t Tree) pr(a string, text string) {
	linePrint(t.PDF, t.Rules.Get(a), t.HTML, t.Rules.Get(a).Prefix+text+t.Rules.Get(a).Postfix)
}

func (t *Tree) Render() {
	var block string
	var lastsection int

	for _, row := range t.F {
		if t.handleSpecialCases(row, &block, &lastsection) {
			continue
		}

		// Handle dual dialogue buffering
		if t.DualDialogue {
			t.bufferDualDialogue(row)
			continue
		}

		contents, level := t.processBookmarkContent(row, lastsection)
		if contents != "" {
			t.PDF.Bookmark(contents, level, -1)
		}

		if t.shouldSkipElement(row, block) {
			continue
		}

		if block == internal.ElementTitle {
			row.Type = block
		}
		t.pr(row.Type, row.Contents)
	}

	// Flush any remaining dual dialogue at the end
	if t.DualDialogue {
		t.flushDualDialogue()
	}
}

// handleSpecialCases processes special element types that require immediate action
func (t *Tree) handleSpecialCases(row lex.Line, block *string, lastsection *int) bool {
	switch row.Type {
	case "newpage":
		if t.DualDialogue {
			t.flushDualDialogue()
		}
		*block = ""
		t.PDF.AddPage()
		t.PDF.SetHeaderFuncMode(func() {
			t.PDF.SetFont("CourierPrime", "", 12)
			t.PDF.SetXY(-1, 0.5)
			t.PDF.Cell(0, 0, strconv.Itoa(t.PDF.PageNo()-1)+".")
		}, true)
		return true
	case "titlepage":
		*block = internal.ElementTitle
		t.PDF.SetY(4)
		return false
	case "title", "Title":
		t.PDF.SetTitle(row.Contents, true)
		return false
	case "metasection":
		*block = ""
		t.PDF.SetY(-2)
		return false
	case "dualspeaker_open":
		t.DualDialogue = true
		t.DualColumn = 0
		t.DualBuffer = []lex.Line{}
		return true
	case "dualspeaker_next":
		t.DualColumn = 1
		return true
	case "dualspeaker_close":
		t.flushDualDialogue()
		t.DualDialogue = false
		t.DualColumn = 0
		t.DualBuffer = []lex.Line{}
		return true
	}
	return false
}

// bufferDualDialogue adds a line to the dual dialogue buffer
func (t *Tree) bufferDualDialogue(row lex.Line) {
	lineCopy := row
	lineCopy.Type = row.Type + "_col" + strconv.Itoa(t.DualColumn)
	t.DualBuffer = append(t.DualBuffer, lineCopy)
}

// processBookmarkContent processes content for bookmarks
func (t *Tree) processBookmarkContent(row lex.Line, lastsection int) (string, int) {
	var contents string
	var level int

	switch row.Type {
	case "section":
		contents = strings.TrimLeft(row.Contents, "#")
		level = len(row.Contents) - len(contents)
		contents = strings.TrimLeft(contents, " ")
	case "scene":
		level = lastsection + 1
		contents = row.Contents
	}

	return contents, level
}

// shouldSkipElement determines if an element should be skipped
func (t *Tree) shouldSkipElement(row lex.Line, block string) bool {
	return t.Rules.Get(row.Type).Hide && block == ""
}

var (
	bolditalic = regexp.MustCompile("\\*{3}([^\\*\n]+)\\*{3}")
	bold       = regexp.MustCompile("\\*{2}([^\\*\n]+)\\*{2}")
	italic     = regexp.MustCompile("\\*{1}([^\\*\n]+)\\*{1}")
	underline  = regexp.MustCompile("_{1}([^\\*\n]+)_{1}")
)

func (t *Tree) flushDualDialogue() {
	if len(t.DualBuffer) == 0 {
		return
	}

	// Separate left and right column elements using generic utilities
	leftElements := internal.Map(
		internal.Filter(t.DualBuffer, func(line lex.Line) bool {
			return strings.HasSuffix(line.Type, "_col0")
		}),
		func(line lex.Line) lex.Line {
			line.Type = strings.TrimSuffix(line.Type, "_col0")
			return line
		},
	)

	rightElements := internal.Map(
		internal.Filter(t.DualBuffer, func(line lex.Line) bool {
			return strings.HasSuffix(line.Type, "_col1")
		}),
		func(line lex.Line) lex.Line {
			line.Type = strings.TrimSuffix(line.Type, "_col1")
			return line
		},
	)

	// Store original position and margins
	startY := t.PDF.GetY()

	// Store original margins for restoration
	origLeftMargin := 1.5
	origRightMargin := 1.0

	// Industry standard dual dialogue column positions:
	// Left column: 1.5" to 3.5" (2" width)
	// Right column: 4.5" to 6.5" (2" width)
	// This provides proper separation and readable columns
	leftColStart := 1.5
	leftColWidth := 2.0
	rightColStart := 4.5
	rightColWidth := 2.0

	// Render left column using precise positioning
	leftCurrentY := startY
	for _, line := range leftElements {
		lineType := line.Type
		var format rules.Format
		switch lineType {
		case "speaker":
			format = t.Rules.Get("dualspeaker")
		case "dialog":
			format = t.Rules.Get("dualdialog")
		case "paren":
			format = t.Rules.Get("dualparen")
		default:
			format = t.Rules.Get(lineType)
		}

		// Position text in left column
		t.PDF.SetXY(leftColStart+format.Left-1.5, leftCurrentY)
		leftCurrentY += t.renderDualDialogueLine(format, line.Contents, leftColWidth)
	}

	// Render right column using precise positioning
	rightCurrentY := startY
	for _, line := range rightElements {
		lineType := line.Type
		var format rules.Format
		switch lineType {
		case "speaker":
			format = t.Rules.Get("dualspeaker")
		case "dialog":
			format = t.Rules.Get("dualdialog")
		case "paren":
			format = t.Rules.Get("dualparen")
		default:
			format = t.Rules.Get(lineType)
		}

		// Position text in right column
		t.PDF.SetXY(rightColStart+format.Left-1.5, rightCurrentY)
		rightCurrentY += t.renderDualDialogueLine(format, line.Contents, rightColWidth)
	}

	// Set final position to the maximum of both columns
	finalY := leftCurrentY
	if rightCurrentY > finalY {
		finalY = rightCurrentY
	}
	t.PDF.SetY(finalY + 0.3) // Add spacing after dual dialogue

	// Restore original margins
	t.PDF.SetLeftMargin(origLeftMargin)
	t.PDF.SetRightMargin(origRightMargin)
}

func linePrint(pdf *gofpdf.Fpdf, format rules.Format, html gofpdf.HTMLBasicType, text string) {
	// Map configuration font names to PDF font names
	fontName := font.GetFontName(format.Font)

	pdf.SetFont(fontName, format.Style, format.Size)
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

// renderDualDialogueLine renders a single line of dual dialogue and returns the height consumed
func (t Tree) renderDualDialogueLine(format rules.Format, text string, columnWidth float64) float64 {
	// Map configuration font names to PDF font names
	fontName := font.GetFontName(format.Font)

	t.PDF.SetFont(fontName, format.Style, format.Size)
	text = strings.TrimRight(text, "\r\n")

	lineHeight := 0.165

	if strings.ContainsAny(text, "*_") {
		text = bolditalic.ReplaceAllString(text, "<b><i>$1</i></b>")
		text = bold.ReplaceAllString(text, "<b>$1</b>")
		text = italic.ReplaceAllString(text, "<i>$1</i>")
		text = underline.ReplaceAllString(text, "<u>$1</u>")

		if format.Align == "C" {
			text = "<center>" + text + "</center>"
		}

		// For HTML-styled text, use current position and write directly
		t.HTML.Write(lineHeight, text)
		return lineHeight
	}

	// For regular text, use MultiCell with constrained width
	currentY := t.PDF.GetY()

	// Use MultiCell with specific width for the column
	t.PDF.MultiCell(columnWidth, lineHeight, text, "", format.Align, false)

	// Calculate actual height used
	newY := t.PDF.GetY()
	heightUsed := newY - currentY

	return heightUsed
}

// Write converts the internal lex.Screenplay format to a PDF file.
// It implements the writer.Writer interface.
// Note: For PDF, the 'w io.Writer' argument is currently ignored as gofpdf
// requires a file path for output. The output file path is taken from PDFWriter.OutputFile.
func (p *PDFWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	pdf := gofpdf.New("P", "in", "Letter", "")

	// Load fonts using modern embed approach
	pdf.AddUTF8FontFromBytes("CourierPrime", "", font.GetFont("CourierPrime", ""))
	pdf.AddUTF8FontFromBytes("CourierPrime", "B", font.GetFont("CourierPrime", "B"))
	pdf.AddUTF8FontFromBytes("CourierPrime", "I", font.GetFont("CourierPrime", "I"))
	pdf.AddUTF8FontFromBytes("CourierPrime", "BI", font.GetFont("CourierPrime", "BI"))

	// Set default font for the document
	pdf.SetFont("CourierPrime", "", 12)
	pdf.AddPage()
	pdf.SetMargins(1, 1, 1)
	pdf.SetXY(1, 1)
	f := &Tree{
		PDF:          pdf,
		Rules:        p.Elements, // Use the Elements from the PDFWriter struct
		F:            screenplay, // Use the screenplay passed to the Write method
		HTML:         pdf.HTMLBasicNew(),
		DualDialogue: false,
		DualColumn:   0,
		DualBuffer:   []lex.Line{},
	}
	f.Render()
	err := pdf.OutputFileAndClose(p.OutputFile) // Use the OutputFile from the PDFWriter struct
	if err != nil {
		return err // Return the error instead of panicking
	}
	return nil
}
