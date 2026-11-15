package lex

import (
	"fmt"
	"regexp"
	"strings"
)

// Filters make a script of part of a script: the scenes and sections
// with a tag (FilterTags), a character's sides (Sides). The title page
// stays; the lines kept are as they were (their scene numbers, their
// notes, which are not printed); the sections above a scene that is kept
// are kept too, as headings to find one's way by.

// titleEnd is where the script after its title page starts.
func titleEnd(s Screenplay) int {
	if len(s) == 0 || s[0].Type != TypeTitlePage {
		return 0
	}
	for i, l := range s {
		if l.Type == TypeNewPage {
			return i + 1
		}
	}
	return len(s)
}

// sectionLevel is a section's level: its number of #s.
func sectionLevel(l Line) int {
	t := strings.TrimSpace(l.Contents)
	return len(t) - len(strings.TrimLeft(t, "#"))
}

// filterScenes keeps the title page, the sections that keepSection
// keeps with all under them (their subsections and scenes), the scenes
// (a heading and what follows it up to the next heading or section) that
// keepScene keeps, given the index of their heading, and the lines
// before the first heading or section if keepScene(-1). Above a scene or
// section that is kept, the sections it is in are kept as headings.
func filterScenes(s Screenplay, keepSection func(Line) bool, keepScene func(int) bool) Screenplay {
	start := titleEnd(s)
	out := append(Screenplay(nil), s[:start]...)
	var open []int // the sections the line is in, outermost first
	written := map[int]bool{}
	context := func() {
		for _, i := range open {
			if !written[i] {
				written[i] = true
				out = append(out, s[i], Line{Type: TypeEmpty})
			}
		}
	}
	keepLevel := 0 // in a section that is kept: its level
	keeping := keepScene(-1)
	for i := start; i < len(s); i++ {
		l := s[i]
		switch l.Type {
		case TypeSection:
			lv := sectionLevel(l)
			if keepLevel > 0 && lv <= keepLevel {
				keepLevel = 0
			}
			keeping = false
			for len(open) > 0 && sectionLevel(s[open[len(open)-1]]) >= lv {
				open = open[:len(open)-1]
			}
			if keepLevel == 0 && keepSection(l) {
				context()
				keepLevel = lv
				written[i] = true
			}
			open = append(open, i)
		case TypeScene:
			keeping = keepLevel > 0 || keepScene(i)
			if keeping && keepLevel == 0 {
				context()
			}
		}
		if keepLevel > 0 || keeping {
			out = append(out, l)
		}
	}
	return out
}

// FilterTags keeps the scenes and sections whose heading has a note that
// is one of the tags ([[London]], [[New York]]), and all under them: a
// section its subsections and scenes. Tags are the whole text of a note,
// in any case; a color is a tag of the notes that give it ([[color
// red]] has the tag red, as [[red]]). Several tags keep what has any of
// them. See also the filters' common rules (above).
func FilterTags(s Screenplay, tags ...string) Screenplay {
	want := map[string]bool{}
	for _, t := range tags {
		want[strings.ToLower(strings.TrimSpace(t))] = true
	}
	has := func(l Line) bool {
		for _, n := range Notes(l.Contents) {
			if want[strings.ToLower(strings.TrimSpace(n))] {
				return true
			}
		}
		c := NoteColor(l.Contents)
		return c != "" && want[c]
	}
	return filterScenes(s, has, func(i int) bool { return i >= 0 && has(s[i]) })
}

// SidesMode is what Sides keeps of a character's scenes.
type SidesMode string

const (
	// SidesFull keeps every scene the character speaks in, whole.
	SidesFull SidesMode = "full"
	// SidesCue keeps the character's speeches, each after its cue: the
	// speech before it, with its speaker (or its last words).
	SidesCue SidesMode = "cue"
	// SidesIsolated keeps only the character's speeches.
	SidesIsolated SidesMode = "isolated"
)

// ParseSidesMode reads a mode by its name: full, cue or isolated.
func ParseSidesMode(name string) (SidesMode, error) {
	switch m := SidesMode(strings.ToLower(strings.TrimSpace(name))); m {
	case SidesFull, SidesCue, SidesIsolated:
		return m, nil
	}
	return "", fmt.Errorf("no sides %q: choose full, cue or isolated", name)
}

var extension = regexp.MustCompile(`\([^)]*\)`)

// Speaker is a character's name as a cue gives it, for comparing: in
// capitals, without extensions ((V.O.), (CONT'D)), notes, emphasis and
// the ^ of dual dialogue.
func Speaker(cue string) string {
	name := extension.ReplaceAllString(Unstyled(StripNotes(cue)), "")
	name = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), "^"))
	return strings.ToUpper(strings.Join(strings.Fields(name), " "))
}

// speech is a speaker's line and the dialogue after it: s[start:end].
type speech struct{ start, end int }

// speeches are the speeches of a script, in order.
func speeches(s Screenplay) []speech {
	var out []speech
	for i := 0; i < len(s); i++ {
		if s[i].Type != TypeSpeaker {
			continue
		}
		j := i + 1
		for j < len(s) && (s[j].Type == TypeDialog || s[j].Type == TypeParen || s[j].Type == TypeLyrics || s[j].Type == TypeNote) {
			j++
		}
		out = append(out, speech{i, j})
		i = j - 1
	}
	return out
}

// Sides are a character's sides, for an actor: in mode full, the scenes
// they speak in; cue, their speeches, each after its cue (the speech
// before it in the scene by someone else, whole or, with cueWords > 0,
// its last cueWords words after "..."); isolated, their speeches only.
// Cue and isolated keep the headings of the scenes they speak in. The
// character is a name as Speaker reads it; their speeches in dual
// dialogue count. See also the filters' common rules (above).
func Sides(s Screenplay, character string, mode SidesMode, cueWords int) Screenplay {
	who := Speaker(character)
	all := speeches(s)
	scene := make([]int, len(s)) // the heading of each line's scene, -1 before the first
	h := -1
	for i, l := range s {
		if l.Type == TypeScene {
			h = i
		}
		scene[i] = h
	}
	speaks := map[int]bool{}
	for _, sp := range all {
		if Speaker(s[sp.start].Contents) == who {
			speaks[scene[sp.start]] = true
		}
	}
	if mode == SidesFull {
		return filterScenes(s, func(Line) bool { return false }, func(i int) bool { return speaks[i] })
	}

	start := titleEnd(s)
	out := append(Screenplay(nil), s[:start]...)
	heading := -2
	for k, sp := range all {
		if sp.start < start || Speaker(s[sp.start].Contents) != who {
			continue
		}
		if h := scene[sp.start]; h != heading {
			heading = h
			if h >= 0 {
				out = append(out, s[h], Line{Type: TypeEmpty})
			}
		}
		if mode == SidesCue && k > 0 {
			if prev := all[k-1]; scene[prev.start] == scene[sp.start] && prev.start >= start &&
				Speaker(s[prev.start].Contents) != who {
				out = append(out, cue(s[prev.start:prev.end], cueWords)...)
				out = append(out, Line{Type: TypeEmpty})
			}
		}
		out = append(out, s[sp.start:sp.end]...)
		out = append(out, Line{Type: TypeEmpty})
	}
	return out
}

// cue is a speech as a cue: whole, or its speaker and its last words.
func cue(speech Screenplay, words int) Screenplay {
	if words <= 0 {
		return append(Screenplay(nil), speech...)
	}
	var text []string
	for _, l := range speech[1:] {
		if l.Type == TypeDialog || l.Type == TypeLyrics {
			// without emphasis: a cut may not leave half a marker
			text = append(text, strings.Fields(EscapeEmphasis(Unstyled(StripNotes(l.Contents))))...)
		}
	}
	last := strings.Join(text, " ")
	if len(text) > words {
		last = "..." + strings.Join(text[len(text)-words:], " ")
	}
	return Screenplay{speech[0], {Type: TypeDialog, Contents: last}}
}
