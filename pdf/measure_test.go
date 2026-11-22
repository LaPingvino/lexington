package pdf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// action is n paragraphs of action, a line each, with a blank line
// after each: 2n printed lines.
func action(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "Line %d.\n\n", i+1)
	}
	return b.String()
}

func parseScript(text string) lex.Screenplay {
	return fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(text))
}

// A Letter page holds 55 lines. A scene is measured from its heading to
// the next: heading and blank line (2) and 2n lines of action.
func TestMeasureEighths(t *testing.T) {
	cases := []struct {
		name    string
		script  string
		eighths []lex.Eighths
		pages   []int
	}{
		// 2+2·13 = 28 lines: 28/55 of a page = 4.07 eighths
		{"half a page", "INT. A - DAY\n\n" + action(13) + "INT. B - DAY\n\nEnd.\n", []lex.Eighths{4, 1}, []int{1, 1}},
		// 2+2·26 = 54 lines, and the next heading goes over (it keeps
		// with what follows): the page whole
		{"a page", "INT. A - DAY\n\n" + action(26) + "INT. B - DAY\n\n" + action(3), []lex.Eighths{8, 1}, []int{1, 2}},
		// 3 lines: 0.44 eighths, never less than one
		{"a short scene", "INT. A - DAY\n\nEnd.\n\nINT. B - DAY\n\n" + action(13), []lex.Eighths{1, 4}, []int{1, 1}},
		// 2+2·33 = 68 lines: 1 page and 13/55 = 9.9 eighths
		{"over a page", "INT. A - DAY\n\n" + action(33) + "INT. B - DAY\n\nEnd.\n", []lex.Eighths{10, 1}, []int{1, 2}},
		// a forced page break: the rest of the page is no one's (2+9
		// lines: 1.6 eighths)
		{"a forced break", "INT. A - DAY\n\n" + action(5) + "===\n\nINT. B - DAY\n\nEnd.\n", []lex.Eighths{2, 1}, []int{1, 2}},
		// a break within a scene: its text on both pages (2+9+10 lines)
		{"a break within", "INT. A - DAY\n\n" + action(5) + "===\n\n" + action(5) + "INT. B - DAY\n\nEnd.\n", []lex.Eighths{3, 1}, []int{1, 2}},
		// the title page is not page 1
		{"title page", "Title: T\n\nINT. A - DAY\n\n" + action(13) + "INT. B - DAY\n\nEnd.\n", []lex.Eighths{4, 1}, []int{1, 1}},
		// an omitted scene is a heading, and its boneyard is not printed
		{"omitted", "INT. A - DAY #1#\n\nGo.\n\n.OMITTED #2#\n\n/* Omitted 2\nINT. B - DAY #2#\n\n" + action(40) + "*/\n\nINT. C - DAY #3#\n\nEnd.\n",
			[]lex.Eighths{1, 1, 1}, []int{1, 1, 1}},
	}
	for _, c := range cases {
		got := Measure(parseScript(c.script), nil)
		if len(got) != len(c.eighths) {
			t.Errorf("%s: %d scenes, want %d", c.name, len(got), len(c.eighths))
			continue
		}
		for i, m := range got {
			if m.Eighths != c.eighths[i] || m.Page != c.pages[i] {
				t.Errorf("%s: scene %d is %s (%.3f pages) on page %d, want %s on page %d",
					c.name, i+1, m.Eighths, m.Pages, m.Page, c.eighths[i], c.pages[i])
			}
		}
	}
}

// A whole page of scenes adds up to a page before rounding.
func TestMeasureAddsUp(t *testing.T) {
	script := "INT. A - DAY\n\n" + action(9) + "INT. B - DAY\n\n" + action(7) + "INT. C - DAY\n\n" + action(8) +
		"INT. D - DAY\n\nEnd.\n"
	got := Measure(parseScript(script), nil)
	sum := got[0].Pages + got[1].Pages + got[2].Pages
	if sum < 0.999 || sum > 1.001 {
		t.Errorf("a page's scenes are %.3f pages: %+v", sum, got)
	}
	if got[3].Page != 2 {
		t.Errorf("D on page %d", got[3].Page)
	}
}

// On A4 a page holds more lines: the same scene is shorter.
func TestMeasureA4(t *testing.T) {
	s := parseScript("INT. A - DAY\n\n" + action(26) + "INT. B - DAY\n\nEnd.\n")
	letter := (&PDFWriter{}).Measure(s)[0]
	a4 := (&PDFWriter{Page: "a4"}).Measure(s)[0]
	if a4.Pages >= letter.Pages {
		t.Errorf("A4 %.3f pages, Letter %.3f", a4.Pages, letter.Pages)
	}
}
