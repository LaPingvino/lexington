package lex

import (
	"strings"
)

// The other programs' formats (Fade In's Open Screenplay Format, Trelby,
// KIT Scenarist) keep a script as paragraphs, each of a type, with what
// is not printed (notes, a scene's synopsis) on the paragraph. Paragraphs
// is a screenplay as such paragraphs, for their writers; a Builder lays
// out paragraphs read from them as Fountain's parser lays out a script,
// for their readers.

// Dual is a paragraph's place in dual dialogue.
type Dual int

const (
	NotDual    Dual = iota
	DualFirst       // the first speaker of dual dialogue
	DualSecond      // the second
)

// Paragraph is one of a screenplay's paragraphs, as the formats that keep
// a script as paragraphs have it.
type Paragraph struct {
	// Type is the line's: scene, action, center, speaker, paren, dialog,
	// trans, lyrics; section (Level its depth) and synopse for the
	// outline's that are not on a heading; boneyard.
	Type ElementType
	// Text is the text with Fountain's emphasis, without its notes (and a
	// heading's number, a section's #s).
	Text string
	// Notes are the line's notes' texts, without their brackets; Before
	// those of the lines of notes alone before it.
	Notes, Before []string
	// Number is a scene heading's number (#1A#), "" for none.
	Number string
	// Level is a section's depth (# is 1).
	Level int
	// Synopses are the synopses after a scene heading or a section.
	Synopses []string
	// NewPage: a page break (===) is before the paragraph.
	NewPage bool
	// Dual is the paragraph's place in dual dialogue, on its speaker.
	Dual Dual
	// Revision is the line's revision set (Line.Revision).
	Revision string
	// Sources are the Fountain lines (Line.Source) of the rows the
	// paragraph was, one a line of its Text when they are rows of their
	// own (a speech's lines), for the tags kept by them (LineTag).
	Sources []int
}

// TitleField is a title page's field: its key and value, a value over
// several lines one field (Contact: Me / Here is "Me\nHere").
type TitleField struct {
	Key, Value string
}

// Paragraphs splits a screenplay into its title page and its paragraphs.
func Paragraphs(s Screenplay) (title []TitleField, out []Paragraph) {
	i := 0
	if len(s) > 0 && s[0].Type == TypeTitlePage {
		for i = 1; i < len(s) && s[i].Type != TypeNewPage; i++ {
			l := s[i]
			if l.Type == TypeMetaSection || l.Type == TypeEmpty {
				continue
			}
			if n := len(title); n > 0 && title[n-1].Key == string(l.Type) {
				title[n-1].Value += "\n" + l.Contents
				continue
			}
			title = append(title, TitleField{string(l.Type), l.Contents})
		}
		i++ // the title page's newpage
	}
	var notes []string // of lines of notes, for the next paragraph
	newPage, dual := false, NotDual
	heading := -1 // the paragraph synopses go on, -1 none
	for ; i < len(s); i++ {
		l := s[i]
		switch l.Type {
		case TypeEmpty, TypeTitlePage, TypeMetaSection:
			continue
		case TypeNewPage:
			newPage = true
			continue
		case TypeDualOpen:
			dual = DualFirst
			continue
		case TypeDualNext:
			dual = DualSecond
			continue
		case TypeDualClose:
			dual = NotDual
			continue
		case TypeNote:
			notes = append(notes, Notes(l.Contents)...)
			continue
		case TypeSynopsis:
			if heading >= 0 {
				out[heading].Synopses = append(out[heading].Synopses, l.Contents)
				continue
			}
		}
		if n := len(out); (l.Type == TypeDialog || l.Type == TypeLyrics) && n > 0 && out[n-1].Type == l.Type &&
			i > 0 && s[i-1].Type == l.Type && len(notes) == 0 && !newPage {
			// the speech's next line: one paragraph, its lines apart
			out[n-1].Text += "\n" + strings.TrimSpace(StripNotes(l.Contents))
			out[n-1].Notes = append(out[n-1].Notes, Notes(l.Contents)...)
			out[n-1].Sources = append(out[n-1].Sources, l.Source)
			continue
		}
		p := Paragraph{Type: l.Type, Notes: Notes(l.Contents), Before: notes, NewPage: newPage, Revision: l.Revision,
			Sources: []int{l.Source}}
		notes, newPage = nil, false
		p.Text = strings.TrimSpace(StripNotes(l.Contents))
		switch l.Type {
		case TypeScene:
			p.Text, p.Number = SceneNumber(l.Contents)
		case TypeSection:
			p.Level = len(p.Text) - len(strings.TrimLeft(p.Text, "#"))
			p.Text = strings.TrimSpace(p.Text[p.Level:])
		case TypeSpeaker:
			p.Dual = dual
			dual = NotDual // on the first speaker of each side only
		case TypeSynopsis, TypeBoneyard:
			p.Text, p.Notes = l.Contents, nil
		}
		if l.Type == TypeScene || l.Type == TypeSection {
			heading = len(out)
		} else if l.Type != TypeSynopsis {
			heading = -1
		}
		out = append(out, p)
	}
	if len(notes) > 0 { // after all else
		out = append(out, Paragraph{Type: TypeAction, Before: notes})
	}
	return title, out
}

// Builder lays out a script read as paragraphs as Fountain's parser lays
// it out: a blank row after each block (a paragraph and the speech under
// a speaker are one), dual dialogue between its markers, notes at the end
// of their line.
type Builder struct {
	out  Screenplay
	dual Dual // the side of dual dialogue the script is on
}

// TitlePage puts the title page's fields first (none: no title page).
func (b *Builder) TitlePage(fields []TitleField) {
	var rows Screenplay
	for _, f := range fields {
		for _, v := range strings.Split(f.Value, "\n") {
			if v = strings.TrimSpace(v); v != "" {
				rows = append(rows, Line{Type: ElementType(f.Key), Contents: v})
			}
		}
	}
	if len(rows) == 0 {
		return
	}
	title := append(Screenplay{{Type: TypeTitlePage}}, rows...)
	b.out = append(append(title, Line{Type: TypeNewPage}), b.out...)
}

// Add adds a paragraph. A speaker's Dual begins a side of dual dialogue
// (DualFirst, DualSecond); dual dialogue ends with the second side's
// speech.
func (b *Builder) Add(p Paragraph) {
	inSpeech := p.Type == TypeParen || p.Type == TypeDialog || (p.Type == TypeLyrics && b.inSpeech())
	secondSide := p.Type == TypeSpeaker && b.dual == DualFirst && p.Dual != DualFirst
	if !inSpeech && !secondSide {
		b.DualClose()
	}
	if p.NewPage {
		b.blank()
		b.out = append(b.out, Line{Type: TypeNewPage})
	}
	for _, n := range p.Before {
		if n = strings.TrimSpace(n); n != "" {
			b.blank()
			b.out = append(b.out, Line{Type: TypeNote, Contents: NoteText(n)})
		}
	}
	if !inSpeech {
		b.blank()
	}
	switch {
	case p.Type == TypeSpeaker && p.Dual == DualFirst:
		b.out = append(b.out, Line{Type: TypeDualOpen})
		b.dual = DualFirst
	case secondSide:
		b.out = append(b.out, Line{Type: TypeDualNext})
		b.dual = DualSecond
	}
	text := p.Text
	switch p.Type {
	case TypeScene:
		if p.Number != "" {
			text += " #" + p.Number + "#"
		}
	case TypeSection:
		if strings.TrimSpace(text) != "" { // a folder without a name is none
			text = strings.Repeat("#", max(p.Level, 1)) + " " + text
		}
	}
	for _, n := range p.Notes {
		if n = strings.TrimSpace(n); n != "" {
			text = strings.TrimSpace(text + " " + NoteText(n))
		}
	}
	t := p.Type
	if strings.TrimSpace(p.Text) == "" && len(p.Notes) > 0 {
		t = TypeNote // a paragraph of notes alone
	}
	if t == TypeDialog && strings.TrimSpace(text) == "" && b.inSpeech() {
		b.out = append(b.out, Line{Type: TypeDialog, Revision: p.Revision}) // a blank line in a speech
	}
	if strings.TrimSpace(text) != "" {
		if t == TypeDialog || t == TypeLyrics {
			// a line a row, as Fountain's parser has a speech
			lines := strings.Split(text, "\n")
			for len(lines) > 1 && strings.TrimSpace(lines[len(lines)-1]) == "" {
				lines = lines[:len(lines)-1]
			}
			for _, l := range lines {
				b.out = append(b.out, Line{Type: t, Contents: strings.TrimRight(l, " \t"), Revision: p.Revision})
			}
		} else {
			b.out = append(b.out, Line{Type: t, Contents: text, Revision: p.Revision})
		}
	}
	for _, s := range p.Synopses {
		if s = strings.TrimSpace(s); s != "" {
			b.blank()
			b.out = append(b.out, Line{Type: TypeSynopsis, Contents: s})
		}
	}
}

// NoteLineBefore tells whether a note on p (a format's note on a
// paragraph: OSF's, Final Draft's) can be a line of notes before it
// (Before), where Fountain has a note of a paragraph of its own. Not in a
// speech, which a line of notes would end, nor on dual dialogue's second
// speaker, which a line would part from the first: those keep it in the
// paragraph (Notes).
func NoteLineBefore(p Paragraph) bool {
	switch p.Type {
	case TypeParen, TypeDialog, TypeLyrics:
		return false
	case TypeSpeaker:
		return p.Dual != DualSecond
	}
	return strings.TrimSpace(p.Text) != ""
}

// Len is the number of rows built so far: the rows of a paragraph Add
// adds are those between Len before and after it.
func (b *Builder) Len() int { return len(b.out) }

// Rows are the rows built so far (not to be changed).
func (b *Builder) Rows() Screenplay { return b.out }

// DualClose ends dual dialogue, if it is open.
func (b *Builder) DualClose() {
	if b.dual == NotDual {
		return
	}
	b.blank()
	b.out = append(b.out, Line{Type: TypeDualClose})
	b.dual = NotDual
}

// Screenplay is the script built.
func (b *Builder) Screenplay() Screenplay {
	b.DualClose()
	b.blank()
	return b.out
}

// blank ends the block before, with a blank row.
func (b *Builder) blank() {
	if n := len(b.out); n == 0 {
		return
	}
	switch b.out[len(b.out)-1].Type {
	case TypeEmpty, TypeNewPage, TypeDualOpen, TypeDualNext, TypeDualClose:
		return
	}
	b.out = append(b.out, Line{Type: TypeEmpty})
}

// inSpeech tells whether the script so far ends in a speech.
func (b *Builder) inSpeech() bool {
	for i := len(b.out) - 1; i >= 0; i-- {
		switch b.out[i].Type {
		case TypeSpeaker, TypeDialog, TypeParen, TypeLyrics:
			return true
		case TypeDualOpen, TypeDualNext:
			continue
		}
		return false
	}
	return false
}

// NoteText is a Fountain note of a text: [[text]], a blank line in it
// two spaces (so that the note goes on), "]]" in it broken.
func NoteText(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, l := range lines {
		l = strings.ReplaceAll(l, "]]", "] ]")
		if strings.TrimSpace(l) == "" {
			l = "  "
		}
		lines[i] = l
	}
	return "[[" + strings.Join(lines, "\n") + "]]"
}
