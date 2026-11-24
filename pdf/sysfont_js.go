//go:build js

package pdf

// In the browser the PDF writer has no fonts of the computer's to read:
// a character its font lacks has none (fallback.go).

func systemGlyphs([]rune, bool, string) map[rune]sysGlyph { return nil }

func systemHas([]rune) map[rune]bool { return nil }
