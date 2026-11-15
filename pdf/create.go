// The PDF package of Lexington creates a Screenplay PDF out of the Lex screenplay parsetree.
// This can be generated with the several other packages, e.g. the fountain package that parses
// fountain to lex in preparation.
package pdf

import (
	"io"
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
	// Page is the paper: "letter" (the default), "a4" or "a5". The rules
	// are for Letter; on other paper their positions scale with the
	// page's width (and the title page's with its height).
	Page string
}

// pages are the paper sizes by name, as gofpdf names them.
var pages = map[string]string{"letter": "Letter", "a4": "A4", "a5": "A5"}

// elementMeta is the format for title page fields other than the title,
// credit and author.
const elementMeta = "meta"

type Tree struct {
	PDF          *gofpdf.Fpdf
	Rules        rules.Set
	F            lex.Screenplay
	DualDialogue bool       // Track if we're in dual dialogue mode
	DualColumn   int        // Track which column we're in (0 = left, 1 = right)
	DualBuffer   []lex.Line // Buffer for dual dialogue elements
	Fonts        *fonts     // the rules' fonts (nil: Courier only)
	Scale        float64    // of horizontal positions: the page's width to Letter's (0: 1)
	VScale       float64    // of vertical ones
}

func (t Tree) sc() float64 {
	if t.Scale == 0 {
		return 1
	}
	return t.Scale
}

func (t Tree) vsc() float64 {
	if t.VScale == 0 {
		return 1
	}
	return t.VScale
}

func (t Tree) pr(a string, text string) {
	if a == lex.TypeNote {
		a = rules.KeyPrintNotes.String()
	}
	f := t.Rules.Get(a)
	switch a {
	case "section":
		text = lex.SectionText(text)
	case "scene":
		var number string
		if text, number = lex.SceneNumber(text); number != "" {
			t.sceneNumber(f, number)
		}
	}
	t.linePrint(f, f.Prefix+text+f.Postfix)
}

// sceneNumber prints a scene's number in both margins, on the line the
// heading is about to be printed on.
func (t Tree) sceneNumber(f rules.Format, number string) {
	y := t.PDF.GetY()
	pageWidth, pageHeight := t.PDF.GetPageSize()
	if _, margin := t.PDF.GetAutoPageBreak(); y+0.165 > pageHeight-margin {
		t.PDF.AddPage() // the heading goes there: its numbers with it
		y = t.PDF.GetY()
	}
	left, top, right, _ := t.PDF.GetMargins()
	defer t.PDF.SetMargins(left, top, right)
	fontName, number := t.Fonts.font(f.Font, number)
	t.PDF.SetFont(fontName, "", f.Size)
	t.PDF.SetMargins(0, top, 0)
	t.PDF.SetXY(f.Left-0.85, y)
	t.PDF.CellFormat(0.6, 0.165, number, "", 0, "R", false, 0, "")
	t.PDF.SetXY(pageWidth-f.Right+0.25, y)
	t.PDF.CellFormat(0.6, 0.165, number, "", 0, "L", false, 0, "")
	t.PDF.SetXY(0, y)
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

		if block == internal.ElementTitle || block == elementMeta {
			row.Type = block
		}
		if c, ok := lex.ColorRGB(row.Color); ok && t.Rules.ShowsColors() {
			t.PDF.SetTextColor(int(c[0]), int(c[1]), int(c[2]))
			t.pr(row.Type, row.Contents)
			t.PDF.SetTextColor(0, 0, 0)
			continue
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
		t.PDF.SetY(4 * t.vsc())
		return false
	case "title", "Title":
		t.PDF.SetTitle(lex.Unstyled(row.Contents), true)
		return false
	case "metasection":
		// Fields after the title block (Contact, Draft date, ...) go to
		// the bottom of the title page in the meta format.
		*block = elementMeta
		t.PDF.SetY(-2)
		return true
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
		section := strings.TrimSpace(row.Contents)
		level = len(section) - len(strings.TrimLeft(section, "#"))
		contents = lex.SectionText(section)
	case "scene":
		level = lastsection + 1
		contents, _ = lex.SceneNumber(row.Contents)
	}

	return contents, level
}

// shouldSkipElement determines if an element should be skipped
func (t *Tree) shouldSkipElement(row lex.Line, block string) bool {
	if row.Type == lex.TypeNote {
		return !t.Rules.PrintsNotes()
	}
	return t.Rules.Get(row.Type).Hide && block == ""
}

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
	sc := t.sc()
	origLeftMargin := 1.5 * sc
	origRightMargin := 1.0 * sc

	// Industry standard dual dialogue column positions:
	// Left column: 1.5" to 3.5" (2" width)
	// Right column: 4.5" to 6.5" (2" width)
	// This provides proper separation and readable columns
	// The columns of afterwriting, Better Fountain and screenplain: 2.5"
	// wide, at 2" and 5" from the page's edge (the right one ends at the
	// 1" margin); the dual rules' left margins, counted from 1", indent
	// within a column (dialogue 0, paren 0.3", speaker 0.5"). Lexington
	// had them at 1.5" and 4.5" with the margins counted from 1.5", which
	// put the dialogue left of the page's margin.
	leftColStart := 2.0 * sc
	leftColWidth := 2.5 * sc
	rightColStart := 5.0 * sc
	rightColWidth := 2.5 * sc

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
		t.PDF.SetXY(leftColStart+format.Left-1.0*sc, leftCurrentY)
		leftCurrentY += t.renderDualDialogueLine(format, line.Contents, leftColWidth-(format.Left-1.0*sc))
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
		t.PDF.SetXY(rightColStart+format.Left-1.0*sc, rightCurrentY)
		rightCurrentY += t.renderDualDialogueLine(format, line.Contents, rightColWidth-(format.Left-1.0*sc))
	}

	// Set final position to the maximum of both columns
	finalY := leftCurrentY
	if rightCurrentY > finalY {
		finalY = rightCurrentY
	}
	t.PDF.SetY(finalY) // the blank line after the block follows as usual

	// Restore original margins
	t.PDF.SetLeftMargin(origLeftMargin)
	t.PDF.SetRightMargin(origRightMargin)
}

func (t Tree) linePrint(format rules.Format, text string) {
	pdf := t.PDF
	fontName, spans := t.Fonts.spans(format.Font, strings.TrimRight(text, "\r\n"))
	pdf.SetFont(fontName, format.Style, format.Size)
	pdf.SetX(0)
	pdf.SetLeftMargin(format.Left)
	pdf.SetRightMargin(format.Right)
	if len(spans) == 1 && !styled(spans[0]) {
		pdf.MultiCell(0, lineHeight, spans[0].Text, "", format.Align, false)
		return
	}
	t.writeSpans(format, fontName, spans)
	pdf.SetY(pdf.GetY() + lineHeight)
}

// lineHeight is a line's height: 12 pt type, six lines to the inch.
const lineHeight = 0.165

func styled(s lex.Span) bool { return s.Bold || s.Italic || s.Underline }

// fontStyle is gofpdf's style for a span in a format's style.
func fontStyle(base string, s lex.Span) string {
	style := strings.ToUpper(base)
	for _, f := range []struct {
		on bool
		c  string
	}{{s.Bold, "B"}, {s.Italic, "I"}, {s.Underline, "U"}} {
		if f.on && !strings.Contains(style, f.c) {
			style += f.c
		}
	}
	return style
}

// writeSpans writes styled text from the current position, wrapping at
// the margins; centred or right-aligned when it fits on one line.
func (t Tree) writeSpans(format rules.Format, fontName string, spans []lex.Span) {
	pdf := t.PDF
	if a := strings.ToUpper(format.Align); a == "C" || a == "R" {
		width := 0.0
		for _, s := range spans {
			pdf.SetFont(fontName, fontStyle(format.Style, s), format.Size)
			width += pdf.GetStringWidth(s.Text)
		}
		left, _, right, _ := pdf.GetMargins()
		pageWidth, _ := pdf.GetPageSize()
		if room := pageWidth - left - right; width <= room {
			x := left + room - width
			if a == "C" {
				x = left + (room-width)/2
			}
			pdf.SetX(x)
		}
	}
	for _, s := range spans {
		pdf.SetFont(fontName, fontStyle(format.Style, s), format.Size)
		pdf.Write(lineHeight, s.Text)
	}
	pdf.SetFont(fontName, format.Style, format.Size)
}

// renderDualDialogueLine renders a single line of dual dialogue and returns the height consumed
func (t Tree) renderDualDialogueLine(format rules.Format, text string, columnWidth float64) float64 {
	fontName, spans := t.Fonts.spans(format.Font, strings.TrimRight(text, "\r\n"))
	t.PDF.SetFont(fontName, format.Style, format.Size)
	currentY := t.PDF.GetY()
	if len(spans) == 1 && !styled(spans[0]) {
		t.PDF.MultiCell(columnWidth, lineHeight, spans[0].Text, "", format.Align, false)
		return t.PDF.GetY() - currentY
	}
	// styled text wraps at the margins: keep it in its column
	x := t.PDF.GetX()
	left, top, right, _ := t.PDF.GetMargins()
	pageWidth, _ := t.PDF.GetPageSize()
	t.PDF.SetMargins(x, top, pageWidth-x-columnWidth)
	t.writeSpans(format, fontName, spans)
	t.PDF.Ln(lineHeight)
	t.PDF.SetMargins(left, top, right)
	return t.PDF.GetY() - currentY
}

// Write converts the internal lex.Screenplay format to a PDF file.
// It implements the writer.Writer interface.
// Note: For PDF, the 'w io.Writer' argument is currently ignored as gofpdf
// requires a file path for output. The output file path is taken from PDFWriter.OutputFile.
func (p *PDFWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	size, ok := pages[strings.ToLower(p.Page)]
	if !ok {
		size = "Letter"
	}
	pdf := gofpdf.New("P", "in", size, "")
	width, height := pdf.GetPageSize()
	elements := p.Elements
	if elements == nil {
		elements = rules.Default
	}
	if size != "Letter" { // the rules' positions, scaled to the page
		scaled := rules.Set{}
		for k, f := range elements {
			f.Left, f.Right = f.Left*width/8.5, f.Right*width/8.5
			scaled[k] = f
		}
		elements = scaled
	}

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
		Rules:        elements,
		F:            lex.Printable(screenplay, elements.PrintsNotes()), // notes are not printed
		DualDialogue: false,
		DualColumn:   0,
		DualBuffer:   []lex.Line{},
		Fonts:        newFonts(pdf),
		Scale:        width / 8.5,
		VScale:       height / 11,
	}
	f.Render()
	err := pdf.OutputFileAndClose(p.OutputFile) // Use the OutputFile from the PDFWriter struct
	if err != nil {
		return err // Return the error instead of panicking
	}
	return nil
}
