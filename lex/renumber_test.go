package lex

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// scenesOf reads "1 1A ~ O4 5": a number a scene, ~ for none, O before
// an OMITTED one's.
func scenesOf(spec string) []RenumberScene {
	var out []RenumberScene
	for _, f := range strings.Fields(spec) {
		sc := RenumberScene{}
		if n, ok := strings.CutPrefix(f, "O"); ok && n != "" && n != "~" || f == "O~" {
			sc.Omitted, f = true, n
		}
		if f != "~" {
			sc.Number = f
		}
		out = append(out, sc)
	}
	return out
}

// renum renumbers spec and writes the result the same way: - for a
// dropped scene, + for one merged into the run before.
func renum(spec string, o RenumberOptions) string {
	scenes := scenesOf(spec)
	var b []string
	for i, r := range Renumber(scenes, o) {
		switch {
		case r.Drop:
			b = append(b, "-")
		case r.Merged:
			b = append(b, "+")
		case r.Number == "":
			b = append(b, "~")
		case scenes[i].Omitted:
			b = append(b, "O"+r.Number)
		default:
			b = append(b, r.Number)
		}
	}
	return strings.Join(b, " ")
}

// freeOf is a Free mask from "x.x": x free.
func freeOf(mask string) []bool {
	var out []bool
	for _, r := range mask {
		out = append(out, r == 'x')
	}
	return out
}

func TestRenumber(t *testing.T) {
	keep := func(spec, list string) []bool { // free: those not in the list
		set, _ := ParseNumberSet(list)
		var out []bool
		for _, sc := range scenesOf(spec) {
			out = append(out, !set.Has(sc.Number))
		}
		return out
	}
	only := func(spec, list string) []bool {
		set, _ := ParseNumberSet(list)
		var out []bool
		for _, sc := range scenesOf(spec) {
			out = append(out, set.Has(sc.Number))
		}
		return out
	}
	const script = "1 1A 3 O4 5 5A 6"
	for _, c := range []struct {
		name, in string
		o        RenumberOptions
		want     string
	}{
		{"clean", script, RenumberOptions{}, "1 2 3 - 4 5 6"},
		{"clean, letters kept", script, RenumberOptions{KeepLetters: true}, "1 1A 2 - 3 3A 4"},
		{"OMITTED kept", script, RenumberOptions{Omitted: OmittedKeep}, "1 2 3 O4 5 6 7"},
		{"OMITTED kept, letters kept", script, RenumberOptions{Omitted: OmittedKeep, KeepLetters: true}, "1 1A 2 O4 5 5A 6"},
		{"OMITTED merged", "1 O2 O3 O4 5 O6", RenumberOptions{Omitted: OmittedMerge}, "1 O2-4 + + 5 O6"},
		{"OMITTED merged with a range", "1 O2-3 O4 7", RenumberOptions{Omitted: OmittedMerge}, "1 O2-4 + 5"},
		{"OMITTED merged out of order: as they are", "1 O4 O2 7", RenumberOptions{Omitted: OmittedMerge}, "1 O4 O2 3"},
		{"OMITTED dropped closes the gap", "1 O2 O3 4", RenumberOptions{}, "1 - - 2"},
		// keep: the kept numbers stay, the rest numbered around them
		{"keep 5", "1 2 2A 3 4 5 9", RenumberOptions{Free: keep("1 2 2A 3 4 5 9", "5")}, "1 2 3 4 4A 5 6"},
		{"keep 3: letters forced", "1 1A 1B 3 4", RenumberOptions{Free: keep("1 1A 1B 3 4", "3")}, "1 2 2A 3 4"},
		{"keep a range", "1 4 5 6 9", RenumberOptions{Free: keep("1 4 5 6 9", "4-6")}, "1 4 5 6 7"},
		{"keep the first", "~ 1 2", RenumberOptions{Free: keep("~ 1 2", "1")}, "A1 1 2"},
		{"keep, A11 style", "1 1A 1B 3", RenumberOptions{Free: keep("1 1A 1B 3", "3"), Style: NumberBefore}, "1 2 A3 3"},
		// only: just those
		{"only 3 and 4", "1 2 3 4 7", RenumberOptions{Free: only("1 2 3 4 7", "3, 4")}, "1 2 3 4 7"},
		{"only the A-scenes", "1 1A 2 2A 3", RenumberOptions{Free: only("1 1A 2 2A 3", "1A 2A")}, "1 1A 2 2A 3"},
		{"only 10-12", "1 10 11 11A 12 20", RenumberOptions{Free: only("1 10 11 11A 12 20", "10-12")}, "1 2 3 4 5 20"},
		// a range of scenes (the scope): the others pinned
		{"scope: the middle", "1 2 2A 3 5 6", RenumberOptions{Free: freeOf("..xxx.")}, "1 2 3 4 5 6"},
		{"scope: can't close what's outside", "1 2 2A 2B 3 4", RenumberOptions{Free: freeOf(".xxx..")}, "1 2 2A 2B 3 4"},
		{"scope: from the cursor on", "1 5 5A 9", RenumberOptions{Free: freeOf(".xxx")}, "1 2 3 4"},
		{"scope and keep", "1 2 2A 3 8 9", RenumberOptions{Free: freeOf(".xx.xx")}, "1 2 2A 3 4 5"},
		{"scope with OMITTED outside", "O1 2 3", RenumberOptions{Free: freeOf(".xx")}, "O1 2 3"},
		{"scope with OMITTED inside", "1 O2 3 O4", RenumberOptions{Free: freeOf("xxx.")}, "1 - 2 O4"},
		// reserved numbers are skipped, never given
		{"reserved", "1 2 2A 3", RenumberOptions{Reserved: []string{"3"}}, "1 2 4 5"},
		{"reserved before a pinned", "1 1A 3", RenumberOptions{Free: freeOf("xx."), Reserved: []string{"2", "3"}}, "1 1A 3"},
		{"reserved range", "1 2 3", RenumberOptions{Reserved: []string{"2-4"}}, "1 5 6"},
		{"a reserved range leaves its letters", "5 ~ 6", RenumberOptions{Free: freeOf(".x."), Reserved: []string{"5-6"}}, "5 5A 6"},
		// a scene without a number gets one when free, keeps none when not
		{"unnumbered", "1 ~ 2 ~", RenumberOptions{Free: freeOf("xx..")}, "1 1A 2 ~"},
		// pinned out of order (a scene moved): letters, no number twice
		{"out of order", "5 1 2", RenumberOptions{Free: freeOf(".x.")}, "5 5A 2"},
	} {
		if got := renum(c.in, c.o); got != c.want {
			t.Errorf("%s: %q → %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

// Whatever the strategy: pinned numbers stay, no number twice, a reserved
// number never given.
func TestRenumberProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	labels := []string{"", "1", "1A", "2", "3", "3A", "3B", "4", "5", "7", "A8", "8", "9", "10", "12", "2-4", "x-1"}
	for round := 0; round < 3000; round++ {
		n := 1 + rng.Intn(9)
		scenes := make([]RenumberScene, n)
		free := make([]bool, n)
		for i := range scenes {
			scenes[i] = RenumberScene{Number: labels[rng.Intn(len(labels))], Omitted: rng.Intn(5) == 0}
			free[i] = rng.Intn(3) > 0
		}
		o := RenumberOptions{Free: free, KeepLetters: rng.Intn(2) == 0, Omitted: OmittedMode(rng.Intn(3)), Style: NumberStyle(rng.Intn(2))}
		if rng.Intn(3) == 0 {
			o.Reserved = []string{labels[1+rng.Intn(len(labels)-1)]}
		}
		got := Renumber(scenes, o)
		pinnedTwice := map[string]int{}
		for i, sc := range scenes {
			if (!free[i] || sc.Omitted) && sc.Number != "" {
				pinnedTwice[sc.Number]++
			}
		}
		seen := map[string]int{}
		desc := fmt.Sprintf("%+v %+v → %+v", scenes, o, got)
		reserved := newTaken()
		for _, r := range o.Reserved {
			reserve(reserved, r)
		}
		for i, r := range got {
			if r.Drop || r.Merged {
				if !scenes[i].Omitted {
					t.Fatalf("a scene not OMITTED dropped: %s", desc)
				}
				continue
			}
			pinned := !free[i] || (scenes[i].Omitted && o.Omitted != OmittedDrop)
			if pinned && r.Number != scenes[i].Number && !(o.Omitted == OmittedMerge && scenes[i].Omitted) {
				t.Fatalf("pinned scene %d changed: %s", i, desc)
			}
			if free[i] && !scenes[i].Omitted {
				if r.Number == "" {
					t.Fatalf("free scene %d got no number: %s", i, desc)
				}
				if reserved.has(r.Number) {
					t.Fatalf("reserved number given: %s", desc)
				}
			}
			if r.Number != "" {
				seen[r.Number]++
			}
		}
		// a free scene lands between its pinned neighbours, when they are
		// in order (ranges aside: an OMITTED one can take all between; and
		// letters of one style only: 1 < A1A < 1A has no order)
		oneStyle := true
		for _, sc := range scenes {
			if l, ok := parseLabel(sc.Number); ok && (o.Style == NumberAfter && l.lead != "" || o.Style == NumberBefore && l.tail != "") {
				oneStyle = false
			}
		}
		if oneStyle && !strings.Contains(fmt.Sprint(scenes, o.Reserved), "-") {
			var prev *label
			for i, r := range got {
				if r.Drop || r.Merged {
					continue
				}
				l, ok := parseLabel(r.Number)
				if !ok {
					continue
				}
				pinned := !free[i] || scenes[i].Omitted
				if !pinned {
					var next *label
					for j := i + 1; j < len(got) && next == nil; j++ {
						if (!free[j] || scenes[j].Omitted) && !got[j].Drop && !got[j].Merged {
							if nl, ok := parseLabel(got[j].Number); ok {
								next = &nl
							}
						}
					}
					if prev != nil && next != nil && compareLabels(*prev, *next) < 0 &&
						(compareLabels(*prev, l) >= 0 || compareLabels(l, *next) >= 0) {
						t.Fatalf("scene %d (%s) not between its neighbours: %s", i, r.Number, desc)
					}
				}
				if pinned {
					prev = &l
				}
			}
		}
		for num, c := range seen {
			if c > 1 && pinnedTwice[num] < c {
				t.Fatalf("%s given twice: %s", num, desc)
			}
		}
	}
}

func TestNumberRange(t *testing.T) {
	for _, c := range []struct {
		in     string
		lo, hi string
		ok     bool
	}{
		{"12-14", "12", "14", true},
		{"12A-14", "12A", "14", true},
		{"14-12", "", "", false},
		{"I-1-A", "", "", false},
		{"3-x2k9q", "", "", false},
		{"12", "", "", false},
	} {
		lo, hi, ok := NumberRange(c.in)
		if lo != c.lo || hi != c.hi || ok != c.ok {
			t.Errorf("%q: %q %q %v", c.in, lo, hi, ok)
		}
	}
	set, bad := ParseNumberSet("5, 12A; 20 - 24 3-x2k9q")
	if strings.Join(bad, ",") != "3-x2k9q" || set.String() != "5, 12A, 20-24, 3-x2k9q" {
		t.Errorf("set %q, bad %v", set, bad)
	}
	for n, want := range map[string]bool{"5": true, "12A": true, "12": false, "20": true, "22A": true, "24": true, "24A": false, "A20": false, "3-x2k9q": true, "21-23": true, "4-6": false} {
		if set.Has(n) != want {
			t.Errorf("Has(%q) = %v", n, !want)
		}
	}
}

// A locked script never gives a number an OMITTED range has taken, and a
// range is a neighbour: its last before, its first after.
func TestNumberLabelsRange(t *testing.T) {
	after := NumberOptions{Locked: true}
	for _, c := range []struct{ in, want string }{
		{"11 12-14 ", "11 12-14 15"},
		{"11  12-14", "11 11A 12-14"},
		{"12-14  15", "12-14 14A 15"},
	} {
		if got := labels(c.in, after); got != c.want {
			t.Errorf("%q → %q, want %q", c.in, got, c.want)
		}
	}
	if got := SceneNumberBetween("11", "", map[string]bool{"11": true, "12-14": true}, NumberAfter); got == "12" || got == "13" || got == "14" {
		t.Errorf("between gave %q, in the range", got)
	}
}
