package rules

import (
	"slices"
	"testing"
)

func TestPartEncodeDecodeWith(t *testing.T) {
	c := DefaultConf()
	c.Languages["xx"] = Language{Scenes: []string{"ENA", "EKA"}, Matchup: []string{"ENA.", "EKA.", "", "", ""}}
	c.Elements["mine"] = Set{"action": {Style: "b"}}
	part := c.Part([]string{"xx", "en"}, []string{"mine", "nothere"})
	if len(part.Languages) != 2 || len(part.Elements) != 1 || part.Standard != "en" {
		t.Fatalf("part: %+v", part)
	}
	text, err := part.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	if !Same(part, back) {
		t.Fatalf("round trip differs:\n%s", text)
	}
	machine := DefaultConf()
	machine.Languages["en"] = Language{Scenes: []string{"INT"}}
	got := machine.With(back)
	if !slices.Equal(got.Scenes("xx"), []string{"ENA", "EKA"}) || len(got.Scenes("en")) != len(DefaultConf().Scenes("en")) {
		t.Fatalf("with: %v", got.Languages)
	}
	// every language travels in full
	if l := back.Languages["xx"]; len(l.Cues) == 0 || len(l.TransitionEnds) == 0 || l.Direction == "" {
		t.Fatalf("xx not in full: %+v", l)
	}
	if len(machine.Scenes("en")) != 1 || machine.Knows("xx") {
		t.Fatal("With changed its receiver")
	}
	if _, ok := got.Elements["mine"]; !ok {
		t.Fatal("the set did not come over")
	}
}

func TestItemsOfRenamed(t *testing.T) {
	c := TOMLConf{Languages: map[string]Language{"xx": {Scenes: []string{"A"}, Matchup: []string{"A."}}}, Elements: map[string]Set{"s": {}}, Standard: "en"}
	items := c.Items()
	want := []Item{{"language", "xx"}, {"elements", "s"}, {"standard", ""}}
	if !slices.Equal(items, want) {
		t.Fatalf("items %v", items)
	}
	one, ok := c.Of(Item{"language", "xx"})
	if !ok || one.Standard != "" || one.Elements != nil {
		t.Fatalf("of: %+v", one)
	}
	r, err := Renamed(one, Item{"language", "xx"}, "xx2")
	if err != nil || r.Scenes("xx2")[0] != "A" || r.Language("xx2").Matchup[0] != "A." {
		t.Fatalf("renamed: %+v %v", r, err)
	}
	if _, err := Renamed(one, Item{"language", "xx"}, "a.b"); err == nil {
		t.Fatal("a dotted name passed")
	}
}

func TestWithPresetFiles(t *testing.T) {
	files := map[string][]byte{
		"child": []byte("Name = \"Child\"\nBase = \"parent\"\n"),
		"parent": []byte("Name = \"Parent\"\nBase = \"stageplay\"\nPage = \"a5\"\n" +
			"[Elements.action]\nStyle = \"b\"\n"),
	}
	list, err := WithPresetFiles(Presets(), files)
	if err != nil {
		t.Fatal(err)
	}
	var child Preset
	for _, p := range list {
		if p.Key == "child" {
			child = p
		}
	}
	if child.Name != "Child" || child.Page != "a5" || child.Elements["action"].Style != "b" {
		t.Fatalf("child: %+v", child)
	}
}

// TestEveryLanguageInFull: the default languages written and read back
// are the same, each with every field.
func TestEveryLanguageInFull(t *testing.T) {
	text, err := DefaultConf().Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	for name, l := range DefaultLanguages() {
		got := back.Languages[name]
		if got.Scenes == nil || got.Cues == nil || got.Extensions == nil || got.TransitionEnds == nil ||
			got.Transitions == nil || got.TitleKeys == nil || got.Headings == nil || got.Direction == "" || got.Repeat == 0 {
			t.Errorf("%s not in full after a round trip: %+v", name, got)
		}
		if !Same(TOMLConf{Languages: map[string]Language{name: l}}, TOMLConf{Languages: map[string]Language{name: back.Language(name)}}) {
			t.Errorf("%s differs after a round trip", name)
		}
	}
}
