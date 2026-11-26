package fdx

import (
	"unicode"

	"github.com/LaPingvino/lexington/lex"
)

// same reports whether spans read from Fountain are the runs' text and
// styles (lex.SameRuns).
func same(spans []lex.Span, runs []run) bool { return lex.SameRuns(spans, runs) }

// styledRune is a character and its style, none for a space.
type styledRune struct {
	r                       rune
	bold, italic, underline bool
}

func styledRunes(spans []lex.Span) []styledRune {
	var out []styledRune
	for _, s := range spans {
		for _, r := range s.Text {
			if unicode.IsSpace(r) {
				out = append(out, styledRune{r: r})
			} else {
				out = append(out, styledRune{r, s.Bold, s.Italic, s.Underline})
			}
		}
	}
	return out
}
