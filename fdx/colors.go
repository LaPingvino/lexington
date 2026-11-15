package fdx

import (
	"fmt"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Final Draft keeps a scene's color in its SceneProperties' Color, 16 bits
// a channel: #RRRRGGGGBBBB. fdColors are the colors of Final Draft's own
// palette, by the names Beat reads them as (Beat's FDXElement.m,
// colorNameFor16bitHex, github.com/lmparppei/Beat).
var fdColors = map[string]string{
	"pink": "#E2E29898DDDD", "red": "#EBEB62627B7B", "orange": "#EFEFA4A46262", "yellow": "#E5E5CBCB6C6C",
	"olive": "#929290900000", "green": "#8F8FC3C36A6A", "mint": "#8888CACAB8B8", "blue": "#6363A7A7EFEF",
	"violet": "#9A9AAEAEDBDB", "purple": "#AFAF9393E8E8", "brown": "#B2B27C7C7373", "gray": "#C0C0C0C0C0C0",
}

// fdColor is a color as lex.NoteColor gives it as Final Draft's: its
// palette's for the names it has, else the color 16 bits a channel (as
// Beat writes them); "" for one that is no color.
func fdColor(color string) string {
	if c, ok := fdColors[strings.ToLower(color)]; ok {
		return c
	}
	rgb, ok := lex.ColorRGB(color)
	if !ok {
		return ""
	}
	return fmt.Sprintf("#%02X%02X%02X%02X%02X%02X", rgb[0], rgb[0], rgb[1], rgb[1], rgb[2], rgb[2])
}

// colorName is a Final Draft color as a Fountain note's: the name for a
// color of its palette (or Beat's), else #rrggbb; "" for none (black, the
// color of a scene without one, or no color at all).
func colorName(fd string) string {
	fd = strings.ToUpper(strings.TrimSpace(fd))
	if len(fd) != 13 || fd[0] != '#' || fd == "#000000000000" {
		return ""
	}
	for name, c := range fdColors {
		if c == fd {
			return name
		}
	}
	var r, g, b uint16
	if n, err := fmt.Sscanf(fd[1:], "%04X%04X%04X", &r, &g, &b); n != 3 || err != nil {
		return ""
	}
	hex := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	for _, name := range []string{"red", "blue", "green", "pink", "magenta", "gray", "purple", "cyan", "teal",
		"yellow", "orange", "brown"} { // Beat's names, as it writes them
		if lex.ColorHex(name) == hex {
			return name
		}
	}
	return hex
}
