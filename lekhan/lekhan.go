// Package lekhan counts the cells a text takes on a monospace grid, by
// Lekhan monospace (Courier Badi's docs/LEKHAN.md): a character one cell,
// an East Asian wide one two, a combining mark or a joiner none, and
// Devanagari by its clusters, as the typewriters did: a simple cluster (a
// consonant or a vowel with its signs: आ, कि, ला) one cell, a conjunct (two
// or more consonants joined by the virama: क्ष, त्न, स्क्री, र्य) two.
//
// Clusters are extended grapheme clusters (UAX #29) with the virama rule
// of Unicode 15.1 (GB9c): a consonant after a virama that follows a
// consonant stays in its cluster, also with a nukta or a zero-width joiner
// in between; a zero-width non-joiner after the virama ends it (क्‌ष is
// two clusters).
//
// It has no dependencies past the standard library, so an editor and a
// typesetter can count with the same code: Lexington's PDF wrap and
// Accolade's editor grid both do.
package lekhan

import "unicode"

const (
	virama = 0x094D
	zwnj   = 0x200C
	zwj    = 0x200D
)

// Consonant tells whether r is a Devanagari consonant (Indic_Conjunct_Break
// = Consonant): U+0915–0939, U+0958–095F, U+0978–097F.
func Consonant(r rune) bool {
	return r >= 0x0915 && r <= 0x0939 || r >= 0x0958 && r <= 0x095F || r >= 0x0978 && r <= 0x097F
}

// Devanagari tells whether r is of the Devanagari script: its block, the
// Extended blocks and the Vedic Extensions.
func Devanagari(r rune) bool {
	return r >= 0x0900 && r <= 0x097F || r >= 0xA8E0 && r <= 0xA8FF || r >= 0x1CD0 && r <= 0x1CFF ||
		r >= 0x11B00 && r <= 0x11B5F
}

// mark tells whether r is a combining mark: drawn on the character before
// it.
func mark(r rune) bool { return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Me) }

// RuneWidth is how many cells r takes by itself: 2 for East Asian wide
// and fullwidth characters and emoji, 0 for what is drawn on the
// character before it or not at all (combining marks, joiners, direction
// marks, variation selectors, Hangul's medial and final jamo), 1 for the
// others. A spacing mark (Mc) takes 1 here; in its cluster, Cells counts
// a Devanagari one as part of the cluster's cell.
func RuneWidth(r rune) int {
	switch {
	case r < 0x300:
		return 1
	case unicode.In(r, unicode.Mn, unicode.Me), r >= 0x200B && r <= 0x200F, r >= 0x202A && r <= 0x202E,
		r >= 0x2066 && r <= 0x2069, r >= 0x1160 && r <= 0x11FF:
		return 0
	case r >= 0x1100 && wide(r):
		return 2
	}
	return 1
}

// wideRanges are the East Asian wide and fullwidth characters and the
// emoji, [from, to].
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, // Hangul jamo
	{0x2E80, 0x303E}, // CJK, kana, Yi (not U+303F)
	{0x3040, 0xA4CF},
	{0xAC00, 0xD7A3}, // Hangul
	{0xF900, 0xFAFF}, // CJK compatibility
	{0xFE30, 0xFE4F}, // CJK compatibility forms
	{0xFF00, 0xFF60}, // fullwidth
	{0xFFE0, 0xFFE6},
	{0x1F300, 0x1F64F}, // emoji
	{0x1F900, 0x1F9FF},
	{0x20000, 0x3FFFD}, // CJK extensions
}

// wide tells whether r is East Asian wide or fullwidth, or an emoji.
func wide(r rune) bool {
	for _, w := range wideRanges {
		if r >= w[0] && r <= w[1] {
			return true
		}
	}
	return false
}

// Joins tells whether b belongs to the cluster of a, the character before
// it, judged by the two alone: b a combining mark or a joiner, a after a
// zero-width joiner, or a Devanagari consonant after a virama. Boundary
// is exact (the virama joins only after a consonant); Joins is for a
// caller that sees two characters, and differs only on malformed text.
func Joins(a, b rune) bool {
	if a < 0x20 { // a line's start: a mark there has no letter
		return false
	}
	return mark(b) || b == zwnj || b == zwj || a == zwj || a == virama && Consonant(b)
}

// Boundary tells whether a cluster starts at rs[i] (at 0 and at len(rs)
// one does).
func Boundary(rs []rune, i int) bool {
	if i <= 0 || i >= len(rs) {
		return true
	}
	a, b := rs[i-1], rs[i]
	if !Joins(a, b) {
		return true
	}
	if !Consonant(b) || mark(a) && a != virama {
		return false // a mark or a joiner, or after a joiner (emoji)
	}
	// a consonant after a virama (or a joiner): GB9c, a consonant before
	// the virama, over viramas, nukta, joiners and other marks
	seen := false
	for k := i - 1; k >= 0; k-- {
		switch c := rs[k]; {
		case c == virama:
			seen = true
		case c == zwj || unicode.Is(unicode.Mn, c) && c != virama:
		case Consonant(c):
			return !seen
		default:
			return true
		}
	}
	return true
}

// ClusterEnd is where the cluster that starts at rs[i] ends.
func ClusterEnd(rs []rune, i int) int {
	j := i + 1
	for j < len(rs) && !Boundary(rs, j) {
		j++
	}
	return min(j, len(rs))
}

// ClusterCells is how many cells a cluster takes: a Devanagari one 2 with
// two or more consonants (a conjunct), else 1; another the sum of its
// characters' RuneWidth.
func ClusterCells(c []rune) int {
	if len(c) == 0 {
		return 0
	}
	if !Devanagari(c[0]) {
		n := 0
		for k, r := range c {
			if k > 0 && Devanagari(r) && mark(r) && !devaBase(c[:k]) {
				n++ // a Devanagari mark without its letter: on a dotted circle
				continue
			}
			n += RuneWidth(r)
		}
		return n
	}
	consonants := 0
	for _, r := range c {
		if Consonant(r) {
			consonants++
		}
	}
	if consonants >= 2 {
		return 2
	}
	return 1
}

// devaBase tells whether a cluster's start has a Devanagari letter that
// its marks are drawn on.
func devaBase(c []rune) bool {
	for _, r := range c {
		if Devanagari(r) && !mark(r) {
			return true
		}
	}
	return false
}

// Cells is the cells each rune of rs takes, so that a cluster's runes add
// up to its ClusterCells: in a Devanagari cluster its first rune 1 and its
// second consonant 1 (the conjunct's second cell), the others 0; elsewhere
// each rune its RuneWidth.
func Cells(rs []rune) []int {
	out := make([]int, len(rs))
	for i := 0; i < len(rs); {
		j := ClusterEnd(rs, i)
		share(rs[i:j], out[i:j])
		i = j
	}
	return out
}

// Share is the cells rune i of rs takes (Cells' value for it), looking no
// further than its cluster.
func Share(rs []rune, i int) int {
	s := i
	for s > 0 && !Boundary(rs, s) {
		s--
	}
	out := make([]int, i-s+1)
	share(rs[s:i+1], out)
	return out[i-s]
}

func share(c []rune, out []int) {
	if !Devanagari(c[0]) {
		for k, r := range c {
			out[k] = RuneWidth(r)
			if k > 0 && Devanagari(r) && mark(r) && !devaBase(c[:k]) {
				out[k] = 1
			}
		}
		return
	}
	n := 0
	for k, r := range c {
		out[k] = 0
		if k == 0 {
			out[k] = 1
		}
		if Consonant(r) {
			n++
			if n == 2 {
				out[k] = 1
			}
		}
	}
}

// Width is how many cells s takes.
func Width(s string) int {
	rs := []rune(s)
	n := 0
	for _, c := range Cells(rs) {
		n += c
	}
	return n
}
