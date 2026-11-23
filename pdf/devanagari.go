package pdf

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"

	"github.com/LaPingvino/lexington/lekhan"
)

// Devanagari is shaped by the font's own features (Courier Badi's, from
// Noto Sans Devanagari: conjuncts, half forms, reph, the i-matra before
// its consonant), by go-text's port of HarfBuzz, at print time: a line is
// wrapped and measured in its characters, by cells as the Lekhan width
// counts them (package lekhan, as Accolade does), and each Devanagari run
// of it is printed as shaped: its glyphs where the shaper puts them (their
// advances, the font's GPOS: mark anchors, the cluster's cells), a glyph
// no character maps to by the private-use code glyphCoded gives it, all in
// a span whose ActualText is the run as written: what is read out of the
// PDF (a copy, a search, pdftotext, Lexington's PDF import).
//
// Courier Badi draws each cluster in Lekhan monospace (its
// docs/LEKHAN.md): a simple cluster in one cell, a conjunct in two, so
// the shaped run is as wide as the cells counted.

// devaRune tells whether a character is shaped with the Devanagari run
// around it: a Devanagari letter or sign (Devanagari, its Extended
// blocks, the Vedic Extensions), or a joiner.
func devaRune(r rune) bool {
	return r >= 0x0900 && r <= 0x097F && r != 0x0964 && r != 0x0965 || r >= 0xA8E0 && r <= 0xA8FF ||
		r >= 0x1CD0 && r <= 0x1CFF || r >= 0x11B00 && r <= 0x11B09 || r == 0x200C || r == 0x200D
}

// devaRunEnd is where the Devanagari run starting at rs[i] ends: over its
// letters, signs and joiners, and the marks of no script (Inherited) on
// them.
func devaRunEnd(rs []rune, i int) int {
	j := i
	for j < len(rs) && (devaRune(rs[j]) || unicode.Is(unicode.Mn, rs[j])) {
		j++
	}
	return j
}

// hasDevanagari tells whether text has a Devanagari letter or sign.
func hasDevanagari(text string) bool {
	for _, r := range text {
		if devaRune(r) && r != 0x200C && r != 0x200D {
			return true
		}
	}
	return false
}

// zeroCell tells whether a character takes no cell by itself (Lekhan's
// RuneWidth 0): a combining mark, a joiner or a zero-width space, a
// direction mark or control.
func zeroCell(r rune) bool { return lekhan.RuneWidth(r) == 0 }

// joinsBefore tells whether a line cannot break between a and b (they
// are of one cluster): lekhan.Joins.
func joinsBefore(a, b rune) bool { return lekhan.Joins(a, b) }

// cellWidth is text's width in the current font as its cells count it:
// gofpdf's, without what takes no cell (a joiner has a blank cell's
// advance in Courier Badi). In a standard font, gofpdf's.
func cellWidth(pdf Drawer, text string, core bool) float64 {
	w := pdf.GetStringWidth(text)
	if core {
		return w
	}
	for _, r := range text {
		if zeroCell(r) {
			w -= pdf.GetStringWidth(string(r))
		}
	}
	return w
}

// glyphRun is shaped glyphs printed as one string: their characters
// (glyphCoded's), from X, Y in ems from the run's origin (Y up); each
// after the one before by its own advance.
type glyphRun struct {
	Text string
	X, Y float64
}

// devanagari is a Devanagari run shaped: its glyphs as runs printed each
// from where the shaper puts its first glyph, and its width, in ems.
func (s *fontShaper) devanagari(run []rune) ([]glyphRun, float64) {
	upem := int(s.face.Upem())
	s.mu.Lock()
	out := s.sh.Shape(shaping.Input{Text: run, RunStart: 0, RunEnd: len(run), Direction: di.DirectionLTR,
		Face: s.face, Size: fixed.I(upem), Script: language.Devanagari})
	s.mu.Unlock()
	em := float64(upem) * 64
	var runs []glyphRun
	var next float64 // where the last run's next glyph goes by its advance
	pen := fixed.Int26_6(0)
	var b strings.Builder
	flush := func() {
		if n := len(runs); n > 0 {
			runs[n-1].Text = b.String()
		}
		b.Reset()
	}
	for _, g := range out.Glyphs {
		x, y := float64(pen+g.XOffset)/em, float64(g.YOffset)/em
		pen += g.Advance
		if s.codes.ign[g.GlyphID] {
			continue // a joiner: it sets the forms, prints nothing
		}
		c, ok := s.codes.code[g.GlyphID]
		if !ok {
			continue // (no character: none is left for it)
		}
		n := len(runs)
		if n == 0 || runs[n-1].Y != y || abs(next-x) > 1e-6 {
			flush()
			runs = append(runs, glyphRun{X: x, Y: y})
			next = x
		}
		b.WriteRune(c)
		next += float64(s.face.HorizontalAdvance(g.GlyphID)) / float64(upem)
	}
	flush()
	return runs, float64(pen) / em
}

// pdfTextString is s as a PDF text string: UTF-16BE, hex.
func pdfTextString(s string) string {
	var b strings.Builder
	b.WriteString("<FEFF")
	for _, u := range utf16.Encode([]rune(s)) {
		fmt.Fprintf(&b, "%04X", u)
	}
	b.WriteString(">")
	return b.String()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// shapedPart is a piece of a line printed by shapedCell: a Devanagari
// run shaped (its glyph runs), or the text between, and its width.
type shapedPart struct {
	text  string
	deva  bool
	runs  []glyphRun
	width float64
}

// shapedParts are text's Devanagari runs, shaped by sh, and the text
// between, in the current font (pdf's), and their width in all: what
// shapedCell prints, as wide as it prints it.
func shapedParts(pdf Drawer, sh *fontShaper, text string) ([]shapedPart, float64) {
	_, em := pdf.GetFontSize()
	rs := []rune(text)
	var parts []shapedPart
	total := 0.0
	for i := 0; i < len(rs); {
		p := shapedPart{}
		j := i
		if devaRune(rs[i]) {
			j = devaRunEnd(rs, i)
			p.deva = true
			var wem float64
			p.runs, wem = sh.devanagari(rs[i:j])
			p.width = wem * em
		} else {
			for j < len(rs) && !devaRune(rs[j]) {
				j++
			}
		}
		p.text = string(rs[i:j])
		if !p.deva {
			p.width = cellWidth(pdf, p.text, false)
		}
		parts = append(parts, p)
		total += p.width
		i = j
	}
	return parts, total
}

// shapedCell prints text from x, w wide, as CellFormat prints a cell in
// a line (no border, the position after it), aligned: each Devanagari run
// in it shaped by sh (the current font's shaper; none: as CellFormat).
// fontName and style are the current font's, size its size in points.
func shapedCell(pdf Drawer, sh *fontShaper, fontName, style string, size, w, h float64, text, align string) {
	if sh == nil || !hasDevanagari(text) {
		pdf.CellFormat(w, h, text, "", 0, align, false, 0, "")
		return
	}
	x := pdf.GetX()
	_, em := pdf.GetFontSize()
	parts, total := shapedParts(pdf, sh, text)
	cm := pdf.GetCellMargin()
	at := x + cm
	switch {
	case strings.Contains(align, "R"):
		at = x + w - cm - total
	case strings.Contains(align, "C"):
		at = x + (w-total)/2
	}
	pdf.SetCellMargin(0)
	defer pdf.SetCellMargin(cm)
	y := pdf.GetY()
	for _, p := range parts {
		pdf.SetX(at)
		if !p.deva {
			pdf.CellFormat(max(p.width, markWidth), h, p.text, "", 0, "L", false, 0, "")
			y = pdf.GetY()
			at += p.width
			continue
		}
		// its cell, empty: a page break breaks it as it would a cell
		pdf.CellFormat(max(p.width, markWidth), h, "", "", 0, "L", false, 0, "")
		y = pdf.GetY()
		auto, margin := pdf.GetAutoPageBreak()
		pdf.SetAutoPageBreak(false, margin)
		pdf.RawWriteStr("/Span <</ActualText " + pdfTextString(p.text) + ">> BDC")
		// a reader puts the text from the first glyph shown to the last
		// one's end by its advance: an invisible space at either edge
		// gives it the run's place (a mark, a reph's empty cell, has none)
		// Underlined, the spaces go all along the run, so their lines
		// make one (the glyphs' would follow their advances, not the
		// cells).
		space := pdf.GetStringWidth(" ")
		edge := func(ex float64) {
			pdf.SetTextRenderingMode(3)
			pdf.SetXY(ex, y)
			pdf.CellFormat(markWidth, h, " ", "", 0, "L", false, 0, "")
			pdf.SetTextRenderingMode(0)
		}
		edge(at)
		underlined := strings.Contains(strings.ToUpper(style), "U")
		for ex := at + space; underlined && space > 0 && ex < at+p.width-space; ex += space {
			edge(ex)
		}
		if underlined {
			pdf.SetFont(fontName, strings.ReplaceAll(strings.ToUpper(style), "U", ""), size)
		}
		for _, g := range p.runs {
			pdf.SetXY(at+g.X*em, y-g.Y*em)
			pdf.CellFormat(markWidth, h, g.Text, "", 0, "L", false, 0, "")
		}
		if underlined {
			pdf.SetFont(fontName, style, size)
		}
		edge(at + max(p.width-space, 0))
		pdf.RawWriteStr("EMC")
		pdf.SetAutoPageBreak(auto, margin)
		at += p.width
	}
	pdf.SetXY(x+w, y)
}
