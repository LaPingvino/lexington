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
	bar          *colorBar  // the color bar being drawn (line style)
	printing     *printing  // where the text being printed lands
	pagination   rules.Pagination
	foot         float64 // the page's bottom margin, while a row's break raises it (0: none)
	heights      map[[2]string]int
	measure      *measuring // where the scenes are printed, when measured (Measure)
}

// span is where printed text is on a page: from its top to its foot.
type span struct {
	page        int
	top, bottom float64
}

// printing follows a piece of text being printed over the pages it
// breaks over: the spans it fills, page by page (none on a page it only
// moves on from).
type printing struct {
	on    bool
	page  int
	top   float64
	spans []span
}

// start begins following text printed from where the PDF is now.
func (p *printing) start(pdf *gofpdf.Fpdf) {
	p.on, p.page, p.top, p.spans = true, pdf.PageNo(), pdf.GetY(), nil
}

// cut ends the span on this page: the PDF is about to break to the next,
// where the text goes on from the top margin.
func (p *printing) cut(pdf *gofpdf.Fpdf) {
	if !p.on {
		return
	}
	p.add(pdf)
	_, top, _, _ := pdf.GetMargins()
	p.page, p.top = pdf.PageNo()+1, top
}

func (p *printing) add(pdf *gofpdf.Fpdf) {
	if y := pdf.GetY(); pdf.PageNo() == p.page && y > p.top {
		p.spans = append(p.spans, span{p.page, p.top, y})
	}
}

// stop ends following, with the spans the text filled.
func (p *printing) stop(pdf *gofpdf.Fpdf) []span {
	p.add(pdf)
	p.on = false
	return p.spans
}

// printed are the spans that what print prints fills.
func (t Tree) printed(print func()) []span {
	if t.printing == nil {
		print()
		return nil
	}
	t.printing.start(t.PDF)
	print()
	return t.printing.stop(t.PDF)
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

// pr prints a line, and draws its marker or band (headingColor) on the
// spans it fills, which it returns (none for a blank line).
func (t Tree) pr(a string, text string) []span {
	f, printed := t.format(a, text)
	if a == lex.TypeScene {
		if _, number := lex.SceneNumber(text); number != "" {
			t.sceneNumber(f, number)
		}
	}
	return t.prf(f, printed, a == lex.TypeScene || a == lex.TypeSection)
}

// prf prints text in a format (see pr), a heading's or not.
func (t Tree) prf(f rules.Format, text string, heading bool) []span {
	if strings.TrimSpace(text) == "" {
		t.linePrint(f, text)
		return nil
	}
	spans := t.printed(func() { t.linePrint(f, text) })
	if t.bar != nil {
		t.headingColor(f, text, t.bar.block, heading, spans)
	}
	return spans
}

// restoreFoot puts the page's bottom margin back when a row's break
// (paginate) raised it.
func (t *Tree) restoreFoot() {
	if t.foot != 0 {
		t.PDF.SetAutoPageBreak(true, t.foot)
		t.foot = 0
	}
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

	if t.bar == nil {
		t.bar = &colorBar{}
	}
	if t.printing == nil {
		t.printing = &printing{}
	}
	t.pagination = t.Rules.Pagination()
	t.PDF.SetAcceptPageBreakFunc(func() bool {
		auto, _ := t.PDF.GetAutoPageBreak()
		if auto {
			t.printing.cut(t.PDF)
		}
		t.restoreFoot()
		return auto
	})
	style, sch := lex.ColorStyle(""), lex.ColorScheme{}
	if t.Rules.ShowsColors() {
		sch = t.Rules.ColorScheme()
		style = sch.Style
	}
	rows := t.F
	blocks, colorOf := lex.ColorBlocks(rows), map[int]string{} // the color block of each row, its color
	for _, row := range rows {
		if row.Block != 0 && colorOf[row.Block] == "" {
			colorOf[row.Block] = row.Color
		}
	}
	for i := 0; i < len(rows); i++ {
		if block == "" && !t.DualDialogue {
			rows, blocks = t.paginate(rows, blocks, i)
		}
		row := rows[i]
		// the block of a color (a scene, a section whole) the row is in;
		// none on the blank lines between two blocks
		t.bar.id, t.bar.color, t.bar.block = blocks[i], "", colorOf[blocks[i]]
		if style == lex.ColorLine || (sch.Block && style == lex.ColorBackground) {
			t.bar.color = t.bar.block
		}
		if row.Type == lex.TypeScene || row.Type == lex.TypeSection {
			t.bar.block = row.Color
		}
		switch row.Type {
		case typeBreak:
			t.PDF.AddPage()
			continue
		case typeMore:
			f := t.Rules.Get(lex.TypeSpeaker)
			f.Prefix, f.Postfix = "", ""
			t.extendBar(t.prf(f, row.Contents, false))
			continue
		}
		if row.Type == lex.TypeDualClose && !t.DualDialogue {
			continue
		}
		if row.Type == lex.TypeDualClose {
			t.extendBar(t.printed(func() { t.handleSpecialCases(row, &block, &lastsection) }))
			continue
		}
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

		if row.Type == lex.TypeScene && block == "" {
			t.measure.scene(t.PDF)
		}
		if t.shouldSkipElement(row, block) {
			continue
		}

		if block == internal.ElementTitle || block == elementMeta {
			row.Type = block
		}
		if c, ok := lex.ColorFor(lex.ColorText, row.Color); ok && style == lex.ColorText &&
			(row.Type == lex.TypeScene || row.Type == lex.TypeSection) {
			t.PDF.SetTextColor(int(c[0]), int(c[1]), int(c[2]))
			spans := t.pr(row.Type, row.Contents)
			if row.Type == lex.TypeScene {
				t.measure.heading(spans)
			}
			t.PDF.SetTextColor(0, 0, 0)
			continue
		}
		// text+block: every line of a block of a color in it (the
		// heading's own, above)
		c, blockText := lex.ColorFor(lex.ColorText, t.bar.block)
		blockText = blockText && style == lex.ColorText && sch.Block && t.bar.block != ""
		if blockText {
			t.PDF.SetTextColor(int(c[0]), int(c[1]), int(c[2]))
		}
		spans := t.pr(row.Type, row.Contents)
		if row.Type == lex.TypeScene && block == "" {
			t.measure.heading(spans)
		}
		if blockText {
			t.PDF.SetTextColor(0, 0, 0)
		}
		t.restoreFoot()
		if block == "" {
			t.extendBar(spans)
		}
	}

	// Flush any remaining dual dialogue at the end
	if t.DualDialogue {
		t.extendBar(t.printed(t.flushDualDialogue))
	}
	t.closeBar()
}

// colorBar is the bar of a scene's color in the left margin (the line
// style), or its band: drawn a page at a time, from the top of the
// block's first text on the page to the foot of its last (never beside
// blank lines alone, before or after it).
type colorBar struct {
	id          int    // the block the line is in (lex.ColorBlocks)
	color       string // of the block drawn beside (bar) or under (band); "" for none
	block       string // the color of the block the line is in (its heading's)
	drawn       string // the color of the bar drawn so far
	drawnID     int    // and its block
	page        int
	top, bottom float64
}

// The bar: its width, and how far it is before the action's text, in
// inches (before the scene numbers).
const (
	barWidth = 0.04
	barGap   = 1.0
)

// extendBar draws the bar of the scene's color beside the text just
// printed, on the spans it filled (blank lines extend nothing: within a
// block the bar runs on over them from text to text).
func (t *Tree) extendBar(spans []span) {
	t.measure.printed(spans)
	b := t.bar
	if b == nil {
		return
	}
	for _, s := range spans {
		if b.drawn != "" && (b.id != b.drawnID || b.color != b.drawn || s.page != b.page) {
			t.closeBar()
		}
		if b.color == "" || b.id == 0 {
			continue
		}
		if b.drawn == "" {
			b.drawn, b.drawnID, b.page, b.top = b.color, b.id, s.page, s.top
		}
		b.bottom = s.bottom
	}
}

// textArea is where the script's text can stand on the page: from the
// leftmost to the rightmost margin of the elements a block holds, so that a
// band covers every line of it and the bar stands left of all of them in
// any format (Inverse puts names and dialogue left of the action).
func (t *Tree) textArea() (left, right float64) {
	pageWidth, _ := t.PDF.GetPageSize()
	left, right = pageWidth, 0
	for _, k := range []string{"scene", "action", "speaker", "dialog", "paren", "trans", "lyrics"} {
		f, err := t.Rules.GetSafe(k)
		if err != nil {
			continue
		}
		left, right = min(left, f.Left), max(right, pageWidth-f.Right)
	}
	if right <= left { // no elements: the action's
		a := t.Rules.Get("action")
		return a.Left, pageWidth - a.Right
	}
	return left, right
}

// closeBar draws the bar so far, on its page.
func (t *Tree) closeBar() {
	b := t.bar
	if b == nil || b.drawn == "" {
		return
	}
	sch := t.Rules.ColorScheme()
	band := lex.ColorStyle("")
	switch {
	case sch.Style == lex.ColorLine && sch.Tint:
		band = lex.ColorTint
	case sch.Style == lex.ColorBackground && sch.Block:
		band = lex.ColorBackground
	}
	if b.bottom > b.top {
		now := t.PDF.PageNo()
		t.PDF.SetPage(b.page)
		left, right := t.textArea() // (scaled to the page already)
		if c, ok := lex.ColorFor(band, b.drawn); ok && band != "" {
			// a band across the text under the block: drawn after its text,
			// multiplied, so that the text stays as it is (black on it)
			t.PDF.SetAlpha(1, "Multiply")
			t.PDF.SetFillColor(int(c[0]), int(c[1]), int(c[2]))
			t.PDF.Rect(left-0.05, b.top, right-left+0.1, b.bottom-b.top, "F")
			t.PDF.SetAlpha(1, "Normal")
		}
		if c, ok := lex.ColorFor(lex.ColorLine, b.drawn); ok && sch.Style == lex.ColorLine {
			t.PDF.SetFillColor(int(c[0]), int(c[1]), int(c[2]))
			if sch.Patterns {
				t.patternBar(lex.BarPattern(b.drawn), left-barGap, b.top, b.bottom)
			} else {
				t.PDF.Rect(left-barGap, b.top, barWidth*t.sc(), b.bottom-b.top, "F")
			}
		}
		t.PDF.SetPage(now)
	}
	b.drawn, b.drawnID = "", 0
}

// patternWidth is the width of a bar with a pattern, in inches: wider
// than a plain one, for its stripes to show.
const patternWidth = 0.07

// patternBar draws a bar in a pattern, from top to bottom at x: its
// stripes, solid or dashed (lengths in points).
func (t *Tree) patternBar(p lex.Pattern, x, top, bottom float64) {
	w := patternWidth * t.sc()
	for _, s := range p.Stripes {
		x0, sw := x+s.From*w, (s.To-s.From)*w
		if len(s.Dash) == 0 {
			t.PDF.Rect(x0, top, sw, bottom-top, "F")
			continue
		}
		y, on := top+s.Offset/72, true
		for k := 0; y < bottom; k++ {
			l := s.Dash[k%len(s.Dash)] / 72
			if on {
				t.PDF.Rect(x0, y, sw, min(l, bottom-y), "F")
			}
			y += l
			on = !on
		}
	}
}

// headingColor draws a marker behind a line's text (a heading's, or any
// line's of a block with a marker over blocks), or a heading's background,
// on the spans the text was printed on: after it, multiplied, so that the
// text stays as it is on it, and on the pages it went on to.
func (t Tree) headingColor(f rules.Format, text, color string, heading bool, spans []span) {
	if !t.Rules.ShowsColors() || len(spans) == 0 {
		return
	}
	sch := t.Rules.ColorScheme()
	style := sch.Style
	c, ok := lex.ColorFor(style, color)
	switch {
	case !ok:
		return
	case style == lex.ColorMarker && (heading || sch.Block):
	case style == lex.ColorBackground && heading && !sch.Block:
	default:
		return
	}
	pdf := t.PDF
	fontName, _ := t.Fonts.font(f.Font, text)
	pdf.SetFont(fontName, f.Style, f.Size)
	pageWidth, _ := pdf.GetPageSize()
	room := pageWidth - f.Left - f.Right
	lines := pdf.SplitText(lex.Unstyled(text), room)
	now := pdf.PageNo()
	defer pdf.SetPage(now)
	pdf.SetAlpha(1, "Multiply")
	defer pdf.SetAlpha(1, "Normal")
	pdf.SetFillColor(int(c[0]), int(c[1]), int(c[2]))
	for _, s := range spans {
		pdf.SetPage(s.page)
		if style == lex.ColorBackground {
			pdf.Rect(f.Left-0.05, s.top, room+0.1, s.bottom-s.top, "F")
			continue
		}
		for y := s.top; y+lineHeight/2 < s.bottom && len(lines) > 0; y += lineHeight {
			l := lines[0]
			lines = lines[1:]
			w := pdf.GetStringWidth(l)
			x := f.Left
			switch strings.ToUpper(f.Align) {
			case "C":
				x += (room - w) / 2
			case "R":
				x += room - w
			}
			pdf.Rect(x-0.03, y, w+0.06, lineHeight, "F")
		}
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
	f := p.tree(screenplay)
	f.Render()
	err := f.PDF.OutputFileAndClose(p.OutputFile) // Use the OutputFile from the PDFWriter struct
	if err != nil {
		return err // Return the error instead of panicking
	}
	return nil
}

// tree is the screenplay ready to render on the writer's paper, with its
// rules.
func (p *PDFWriter) tree(screenplay lex.Screenplay) *Tree {
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
	return &Tree{
		PDF:          pdf,
		Rules:        elements,
		F:            lex.Printable(screenplay, elements.Hides), // notes are not printed
		DualDialogue: false,
		DualColumn:   0,
		DualBuffer:   []lex.Line{},
		Fonts:        newFonts(pdf),
		Scale:        width / 8.5,
		VScale:       height / 11,
	}
}
