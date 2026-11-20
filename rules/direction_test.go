package rules

import "testing"

func TestDirections(t *testing.T) {
	c := DefaultConf()
	for lang, want := range map[string]bool{"he": true, "ar": true, "en": false, "eo": false, "nope": false} {
		if c.RTL(lang) != want {
			t.Errorf("default RTL(%q) = %v", lang, !want)
		}
	}
	// a configuration written before Directions has the default's
	old, err := Decode([]byte("[Scenes]\nhe = [\"פנים\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !old.RTL("he") {
		t.Error("a configuration without Directions: he not right to left")
	}
	// one's own: a custom language right to left, Hebrew turned
	mine, err := Decode([]byte("[Scenes]\nxx = [\"A\"]\nhe = [\"פנים\"]\n[Directions]\nxx = \"RTL\"\nhe = \"ltr\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	// a language it does not name keeps the default's
	if !mine.RTL("xx") || mine.RTL("he") || !mine.RTL("ar") || mine.Direction("he") != "ltr" {
		t.Errorf("own Directions: xx %v he %v ar %v", mine.RTL("xx"), mine.RTL("he"), mine.RTL("ar"))
	}
}

func TestDirectionsTravel(t *testing.T) {
	c := DefaultConf()
	c.Scenes["xx"] = []string{"A"}
	c.Directions["xx"] = "rtl"
	part := c.Part([]string{"xx", "en"}, nil)
	if part.Directions["xx"] != "rtl" || part.Directions["en"] != "ltr" {
		t.Fatalf("part's Directions: %v", part.Directions)
	}
	text, err := part.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	// laid over a machine that has never heard of xx, nor of Directions
	machine, _ := Decode([]byte("[Scenes]\nen = [\"INT\"]\n"))
	got := machine.With(back)
	if !got.RTL("xx") || !got.RTL("he") || got.RTL("en") {
		t.Errorf("layered: xx %v he %v en %v", got.RTL("xx"), got.RTL("he"), got.RTL("en"))
	}
	if machine.Directions != nil {
		t.Error("With changed the machine's configuration")
	}
	// an old bundle's language reads the same as the machine's default
	old, _ := Decode([]byte("[Scenes]\nhe = [\"פנים\", \"חוץ\", \"פנים/חוץ\", \"חוץ/פנים\"]\n[Matchups]\nhe = [\"פנים\", \"חוץ\", \"פנים/חוץ\", \"חוץ/פנים\", \"\"]\n"))
	it := Item{"language", "he"}
	a, _ := old.Of(it)
	b, _ := DefaultConf().Of(it)
	if !Same(a, b) {
		t.Errorf("an old bundle's Hebrew differs from the default's: %v / %v", a.Directions, b.Directions)
	}
	// renamed (keep both), the direction goes along
	r, err := Renamed(part, Item{"language", "xx"}, "yy")
	if err != nil || r.Directions["yy"] != "rtl" {
		t.Errorf("renamed: %v %v", r.Directions, err)
	}
	// a language only named in Directions is a language
	only := TOMLConf{Directions: map[string]string{"zz": "rtl"}}
	if items := only.Items(); len(items) != 1 || items[0] != (Item{"language", "zz"}) {
		t.Errorf("items: %v", items)
	}
	if p, ok := only.Of(Item{"language", "zz"}); !ok || !p.RTL("zz") {
		t.Errorf("of: %v %v", p, ok)
	}
}
