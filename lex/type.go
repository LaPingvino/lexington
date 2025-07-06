// The lex format is basically a parse tree for screenplays, which enables quick debugging.
package lex

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
