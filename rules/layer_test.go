package rules

import (
	"slices"
	"testing"
)

func TestPartEncodeDecodeWith(t *testing.T) {
	c := DefaultConf()
	c.Scenes["xx"] = []string{"ENA", "EKA"}
	c.Matchups["xx"] = []string{"ENA.", "EKA.", "", "", ""}
	c.Elements["mine"] = Set{"action": {Style: "b"}}
	part := c.Part([]string{"xx", "en"}, []string{"mine", "nothere"})
	if len(part.Scenes) != 2 || len(part.Elements) != 1 || part.Standard != "en" {
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
	machine.Scenes["en"] = []string{"INT"}
	got := machine.With(back)
	if !slices.Equal(got.Scenes["xx"], []string{"ENA", "EKA"}) || len(got.Scenes["en"]) != len(DefaultConf().Scenes["en"]) {
		t.Fatalf("with: %v", got.Scenes)
	}
	if len(machine.Scenes["en"]) != 1 || machine.Scenes["xx"] != nil {
		t.Fatal("With changed its receiver")
	}
	if _, ok := got.Elements["mine"]; !ok {
		t.Fatal("the set did not come over")
	}
}

func TestItemsOfRenamed(t *testing.T) {
	c := TOMLConf{Scenes: map[string][]string{"xx": {"A"}}, Matchups: map[string][]string{"xx": {"A."}}, Elements: map[string]Set{"s": {}}, Standard: "en"}
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
	if err != nil || r.Scenes["xx2"][0] != "A" || r.Matchups["xx2"][0] != "A." {
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
