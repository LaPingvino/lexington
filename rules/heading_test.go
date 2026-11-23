package rules

import (
	"slices"
	"testing"
)

// Every default pattern comes apart and together again byte for byte.
func TestPatternRoundTrip(t *testing.T) {
	ps := []string{SceneForm, "{x} {n}", "{", "}{n}{", "a{{n}}b"}
	for _, l := range DefaultLanguages() {
		ps = append(ps, l.Headings...)
		ps = append(ps, l.NewHeading)
	}
	for _, p := range ps {
		if got := JoinPattern(ParsePattern(p)); got != p {
			t.Errorf("%q: %q", p, got)
		}
	}
	if got := PatternSlots("{n}.{time} {io} {place}"); !slices.Equal(got, []string{"n", "time", "io", "place"}) {
		t.Errorf("slots: %v", got)
	}
	if got := ParsePattern("{x}{n}"); len(got) != 2 || got[0].Text != "{x}" {
		t.Errorf("unknown slot as text: %#v", got)
	}
}

func TestFillPattern(t *testing.T) {
	v := map[string]string{"n": "12", "io": "INT.", "place": "KITCHEN", "time": "DAY"}
	for _, c := range []struct {
		p    string
		drop []string
		want string
	}{
		{SceneForm, nil, "INT. KITCHEN - DAY"},
		{SceneForm, []string{"time"}, "INT. KITCHEN"},
		{"{n}. {io} {any}", []string{"any"}, "12. INT."},
		{"场景{n} {place} {time} {io}", nil, "场景12 KITCHEN DAY INT."},
		{"场景{n} {place} {time} {io}", []string{"n"}, "KITCHEN DAY INT."},
		{"○{place}（{time}）", []string{"time"}, "○KITCHEN"},
		{"○{place}（{time}）", nil, "○KITCHEN（DAY）"},
		{"SÉQUENCE {n} - {io}. {time}", []string{"time"}, "SÉQUENCE 12 - INT."},
		{"{place}/{time}/{io}", []string{"time"}, "KITCHEN/INT."},
	} {
		vals := map[string]string{}
		for k, x := range v {
			if !slices.Contains(c.drop, k) {
				vals[k] = x
			}
		}
		if got := FillPattern(c.p, vals); got != c.want {
			t.Errorf("%q without %v: %q, want %q", c.p, c.drop, got, c.want)
		}
	}
}

func TestHeadingForm(t *testing.T) {
	c := DefaultConf()
	for lang, want := range map[string]string{
		"en": SceneForm, "eo": SceneForm, "zh": "场景{n} {place} {time} {io}", "ja": "○{place}（{time}）",
		"hi-Deva": "{place}/{time}/{io}", "he": "{n}. {io} {any}",
	} {
		if got := c.Language(lang).HeadingForm(); got != want {
			t.Errorf("%s: %q, want %q", lang, got, want)
		}
	}
	mine, err := Decode([]byte("[Languages.zh]\nNewHeading = \"{io} {place} - {time}\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := mine.Language("zh"); got.HeadingForm() != SceneForm || len(got.Headings) == 0 {
		t.Errorf("zh's own NewHeading: %q, headings %v", got.HeadingForm(), got.Headings)
	}
	en := c.Language("en").IOWords(SceneForm)
	if !slices.Equal(en, []string{"INT.", "EXT.", "INT./EXT.", "EXT./INT.", "EST."}) {
		t.Errorf("en io: %v", en)
	}
	zh := c.Language("zh").IOWords("{n}. {place} {time} {io}")
	if len(zh) < 2 || zh[0] != "内" || zh[1] != "外" || !slices.Contains(zh, "内景/外景") {
		t.Errorf("zh io: %v", zh)
	}
	if deva := c.Language("hi-Deva").IOWords("{place}/{time}/{io}"); slices.Contains(deva, "अंदर.") || deva[0] != "अंदर" {
		t.Errorf("hi-Deva io: %v", deva)
	}
	if c.Language("ja").IOWords("○{place}（{time}）") != nil {
		t.Error("a form without {io} has io words")
	}
}
