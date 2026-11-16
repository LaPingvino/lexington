package lex

import (
	"fmt"
	"math"
	"strings"
)

// beatColors are the colors Beat gives its color names, as RGB (Beat's
// BeatColors.m, github.com/lmparppei/Beat), without its user interface's.
var beatColors = map[string][3]uint8{
	"red": {239, 0, 73}, "blue": {0, 129, 239}, "green": {0, 223, 121}, "pink": {254, 131, 223},
	"magenta": {236, 0, 140}, "gray": {128, 128, 128}, "grey": {128, 128, 128}, "purple": {181, 32, 218},
	"yellow": {251, 200, 45}, "goldenrod": {215, 148, 15}, "rose": {236, 184, 152}, "buff": {118, 100, 86},
	"cherry": {236, 90, 150}, "cyan": {7, 189, 235}, "teal": {12, 224, 227}, "orange": {255, 161, 13},
	"brown": {169, 106, 7}, "lightgray": {220, 220, 220}, "darkgray": {170, 170, 170},
	"verydarkgray": {100, 100, 100}, "mint": {72, 231, 211}, "violet": {116, 62, 230}, "olive": {77, 147, 44},
}

// ColorRGB is a color as NoteColor gives it (a name or #rrggbb) in RGB,
// with Beat's values for the names; ok is false for anything else.
func ColorRGB(color string) (rgb [3]uint8, ok bool) {
	color = strings.ToLower(color)
	if c, ok := beatColors[color]; ok {
		return c, true
	}
	if len(color) == 7 && color[0] == '#' {
		var r, g, b uint8
		if n, err := fmt.Sscanf(color[1:], "%02x%02x%02x", &r, &g, &b); n == 3 && err == nil {
			return [3]uint8{r, g, b}, true
		}
	}
	return rgb, false
}

// ColorHex is a color as NoteColor gives it as #rrggbb (Beat's values for
// the names), "" for one that is not a color.
func ColorHex(color string) string {
	c, ok := ColorRGB(color)
	if !ok {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
}

// ColorStyle is how a color is shown.
type ColorStyle string

const (
	// ColorLine is a bar in the left margin down the scene (or section).
	ColorLine ColorStyle = "line"
	// ColorMarker is a highlighter behind the heading's text.
	ColorMarker ColorStyle = "marker"
	// ColorBackground is a band behind the heading, across the text's width.
	ColorBackground ColorStyle = "background"
	// ColorText is the heading's text in the color (with +block, every
	// line's of its block).
	ColorText ColorStyle = "text"
)

// ColorTint is no style of its own: the light wash a line style can put
// under its block (ColorScheme.Tint).
const ColorTint ColorStyle = "tint"

// ColorScheme is how colors show: a style; for marker and background,
// over the heading only or the whole block (the scene or section, up to
// the next heading or section); for line, a light tint under the block
// besides the bar.
type ColorScheme struct {
	Style ColorStyle
	Block bool // marker, background, text: the whole block, not the heading
	Tint  bool // line: a tint under the block
	// Patterns: line, the bar in its color's pattern (BarPattern)
	Patterns bool
}

// ParseColorScheme reads a scheme as String writes it: a style, then
// "+block" (marker, background, text) or "+tint" and "+patterns" (line):
// "background+block", "line+tint+patterns"; "" is line.
func ParseColorScheme(text string) (ColorScheme, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(text)), "+")
	style, err := ParseColorStyle(parts[0])
	if err != nil {
		return ColorScheme{}, err
	}
	sch := ColorScheme{Style: style}
	for _, p := range parts[1:] {
		switch {
		case p == "block" && (style == ColorMarker || style == ColorBackground || style == ColorText):
			sch.Block = true
		case p == "tint" && style == ColorLine:
			sch.Tint = true
		case p == "patterns" && style == ColorLine:
			sch.Patterns = true
		default:
			return ColorScheme{}, fmt.Errorf("no color scheme %q: a marker, background or text can have +block, a line +tint and +patterns", text)
		}
	}
	return sch, nil
}

// String is the scheme as ParseColorScheme reads it.
func (c ColorScheme) String() string {
	s := string(c.Style)
	if s == "" {
		s = string(ColorLine)
	}
	if c.Block {
		s += "+block"
	}
	if c.Tint {
		s += "+tint"
	}
	if c.Patterns {
		s += "+patterns"
	}
	return s
}

// ParseColorStyle reads a style by its name; "" is line.
func ParseColorStyle(name string) (ColorStyle, error) {
	switch st := ColorStyle(strings.ToLower(strings.TrimSpace(name))); st {
	case "":
		return ColorLine, nil
	case ColorLine, ColorMarker, ColorBackground, ColorText:
		return st, nil
	}
	return "", fmt.Errorf("no color style %q: choose line, marker, background or text", name)
}

// The contrasts ColorFor keeps (WCAG 2): black text on a marker or
// background at least 7:1 (AAA), colored text on white at least 4.5:1
// (AA), a bar against white at least 3:1 (non-text).
const (
	ContrastFill = 7.0
	ContrastText = 4.5
	ContrastBar  = 3.0
)

// ColorFor is the RGB a color (as NoteColor gives it) is drawn with in a
// style, the same in every format: for a marker and a background a tint,
// for ColorTint (the wash under a line's block) a lighter one still,
// mixed with white (a marker less) until black text on it has
// ContrastFill; for text the color, darkened (its hue kept) until it has
// ContrastText on white; for a line the color, darkened until it has
// ContrastBar on white. ok is false for what is no color.
func ColorFor(style ColorStyle, color string) (rgb [3]uint8, ok bool) {
	c, ok := ColorRGB(color)
	if !ok {
		return rgb, false
	}
	black, white := [3]uint8{}, [3]uint8{255, 255, 255}
	switch style {
	case ColorMarker, ColorBackground, ColorTint:
		t := 0.55 // white in a marker
		if style != ColorMarker {
			t = 0.75
		}
		for ; t < 1 && Contrast(mix(c, white, t), black) < ContrastFill; t += 0.01 {
		}
		if style == ColorTint { // lighter than a background
			t = max(t+0.08, 0.88)
		}
		return mix(c, white, min(t, 1)), true
	}
	need := ContrastBar
	if style == ColorText {
		need = ContrastText
	}
	t := 0.0
	for ; t < 1 && Contrast(mix(c, black, t), white) < need; t += 0.01 {
	}
	return mix(c, black, min(t, 1)), true
}

// mix is a with t of b in it.
func mix(a, b [3]uint8, t float64) [3]uint8 {
	var out [3]uint8
	for i := range a {
		out[i] = uint8(math.Round(float64(a[i])*(1-t) + float64(b[i])*t))
	}
	return out
}

// Contrast is the WCAG contrast ratio of two colors, from 1 to 21.
func Contrast(a, b [3]uint8) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// luminance is a color's relative luminance (WCAG 2, sRGB).
func luminance(c [3]uint8) float64 {
	var l [3]float64
	for i, v := range c {
		x := float64(v) / 255
		if x <= 0.04045 {
			l[i] = x / 12.92
		} else {
			l[i] = math.Pow((x+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
}

// HexOf is an RGB color as #rrggbb.
func HexOf(c [3]uint8) string { return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]) }
