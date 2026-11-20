// The lex format is basically a parse tree for screenplays, which enables quick debugging.
package lex

import (
	"regexp"
	"strings"
)

// Type aliases for better readability
type (
	ElementType = string
	Content     = string
)

// Common element types
const (
	TypeScene     ElementType = "scene"
	TypeAction    ElementType = "action"
	TypeSpeaker   ElementType = "speaker"
	TypeDialog    ElementType = "dialog"
	TypeParen     ElementType = "paren"
	TypeTrans     ElementType = "trans"
	TypeEmpty     ElementType = "empty"
	TypeDualOpen  ElementType = "dualspeaker_open"
	TypeDualNext  ElementType = "dualspeaker_next"
	TypeDualClose ElementType = "dualspeaker_close"
	TypeTitle     ElementType = "title"
	TypeTitlePage ElementType = "titlepage"
	TypeNewPage   ElementType = "newpage"
	TypeCenter    ElementType = "center"
	TypeLyrics    ElementType = "lyrics"
	TypeSection   ElementType = "section"
	TypeSynopsis  ElementType = "synopse"
)

type Screenplay []Line

type Line struct {
	Type     ElementType
	Contents Content
	// Color is the color the notes of a scene heading or section gave it
	// (NoteColor), set by Printable, which takes the notes out.
	Color string
	// Block is the color block a heading begins or is in (Printable,
	// ColorBlocks): the same for a section and its scenes without a color
	// of their own, another for a scene with one; 0 for none.
	Block int
	// Revision is the revision set the line was last changed in (a
	// RevisionSet's Name; "" none): printed with its mark in the right
	// margin, its page with the set's header (Revisions).
	Revision string
	// Source is the line of the Fountain text the row was read from,
	// counted from 1 (fountain.ParseLines; 0 not known).
	Source int
}

// IsDialogueElement returns true if the line is part of dialogue
func (l Line) IsDialogueElement() bool {
	return l.Type == TypeSpeaker || l.Type == TypeDialog || l.Type == TypeParen
}

// IsDualDialogueMarker returns true if the line is a dual dialogue marker
func (l Line) IsDualDialogueMarker() bool {
	return l.Type == TypeDualOpen || l.Type == TypeDualNext || l.Type == TypeDualClose
}

// IsEmpty returns true if the line has no content or is an empty type
func (l Line) IsEmpty() bool {
	return l.Type == TypeEmpty || l.Contents == ""
}

var sceneNumber = regexp.MustCompile(`\s*#([^#\s]+)#\s*$`)

// SceneNumber splits Fountain's scene number off a scene heading:
// "INT. HOUSE - DAY #1A#" is "INT. HOUSE - DAY" and "1A". Notes are left
// out (StripNotes), wherever they are: "INT. HOUSE - DAY #1A# [[red]]" is
// the same.
func SceneNumber(heading string) (string, string) {
	heading = StripNotes(heading)
	if m := sceneNumber.FindStringSubmatchIndex(heading); m != nil {
		return heading[:m[0]], heading[m[2]:m[3]]
	}
	return heading, ""
}

// SectionText is a section's title, without the #s of its level or its
// notes.
func SectionText(section string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(StripNotes(section)), "#"))
}
