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
// dropped scene.
func renum(spec string, o RenumberOptions) string {
	scenes := scenesOf(spec)
	var b []string
	for i, r := range Renumber(scenes, o) {
		switch {
		case r.Drop:
			b = append(b, "-")
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
	labelsOf := func(spec string) []string {
		var out []string
		for _, sc := range scenesOf(spec) {
			out = append(out, sc.Number)
		}
		return out
	}
	keep := func(spec, list string) []bool { // free: those not in the list
		set := ParseNumberSet(list, labelsOf(spec))
		var out []bool
		for i := range scenesOf(spec) {
			out = append(out, !set.Has(i))
		}
		return out
	}
	only := func(spec, list string) []bool {
		set := ParseNumberSet(list, labelsOf(spec))
		var out []bool
		for i := range scenesOf(spec) {
			out = append(out, set.Has(i))
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
		{"a v1.13.0 OMITTED run kept takes its numbers", "1 O2-4 7", RenumberOptions{Omitted: OmittedKeep}, "1 O2-4 5"},
		{"a v1.13.0 OMITTED run dropped", "1 O2-4 7", RenumberOptions{}, "1 - 2"},
		{"OMITTED dropped closes the gap", "1 O2 O3 4", RenumberOptions{}, "1 - - 2"},
		// keep: the kept numbers stay, the rest numbered around them
		{"keep 5", "1 2 2A 3 4 5 9", RenumberOptions{Free: keep("1 2 2A 3 4 5 9", "5")}, "1 2 3 4 4A 5 6"},
		{"keep 3: letters forced", "1 1A 1B 3 4", RenumberOptions{Free: keep("1 1A 1B 3 4", "3")}, "1 2 2A 3 4"},
		{"keep a range", "1 4 5 6 9", RenumberOptions{Free: keep("1 4 5 6 9", "4-6")}, "1 4 5 6 7"},
		{"keep a range in script order: 4 before 2", "1 4 3 2 9", RenumberOptions{Free: keep("1 4 3 2 9", "4-2")}, "1 4 3 2 5"},
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
		{"reserved labels only: a dash is no range", "1 2 3", RenumberOptions{Reserved: []string{"2-4"}}, "1 2 3"},
		{"reserved letter", "5 ~ 6", RenumberOptions{Free: freeOf(".x."), Reserved: []string{"5A"}}, "5 5B 6"},
		// a scene without a number gets one when free, keeps none when not
		{"unnumbered", "1 ~ 2 ~", RenumberOptions{Free: freeOf("xx..")}, "1 1A 2 ~"},
		// pinned out of order (a scene moved): letters, no number twice
		{"out of order", "5 1 2", RenumberOptions{Free: freeOf(".x.")}, "5 5A 2"},
		// fixed labels (out of the scheme) are never renumbered, nor
		// neighbours: the count goes on from the scheme number before
		{"fixed labels stay", "1 I-1-A 1A 1.1 PROLOGUE 4", RenumberOptions{}, "1 I-1-A 2 1.1 PROLOGUE 3"},
		{"fixed labels stay, letters kept", "1 1-A 1A 5", RenumberOptions{KeepLetters: true}, "1 1-A 1A 2"},
		{"a fixed OMITTED is never dropped", "1 OI-1-A O2 3", RenumberOptions{}, "1 OI-1-A - 2"},
		{"Fountain's examples, all fixed", "1-A I-1-A 1. 01 10a 3-x2k9q", RenumberOptions{}, "1-A I-1-A 1. 01 10a 3-x2k9q"},
		{"110A is in the scheme", "100 110A ~", RenumberOptions{}, "1 2 3"},
		{"a fixed label is never given", "~ ~ ~", RenumberOptions{Reserved: []string{"2"}}, "1 3 4"},
		{"a new scene beside a fixed one", "10 PROLOGUE ~ 11", RenumberOptions{Free: freeOf("..x.")}, "10 PROLOGUE 10A 11"},
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
	labels := []string{"", "1", "1A", "2", "3", "3A", "3B", "4", "5", "7", "A8", "8", "9", "10", "12", "2-4", "x-1", "I-1-A", "1.1", "PROLOGUE"}
	for round := 0; round < 3000; round++ {
		n := 1 + rng.Intn(9)
		scenes := make([]RenumberScene, n)
		free := make([]bool, n)
		for i := range scenes {
			scenes[i] = RenumberScene{Number: labels[rng.Intn(len(labels))], Omitted: rng.Intn(5) == 0}
			free[i] = rng.Intn(3) > 0
		}
		o := RenumberOptions{Free: free, KeepLetters: rng.Intn(2) == 0, Omitted: OmittedMode(rng.Intn(2)), Style: NumberStyle(rng.Intn(2))}
		if rng.Intn(3) == 0 {
			o.Reserved = []string{labels[1+rng.Intn(len(labels)-1)]}
		}
		got := Renumber(scenes, o)
		pinnedTwice := map[string]int{}
		for i, sc := range scenes {
			if (!free[i] || sc.Omitted || readNo(sc.Number, sc.Omitted).fixed()) && sc.Number != "" {
				pinnedTwice[sc.Number]++
			}
		}
		seen := map[string]int{}
		desc := fmt.Sprintf("%+v %+v → %+v", scenes, o, got)
		reserved := newTaken()
		for _, r := range o.Reserved {
			reserved.add(r)
		}
		for i, r := range got {
			fixed := readNo(scenes[i].Number, scenes[i].Omitted).fixed()
			if r.Drop {
				if !scenes[i].Omitted || fixed {
					t.Fatalf("a scene not OMITTED, or a fixed label, dropped: %s", desc)
				}
				continue
			}
			pinned := !free[i] || fixed || (scenes[i].Omitted && o.Omitted != OmittedDrop)
			if pinned && r.Number != scenes[i].Number {
				t.Fatalf("pinned scene %d changed: %s", i, desc)
			}
			if r.Number != scenes[i].Number && !InNumberScheme(r.Number) {
				t.Fatalf("scene %d given %q, out of the scheme: %s", i, r.Number, desc)
			}
			if free[i] && !fixed && !scenes[i].Omitted {
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
				if r.Drop {
					continue
				}
				l, ok := parseLabel(r.Number)
				if !ok {
					continue
				}
				pinned := !free[i] || scenes[i].Omitted || readNo(scenes[i].Number, scenes[i].Omitted).fixed()
				if !pinned {
					var next *label
					for j := i + 1; j < len(got) && next == nil; j++ {
						if (!free[j] || scenes[j].Omitted) && !got[j].Drop {
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

