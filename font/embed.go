// Package font provides embedded font files for PDF generation
package font

import (
	_ "embed"
)

// Font name constants
const (
	CourierBadiName  = "CourierBadi"
	CourierPrimeName = "CourierPrime"
	CourierName      = "Courier"
)

// Embedded font files using Go 1.16+ embed directive
//
//go:embed CourierBadi-Regular.ttf
var CourierBadiRegular []byte

//go:embed CourierBadi-Italic.ttf
var CourierBadiItalic []byte

// GetFont returns the font data for the specified font name and style
func GetFont(name, style string) []byte {
	switch name {
	case CourierBadiName, CourierPrimeName, CourierName, "":
		switch style {
		case "I", "i":
			return CourierBadiItalic
		case "B", "b", "BI", "bi":
			return CourierBadiRegular // Using regular for bold since we only have regular and italic
		default:
			return CourierBadiRegular
		}
	default:
		// Default to CourierBadi Regular for unknown fonts
		return CourierBadiRegular
	}
}

// FontExists checks if a font with the given name exists
func FontExists(name string) bool {
	switch name {
	case CourierBadiName, CourierPrimeName, CourierName, "":
		return true
	default:
		return false
	}
}

// GetFontName returns the standard font name for PDF usage
func GetFontName(configFont string) string {
	switch configFont {
	case CourierBadiName, CourierPrimeName, CourierName, "":
		return CourierPrimeName
	default:
		return CourierPrimeName // Default fallback
	}
}
