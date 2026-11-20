package lex

import (
	"regexp"
	"strconv"
	"strings"
)

// Scene numbers are labels. Fountain allows "any alphanumerics, plus
// dashes and periods" between the #s (https://fountain.io/syntax: #1#,
// #1-A#, #I-1-A#, #1.#, #110A# are all scene numbers), so a script's
// numbers are whatever its writer, a production office or another app
// wrote. The engine (NumberLabels, SceneNumberBetween, Renumber) reads
// only the labels of one scheme, the scheme it writes itself:
//
//   - a whole number, no leading zero: 1, 10, 110;
//   - with letters after it (NumberAfter): 10A, 10AA, 10B; a small a
//     before a letter is Wikipedia's filler for "between 10 and 10A":
//     10aA, 10AaA (never at the end: 10a is not one);
//   - with capitals before it (NumberBefore): A11, AB11, B11;
//   - both, as NumberBefore letters a scene before a lettered one: A11B.
//
// Every other label (I-1-A, 1-A, 1.1, 1., PROLOGUE, 01, 10a,
// Storyboarder's 3-x2k9q) is fixed: the engine never gives a fixed scene
// a new number (Renumber leaves it, OMITTED or not, whatever its
// strategy says), never gives its label to another scene, and never
// numbers around it: a fixed label has no place among the scheme's
// numbers, so looking for a new scene's neighbours it is passed over,
// and the nearest scheme numbers before and after (in the script's
// order) are its neighbours. A scene between 10 and PROLOGUE and 11 gets
// 10A; one after the last scheme number gets the next whole number.
//
// The scheme's numbers are ordered (12 < 12A < A13 < 13) only to find a
// free number between two neighbours. Nothing else orders labels: a
// list's range ("20-24", NumberSet) runs from scene to scene in the
// script's order, whatever their labels.

// label is a scheme number taken apart: A11B is lead "A", number 11,
// tail "B".
type label struct {
	lead, tail string
	num        int
	digits     string
}

var labelForm = regexp.MustCompile(`^([A-Z]*)([1-9][0-9]*)((?:a*[A-Z])*)$`)

func parseLabel(s string) (label, bool) {
	m := labelForm.FindStringSubmatch(s)
	if m == nil {
		return label{}, false
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		return label{}, false
	}
	return label{lead: m[1], num: n, digits: m[2], tail: m[3]}, true
}

// InNumberScheme tells whether the engine reads a scene number as one of
// its scheme (10, 10A, 10aA, A11, AB11, A11B): one it may number new
// scenes around and give a new number. Every other label is fixed.
func InNumberScheme(s string) bool {
	_, ok := parseLabel(s)
	return ok
}

// String is the label as written: A11B.
func (l label) String() string { return l.lead + l.digits + l.tail }

// head is the label up to its tail's letters (A11 of A11B); rest from
// its number on (11B).
func (l label) head() string { return l.lead + l.digits }
func (l label) rest() string { return l.digits + l.tail }

// compareLabels orders scheme numbers: by their number, then a letter
// before it (A13 before 13), then the letters after it (13 before 13A).
func compareLabels(a, b label) int {
	switch {
	case a.num != b.num:
		if a.num < b.num {
			return -1
		}
		return 1
	case a.lead != b.lead:
		return strings.Compare(beforeKey(a.lead), beforeKey(b.lead))
	}
	return strings.Compare(afterKey(a.tail), afterKey(b.tail))
}

// sceneNo is a scene's number as the engine reads it: in the scheme
// (l the label), or a fixed label (raw only; "12-14" is one, OMITTED or
// not); raw "" is no number.
type sceneNo struct {
	raw    string
	l      label
	scheme bool
}

// readNo reads a scene's number.
func readNo(raw string) sceneNo {
	if l, ok := parseLabel(raw); ok {
		return sceneNo{raw: raw, l: l, scheme: true}
	}
	return sceneNo{raw: raw}
}

// fixed tells whether the number is a label the engine leaves alone.
func (n sceneNo) fixed() bool { return n.raw != "" && !n.scheme }

// prev and next are the number as a neighbour before a new scene and
// after it; nil for none or a fixed label.
func (n sceneNo) prev() *label { return n.next() }

func (n sceneNo) next() *label {
	if !n.scheme {
		return nil
	}
	l := n.l
	return &l
}

// takenSet are the labels that may not be given, as written.
type takenSet struct {
	m map[string]bool
}

func newTaken() *takenSet { return &takenSet{m: map[string]bool{}} }

func (t *takenSet) add(s string) { t.addNo(readNo(s)) }

func (t *takenSet) addNo(n sceneNo) {
	if n.raw != "" {
		t.m[n.raw] = true
	}
}

func (t *takenSet) has(s string) bool { return t.m[s] }

// beyond is the first whole number above every scheme number taken and
// prev.
func (t *takenSet) beyond(prev *label) string {
	top := 0
	if prev != nil {
		top = prev.num
	}
	for s := range t.m {
		if l, ok := parseLabel(s); ok && l.num > top {
			top = l.num
		}
	}
	return strconv.Itoa(top + 1)
}
