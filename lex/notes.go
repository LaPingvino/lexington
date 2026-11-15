package lex

import (
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

// NoteColor is the color the notes on a scene heading, section or
// synopsis give it, exactly as Beat (ContinuousFountainParser's
// colorForHeading, checked 2026-11-15) reads them: each note in lower case,
// not trimmed; one starting with "color " gives the rest of it ("[[COLOR
// BLUE]]" is "blue"), one that is a color's name (red, blue, green, pink,
// magenta, gray, purple, cyan, teal, yellow, orange, brown) or seven
// characters starting with "#" ("[[#ff0000]]") gives itself. The last
// color note wins; other notes do not count. "" when there is none. Beat
// itself writes "[[red]]" at the end of the heading.
func NoteColor(s string) string {
	color := ""
	for _, n := range Notes(s) {
		n = strings.ToLower(n)
		switch {
		case strings.HasPrefix(n, "color "):
			color = n[len("color "):]
		case noteColors[n], len(n) == 7 && n[0] == '#':
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
// with the notes' text. Scene headings and sections keep the color their
// notes give them in Color, or else the color of the section they are in
// (the deepest one with a color): colors on sections, which are not
// printed, color their scenes. Block numbers the block of a color a
// heading begins or is in (see ColorBlocks). The screenplay itself is
// not changed.
func Printable(s Screenplay, keepNotes bool) Screenplay {
	out := make(Screenplay, 0, len(s))
	var sections []Line // the sections a line is in, with their colors
	blocks := 0         // the color blocks so far (more, if s is printable already)
	for _, l := range s {
		blocks = max(blocks, l.Block)
	}
	for i, l := range s {
		if IsNoteLine(l) {
			if keepNotes {
				if HasNotes(l.Contents) { // (or printable already)
					l.Contents = strings.Join(Notes(l.Contents), " ")
				}
				out = append(out, Line{Type: TypeNote, Contents: l.Contents})
				continue
			}
			// a blank line before it and after it: one of them goes
			if len(out) > 0 && out[len(out)-1].Type == TypeEmpty && i+1 < len(s) && s[i+1].Type == TypeEmpty {
				out = out[:len(out)-1]
			}
			continue
		}
		if l.Type == TypeScene || l.Type == TypeSection {
			if c := NoteColor(l.Contents); c != "" || (l.Color != "" && l.Block == 0) {
				if c != "" {
					l.Color = c
				}
				blocks++
				l.Block = blocks // a block of its own
			}
			if l.Type == TypeSection {
				lv := sectionLevel(l)
				for len(sections) > 0 && sectionLevel(sections[len(sections)-1]) >= lv {
					sections = sections[:len(sections)-1]
				}
			}
			for k := len(sections) - 1; k >= 0 && l.Color == ""; k-- {
				l.Color, l.Block = sections[k].Color, sections[k].Block // the deepest section's with one
			}
			if l.Type == TypeSection {
				sections = append(sections, l)
			}
		}
		l.Contents = StripNotes(l.Contents)
		out = append(out, l)
	}
	return out
}

// IsNoteLine reports whether a line is nothing but notes: a TypeNote
// line, or script text (a .lex file's "action: [[...]]") with nothing
// else in it.
func IsNoteLine(l Line) bool {
	if l.Type == TypeNote {
		return true
	}
	stripped := StripNotes(l.Contents)
	return stripped != l.Contents && strings.TrimSpace(stripped) == "" && printsText(l.Type)
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

// ColorBlocks are, for each line of a printable screenplay (Printable),
// the color block it is in: the number of its heading's Block (0 for
// none). A block is a scene with a color of its own, or a section with
// one whole (its scenes without one inherit it; a scene with its own is
// a block of its own within it). Where one block follows another, the
// blank lines between them are in none: a gap, so that a boundary never
// shows by its hue alone, also between two blocks of the same color.
func ColorBlocks(s Screenplay) []int {
	out := make([]int, len(s))
	block, last := 0, -1 // last: the line before, not blank
	for i, l := range s {
		if l.Type == TypeScene || l.Type == TypeSection {
			if l.Block != 0 && last >= 0 && out[last] != 0 && out[last] != l.Block {
				for k := last + 1; k < i; k++ {
					out[k] = 0
				}
			}
			block = l.Block
		}
		out[i] = block
		if l.Type != TypeEmpty {
			last = i
		}
	}
	return out
}
