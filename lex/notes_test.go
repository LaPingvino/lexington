package lex

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestStripNotes(t *testing.T) {
	for in, want := range map[string]string{
		"INT. HOUSE - DAY [[red]]":                   "INT. HOUSE - DAY",
		"INT. HOUSE - DAY [[COLOR BLUE]]":            "INT. HOUSE - DAY",
		"INT. HOUSE - DAY [[some remark]] [[green]]": "INT. HOUSE - DAY",
		"Action with [[a note]] inside.":             "Action with inside.",
		"Hello [[dialogue note]] there.":             "Hello there.",
		"DAN and JACK[[Or did we name them?]]. They": "DAN and JACK. They",
		"[[A standalone note]]":                      "",
		"[[first]] Action.":                          "Action.",
		"No notes here.":                             "No notes here.",
		"Unterminated [[note and the rest":           "Unterminated [[note and the rest",
		"A [[note]] and [[an open one":               "A and [[an open one",
		"Some reason.[[Needs work.\nOr coffee.\n  \nDefinitely coffee.]] He looks around.": "Some reason. He looks around.",
		"Text ]] without an opening": "Text ]] without an opening",
	} {
		if got := StripNotes(in); got != want {
			t.Errorf("StripNotes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNoteColor(t *testing.T) {
	for in, want := range map[string]string{
		"INT. HOUSE - DAY [[COLOR BLUE]]":            "blue",
		"INT. HOUSE - DAY [[color red]]":             "red",
		"EXT. STREET - NIGHT [[red]]":                "red",
		"EXT. STREET - NIGHT [[RED]]":                "red",
		"INT. X [[#3366ff]]":                         "#3366ff",
		"INT. X [[#3366FF]]":                         "#3366ff",
		"INT. HOUSE - DAY [[some remark]] [[green]]": "green",
		"INT. HOUSE - DAY [[green]] [[some remark]]": "green",
		"INT. HOUSE - DAY [[red]] [[blue]]":          "blue",
		"INT. HOUSE - DAY #12# [[blue]]":             "blue",
		"INT. HOUSE - DAY [[blue]] #12#":             "blue",
		"# War 6 [[red]]":                            "red",
		"= A synopsis [[blue]]":                      "blue",
		"INT. HOUSE - DAY [[a remark]]":              "",
		"INT. HOUSE - DAY [[#12345]]":                "",
		"INT. HOUSE - DAY [[#zzzzzz]]":               "#zzzzzz", // Beat: any seven characters after "#"
		"INT. HOUSE - DAY [[ red ]]":                 "",        // Beat doesn't trim
		"INT. HOUSE - DAY [[color  red]]":            " red",
		"INT. HOUSE - DAY [[reddish]]":               "",
		"INT. HOUSE - DAY":                           "",
		"INT. HOUSE - DAY [[red":                     "",
	} {
		if got := NoteColor(in); got != want {
			t.Errorf("NoteColor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSceneNumberWithNotes(t *testing.T) {
	for _, c := range [][3]string{
		{"INT. HOUSE - DAY #12# [[blue]]", "INT. HOUSE - DAY", "12"},
		{"INT. HOUSE - DAY [[blue]] #12#", "INT. HOUSE - DAY", "12"},
		{"INT. HOUSE - DAY [[COLOR BLUE]]", "INT. HOUSE - DAY", ""},
		{"EXT. STREET - NIGHT #12# [[red]]", "EXT. STREET - NIGHT", "12"},
	} {
		if h, n := SceneNumber(c[0]); h != c[1] || n != c[2] {
			t.Errorf("SceneNumber(%q) = %q, %q", c[0], h, n)
		}
	}
	for in, want := range map[string]string{
		"# War 6 [[red]]":       "War 6",
		"## Section [[green]]":  "Section",
		"# [[note]] Dominik 1 ": "Dominik 1",
	} {
		if got := SectionText(in); got != want {
			t.Errorf("SectionText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPrintable(t *testing.T) {
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. HOUSE - DAY [[COLOR BLUE]]"},
		{Type: TypeEmpty},
		{Type: TypeAction, Contents: "Action with [[a note]] inside."},
		{Type: TypeEmpty},
		{Type: TypeNote, Contents: "[[A standalone note]]"},
		{Type: TypeEmpty},
		{Type: TypeAction, Contents: "[[a note in a .lex file]]"},
		{Type: TypeAction, Contents: "More."},
	}
	orig := append(Screenplay(nil), s...)
	got := Printable(s, nil)
	want := Screenplay{
		{Type: TypeScene, Contents: "INT. HOUSE - DAY", Color: "blue", Block: 1},
		{Type: TypeEmpty},
		{Type: TypeAction, Contents: "Action with inside."},
		{Type: TypeEmpty},
		{Type: TypeAction, Contents: "More."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Printable:\n got %v\nwant %v", got, want)
	}
	if !reflect.DeepEqual(s, orig) {
		t.Error("Printable changed its screenplay")
	}
	kept := Printable(s, func(t ElementType) bool { return t == TypeBoneyard })
	if len(kept) != 8 || kept[4] != (Line{Type: TypeNote, Contents: "A standalone note"}) ||
		kept[6] != (Line{Type: TypeNote, Contents: "a note in a .lex file"}) {
		t.Errorf("Printable keeping notes: %v", kept)
	}
}

// Text over several lines (a Fountain note) goes to the lex format as
// "+: " lines and back, blank lines with two spaces as they are.
func TestLexContinuedLines(t *testing.T) {
	s := Screenplay{
		{Type: TypeAction, Contents: "Some reason.[[Needs work.\nOr coffee.\n  \nDefinitely coffee.]] He looks."},
		{Type: TypeNote, Contents: "[[one\ntwo]]"},
	}
	var b bytes.Buffer
	if err := (&LexWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	want := "action: Some reason.[[Needs work.\n+: Or coffee.\n+:   \n+: Definitely coffee.]] He looks.\nnote: [[one\n+: two]]\n"
	if b.String() != want {
		t.Errorf("lex:\n%s\nwant\n%s", b.String(), want)
	}
	if got := Parse(&b); !reflect.DeepEqual(got, s) {
		t.Errorf("parsed back: %v", got)
	}
}

func TestColorHex(t *testing.T) {
	for in, want := range map[string]string{
		"red": "#ef0049", "BLUE": "#0081ef", "#3366FF": "#3366ff", "goldenrod": "#d7940f",
		"#12345": "", "#zzzzzz": "", "whatever": "", "": "",
	} {
		if got := ColorHex(in); got != want {
			t.Errorf("ColorHex(%q) = %q, want %q", in, got, want)
		}
	}
}

// Scenes and sections without a color of their own take their section's;
// a deeper section's wins, the scene's own wins over both.
func TestInheritedColors(t *testing.T) {
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. BEFORE"},
		{Type: TypeSection, Contents: "# Act One [[red]]"},
		{Type: TypeScene, Contents: "INT. A"},
		{Type: TypeSection, Contents: "## Part [[blue]]"},
		{Type: TypeScene, Contents: "INT. B"},
		{Type: TypeScene, Contents: "INT. C [[green]]"},
		{Type: TypeSection, Contents: "## Other part"},
		{Type: TypeScene, Contents: "INT. D"},
		{Type: TypeSection, Contents: "# Act Two"},
		{Type: TypeScene, Contents: "INT. E"},
	}
	var got []string
	for _, l := range Printable(Printable(s, nil), nil) {
		got = append(got, l.Color)
	}
	if g := strings.Join(got, ","); g != ",red,red,blue,blue,green,red,red,," {
		t.Errorf("colors %s", g)
	}
}

// Every style keeps the text readable for every color: Beat's and hex
// extremes.
func TestColorForContrast(t *testing.T) {
	colors := []string{"red", "blue", "green", "pink", "magenta", "gray", "purple", "cyan", "teal", "yellow", "orange", "brown",
		"#ffff00", "#ffffff", "#000000", "#111111", "#00ffff"}
	black, white := [3]uint8{}, [3]uint8{255, 255, 255}
	for _, c := range colors {
		for style, check := range map[ColorStyle]func([3]uint8) float64{
			ColorMarker:     func(f [3]uint8) float64 { return Contrast(f, black) - ContrastFill },
			ColorBackground: func(f [3]uint8) float64 { return Contrast(f, black) - ContrastFill },
			ColorTint:       func(f [3]uint8) float64 { return Contrast(f, black) - ContrastFill },
			ColorText:       func(f [3]uint8) float64 { return Contrast(f, white) - ContrastText },
			ColorLine:       func(f [3]uint8) float64 { return Contrast(f, white) - ContrastBar },
		} {
			rgb, ok := ColorFor(style, c)
			if !ok || check(rgb) < 0 {
				t.Errorf("%s %s: %s, short of its contrast by %.2f", style, c, HexOf(rgb), -check(rgb))
			}
		}
	}
	if _, ok := ColorFor(ColorLine, "whatever"); ok {
		t.Error("a color of what is none")
	}
	// a background is lighter than a marker; a dark text color is kept
	m, _ := ColorFor(ColorMarker, "blue")
	b, _ := ColorFor(ColorBackground, "blue")
	if luminance(b) <= luminance(m) {
		t.Errorf("background %s not lighter than marker %s", HexOf(b), HexOf(m))
	}
	if c, _ := ColorFor(ColorText, "#111111"); HexOf(c) != "#111111" {
		t.Errorf("dark text changed: %s", HexOf(c))
	}
}

// The tint under a line's block is lighter than a background; schemes
// read as written, only the combinations that are some.
func TestColorTintAndSchemes(t *testing.T) {
	for _, c := range []string{"red", "blue", "yellow", "#000000", "#3366ff"} {
		b, _ := ColorFor(ColorBackground, c)
		w, _ := ColorFor(ColorTint, c)
		if luminance(w) <= luminance(b) {
			t.Errorf("%s: tint %s not lighter than background %s", c, HexOf(w), HexOf(b))
		}
	}
	for in, want := range map[string]string{"": "line", "line": "line", "Marker+Block": "marker+block",
		"background+block": "background+block", "line+tint": "line+tint", "text": "text", "text+block": "text+block"} {
		if sch, err := ParseColorScheme(in); err != nil || sch.String() != want {
			t.Errorf("ParseColorScheme(%q) = %v, %v", in, sch, err)
		}
	}
	for _, bad := range []string{"text+tint", "line+block", "marker+tint", "neon", "marker+all"} {
		if _, err := ParseColorScheme(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// Blocks: two scenes of the same color are two blocks, with a gap
// between them; a section is one block through its scenes without a
// color; a scene's own color is a block of its own in it.
func TestColorBlocks(t *testing.T) {
	s := Printable(Screenplay{
		{Type: TypeScene, Contents: "INT. A [[blue]]"}, {Type: TypeEmpty}, {Type: TypeAction, Contents: "a"}, {Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. B [[blue]]"}, {Type: TypeEmpty}, {Type: TypeAction, Contents: "b"}, {Type: TypeEmpty},
		{Type: TypeSection, Contents: "# Act [[red]]"}, {Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. C"}, {Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. D"}, {Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. E [[red]]"}, {Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. F"}, {Type: TypeEmpty},
		{Type: TypeSection, Contents: "# Act Two"}, {Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. G"},
	}, nil)
	var got []string
	for _, b := range ColorBlocks(s) {
		got = append(got, fmt.Sprint(b))
	}
	// A: 1, the gap, B: 2, the gap, the act: 3 through C and D, the gap,
	// E: 4, the gap, F: the act's again, 3; no gap to no color
	want := "1 1 1 0 2 2 2 0 3 3 3 3 3 0 4 0 3 3 0 0 0"
	if g := strings.Join(got, " "); g != want {
		t.Errorf("blocks %s\n  want %s", g, want)
	}
}

// Beat's twelve colors have twelve patterns, distinct in their stripes
// and rhythm (their shape, not their tone); other colors one by a hash,
// the same each time.
func TestBarPatterns(t *testing.T) {
	seen := map[string]string{}
	for _, c := range []string{"red", "blue", "green", "pink", "magenta", "gray", "purple", "cyan", "teal", "yellow", "orange", "brown"} {
		p := BarPattern(c)
		key := fmt.Sprint(p.Stripes)
		if other, ok := seen[key]; ok {
			t.Errorf("%s and %s have the same pattern", c, other)
		}
		seen[key] = c
	}
	if fmt.Sprint(BarPattern("#3366ff")) != fmt.Sprint(BarPattern("#3366FF")) || BarPattern("grey").Name != BarPattern("gray").Name {
		t.Error("a pattern that is not the same each time")
	}
	if sch, err := ParseColorScheme("line+tint+patterns"); err != nil || !sch.Patterns || sch.String() != "line+tint+patterns" {
		t.Errorf("%v %v", sch, err)
	}
	if _, err := ParseColorScheme("marker+patterns"); err == nil {
		t.Error("patterns on a marker")
	}
}

// Lines a format does not print (sections, synopses, the boneyard, notes)
// are as if they were not there: blank, hidden, blank is one blank line;
// a run of them the same; none at the top of the script or a page. A
// format that prints them keeps them and their blank lines.
func TestPrintableHidden(t *testing.T) {
	E, A := Line{Type: TypeEmpty}, func(s string) Line { return Line{Type: TypeAction, Contents: s} }
	sec, syn := Line{Type: TypeSection, Contents: "# Act"}, Line{Type: TypeSynopsis, Contents: "What happens."}
	bone, note := Line{Type: TypeBoneyard, Contents: "/* gone */"}, Line{Type: TypeNote, Contents: "[[a note]]"}
	hides := func(t ElementType) bool { return t != TypeAction && t != TypeScene }
	// shape is the printed lines: "a" for action, "_" blank, "|" a page
	shape := func(s Screenplay) string {
		var b strings.Builder
		for _, l := range s {
			switch {
			case l.Type == TypeEmpty:
				b.WriteString("_")
			case l.Type == TypeNewPage:
				b.WriteString("|")
			case hides(l.Type):
			default:
				b.WriteString(l.Contents)
			}
		}
		return b.String()
	}
	for _, c := range []struct {
		name string
		in   Screenplay
		want string
	}{
		{"section", Screenplay{A("a"), E, sec, E, A("b")}, "a_b"},
		{"synopsis", Screenplay{A("a"), E, syn, E, A("b")}, "a_b"},
		{"boneyard", Screenplay{A("a"), E, bone, E, A("b")}, "a_b"},
		{"note", Screenplay{A("a"), E, note, E, A("b")}, "a_b"},
		{"a run", Screenplay{A("a"), E, sec, syn, syn, note, E, A("b")}, "a_b"},
		{"a run with blanks", Screenplay{A("a"), E, sec, E, syn, E, bone, E, A("b")}, "a_b"},
		{"at the top", Screenplay{sec, E, syn, E, A("a")}, "a"},
		{"at a page's top", Screenplay{A("a"), {Type: TypeNewPage}, sec, E, A("b")}, "a|b"},
		{"a page's blank, then", Screenplay{A("a"), {Type: TypeNewPage}, E, sec, E, A("b")}, "a|_b"},
		{"the script's own double blank", Screenplay{A("a"), E, E, sec, E, A("b")}, "a__b"},
		{"no blank after", Screenplay{A("a"), E, syn, A("b")}, "a_b"},
	} {
		if got := shape(Printable(c.in, hides)); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// the hidden lines are kept for the writers (a section's color, a
	// bookmark); a format that prints sections keeps their blank lines
	in := Screenplay{A("a"), E, sec, E, A("b")}
	if got := Printable(in, hides); len(got) != 4 || got[2].Type != TypeSection {
		t.Errorf("the section is not kept: %v", got)
	}
	if got := Printable(in, nil); len(got) != 5 {
		t.Errorf("printing sections: %v", got)
	}
}

// TrimBlocks: a block's color only from its first line that prints to
// its last, page by page.
func TestTrimBlocks(t *testing.T) {
	// "t" prints, "_" does not, "|" a page break; with the blocks under it
	lines, blocks := "_t_t__|_t_|_t_t_", []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2}
	TrimBlocks(blocks, func(i int) bool { return lines[i] == 't' }, func(i int) bool { return lines[i] == '|' })
	if got, want := fmt.Sprint(blocks), fmt.Sprint([]int{0, 1, 1, 1, 0, 0, 0, 0, 1, 0, 0, 0, 2, 2, 2, 0}); got != want {
		t.Errorf("TrimBlocks: %s, want %s", got, want)
	}
}
