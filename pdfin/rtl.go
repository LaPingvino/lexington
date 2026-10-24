package pdfin

import (
	"regexp"
	"sort"
	"strings"

	"golang.org/x/text/unicode/bidi"
)

// Right-to-left scripts (Hebrew, Arabic, Persian): their PDFs print the
// text in the order it is seen, right to left, and lay it out from the
// right margin. A page mostly in such letters is mirrored, so it reads as
// a left-to-right one: its runs from the other edge, their text in the
// order it is written.

// rtlPage reports whether most of the letters on a page are written right
// to left.
func rtlPage(ls []line) bool {
	rtl, ltr := 0, 0
	for _, l := range ls {
		for _, r := range l.text() {
			switch class(r) {
			case bidi.R, bidi.AL:
				rtl++
			case bidi.L:
				ltr++
			}
		}
	}
	return rtl > ltr
}

func class(r rune) bidi.Class {
	p, _ := bidi.LookupRune(r)
	return p.Class()
}

// mirror lays a right-to-left page's lines out from the right edge of a
// page width inches wide; seen holds whether the runs' text is as seen
// (a text layer) or as written, word by word (OCR).
func mirror(ls []line, width float64, seen bool) {
	for i := range ls {
		runs := ls[i].Runs
		for j := range runs {
			r := &runs[j]
			r.X, r.End = width-r.End, width-r.X
			if !seen {
				r.Text = asSeen(r.Text)
			}
			r.Text = tidy(asWritten(r.Text))
		}
		sort.SliceStable(runs, func(a, b int) bool { return runs[a].X < runs[b].X })
	}
}

// asSeen is OCR's words, each as written, in the order they are seen.
func asSeen(text string) string {
	words := strings.Split(text, " ")
	for i, w := range words {
		if strings.IndexFunc(w, func(r rune) bool { c := class(r); return c == bidi.R || c == bidi.AL }) >= 0 {
			words[i] = mirrored(reversed(w))
		}
	}
	return strings.Join(words, " ")
}

// asWritten is right-to-left text as written, from the order it is seen:
// turned around, but with its stretches of left-to-right text (numbers,
// Latin) kept the way round they are, and brackets turned.
func asWritten(seen string) string {
	rs := []rune(reversed(stripMarks(seen)))
	ltr := func(r rune) bool {
		switch class(r) {
		case bidi.L, bidi.EN, bidi.AN:
			return true
		}
		return false
	}
	// what may stand inside a left-to-right stretch: 3.5, 12:30, A-B
	inside := func(r rune) bool {
		switch class(r) {
		case bidi.CS, bidi.ES, bidi.ET:
			return true
		}
		return false
	}
	for i := 0; i < len(rs); {
		if !ltr(rs[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(rs) && (ltr(rs[j]) || (inside(rs[j]) && j+1 < len(rs) && ltr(rs[j+1]))) {
			j++
		}
		for a, b := i, j-1; a < b; a, b = a+1, b-1 {
			rs[a], rs[b] = rs[b], rs[a]
		}
		i = j
	}
	return mirrored(string(rs))
}

// The spaces a right-to-left text layer's gaps suggest around its signs:
// "הומה , שילוב" is "הומה, שילוב", "( הפסקה )" is "(הפסקה)", "ג ' ורג" is
// "ג'ורג".
var (
	spaceBefore = regexp.MustCompile(` +([,.;:?!)\]]+)`)
	spaceAfter  = regexp.MustCompile(`([(\[]) +`)
	geresh      = regexp.MustCompile(`(\p{Hebrew}) ?(['"׳״]) ?(\p{Hebrew})`)
)

func tidy(s string) string {
	s = spaceBefore.ReplaceAllString(s, "$1")
	s = spaceAfter.ReplaceAllString(s, "$1")
	return geresh.ReplaceAllString(s, "$1$2$3")
}

func reversed(s string) string {
	rs := []rune(s)
	for a, b := 0, len(rs)-1; a < b; a, b = a+1, b-1 {
		rs[a], rs[b] = rs[b], rs[a]
	}
	return string(rs)
}

// mirrored turns brackets the other way: ( for ).
func mirrored(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '(':
			return ')'
		case ')':
			return '('
		case '[':
			return ']'
		case ']':
			return '['
		case '{':
			return '}'
		case '}':
			return '{'
		case '<':
			return '>'
		case '>':
			return '<'
		case '«':
			return '»'
		case '»':
			return '«'
		}
		return r
	}, s)
}

// stripMarks takes out the invisible marks that set text's direction.
func stripMarks(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '‎' || r == '‏' || (r >= '‪' && r <= '‮') || (r >= '⁦' && r <= '⁩') {
			return -1
		}
		return r
	}, s)
}
