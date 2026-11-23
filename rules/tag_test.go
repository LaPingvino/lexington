package rules

import (
	"slices"
	"testing"
)

func TestTag(t *testing.T) {
	for in, want := range map[string]string{
		"hi-Deva": "hi-Deva", "hi-deva": "hi-Deva", "HI_deva_in": "hi-Deva-IN", "zh-hant-tw": "zh-Hant-TW",
		"en": "en", "TOK": "tok", "es-419": "es-419", "": "", "hi--Deva": "", "hi-": "", "my lang": "",
		"1x": "", "hi-Devanagari": "",
	} {
		if got := Tag(in); got != want {
			t.Errorf("Tag(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestLookup: a BCP 47 tag reads as itself, in any case, else as the tag
// without its last subtag, down to the language; an unknown one as none.
func TestLookup(t *testing.T) {
	c := DefaultConf()
	for in, want := range map[string]string{
		"hi-Deva": "hi-Deva", "hi-deva": "hi-Deva", "hi_Deva": "hi-Deva", "hi-Deva-IN": "hi-Deva",
		"hi-Latn": "hi-Latn", "hi-latn-in": "hi-Latn", "hi-Arab": "hi", "hi": "hi", "HI": "hi",
		"de-CH": "de", "zh-Hant": "zh", "hekaya": Hekaya, "xx-Yyyy": "", "xx": "",
	} {
		got, ok := c.Lookup(in)
		if got != want || ok != (want != "") {
			t.Errorf("Lookup(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	if !c.Knows("hi-Deva-IN") || c.Knows("xx-Yyyy") {
		t.Error("Knows does not follow Lookup")
	}
	if !slices.Equal(c.Scenes("hi-deva"), c.Scenes("hi-Deva")) || c.Scenes("xx-Yyyy") != nil {
		t.Error("Scenes does not follow Lookup")
	}
	if c.Language("de-CH").Times[0] != "TAG" {
		t.Error("de-CH is not read as German")
	}
	// one's own language written in small letters, and the more exact
	// tag of one's own before the default's broader one
	mine, err := Decode([]byte("[Languages.hi-latn]\nScenes = [\"DRISHYA\"]\n[Languages.pt-BR]\nScenes = [\"INT\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := mine.Lookup("hi-Latn"); k != "hi-latn" {
		t.Errorf("own hi-latn: %q", k)
	}
	if got := mine.Language("hi-Latn-IN").Scenes; !slices.Equal(got, []string{"DRISHYA"}) {
		t.Errorf("own hi-latn's scenes: %v", got)
	}
	if k, _ := DefaultConf().With(mine).Lookup("pt-br-x-foo"); k != "pt-BR" {
		t.Errorf("pt-br-x-foo: %q", k)
	}
	// a machine without Hindi (a configuration file from before) still
	// reads a hi-Deva script with the default's Hindi
	old, _ := Decode([]byte("[Languages.en]\nScenes = [\"INT\"]\n"))
	if old.Knows("hi-Deva") || !slices.Contains(old.Language("hi-Deva-IN").Scenes, "अंदर.") {
		t.Error("an old configuration's hi-Deva")
	}
}

// TestHindiBoth: hi reads all that hi-Latn and hi-Deva read.
func TestHindiBoth(t *testing.T) {
	c := DefaultConf()
	hi, latn, deva := c.Language("hi"), c.Language("hi-Latn"), c.Language("hi-Deva")
	for _, part := range []Language{latn, deva} {
		for name, f := range map[string]func(Language) []string{
			"Scenes": func(l Language) []string { return l.Scenes }, "Headings": func(l Language) []string { return l.Headings },
			"Settings": func(l Language) []string { return l.Settings }, "Times": func(l Language) []string { return l.Times },
			"Extensions": func(l Language) []string { return l.Extensions }, "Transitions": func(l Language) []string { return l.Transitions },
			"TransitionEnds": func(l Language) []string { return l.TransitionEnds }, "Cues": func(l Language) []string { return l.Cues },
		} {
			for _, w := range f(part) {
				if !slices.Contains(f(hi), w) {
					t.Errorf("hi's %s lack %q", name, w)
				}
			}
		}
		for k := range part.TitleKeys {
			if _, ok := hi.TitleKeys[k]; !ok {
				t.Errorf("hi's title keys lack %q", k)
			}
		}
	}
}
