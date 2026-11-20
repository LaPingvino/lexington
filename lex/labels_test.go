package lex

import (
	"strings"
	"testing"
)

// The scheme: whole numbers with letters after (10A, 10aA) or capitals
// before (A11); every other label of Fountain's is fixed.
func TestInNumberScheme(t *testing.T) {
	for s, want := range map[string]bool{
		"1": true, "10": true, "110A": true, "10A": true, "10AA": true, "10aA": true, "10AaA": true,
		"A11": true, "AB11": true, "A11B": true,
		"": false, "01": false, "0": false, "10a": false, "10aa": false, "a11": false, "1-A": false,
		"I-1-A": false, "1.": false, "1.1": false, "PROLOGUE": false, "3-x2k9q": false, "12-14": false, "1 A": false,
	} {
		if InNumberScheme(s) != want {
			t.Errorf("%q in the scheme: %v", s, !want)
		}
	}
}

// Fixed labels keep their place in a locked script: never given, no
// neighbour.
func TestNumberLabelsFixed(t *testing.T) {
	after := NumberOptions{Locked: true}
	for _, c := range []struct{ in, want string }{
		{"10 I-1-A  11", "10 I-1-A 10A 11"},          // the scheme numbers around it
		{"10  PROLOGUE 11", "10 10A PROLOGUE 11"},    // after it: the next scheme number
		{"I-1-A  1.1 ", "I-1-A 1 1.1 2"},             // no scheme number at all: count
		{"1-A  1. #1-A#", "1-A 1 1. #1-A#"},          // Fountain's examples
		{"12-14  15", "12-14 A15 15"},                // a dashed label on a scene is a label
		{"10 1.1 ", "10 1.1 11"},
	} {
		if got := labels(c.in, after); got != c.want {
			t.Errorf("%q → %q, want %q", c.in, got, c.want)
		}
	}
	// the unlocked count goes on from whole numbers only
	if got := labels("7 I-1-A  ", NumberOptions{}); got != "7 I-1-A 8 9" {
		t.Errorf("unlocked: %q", got)
	}
	if got := SceneNumberBetween("I-1-A", "11", nil, NumberAfter); got != "A11" {
		t.Errorf("between a fixed label and 11: %q", got)
	}
}

// v1.13.0's OMITTED run (#12-14#) takes its numbers, on an OMITTED
// heading only.
func TestNumberLabelsOldOmittedRun(t *testing.T) {
	o := NumberOptions{Locked: true, Omitted: []bool{false, true, false}}
	if got := strings.Join(NumberLabels([]string{"11", "12-14", ""}, o), " "); got != "11 12-14 15" {
		t.Errorf("after the run: %q", got)
	}
	o.Omitted = []bool{false, false, true}
	if got := strings.Join(NumberLabels([]string{"11", "", "12-14"}, o), " "); got != "11 11A 12-14" {
		t.Errorf("before the run: %q", got)
	}
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. A - DAY #11#"},
		{Type: TypeScene, Contents: "OMITTED #12-14#"},
		{Type: TypeScene, Contents: "INT. NEW - DAY"},
	}
	if got := NumberScenes(s, NumberOptions{Locked: true})[2].Contents; got != "INT. NEW - DAY #15#" {
		t.Errorf("NumberScenes: %q", got)
	}
	for h, want := range map[string]string{".OMITTED #12-14#": "12 14", "OMITTED #12A-14#": "12A 14", "OMITTED #14-12#": "",
		"OMITTED #I-1-A#": "", "INT. A #12-14#": "", "OMITTED #12-14-16#": ""} {
		a, b, ok := OmittedRange(h)
		got := ""
		if ok {
			got = a + " " + b
		}
		if got != want {
			t.Errorf("OmittedRange(%q) = %q", h, got)
		}
	}
}

// Lists by the script's order.
func TestNumberSet(t *testing.T) {
	script := []string{"1", "4", "3", "", "2", "I-1-A", "PROLOGUE", "10", "10-11", "11", "12A"}
	scenes := func(set NumberSet) string {
		var out []string
		for i := range script {
			if set.Has(i) {
				out = append(out, script[i])
				if script[i] == "" {
					out[len(out)-1] = "~"
				}
			}
		}
		return strings.Join(out, " ")
	}
	for _, c := range []struct{ in, want, errs, unknown string }{
		{"4-2", "4 3 ~ 2", "", ""}, // 4 before 2 in the script: a forward range
		{"4 - 2", "4 3 ~ 2", "", ""},
		{"4–2", "4 3 ~ 2", "", ""},
		{"4..2", "4 3 ~ 2", "", ""},
		{"2-4", "", "2-4: scene 4 comes before scene 2 in the script", ""},
		{"I-1-A – PROLOGUE", "I-1-A PROLOGUE", "", ""},
		{"I-1-A-PROLOGUE", "I-1-A PROLOGUE", "", ""}, // splits only one way
		{"10-11", "10-11", "", ""},                    // a label wins over a split
		{"10 .. 12A", "10 10-11 11 12A", "", ""},
		{"1, 12A; 99", "1 12A", "", "99"},
		{"1-99", "", "1-99: no scene 99", ""},
		{"1 -", "", "1 -: a range needs a scene after it", ""},
		{"3-x2k9q", "", "", "3-x2k9q"},
	} {
		set := ParseNumberSet(c.in, script)
		if got := scenes(set); got != c.want {
			t.Errorf("%q: scenes %q, want %q", c.in, got, c.want)
		}
		if got := strings.Join(set.Errors, "; "); got != c.errs {
			t.Errorf("%q: errors %q, want %q", c.in, got, c.errs)
		}
		if got := strings.Join(set.Unknown, " "); got != c.unknown {
			t.Errorf("%q: unknown %q, want %q", c.in, got, c.unknown)
		}
	}
	// a dash that splits two ways is an error
	set := ParseNumberSet("A-B-C", []string{"A", "B-C", "A-B", "C"})
	if len(set.Errors) != 1 || !strings.Contains(set.Errors[0], "more than one way") {
		t.Errorf("ambiguous: %q", set.Errors)
	}
	if got := ParseNumberSet("4-2, 99", script).Read(script); got != "4 to 2 (4 scenes) · 99 (no such scene)" {
		t.Errorf("read: %q", got)
	}
}

// A run of OMITTED headings prints as one; the rows between stay.
func TestMergeOmittedRuns(t *testing.T) {
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. A - DAY #11#"},
		{Type: TypeEmpty},
		{Type: TypeScene, Contents: "OMITTED #12#"},
		{Type: TypeEmpty},
		{Type: TypeBoneyard, Contents: "/* Omitted 12"},
		{Type: TypeBoneyard, Contents: "*/"},
		{Type: TypeEmpty},
		{Type: TypeScene, Contents: "OMITTED #12A#", Revision: "Blue"},
		{Type: TypeEmpty},
		{Type: TypeScene, Contents: "OMITTED #13#"},
		{Type: TypeEmpty},
		{Type: TypeScene, Contents: "INT. B - DAY #14#"},
		{Type: TypeScene, Contents: "OMITTED #I-1-A#"},
		{Type: TypeScene, Contents: "OMITTED #I-1-C#"},
		{Type: TypeAction, Contents: "Hi."},
		{Type: TypeScene, Contents: "OMITTED #20#"},
	}
	var heads []string
	m := MergeOmittedRuns(s)
	for _, l := range m {
		if l.Type == TypeScene {
			heads = append(heads, l.Contents)
		}
	}
	if got := strings.Join(heads, "|"); got != "INT. A - DAY #11#|OMITTED #12-13#|INT. B - DAY #14#|OMITTED #I-1-A–I-1-C#|OMITTED #20#" {
		t.Errorf("merged: %q", got)
	}
	if m[2].Revision != "Blue" {
		t.Errorf("the run's revision: %q", m[2].Revision)
	}
	if got := OmittedRunSizes(s); len(got) != 5 || got[1] != 3 || got[3] != 2 {
		t.Errorf("sizes %v", got)
	}
	if len(s) != 16 || s[2].Contents != "OMITTED #12#" {
		t.Error("the screenplay given was changed")
	}
}

// v1.13.0's run read as its scenes: one heading a block, the ends kept
// where their block is gone.
func TestSplitOmittedRanges(t *testing.T) {
	heads := func(s Screenplay) string {
		var out []string
		for _, l := range s {
			if l.Type == TypeScene {
				out = append(out, l.Contents)
			}
		}
		return strings.Join(out, "|")
	}
	block := func(n string) []Line {
		return []Line{{Type: TypeBoneyard, Contents: "/* Omitted " + n, Source: 50}, {Type: TypeBoneyard, Contents: "*/"}, {Type: TypeEmpty}}
	}
	run := func(n string, blocks ...string) Screenplay {
		s := Screenplay{{Type: TypeScene, Contents: "INT. A - DAY #11#"}, {Type: TypeScene, Contents: "OMITTED #" + n + "#", Source: 3}, {Type: TypeEmpty}}
		for _, b := range blocks {
			s = append(s, block(b)...)
		}
		return append(s, Line{Type: TypeScene, Contents: "INT. B - DAY #15#"})
	}
	for _, c := range []struct {
		in   Screenplay
		want string
	}{
		{run("12-14", "12", "13", "14"), "INT. A - DAY #11#|OMITTED #12#|OMITTED #13#|OMITTED #14#|INT. B - DAY #15#"},
		{run("12-14", "13"), "INT. A - DAY #11#|OMITTED #12#|OMITTED #13#|OMITTED #14#|INT. B - DAY #15#"},
		{run("12-14"), "INT. A - DAY #11#|OMITTED #12#|OMITTED #14#|INT. B - DAY #15#"},
		{run("I-1-A", "I"), "INT. A - DAY #11#|OMITTED #I-1-A#|INT. B - DAY #15#"},
	} {
		if got := heads(SplitOmittedRanges(c.in)); got != c.want {
			t.Errorf("%s: %q", heads(c.in), got)
		}
	}
	out := SplitOmittedRanges(run("12-14", "12", "13", "14"))
	if out[1].Source != 3 || out[6].Source != 50 {
		t.Errorf("sources %d %d", out[1].Source, out[6].Source)
	}
}
