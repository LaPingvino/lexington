package lex

import (
	"slices"
	"strings"
	"unicode"
)

// Run is a piece of a paragraph as a program that styles its text by
// runs keeps it (Final Draft, Fade In, KIT Scenarist): its text and
// emphasis, or a note, written as Fountain has it ([[…]]).
type Run struct {
	Span
	Note bool
}

// FountainRuns is runs as Fountain text that reads back as them
// (Emphasis), with as few markers escaped as will do. Emphasis Fountain
// cannot write as it is (part of a word underlined, styles switching
// inside a word) is left out run by run, as far as it must be; the rest
// is kept. Notes stay where they are among the text.
func FountainRuns(runs []Run) string {
	text := fountainText(runs)
	if text != "" || RunsText(runs) == "" {
		return text
	}
	kept := slices.Clone(runs)
	for i := range kept {
		if kept[i].Note || (!kept[i].Bold && !kept[i].Italic && !kept[i].Underline) {
			continue
		}
		try := slices.Clone(kept)
		for j := i + 1; j < len(try); j++ {
			if !try[j].Note {
				try[j].Bold, try[j].Italic, try[j].Underline = false, false, false
			}
		}
		if fountainText(try) == "" {
			kept[i].Bold, kept[i].Italic, kept[i].Underline = false, false, false
		}
	}
	if text = fountainText(kept); text == "" { // cannot be: plain text escaped reads as itself
		text = markup(kept, "*_")
	}
	return text
}

// fountainText is the runs as Fountain that reads back as them, with as
// few markers escaped as will do; "" if there is none.
func fountainText(runs []Run) string {
	for _, escape := range []string{"", "*", "_", "*_"} {
		if s := markup(runs, escape); same(Emphasis(StripNotes(s)), runs) {
			return s
		}
	}
	return ""
}

// markers are Fountain's emphasis markers, in the order they open.
var markers = []struct {
	marker string
	on     func(Span) bool
}{
	{"_", func(s Span) bool { return s.Underline }},
	{"**", func(s Span) bool { return s.Bold }},
	{"*", func(s Span) bool { return s.Italic }},
}

// markup writes runs with Fountain's emphasis markers, escaping the ones
// in their text that are in escape ("*", "_", "*_"). A marker opens and closes where the style
// changes, next to the text: the spaces around a change are outside it.
func markup(runs []Run, escape string) string {
	var b strings.Builder
	var open []int // markers open, in order
	space := ""    // spaces not yet written
	bare := false  // some of them are not underlined (an underline would show under them)
	change := func(s Span) {
		keep := 0 // the markers open that stay open: up to the first that does not
		for keep < len(open) && markers[open[keep]].on(s) && (!bare || markers[open[keep]].marker != "_") {
			keep++
		}
		for i := len(open) - 1; i >= keep; i-- {
			b.WriteString(markers[open[i]].marker)
		}
		open = open[:keep]
		b.WriteString(space)
		space, bare = "", false
		for m := range markers {
			if markers[m].on(s) && !slices.Contains(open, m) {
				b.WriteString(markers[m].marker)
				open = append(open, m)
			}
		}
	}
	for i, t := range runs {
		if t.Note {
			// with no space after it, before the spaces in front of
			// it, so that the text without it keeps them ("Rain[[note]]
			// falls.")
			if next := RunsText(runs[i+1:]); next == "" || unicode.IsSpace([]rune(next)[0]) {
				b.WriteString(space + t.Text)
				space = ""
			} else {
				b.WriteString(t.Text)
			}
			continue
		}
		text := t.Text
		for _, m := range escape {
			text = strings.ReplaceAll(text, string(m), `\`+string(m))
		}
		content := strings.TrimSpace(text)
		if content == "" {
			space += text
			bare = bare || !t.Underline
			continue
		}
		lead := text[:strings.Index(text, content)]
		trail := text[len(lead)+len(content):]
		space += lead
		bare = bare || lead != "" && !t.Underline
		change(t.Span)
		b.WriteString(content)
		space, bare = trail, trail != "" && !t.Underline
	}
	change(Span{})
	b.WriteString(space)
	return b.String()
}

// RunsText is the text of the first of runs that is not a note.
func RunsText(runs []Run) string {
	for _, t := range runs {
		if !t.Note && t.Text != "" {
			return t.Text
		}
	}
	return ""
}

// same reports whether spans read from Fountain are the runs' text and
// styles (notes aside; spaces have no style that shows).
func same(spans []Span, runs []Run) bool {
	var want []Span
	for _, t := range runs {
		if !t.Note {
			want = append(want, t.Span)
		}
	}
	return slices.Equal(styledRunes(spans), styledRunes(want))
}

// styledRune is a character and its style, none for a space.
type styledRune struct {
	r                       rune
	bold, italic, underline bool
}

func styledRunes(spans []Span) []styledRune {
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


// SameRuns reports whether spans read from Fountain are the runs' text
// and styles (notes aside; spaces have no style that shows).
func SameRuns(spans []Span, runs []Run) bool { return same(spans, runs) }

// SameStyled reports whether two texts' spans are the same characters in
// the same styles (spaces have no style that shows).
func SameStyled(a, b []Span) bool { return slices.Equal(styledRunes(a), styledRunes(b)) }
