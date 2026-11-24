package pdf

import (
	"strings"
	"unicode"
	"unicode/utf8"

	gobidi "github.com/go-text/typesetting/bidi"
	"golang.org/x/text/unicode/bidi"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Right to left. A line runs right to left by its own letters (or its
// direction mark), a line with none by its language's direction
// (lex.LineRTL, PDFWriter.RTL), as Accolade's editor reads it. Such a
// line is laid out from the other side of the page (mirrored: its
// margins swapped, its alignment turned) and printed in visual order: its
// levels resolved over the whole row by the Unicode bidi algorithm, its
// wrapped lines reordered each (rule L2), the brackets in its
// right-to-left runs mirrored. A left-to-right line with right-to-left
// words in it keeps its side and is reordered the same way. Any other
// line prints as it always has.
//
// Combining marks (Arabic vowels, Hebrew points) in right-to-left text
// are printed apart, each where the font's GPOS anchors put it on its
// letter (mark to base, mark to mark: fontShaper.markOffsets); Courier
// Badi's marks have no advance, so the letters print as without them.

// lineRTL tells whether a line reads right to left.
func (t Tree) lineRTL(text string) bool { return lex.LineRTL(lex.Unstyled(text), t.rtl) }

// needsBidi tells whether a line has to be printed piece by piece
// (bidiPrint): reordered, as it reads right to left, or has right-to-left
// letters or direction controls in it; or shaped, as it has Devanagari
// (devanagari.go).
func needsBidi(text string, rtl bool) bool {
	if rtl || hasDevanagari(text) {
		return true
	}
	for _, r := range text {
		if control(r) {
			return true
		}
		if p, _ := bidi.LookupRune(r); p.Class() == bidi.R || p.Class() == bidi.AL {
			return true
		}
	}
	return false
}

// control tells whether a character is a direction control (a mark, an
// embedding, an isolate): it sets the order and prints as nothing
// (Courier Badi gives each a blank cell).
func control(r rune) bool {
	return r == lex.LRM || r == lex.RLM || r == 0x061C || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069
}

// mirrored is a format laid out from the other side of the page, for a
// right-to-left line: its margins swapped, its alignment turned. A
// centred line stays centred where a left-to-right one is (the title
// page's lines one under the other, whatever their direction).
func mirrored(f rules.Format) rules.Format {
	if strings.EqualFold(f.Align, "C") {
		return f
	}
	f.Left, f.Right = f.Right, f.Left
	f.Align = mirrorAlign(f.Align)
	return f
}

// mirrorAlign is an alignment from the other side: left is right.
func mirrorAlign(a string) string {
	switch strings.ToUpper(a) {
	case "R":
		return "L"
	case "C":
		return "C"
	}
	return "R"
}

// directed is the format a line prints in: mirrored when it reads right
// to left.
func (t Tree) directed(f rules.Format, text string) rules.Format {
	if t.lineRTL(text) {
		return mirrored(f)
	}
	return f
}

// levels are the bidi embedding levels of a paragraph's runes, in the
// direction given.
func levels(runes []rune, rtl bool) []gobidi.Level {
	out := make([]gobidi.Level, len(runes))
	if len(runes) == 0 {
		return out
	}
	dir := gobidi.LeftToRight
	if rtl {
		dir = gobidi.RightToLeft
	}
	var p gobidi.Paragraph
	runs := p.Segment(runes, dir)
	for i := 0; i < runs.NumRuns(); i++ {
		r := runs.Run(i)
		for k := r.Start; k < r.End; k++ {
			out[k] = r.Level
		}
	}
	return out
}

// visualOrder is the order a line's runes (from..to of the paragraph)
// are shown in, left to right (rule L2): from the highest level down to
// the lowest odd one, every run at that level or higher reversed.
func visualOrder(lv []gobidi.Level, from, to int) []int {
	order := make([]int, 0, to-from)
	high, lowOdd := gobidi.Level(0), gobidi.Level(127)
	for i := from; i < to; i++ {
		order = append(order, i)
		high = max(high, lv[i])
		if lv[i]%2 == 1 {
			lowOdd = min(lowOdd, lv[i])
		}
	}
	for l := high; l >= lowOdd && l > 0; l-- {
		for i := 0; i < len(order); {
			if lv[order[i]] < l {
				i++
				continue
			}
			j := i
			for j < len(order) && lv[order[j]] >= l {
				j++
			}
			for a, b := i, j-1; a < b; a, b = a+1, b-1 {
				order[a], order[b] = order[b], order[a]
			}
			i = j
		}
	}
	return order
}

// mirrors are the characters other than brackets that a right-to-left
// run shows mirrored.
var mirrors = map[rune]rune{'<': '>', '>': '<', '«': '»', '»': '«', '‹': '›', '›': '‹', '≤': '≥', '≥': '≤'}

// mirror is how a character shows in a right-to-left run (rule L4).
func mirror(r rune) rune {
	if m, ok := mirrors[r]; ok {
		return m
	}
	if p, _ := bidi.LookupRune(r); p.IsBracket() {
		return []rune(bidi.ReverseString(string(r)))[0]
	}
	return r
}

// piece is a run of a line in visual order printed in one style.
type piece struct {
	text  string
	style lex.Span // its emphasis (Text unused)
	marks []placed // the marks on its right-to-left letters, printed apart
}

// placed is a mark on the character at (a rune index into its piece's
// text), offset from that character's origin by dx, dy (in ems, y up).
type placed struct {
	at     int
	mark   string
	dx, dy float64
	known  markAt // its place from the shaping of the text as written (fonts.marks), if known
}

// markRune tells whether a character is a combining mark that the font's
// GPOS places on its letter.
func markRune(r rune) bool { return unicode.Is(unicode.Mn, r) }

// visualLines are a line's text wrapped in width as the PDF wraps it
// (Tree.wrap) and each line's pieces in visual order; rtl is the
// paragraph's direction. fontName is the font it prints in.
func (t Tree) visualLines(f rules.Format, text string, rtl bool, width float64) (string, [][]piece) {
	text = strings.TrimRight(text, "\r\n")
	fontName, spans := t.Fonts.spans(f.Font, text) // as printed (a standard font's bytes)
	plain := lex.Emphasis(text)                    // the characters, for their classes
	core := fontName == "Helvetica" || fontName == "Times"
	var chars []string
	var runes []rune
	var style []lex.Span
	var known []markAt // each char's mark place, from the shaping of the text as written
	for i, s := range spans {
		var marks []markAt
		if !core && i < len(plain) {
			marks = t.Fonts.marks(fontName, fontStyle(f.Style, s), plain[i].Text, s.Text)
		}
		for k, r := range []rune(s.Text) {
			if r == '\r' {
				continue
			}
			if k < len(marks) {
				known = append(known, marks[k])
			} else {
				known = append(known, markAt{})
			}
		}
		printed := strings.ReplaceAll(s.Text, "\r", "")
		var cs []string
		if core {
			for k := 0; k < len(printed); k++ {
				cs = append(cs, printed[k:k+1])
			}
		} else {
			for _, r := range printed {
				cs = append(cs, string(r))
			}
		}
		rs := []rune(strings.ReplaceAll(plain[i].Text, "\r", ""))
		if len(rs) != len(cs) { // (a standard font's translation is one byte a character)
			rs = rs[:0]
			for _, c := range cs {
				rs = append(rs, []rune(c)[0])
			}
		}
		chars, runes = append(chars, cs...), append(runes, rs...)
		for range cs {
			style = append(style, s)
		}
	}
	if len(known) != len(chars) { // (a standard font's bytes)
		known = make([]markAt, len(chars))
	}
	for len(chars) > 0 && chars[len(chars)-1] == "\n" {
		chars, runes, style = chars[:len(chars)-1], runes[:len(runes)-1], style[:len(style)-1]
	}
	t.PDF.SetFont(fontName, f.Style, f.Size)
	starts := t.breaks(chars, width, core)
	lv := levels(runes, rtl)
	shaper := func(s lex.Span) *fontShaper { // the marks' font: the letter's style
		if core {
			return nil
		}
		return t.Fonts.shaper(fontName, fontStyle(f.Style, s))
	}
	var lines [][]piece
	for k, a := range starts {
		b := len(chars)
		if k+1 < len(starts) {
			b = starts[k+1]
		}
		for b > a && (chars[b-1] == "\n" || (b < len(chars) && chars[b-1] == " ")) {
			b-- // the break: a new line, or the space wrapped at
		}
		var line []piece
		var pending []placed // marks before their letter (in visual order)
		order := visualOrder(lv, a, b)
		for v, i := range order {
			if control(runes[i]) {
				continue
			}
			c := chars[i]
			if !core && lv[i]%2 == 1 && markRune(runes[i]) {
				// in a right-to-left run a mark comes before its letter:
				// held for it, unless no letter of the run follows
				if v+1 < len(order) && lv[order[v+1]]%2 == 1 && order[v+1] == i-1 {
					pending = append(pending, placed{mark: c, known: known[i]})
					continue
				}
			}
			if lv[i]%2 == 1 {
				if m := mirror(runes[i]); m != runes[i] {
					c = string(m)
					if core {
						c = t.Fonts.translate(c)
					}
				}
			}
			s := style[i]
			if n := len(line); n == 0 || !sameStyle(line[n-1].style, s) {
				line = append(line, piece{style: s})
			}
			p := &line[len(line)-1]
			if len(pending) > 0 { // the letter of the marks held: placed on it
				p.marks = append(p.marks, placeMarks(shaper(s), c, pending, utf8.RuneCountInString(p.text))...)
				pending = pending[:0]
			}
			p.text += c
		}
		for _, m := range pending { // (none: a mark always has its letter after it)
			line = append(line, piece{text: m.mark, style: style[a]})
		}
		lines = append(lines, line)
	}
	return fontName, lines
}

// printPiece prints a piece from x, w wide. Its placed marks go where
// they stood unplaced (before their letter, in visual order) but
// invisible, so that the text copied out of the PDF reads as written;
// printMarks draws them where they show.
//
// Its Devanagari is shaped by sh, the font's (fontName in style, size
// points) shaper: shapedCell.
func printPiece(pdf Drawer, sh *fontShaper, fontName, style string, size float64, p piece, x, w, lineHeight float64) {
	if len(p.marks) == 0 {
		pdf.SetX(x)
		shapedCell(pdf, sh, fontName, style, size, w, lineHeight, p.text, "L")
		return
	}
	rs := []rune(p.text)
	from := 0
	seg := func(text string) {
		sw := cellWidth(pdf, text, sh == nil)
		pdf.SetX(x)
		shapedCell(pdf, sh, fontName, style, size, max(sw, markWidth), lineHeight, text, "L")
		x += sw
	}
	for k := 0; k < len(p.marks); {
		at := min(p.marks[k].at, len(rs))
		if at > from {
			seg(string(rs[from:at]))
			from = at
		}
		j := k
		var marks []string // the letter's marks, as they stood: in visual order
		for j < len(p.marks) && p.marks[j].at == p.marks[k].at {
			marks = append([]string{p.marks[j].mark}, marks...)
			j++
		}
		pdf.SetTextRenderingMode(3)
		seg(strings.Join(marks, ""))
		pdf.SetTextRenderingMode(0)
		k = j
	}
	if from < len(rs) {
		seg(string(rs[from:]))
	}
}

// placeMarks are the marks held for a letter (pending, in visual order:
// the last is the first written) placed on it, the letter at rune index
// at of its piece, printed as base: in logical order, each where sh (the
// font's shaper; nil: none) puts it by the font's GPOS, else where it
// stands unplaced.
func placeMarks(sh *fontShaper, base string, pending []placed, at int) []placed {
	cluster := []rune{[]rune(base)[0]}
	for k := len(pending) - 1; k >= 0; k-- {
		cluster = append(cluster, []rune(pending[k].mark)[0])
	}
	var dx, dy []float64
	ok := true
	for k := range pending { // placed by the shaping of the text as written
		m := pending[len(pending)-1-k]
		ok = ok && m.known.ok
		dx, dy = append(dx, m.known.dx), append(dy, m.known.dy)
	}
	if !ok && sh != nil {
		dx, dy, ok = sh.markOffsets(cluster)
	}
	out := make([]placed, 0, len(pending))
	for k := range pending {
		m := pending[len(pending)-1-k] // in logical order
		m.at = at
		if ok {
			m.dx, m.dy = dx[k], dy[k]
		}
		out = append(out, m)
	}
	return out
}

// printMarks prints the marks of a line's pieces (printed from starts),
// each offset from its letter's origin as the font places it: the line's
// cells moved by the offset, with no page break (a mark below the last
// line of a page stays on it).
func (t Tree) printMarks(fontName string, f rules.Format, line []piece, starts []float64) {
	for i, p := range line {
		drawMarks(t.PDF, fontName, fontStyle(f.Style, p.style), f.Size, lineHeight, p, starts[i])
	}
}

// drawMarks draws a piece's placed marks, the piece printed from start
// in fontName, style, size points, in cells h high: each offset from its
// letter's origin as the font places it, the position kept, with no
// page break (a mark below the last line of a page stays on it). They
// are shown, not text: the copy printPiece prints invisible where they
// stood is what is read out.
func drawMarks(pdf Drawer, fontName, style string, size, h float64, p piece, start float64) {
	if len(p.marks) == 0 {
		return
	}
	x0, y := pdf.GetX(), pdf.GetY()
	auto, margin := pdf.GetAutoPageBreak()
	pdf.SetAutoPageBreak(false, margin)
	pdf.SetFont(fontName, style, size)
	_, em := pdf.GetFontSize()
	rs := []rune(p.text)
	for _, m := range p.marks {
		at := start + cellWidth(pdf, string(rs[:min(m.at, len(rs))]), false)
		pdf.SetXY(at+m.dx*em, y-m.dy*em)
		pdf.RawWriteStr("/Span <</ActualText ()>> BDC")
		pdf.CellFormat(markWidth, h, m.mark, "", 0, "L", false, 0, "")
		pdf.RawWriteStr("EMC")
	}
	pdf.SetAutoPageBreak(auto, margin)
	pdf.SetXY(x0, y)
}

// markWidth is the width of a mark's cell (it has no advance of its own).
const markWidth = 0.001

func sameStyle(a, b lex.Span) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline
}

// bidiPrint prints a line that needs reordering (needsBidi) from left,
// width wide, in its format (mirrored already if it reads right to
// left): wrapped as MultiCell wraps (so as many lines as paginate
// counts), each line in visual order, aligned as MultiCell aligns.
func (t Tree) bidiPrint(f rules.Format, text string, rtl bool, left, width float64) {
	pdf := t.PDF
	fontName, lines := t.visualLines(f, text, rtl, width)
	core := fontName == "Helvetica" || fontName == "Times"
	shaper := func(p piece) *fontShaper {
		if core {
			return nil
		}
		return t.Fonts.shaper(fontName, fontStyle(f.Style, p.style))
	}
	cell := pdf.GetCellMargin()
	pdf.SetCellMargin(0)
	defer pdf.SetCellMargin(cell)
	for _, line := range lines {
		widths, total := make([]float64, len(line)), 0.0
		for i, p := range line {
			pdf.SetFont(fontName, fontStyle(f.Style, p.style), f.Size)
			widths[i] = cellWidth(pdf, p.text, core)
			total += widths[i]
		}
		x := left + cell
		switch strings.ToUpper(f.Align) {
		case "R":
			x = left + width - cell - total
		case "C":
			x = left + (width-total)/2
		}
		if len(line) == 0 { // an empty line: its height, breaking the page as a cell does
			pdf.SetX(left)
			pdf.CellFormat(width, t.lineH(), "", "", 0, "L", false, 0, "")
		}
		starts := make([]float64, len(line))
		for i, p := range line {
			pdf.SetFont(fontName, fontStyle(f.Style, p.style), f.Size)
			starts[i] = x
			printPiece(pdf, shaper(p), fontName, fontStyle(f.Style, p.style), f.Size, p, x, widths[i], t.lineH())
			x += widths[i]
		}
		t.printMarks(fontName, f, line, starts)
		pdf.Ln(t.lineH())
	}
	pdf.SetFont(fontName, f.Style, f.Size)
}
