package lex

import (
	"strings"
	"testing"
)

// labels fills a script's numbers ("" new) and gives them joined.
func labels(nums string, o NumberOptions) string {
	return strings.Join(NumberLabels(strings.Split(nums, " "), o), " ")
}

func TestNumberLabelsUnlocked(t *testing.T) {
	// as Accolade's export did: on from the last whole number
	if got := labels(" 7  ", NumberOptions{}); got != "1 7 8 9" {
		t.Errorf("got %q", got)
	}
}

// Wikipedia's examples (https://en.wikipedia.org/wiki/Shooting_script),
// and the start and the end, in both styles.
func TestNumberLabelsLocked(t *testing.T) {
	after := NumberOptions{Locked: true}
	before := NumberOptions{Locked: true, Style: NumberBefore}
	for _, c := range []struct {
		in   string
		o    NumberOptions
		want string
	}{
		{"10  11", after, "10 10A 11"},         // between 10 and 11
		{"10   11", after, "10 10A 10B 11"},    // two of them: 10A, 10B
		{"10 10A  11", after, "10 10A 10B 11"}, // after 10A
		{"10 10A  10B 11", after, "10 10A 10AA 10B 11"},
		{"10  10A 11", after, "10 10aA 10A 11"}, // nothing between 10 and 10A but Wikipedia's small a
		{"10 10A  10AA", after, "10 10A 10AaA 10AA"},
		{" 1 2", after, "A1 1 2"},     // before the first
		{"  1 2", after, "A1 B1 1 2"}, // two before it
		{" A1 1", after, "AA1 A1 1"},
		{"86 87 ", after, "86 87 88"},     // at the end: the next number
		{"86 87  ", after, "86 87 88 89"}, // two
		{"10  11", before, "10 A11 11"},
		{"10   11", before, "10 A11 B11 11"},
		{"10 A11  11", before, "10 A11 B11 11"},
		{"10  A11 11", before, "10 AA11 A11 11"},
		{"10 A11  B11 11", before, "10 A11 AB11 B11 11"},
		{" 1", before, "A1 1"},
		{"87 ", before, "87 88"},
		{"  ", after, "1 2 3"}, // nothing numbered: count
		// numbers kept by scenes elsewhere (moved, or OMITTED) are not taken again
		{"10 10A 11  12", after, "10 10A 11 11A 12"},
		{"10  11 10A", after, "10 10B 11 10A"},
		{"87  88", after, "87 87A 88"},
		{"87 88 ", after, "87 88 89"},
		{"12 13 88 ", after, "12 13 88 89"},
		{"3-x2k9q  4", after, "3-x2k9q A4 4"}, // Storyboarder's are no anchors
	} {
		if got := labels(c.in, c.o); got != c.want {
			t.Errorf("%q (%v): got %q, want %q", c.in, c.o.Style, got, c.want)
		}
	}
}

// Past Z: two letters.
func TestNumberLabelsPastZ(t *testing.T) {
	in := "10" + strings.Repeat(" ", 29) + "11"
	got := strings.Split(labels(in, NumberOptions{Locked: true}), " ")
	if got[1] != "10A" || got[26] != "10Z" || got[27] != "10ZA" || got[28] != "10ZB" {
		t.Errorf("got %q", got)
	}
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Errorf("%s twice", n)
		}
		seen[n] = true
	}
}

func TestNumberScenes(t *testing.T) {
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. ONE - DAY #10#"},
		{Type: TypeAction, Contents: "Hi."},
		{Type: TypeScene, Contents: "INT. NEW - DAY [[red]]"},
		{Type: TypeScene, Contents: "OMITTED #11#"},
		{Type: TypeScene, Contents: "INT. TWO - DAY"},
	}
	out := NumberScenes(s, NumberOptions{Locked: true})
	want := []string{"INT. ONE - DAY #10#", "Hi.", "INT. NEW - DAY #10A# [[red]]", "OMITTED #11#", "INT. TWO - DAY #12#"}
	for i, l := range out {
		if l.Contents != want[i] {
			t.Errorf("%d: %q, want %q", i, l.Contents, want[i])
		}
	}
	if s[2].Contents != "INT. NEW - DAY [[red]]" {
		t.Error("the screenplay given was changed")
	}
}

func TestWithSceneNumber(t *testing.T) {
	for _, c := range [][3]string{
		{"INT. A - DAY", "1", "INT. A - DAY #1#"},
		{"INT. A - DAY #4#", "4A", "INT. A - DAY #4A#"},
		{"INT. A - DAY #4# [[red]] [[x]]", "5", "INT. A - DAY #5# [[red]] [[x]]"},
		{"INT. A - DAY  [[red]]", "2", "INT. A - DAY #2# [[red]]"},
		{"INT. A - DAY #4#", "", "INT. A - DAY"},
	} {
		if got := WithSceneNumber(c[0], c[1]); got != c[2] {
			t.Errorf("%q %q: %q", c[0], c[1], got)
		}
	}
}

func TestOmittedNumber(t *testing.T) {
	for in, want := range map[string]string{".OMITTED #12#": "12", "OMITTED #3A#": "3A", "omitted": ""} {
		if n, ok := OmittedNumber(in); !ok || n != want {
			t.Errorf("%q: %q %v", in, n, ok)
		}
	}
	if _, ok := OmittedNumber("INT. OMITTED ROOM #2#"); ok {
		t.Error("a heading with OMITTED in it is no omitted scene")
	}
}

func TestNumberStyle(t *testing.T) {
	if ParseNumberStyle(NumberBefore.String()) != NumberBefore || ParseNumberStyle("10A") != NumberAfter || ParseNumberStyle("?") != NumberAfter {
		t.Error("styles do not read back")
	}
}
