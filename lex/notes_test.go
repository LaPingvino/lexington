package lex

import (
	"bytes"
	"reflect"
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
		"INT. HOUSE - DAY [[#zzzzzz]]":               "",
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
	got := Printable(s, false)
	want := Screenplay{
		{Type: TypeScene, Contents: "INT. HOUSE - DAY"},
		{Type: TypeEmpty},
		{Type: TypeAction, Contents: "Action with inside."},
		{Type: TypeEmpty},
		{Type: TypeAction, Contents: "More."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Printable:\n got %q\nwant %q", got, want)
	}
	if !reflect.DeepEqual(s, orig) {
		t.Error("Printable changed its screenplay")
	}
	kept := Printable(s, true)
	if len(kept) != 8 || kept[4] != (Line{Type: TypeNote, Contents: "A standalone note"}) ||
		kept[6] != (Line{Type: TypeNote, Contents: "a note in a .lex file"}) {
		t.Errorf("Printable keeping notes: %q", kept)
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
		t.Errorf("parsed back: %q", got)
	}
}
