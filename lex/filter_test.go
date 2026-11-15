package lex

import (
	"strings"
	"testing"
)

// script is a small script as the Fountain parser gives it (with blank
// lines): a title page, two acts, scenes with tags and colors.
var script = Screenplay{
	{Type: TypeTitlePage}, {Type: "Title", Contents: "Tags"}, {Type: TypeNewPage},
	{Type: TypeAction, Contents: "FADE IN:"}, {Type: TypeEmpty},
	{Type: TypeSection, Contents: "# Act One"}, {Type: TypeEmpty},
	{Type: TypeSection, Contents: "## London [[London]]"}, {Type: TypeEmpty},
	{Type: TypeScene, Contents: "INT. PUB - NIGHT #1#"}, {Type: TypeEmpty},
	{Type: TypeSpeaker, Contents: "ANNA"}, {Type: TypeDialog, Contents: "Another round?"}, {Type: TypeEmpty},
	{Type: TypeSpeaker, Contents: "BOB (V.O.)"}, {Type: TypeParen, Contents: "(tired)"}, {Type: TypeDialog, Contents: "No, I have to go home now."}, {Type: TypeEmpty},
	{Type: TypeScene, Contents: "EXT. BRIDGE - NIGHT #2# [[COLOR RED]]"}, {Type: TypeEmpty},
	{Type: TypeAction, Contents: "Rain."}, {Type: TypeEmpty},
	{Type: TypeSection, Contents: "## New York"}, {Type: TypeEmpty},
	{Type: TypeScene, Contents: "INT. LOFT - DAY #3# [[New York]] [[red]]"}, {Type: TypeEmpty},
	{Type: TypeSpeaker, Contents: "CAROL"}, {Type: TypeDialog, Contents: "Who's there?"}, {Type: TypeEmpty},
	{Type: TypeDualOpen}, {Type: TypeSpeaker, Contents: "ANNA"}, {Type: TypeDialog, Contents: "Me!"}, {Type: TypeEmpty},
	{Type: TypeDualNext}, {Type: TypeSpeaker, Contents: "Bob (CONT'D) ^"}, {Type: TypeDialog, Contents: "Us!"}, {Type: TypeEmpty},
	{Type: TypeDualClose},
	{Type: TypeSection, Contents: "# Act Two"}, {Type: TypeEmpty},
	{Type: TypeScene, Contents: "INT. PUB - LATER #4# [[london]]"}, {Type: TypeEmpty},
	{Type: TypeAction, Contents: "Empty."}, {Type: TypeEmpty},
}

func showLines(s Screenplay) string {
	var b []string
	for _, l := range s {
		if l.Type != TypeEmpty {
			b = append(b, string(l.Type)+"="+l.Contents)
		}
	}
	return strings.Join(b, "\n")
}

func want(t *testing.T, got Screenplay, lines ...string) {
	t.Helper()
	if g := showLines(got); g != strings.Join(lines, "\n") {
		t.Errorf("got\n%s\nwant\n%s", g, strings.Join(lines, "\n"))
	}
}

var title = []string{"titlepage=", "Title=Tags", "newpage="}

func TestFilterTags(t *testing.T) {
	// a section's tag keeps all of it, a scene's that scene, in any case;
	// the sections above them stay as headings
	want(t, FilterTags(script, "LONDON"), append(title,
		"section=# Act One", "section=## London [[London]]",
		"scene=INT. PUB - NIGHT #1#", "speaker=ANNA", "dialog=Another round?",
		"speaker=BOB (V.O.)", "paren=(tired)", "dialog=No, I have to go home now.",
		"scene=EXT. BRIDGE - NIGHT #2# [[COLOR RED]]", "action=Rain.",
		"section=# Act Two", "scene=INT. PUB - LATER #4# [[london]]", "action=Empty.")...)
	// a color is the tag of [[color red]] and [[red]]; tags are a union
	got := FilterTags(script, "red", "nowhere")
	want(t, got, append(title,
		"section=# Act One", "section=## London [[London]]",
		"scene=EXT. BRIDGE - NIGHT #2# [[COLOR RED]]", "action=Rain.",
		"section=## New York",
		"scene=INT. LOFT - DAY #3# [[New York]] [[red]]", "speaker=CAROL", "dialog=Who's there?",
		"dualspeaker_open=", "speaker=ANNA", "dialog=Me!", "dualspeaker_next=", "speaker=Bob (CONT'D) ^", "dialog=Us!",
		"dualspeaker_close=")...)
	// a tag is the whole note
	want(t, FilterTags(script, "New"), title...)
}

func TestSides(t *testing.T) {
	want(t, Sides(script, "bob", SidesFull, 0), append(title,
		"section=# Act One", "section=## London [[London]]",
		"scene=INT. PUB - NIGHT #1#", "speaker=ANNA", "dialog=Another round?",
		"speaker=BOB (V.O.)", "paren=(tired)", "dialog=No, I have to go home now.",
		"section=## New York",
		"scene=INT. LOFT - DAY #3# [[New York]] [[red]]", "speaker=CAROL", "dialog=Who's there?",
		"dualspeaker_open=", "speaker=ANNA", "dialog=Me!", "dualspeaker_next=", "speaker=Bob (CONT'D) ^", "dialog=Us!",
		"dualspeaker_close=")...)
	want(t, Sides(script, "BOB", SidesCue, 0), append(title,
		"scene=INT. PUB - NIGHT #1#", "speaker=ANNA", "dialog=Another round?",
		"speaker=BOB (V.O.)", "paren=(tired)", "dialog=No, I have to go home now.",
		"scene=INT. LOFT - DAY #3# [[New York]] [[red]]", "speaker=ANNA", "dialog=Me!",
		"speaker=Bob (CONT'D) ^", "dialog=Us!")...)
	want(t, Sides(script, "Anna", SidesCue, 3), append(title,
		"scene=INT. PUB - NIGHT #1#", "speaker=ANNA", "dialog=Another round?",
		"scene=INT. LOFT - DAY #3# [[New York]] [[red]]", "speaker=CAROL", "dialog=Who's there?", "speaker=ANNA", "dialog=Me!")...)
	want(t, Sides(script, "bob", SidesCue, 3), append(title,
		"scene=INT. PUB - NIGHT #1#", "speaker=ANNA", "dialog=Another round?",
		"speaker=BOB (V.O.)", "paren=(tired)", "dialog=No, I have to go home now.",
		"scene=INT. LOFT - DAY #3# [[New York]] [[red]]", "speaker=ANNA", "dialog=Me!",
		"speaker=Bob (CONT'D) ^", "dialog=Us!")...)
	want(t, Sides(script, "bob", SidesIsolated, 0), append(title,
		"scene=INT. PUB - NIGHT #1#", "speaker=BOB (V.O.)", "paren=(tired)", "dialog=No, I have to go home now.",
		"scene=INT. LOFT - DAY #3# [[New York]] [[red]]", "speaker=Bob (CONT'D) ^", "dialog=Us!")...)
	want(t, Sides(script, "nobody", SidesIsolated, 0), title...)
}

func TestCueWords(t *testing.T) {
	sp := Screenplay{{Type: TypeSpeaker, Contents: "BOB"}, {Type: TypeParen, Contents: "(tired)"},
		{Type: TypeDialog, Contents: "No, I have to go"}, {Type: TypeDialog, Contents: "**home** now [[note]]."}}
	if got := showLines(cue(sp, 3)); got != "speaker=BOB\ndialog=...go home now." {
		t.Errorf("%q", got)
	}
}

func TestSpeakerAndModes(t *testing.T) {
	for in, w := range map[string]string{"EDWARD (V.O.)": "EDWARD", "edward  (cont'd)": "EDWARD", "Bob ^": "BOB",
		"**WILL** [[note]]": "WILL", "MRS. SMITH (O.S.) (CONT'D)": "MRS. SMITH"} {
		if got := Speaker(in); got != w {
			t.Errorf("Speaker(%q) = %q", in, got)
		}
	}
	if _, err := ParseSidesMode("half"); err == nil {
		t.Error("no error for a mode that is none")
	}
	if m, _ := ParseSidesMode("Cue"); m != SidesCue {
		t.Error(m)
	}
}
