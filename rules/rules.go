// The rules package of Lexington provides the tools around configuration of how a screenplay should look. The default should work but can be adjusted for a personal touch..
package rules

import (
	"fmt"
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
	KeyAction      ConfigKey = "action"
	KeySpeaker     ConfigKey = "speaker"
	KeyDialog      ConfigKey = "dialog"
	KeyScene       ConfigKey = "scene"
	KeyParen       ConfigKey = "paren"
	KeyTrans       ConfigKey = "trans"
	KeyNote        ConfigKey = "note"
	KeyAllCaps     ConfigKey = "allcaps"
	KeyEmpty       ConfigKey = "empty"
	KeyDualSpeaker ConfigKey = "dualspeaker"
	KeyDualDialog  ConfigKey = "dualdialog"
	KeyDualParen   ConfigKey = "dualparen"
	KeyTitle       ConfigKey = "title"
	KeyMeta        ConfigKey = "meta"
	KeyCenter      ConfigKey = "center"
	KeyLyrics      ConfigKey = "lyrics"
)

// String returns the string representation of the key
func (k ConfigKey) String() string {
	return string(k)
}

// IsValid checks if the key is a valid configuration key
func (k ConfigKey) IsValid() bool {
	switch k {
	case KeyAction, KeySpeaker, KeyDialog, KeyScene, KeyParen, KeyTrans,
		KeyNote, KeyAllCaps, KeyEmpty, KeyDualSpeaker, KeyDualDialog,
		KeyDualParen, KeyTitle, KeyMeta, KeyCenter, KeyLyrics:
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
	"note": {
		Left:  1.5,
		Right: 1,
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
