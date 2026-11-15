package layout

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Colors from notes only when the rules ask for them; what is no color
// gives none.
func TestColors(t *testing.T) {
	s := fountain.Parse([]string{"INT"}, strings.NewReader("INT. A [[COLOR BLUE]]\n\nRain.\n\nINT. B [[color whatever]]\n\nINT. C [[#3366FF]]\n"))
	colors := func(set rules.Set, field func(Colors) string) (out []string) {
		for _, p := range Paragraphs(s, set) {
			if p.Type != "" {
				out = append(out, field(p.Colors))
			}
		}
		return out
	}
	hex := func(st lex.ColorStyle, c string) string { rgb, _ := lex.ColorFor(st, c); return lex.HexOf(rgb) }
	color := func(c Colors) string { return c.Color + c.Marker + c.Background + c.Bar }
	if got := strings.Join(colors(nil, color), ","); got != ",,," {
		t.Errorf("by default: %q", got)
	}
	// a heading in each style; the bar also down the lines of its scene
	for st, field := range map[lex.ColorStyle]func(Colors) string{
		lex.ColorText:       func(c Colors) string { return c.Color },
		lex.ColorMarker:     func(c Colors) string { return c.Marker },
		lex.ColorBackground: func(c Colors) string { return c.Background },
	} {
		want := hex(st, "blue") + ",,," + hex(st, "#3366ff")
		if got := strings.Join(colors(rules.Default.WithColorStyle(st), field), ","); got != want {
			t.Errorf("%s: %q, want %q", st, got, want)
		}
	}
	bar := hex(lex.ColorLine, "blue")
	if got := strings.Join(colors(rules.Default.WithColors(), func(c Colors) string { return c.Bar }), ","); got != bar+","+bar+",,"+hex(lex.ColorLine, "#3366ff") {
		t.Errorf("line: %q", got)
	}
	if Margin(rules.Default.WithColors()) != Margin(rules.Default) {
		t.Error("the switch changes the margin")
	}
}
