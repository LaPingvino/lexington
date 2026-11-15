package lex

import (
	"regexp"
	"strings"
	"unicode"
)

// TypeNote is a line that is nothing but Fountain notes ([[...]]): kept
// for writing the source again, never printed unless a set of rules asks
// for notes (see Printable).
const TypeNote ElementType = "note"

// note is a Fountain note in s: s[start:end] is "[[...]]", s[start+2:end-2]
// its text.
type note struct{ start, end int }

// notes finds the notes in s, in order: from a "[[" to the first "]]"
// after it. A "[[" without a "]]" after it is no note, nor is anything
// after it.
func notes(s string) []note {
	var out []note
	for i := 0; ; {
		open := strings.Index(s[i:], "[[")
		if open < 0 {
			return out
		}
		open += i
		end := strings.Index(s[open+2:], "]]")
		if end < 0 {
			return out
		}
		end += open + 4
		out = append(out, note{open, end})
		i = end
	}
}

// Notes are the texts of the notes in s, without their brackets.
func Notes(s string) []string {
	var out []string
	for _, n := range notes(s) {
		out = append(out, s[n.start+2:n.end-2])
	}
	return out
}

// HasNotes reports whether s has a note in it.
func HasNotes(s string) bool { return len(notes(s)) > 0 }

func isBlank(r rune) bool { return r != '\n' && unicode.IsSpace(r) }

// StripNotes takes Fountain's notes out of s, as they are never printed:
// each note with the white space in front of it, so that nothing is left
// dangling ("INT. HOUSE - DAY [[red]]" is "INT. HOUSE - DAY", "Action
// with [[a note]] inside." is "Action with inside."); a note at the start
// of a line goes with the white space after it. Notes may span lines. A
// "[[" that is never closed is left as it is, with all after it.
func StripNotes(s string) string {
	ns := notes(s)
	if len(ns) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, n := range ns {
		before := strings.TrimRightFunc(s[last:n.start], isBlank)
		b.WriteString(before)
		last = n.end
		atStart := b.Len() == 0 || strings.HasSuffix(b.String(), "\n")
		if atStart { // nothing before it on its line: drop the space after it
			for last < len(s) && isBlank(rune(s[last])) {
				last++
			}
		}
	}
	b.WriteString(s[last:])
	return b.String()
}

// noteColors are the color names Beat takes from a note.
var noteColors = map[string]bool{
	"red": true, "blue": true, "green": true, "pink": true, "magenta": true, "gray": true,
	"purple": true, "cyan": true, "teal": true, "yellow": true, "orange": true, "brown": true,
}

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// NoteColor is the color the notes on a scene heading, section or
// synopsis give it, as Beat (ContinuousFountainParser's colorForHeading)
// reads them, in lower case: "[[color red]]" (any case) is the text after
// "color ", "[[red]]" a color's name (red, blue, green, pink, magenta,
// gray, purple, cyan, teal, yellow, orange, brown) and "[[#ff0000]]" a
// hex color. The last color note wins; other notes do not count. "" when
// there is none. Beat itself writes "[[red]]" at the end of the heading.
func NoteColor(s string) string {
	color := ""
	for _, n := range Notes(s) {
		n = strings.ToLower(strings.TrimSpace(n))
		switch {
		case strings.HasPrefix(n, "color "):
			if c := strings.TrimSpace(n[len("color "):]); c != "" {
				color = c
			}
		case noteColors[n], hexColor.MatchString(n):
			color = n
		}
	}
	return color
}

// Printable is a screenplay as it is printed: the notes taken out of
// every line's text (StripNotes). Lines that are only notes are left out,
// with one of the blank lines around them (so that "Action.", blank,
// note, blank, "More." prints as two paragraphs with one blank line
// between them), unless keepNotes: then they are kept, as TypeNote lines
// with the notes' text. The screenplay itself is not changed.
func Printable(s Screenplay, keepNotes bool) Screenplay {
	out := make(Screenplay, 0, len(s))
	for i, l := range s {
		stripped := StripNotes(l.Contents)
		if l.Type == TypeNote || (stripped != l.Contents && strings.TrimSpace(stripped) == "" && printsText(l.Type)) {
			if keepNotes {
				out = append(out, Line{Type: TypeNote, Contents: strings.Join(Notes(l.Contents), " ")})
				continue
			}
			// a blank line before it and after it: one of them goes
			if len(out) > 0 && out[len(out)-1].Type == TypeEmpty && i+1 < len(s) && s[i+1].Type == TypeEmpty {
				out = out[:len(out)-1]
			}
			continue
		}
		l.Contents = stripped
		out = append(out, l)
	}
	return out
}

// printsText reports whether a line of the type is the script's text,
// rather than the title page's or a marker.
func printsText(t ElementType) bool {
	switch t {
	case TypeScene, TypeAction, TypeSpeaker, TypeDialog, TypeParen, TypeTrans, TypeCenter, TypeLyrics,
		TypeSection, TypeSynopsis, "general":
		return true
	}
	return false
}
