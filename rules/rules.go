// The rules package of Lexington provides the tools around configuration of how a screenplay should look.
// The default should work but can be adjusted for a personal touch.
package rules

import (
	"fmt"

	"github.com/LaPingvino/lexington/lex"
)

type Format struct {
	Right   float64
	Left    float64
	Font    string
	Style   string
	Size    float64
	Hide    bool
	Align   string
	Prefix  string
	Postfix string
}

type Set map[string]Format

// ConfigKey represents valid configuration keys using type safety
type ConfigKey string

const (
	KeyAction  ConfigKey = "action"
	KeySpeaker ConfigKey = "speaker"
	KeyDialog  ConfigKey = "dialog"
	KeyScene   ConfigKey = "scene"
	KeyParen   ConfigKey = "paren"
	KeyTrans   ConfigKey = "trans"
	// KeyPrintNotes is the format of lines that are only Fountain notes
	// ([[...]]), which are printed only when a set has this rule (a
	// visible one). Notes inside a line are never printed. (The rule is
	// not called "note": that one was in Default, and so is in the
	// configuration files written with it, from before notes were read.)
	KeyPrintNotes ConfigKey = "printnotes"
	// KeyNoteColors, when a set has it (visible), prints scene headings
	// and sections in the color their notes give them ([[red]],
	// [[COLOR BLUE]], [[#3366ff]]: lex.NoteColor). Off by default. Its
	// Style is how: line (the default), marker, background or text.
	KeyNoteColors  ConfigKey = "notecolors"
	KeyAllCaps     ConfigKey = "allcaps"
	KeyEmpty       ConfigKey = "empty"
	KeyDualSpeaker ConfigKey = "dualspeaker"
	KeyDualDialog  ConfigKey = "dualdialog"
	KeyDualParen   ConfigKey = "dualparen"
	KeyTitle       ConfigKey = "title"
	KeyMeta        ConfigKey = "meta"
	KeyCenter      ConfigKey = "center"
	KeyLyrics      ConfigKey = "lyrics"
	KeySection     ConfigKey = "section"
)

// String returns the string representation of the key
func (k ConfigKey) String() string {
	return string(k)
}

// IsValid checks if the key is a valid configuration key
func (k ConfigKey) IsValid() bool {
	switch k {
	case KeyAction, KeySpeaker, KeyDialog, KeyScene, KeyParen, KeyTrans,
		KeyPrintNotes, KeyNoteColors, KeyAllCaps, KeyEmpty, KeyDualSpeaker, KeyDualDialog,
		KeyDualParen, KeyTitle, KeyMeta, KeyCenter, KeyLyrics, KeySection:
		return true
	default:
		return false
	}
}

// Get retrieves a format with fallback and validation
func (s Set) Get(action string) Format {
	return s.GetWithKey(ConfigKey(action))
}

// GetWithKey retrieves a format using a typed key
func (s Set) GetWithKey(key ConfigKey) Format {
	f, ok := s[key.String()]
	if !ok {
		// Fallback to action format with hide flag
		f = s.getDefaultFormat()
		f.Hide = true
	}

	// Apply defaults for missing values
	return s.applyDefaults(f)
}

// PrintsNotes reports whether the set prints lines that are only notes:
// whether it has a visible printnotes rule.
func (s Set) PrintsNotes() bool {
	f, ok := s[KeyPrintNotes.String()]
	return ok && !f.Hide
}

// Hides reports whether the set leaves the script's lines of a type out
// (lex.Printable): lines of notes unless it prints them (PrintsNotes),
// the boneyard, elements without a visible rule (sections and synopses,
// in most).
func (s Set) Hides(t lex.ElementType) bool {
	switch t {
	case lex.TypeNote:
		return !s.PrintsNotes()
	case lex.TypeBoneyard:
		return true
	}
	return s.Get(t).Hide
}

// ShowsColors reports whether the set prints headings and sections in
// their notes' colors: whether it has a visible notecolors rule.
func (s Set) ShowsColors() bool {
	f, ok := s[KeyNoteColors.String()]
	return ok && !f.Hide
}

// IsSwitch reports whether a key of a set switches something on rather
// than formats an element (notecolors): it has no margins to count.
func IsSwitch(key string) bool { return key == KeyNoteColors.String() }

// WithColors is a copy of the set that prints colors (ShowsColors), in
// the scheme it has (ColorScheme).
func (s Set) WithColors() Set {
	return s.WithColorScheme(s.ColorScheme())
}

// ColorScheme is how the set shows colors: its notecolors rule's Style,
// as lex.ParseColorScheme reads it (line, marker+block, line+tint, ...);
// line when it is none.
func (s Set) ColorScheme() lex.ColorScheme {
	sch, err := lex.ParseColorScheme(s[KeyNoteColors.String()].Style)
	if err != nil {
		return lex.ColorScheme{Style: lex.ColorLine}
	}
	return sch
}

// ColorStyle is the style of the set's ColorScheme.
func (s Set) ColorStyle() lex.ColorStyle { return s.ColorScheme().Style }

// WithColorScheme is a copy of the set that prints colors in a scheme.
func (s Set) WithColorScheme(sch lex.ColorScheme) Set {
	out := Set{}
	for k, f := range s {
		out[k] = f
	}
	out[KeyNoteColors.String()] = Format{Style: sch.String()}
	return out
}

// WithColorStyle is a copy of the set that prints colors in a style, over
// headings and without a tint.
func (s Set) WithColorStyle(style lex.ColorStyle) Set {
	return s.WithColorScheme(lex.ColorScheme{Style: style})
}

// GetSafe retrieves a format safely with error reporting
func (s Set) GetSafe(action string) (Format, error) {
	key := ConfigKey(action)
	if !key.IsValid() {
		return Format{}, fmt.Errorf("invalid configuration key: %s", action)
	}

	f, ok := s[action]
	if !ok {
		return s.applyDefaults(s.getDefaultFormat()), nil
	}

	return s.applyDefaults(f), nil
}

// getDefaultFormat returns the default action format
func (s Set) getDefaultFormat() Format {
	if f, ok := s[KeyAction.String()]; ok {
		return f
	}
	// Ultimate fallback
	return Format{
		Left:  1.5,
		Right: 1.0,
		Font:  "CourierPrime",
		Size:  12,
		Align: "L",
	}
}

// applyDefaults applies default values to missing format fields
func (s Set) applyDefaults(f Format) Format {
	if f.Font == "" {
		f.Font = "CourierPrime"
	}
	if f.Size == 0 {
		f.Size = 12
	}
	if f.Align == "" {
		f.Align = "L"
	}
	return f
}

// Validate ensures the Set contains all required keys with valid values
func (s Set) Validate() error {
	requiredKeys := []ConfigKey{
		KeyAction, KeySpeaker, KeyDialog, KeyScene,
	}

	for _, key := range requiredKeys {
		if _, ok := s[key.String()]; !ok {
			return fmt.Errorf("missing required configuration key: %s", key)
		}
	}

	// Validate format values
	for key, format := range s {
		if err := s.validateFormat(ConfigKey(key), format); err != nil {
			return fmt.Errorf("invalid format for key %s: %w", key, err)
		}
	}

	return nil
}

// validateFormat validates individual format values
func (s Set) validateFormat(key ConfigKey, f Format) error {
	if f.Size < 0 || f.Size > 72 {
		return fmt.Errorf("font size must be between 0 and 72, got %f", f.Size)
	}

	if f.Left < 0 || f.Right < 0 {
		return fmt.Errorf("margins cannot be negative")
	}

	validAligns := map[string]bool{"L": true, "R": true, "C": true, "": true}
	if !validAligns[f.Align] {
		return fmt.Errorf("invalid alignment %s, must be L, R, or C", f.Align)
	}

	return nil
}

var Default = Set{
	"action": {
		Left:  1.5,
		Right: 1,
	},
	"speaker": {
		Left:  3.7,
		Right: 1.5,
	},
	"dialog": {
		Left:  2.5,
		Right: 1.5,
	},
	"scene": {
		Left:  1.5,
		Right: 1,
		Style: "b",
	},
	"paren": {
		Left:  3.1,
		Right: 1.5,
	},
	"trans": {
		Left:  1.5,
		Right: 1,
		Align: "R",
	},
	"allcaps": {
		Left:  1.5,
		Right: 1,
	},
	"empty": {
		Left:  1.5,
		Right: 1,
	},
	"dualspeaker": {
		Left:  1.5,
		Right: 0.5,
	},
	"dualdialog": {
		Left:  1.0,
		Right: 0.5,
	},
	"dualparen": {
		Left:  1.3,
		Right: 0.5,
	},
	"title": {
		Left:  1.5,
		Right: 1,
		Align: "C",
	},
	"meta": {
		Left:  1.5,
		Right: 1,
	},
	"center": {
		Left:  1.5,
		Right: 1,
		Align: "C",
	},
	"lyrics": {
		Left:  2,
		Right: 2,
		Style: "i",
		Font:  "Helvetica",
	},
}
