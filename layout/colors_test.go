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

// A blank line at a block's edge, or a page's, has none of its colors:
// only blank lines between two of its lines that print, on one page.
func TestBlankEdgesUncolored(t *testing.T) {
	script := "# Part [[green]]\n\n\nINT. A [[blue]]\n\nOne.\n\nTwo.\n\n===\n\nThree.\n\n\n\nINT. B\n\nFour.\n\nINT. C [[green]]\n\nFive.\n"
	s := fountain.Parse([]string{"INT"}, strings.NewReader(script))
	for _, set := range []rules.Set{rules.Default.WithColors(), rules.Default.WithColorScheme(lex.ColorScheme{Style: lex.ColorBackground, Block: true})} {
		ps := Paragraphs(s, set)
		colored := func(p Paragraph) bool { return p.Colors != (Colors{}) }
		for i, p := range ps {
			if p.Type != "" || p.PageBreak || !colored(p) {
				continue
			}
			prev, next := i-1, i+1
			for prev >= 0 && ps[prev].Type == "" && !ps[prev].PageBreak {
				prev--
			}
			for next < len(ps) && ps[next].Type == "" && !ps[next].PageBreak {
				next++
			}
			if prev < 0 || next >= len(ps) || ps[prev].PageBreak || ps[next].PageBreak || !colored(ps[prev]) || !colored(ps[next]) {
				t.Errorf("%v: blank paragraph %d colored at an edge", set.ColorScheme(), i)
			}
		}
		// and between One and Two, it is
		for i, p := range ps {
			if p.Text() == "One." && !colored(ps[i+1]) {
				t.Errorf("%v: the blank line within the block has no color", set.ColorScheme())
			}
		}
	}
}
