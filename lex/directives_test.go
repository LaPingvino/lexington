package lex

import "testing"

func TestStripDirectives(t *testing.T) {
	for in, want := range map[string]string{
		"{{HEADER: %# | %p}}":                         "",
		"{{%m}}":                                      "",
		"An action. {{%m: Marker D}}":                 "An action.",
		"Inside {{%m: C}} the line.":                  "Inside the line.",
		"{{%m: A marker}}Action after.":               "Action after.",
		"{{REVISION: #ee2d36}}Red.{{/REVISION}} Not.": "Red. Not.",
		"Old {{revision}}new{{ / REVISION }} old.":    "Old new old.",
		"{{SERIES}} and {{TIMESTAMP}} print":          "{{SERIES}} and {{TIMESTAMP}} print",
		"{{unknown}} stays":                           "{{unknown}} stays",
		"no braces":                                   "no braces",
	} {
		if got := StripDirectives(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// Lines of nothing but Highland's directives are not printed, nor a blank
// line around them; inline ones go from the text; the screenplay keeps
// them; {{SERIES}} prints its number.
func TestPrintableDirectives(t *testing.T) {
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. KITCHEN - DAY"}, {Type: TypeEmpty},
		{Type: TypeAction, Contents: "{{HEADER: %p}}"}, {Type: TypeEmpty},
		{Type: TypeAction, Contents: "{{FOOTER: K}} [[a note]]"}, {Type: TypeEmpty},
		{Type: TypeSpeaker, Contents: "BOB"}, {Type: TypeDialog, Contents: "Dialogue {{%m: x}} here."}, {Type: TypeEmpty},
		{Type: TypeAction, Contents: "{{SERIES}}"},
	}
	var got []string
	for _, l := range Printable(s, nil) {
		got = append(got, l.Type+"="+l.Contents)
	}
	want := []string{"scene=INT. KITCHEN - DAY", "empty=", "speaker=BOB", "dialog=Dialogue here.", "empty=", "action=1"}
	if len(got) != len(want) {
		t.Fatalf("%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, got[i], want[i])
		}
	}
	if s[2].Contents != "{{HEADER: %p}}" {
		t.Error("the screenplay changed")
	}
}
