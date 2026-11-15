package lex

import (
	"fmt"
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
