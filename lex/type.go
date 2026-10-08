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
)

type Screenplay []Line

type Line struct {
	Type     ElementType
	Contents Content
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
// "INT. HOUSE - DAY #1A#" is "INT. HOUSE - DAY" and "1A".
func SceneNumber(heading string) (string, string) {
	if m := sceneNumber.FindStringSubmatchIndex(heading); m != nil {
		return heading[:m[0]], heading[m[2]:m[3]]
	}
	return heading, ""
}

// SectionText is a section's title, without the #s of its level.
func SectionText(section string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(section), "#"))
}
