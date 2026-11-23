package pdf

import (
	"strings"
	"sync"

	gobidi "github.com/go-text/typesetting/bidi"
)

// Shaper prints a font's text shaped, for a program that lays out its
// own text through a Drawer (Accolade's prose PDF) in a font it added
// with AddFont, as Lexington's screenplay PDF prints it, without its
// layout:
//   - Devanagari: Width measures text as Cell prints it, and Cell prints
//     it as CellFormat would, each Devanagari run shaped by the font's
//     own features, its glyphs where the shaper puts them, in a span
//     whose ActualText is the run as written (devanagari.go);
//   - right to left: Compose joins Arabic (its letters' contextual forms,
//     lam-alef) and gives pointed Hebrew letters their presentation forms,
//     in logical order, before the text is measured and wrapped; Levels
//     and VisualRuns give a paragraph's wrapped lines their visual order
//     (UAX #9), and CellRTL prints a right-to-left run of one: turned
//     around, its brackets mirrored, its marks where the font's GPOS puts
//     them (bidi.go). The PDF holds such text in the order it is seen,
//     as browsers print it; a reader (pdftotext) gives it back as written.
//
// Other text is printed and measured as gofpdf does, without the
// direction controls (which set the order and print as nothing).
type Shaper struct {
	s   *fontShaper
	has func(rune) bool // the font has a glyph for it
}

var exportedShapers sync.Map // a font's *coded (its file's identity) to its *Shaper

// ShaperOf is the shaper of a font file, the bytes given to AddFont,
// made once a file (by its contents); nil if it cannot be read.
func ShaperOf(data []byte) *Shaper {
	if len(data) == 0 {
		return nil
	}
	c := glyphCoded(data)
	if s, ok := exportedShapers.Load(c); ok {
		if sh, ok := s.(*Shaper); ok {
			return sh
		}
	}
	var out *Shaper
	if fs := newFontShaper(data); fs != nil {
		out = &Shaper{s: fs, has: glyphTest(data)}
	}
	s, _ := exportedShapers.LoadOrStore(c, out) // one Shaper per font, whoever stored it first
	if sh, ok := s.(*Shaper); ok {
		return sh
	}
	return out
}

// Width is text's width in d's current font (this shaper's) as Cell or
// CellRTL prints it: each Devanagari run as wide as shaped, the rest as
// gofpdf measures it, without the cells of what takes none (marks,
// joiners, direction controls).
func (s *Shaper) Width(d Drawer, text string) float64 {
	if s == nil {
		return d.GetStringWidth(text)
	}
	if !hasDevanagari(text) {
		return cellWidth(d, text, false)
	}
	_, w := shapedParts(d, s.s, text)
	return w
}

// Cell prints text from d's position, w wide and h high, as
// CellFormat(w, h, text, "", 0, align, false, 0, "") does, its
// Devanagari shaped. family, style and size (in points) are d's current
// font, this shaper's.
func (s *Shaper) Cell(d Drawer, family, style string, size, w, h float64, text, align string) {
	var fs *fontShaper
	if s != nil {
		fs = s.s
		text = withoutControls(text)
	}
	shapedCell(d, fs, family, style, size, w, h, text, align)
}

// Compose is text as this font prints it, in logical order: its Arabic
// runs shaped by the font's own features (each letter its contextual
// form, lam-alef its ligature, as presentation forms: arabic.go), and a
// pointed Hebrew letter as its presentation form where the font has one
// (hebrew.go). A program measures, wraps and orders the composed text.
// Its Arabic is shaped in context when it is composed whole: a word
// split (by its emphasis) and composed in parts does not join across.
func (s *Shaper) Compose(text string) string {
	if s == nil {
		return text
	}
	if s.has != nil {
		text = composeHebrew(text, s.has)
	}
	if hasArabic(text) {
		text, _ = s.s.shape(text)
	}
	return text
}

// CellRTL prints a run of text read right to left (a stretch of a line
// at an odd level: VisualRuns), given in logical order, from d's
// position, w wide and h high, aligned, as CellFormat prints a cell:
// turned around, its brackets mirrored (rule L4), its marks (Arabic
// vowels, Hebrew points) each where the font's GPOS puts it on its
// letter, a copy of them invisible where they stand unplaced so that the
// text read out of the PDF has them. family, style and size (in points)
// are d's current font, this shaper's; text is composed (Compose).
func (s *Shaper) CellRTL(d Drawer, family, style string, size, w, h float64, text, align string) {
	var fs *fontShaper
	if s != nil {
		fs = s.s
	}
	p := rtlPiece(fs, text)
	x, y := d.GetX(), d.GetY()
	pw := cellWidth(d, p.text, false)
	cm := d.GetCellMargin()
	at := x + cm
	switch {
	case strings.Contains(align, "R"):
		at = x + w - cm - pw
	case strings.Contains(align, "C"):
		at = x + (w-pw)/2
	}
	d.SetCellMargin(0)
	printPiece(d, fs, family, style, size, p, at, pw, h)
	drawMarks(d, family, style, size, h, p, at)
	d.SetCellMargin(cm)
	d.SetXY(x+w, y)
}

// Level is a character's bidi embedding level (UAX #9): even reads left
// to right, odd right to left.
type Level = gobidi.Level

// Levels are the embedding levels of a paragraph's characters, rtl its
// direction (lex.LineRTL tells it by its first letter), resolved over the
// whole paragraph (rules up to I2), before it is wrapped.
func Levels(text []rune, rtl bool) []Level { return levels(text, rtl) }

// VisualRun is a stretch of a line, the paragraph's characters from
// Start to End (in logical order), shown in one direction: left to right,
// or right to left (RTL: printed by CellRTL).
type VisualRun struct {
	Start, End int
	RTL        bool
}

// VisualRuns are a line's runs, the paragraph's characters from..to of
// levels (Levels), in the order they are shown, left to right (rule L2).
func VisualRuns(lv []Level, from, to int) []VisualRun {
	var out []VisualRun
	for _, i := range visualOrder(lv, from, to) {
		rtl := lv[i]%2 == 1
		if n := len(out); n > 0 && out[n-1].RTL == rtl {
			if r := &out[n-1]; !rtl && r.End == i {
				r.End++
				continue
			} else if rtl && r.Start == i+1 {
				r.Start--
				continue
			}
		}
		out = append(out, VisualRun{Start: i, End: i + 1, RTL: rtl})
	}
	return out
}

// Mirror is how a character shows in a right-to-left run: a bracket or
// another mirrored sign turned (rule L4), any other as it is.
func Mirror(r rune) rune { return mirror(r) }

// rtlPiece is a right-to-left run, given in logical order, as printed:
// turned around without its direction controls, its brackets mirrored,
// each mark held for its letter and placed on it by sh (nil: where it
// stands).
func rtlPiece(sh *fontShaper, text string) piece {
	rs := []rune(text)
	var p piece
	var b []rune
	var pending []placed
	for i := len(rs) - 1; i >= 0; i-- {
		r := rs[i]
		if control(r) {
			continue
		}
		if markRune(r) && i > 0 && !control(rs[i-1]) {
			pending = append(pending, placed{mark: string(r)}) // before its letter: held for it
			continue
		}
		c := mirror(r)
		if len(pending) > 0 {
			p.marks = append(p.marks, placeMarks(sh, string(c), pending, len(b))...)
			pending = pending[:0]
		}
		b = append(b, c)
	}
	p.text = string(b)
	return p
}

// withoutControls is text without its direction controls.
func withoutControls(text string) string {
	if strings.IndexFunc(text, control) < 0 {
		return text
	}
	return strings.Map(func(r rune) rune {
		if control(r) {
			return -1
		}
		return r
	}, text)
}

// HasDevanagari tells whether text has a Devanagari letter or sign: one
// Shaper shapes.
func HasDevanagari(text string) bool { return hasDevanagari(text) }

// DevanagariRune tells whether a character is shaped with the
// Devanagari run around it: a Devanagari letter or sign, or a joiner.
func DevanagariRune(r rune) bool { return devaRune(r) }

// DevanagariRunEnd is where the Devanagari run starting at rs[i] ends:
// over its letters, signs and joiners and the combining marks on them.
// A run is shaped whole, so it is printed in one font.
func DevanagariRunEnd(rs []rune, i int) int { return devaRunEnd(rs, i) }

// JoinsBefore tells whether a line cannot break between a and b, as
// they are of one cluster: b a mark or a joiner, a after a joiner, or a
// Devanagari consonant after a virama.
func JoinsBefore(a, b rune) bool { return joinsBefore(a, b) }
