// Package internal provides shared constants for the Lexington application
package internal

// Common file format constants
const (
	FormatFountain = "fountain"
	FormatLex      = "lex"
	FormatPDF      = "pdf"
	FormatHTML     = "html"
	FormatLaTeX    = "latex"
	FormatFDX      = "fdx"
)

// Common element type constants (in addition to those in lex package)
const (
	ElementAction = "action"
	ElementTitle  = "title"
	ElementScene  = "scene"
)

// Common configuration constants
const (
	ConfigStart     = "start"
	ConfigTitlePage = "titlepage"
	ConfigDefault   = "default"
)

// Common messages
const (
	MsgNestedDualDialogue = "Nested dual dialogue block detected. Fountain specification allows only " +
		"one dual dialogue block at a time."
	MsgTooManyDualSpeakers    = "More than two speakers in a dual dialogue block. Fountain specifies only two."
	MsgMisplacedParenthetical = "Parenthetical without a preceding speaker or dialogue line. " +
		"This might be interpreted as action."
	MsgEmptySpeaker = "Empty speaker name detected."
)
