package pdf

import "sync"

// Shaper prints a font's Devanagari shaped, for a program that lays out
// its own text through a Drawer (Accolade's prose PDF) in a font it
// added with AddFont: Width measures text as Cell prints it, and Cell
// prints it as CellFormat would, each Devanagari run shaped by the font's
// own features, its glyphs where the shaper puts them, in a span whose
// ActualText is the run as written (devanagari.go). Text without
// Devanagari is printed and measured as gofpdf does.
type Shaper struct {
	s *fontShaper
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
		return s.(*Shaper)
	}
	var out *Shaper
	if fs := newFontShaper(data); fs != nil {
		out = &Shaper{s: fs}
	}
	s, _ := exportedShapers.LoadOrStore(c, out)
	return s.(*Shaper)
}

// Width is text's width in d's current font (this shaper's) as Cell
// prints it: each Devanagari run as wide as shaped, without the cells
// of what takes none (joiners, direction controls), the rest as gofpdf
// measures it.
func (s *Shaper) Width(d Drawer, text string) float64 {
	if s == nil || !hasDevanagari(text) {
		return d.GetStringWidth(text)
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
	}
	shapedCell(d, fs, family, style, size, w, h, text, align)
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
