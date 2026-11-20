package lex

import (
	"strconv"
	"strings"
)

// Ranges of scene numbers. A run of omitted scenes can be one heading,
// "OMITTED #12-14#", printed "12-14 OMITTED 12-14" (the form I know
// from shooting scripts; not checked against a source);
// Fountain allows the dash in a scene number ("any alphanumerics, plus
// dashes and periods", https://fountain.io/syntax). Such a range is one
// number written as its first and last, a dash between: both ends
// numbers in their own right (12A-14), the first not after the last.
// "I-1-A" (Fountain's own example of a dashed number) is none. A range
// takes every number from its first to its last in their order (12, 12A,
// 13, 13aA … 14), so none of them is given to another scene.

// NumberRange splits a range of scene numbers ("12-14") into its first
// and last; false if s is not one.
func NumberRange(s string) (first, last string, ok bool) {
	lo, hi, found := strings.Cut(s, "-")
	if !found {
		return "", "", false
	}
	a, okA := parseLabel(lo)
	b, okB := parseLabel(hi)
	if !okA || !okB || compareLabels(a, b) > 0 {
		return "", "", false
	}
	return lo, hi, true
}

// CompareNumbers orders two scene numbers as a script has them: 12 <
// 12A < 12B < A13 < 13; false if either is not a number of digits with
// letters.
func CompareNumbers(a, b string) (int, bool) {
	x, okA := parseLabel(a)
	y, okB := parseLabel(b)
	if !okA || !okB {
		return 0, false
	}
	return compareLabels(x, y), true
}

// compareLabels orders labels: by their number, then a letter before it
// (A13 before 13), then the letters after it (13 before 13A).
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

// anchorLabel is the label a number stands for as a neighbour: itself,
// or for a range its last (last) or first.
func anchorLabel(s string, last bool) (label, bool) {
	if l, ok := parseLabel(s); ok {
		return l, true
	}
	lo, hi, ok := NumberRange(s)
	if !ok {
		return label{}, false
	}
	if last {
		return parseLabel(hi)
	}
	return parseLabel(lo)
}

// takenSet are the numbers that may not be given: as written, and the
// ranges' every number.
type takenSet struct {
	m      map[string]bool
	ranges [][2]label
}

func newTaken() *takenSet { return &takenSet{m: map[string]bool{}} }

func (t *takenSet) add(s string) {
	t.m[s] = true
	if lo, hi, ok := NumberRange(s); ok {
		a, _ := parseLabel(lo)
		b, _ := parseLabel(hi)
		t.ranges = append(t.ranges, [2]label{a, b})
	}
}

func (t *takenSet) has(s string) bool {
	if t.m[s] {
		return true
	}
	if len(t.ranges) == 0 {
		return false
	}
	l, ok := parseLabel(s)
	if !ok {
		return false
	}
	for _, r := range t.ranges {
		if compareLabels(r[0], l) <= 0 && compareLabels(l, r[1]) <= 0 {
			return true
		}
	}
	return false
}

// NumberSet is a list of scene numbers as a writer types it: "5, 12A,
// 20-24", a range standing for every number from its first to its last
// (20, 20A … 24).
type NumberSet struct {
	items []string
	taken *takenSet
}

// ParseNumberSet reads a list of numbers, split by commas, semicolons or
// spaces ("20 - 24" is one range). bad are the items that are no number
// of digits and letters nor a range of them; they are kept as written
// (a scene can be numbered 3-x2k9q), and match only themselves.
func ParseNumberSet(s string) (set NumberSet, bad []string) {
	set.taken = newTaken()
	s = strings.NewReplacer(" - ", "-", " -", "-", "- ", "-", "–", "-").Replace(s)
	for _, it := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' }) {
		set.items = append(set.items, it)
		set.taken.add(it)
		if _, ok := parseLabel(it); !ok {
			if _, _, ok := NumberRange(it); !ok {
				bad = append(bad, it)
			}
		}
	}
	return set, bad
}

// Items are the set's numbers and ranges as typed.
func (s NumberSet) Items() []string { return append([]string(nil), s.items...) }

// Empty tells whether the set has no number.
func (s NumberSet) Empty() bool { return len(s.items) == 0 }

// Has tells whether n is in the set: one of its numbers, or in one of
// its ranges; a range n (an OMITTED run's 12-14) when written so, or
// when its first and last both are.
func (s NumberSet) Has(n string) bool {
	if s.taken == nil || n == "" {
		return false
	}
	if s.taken.has(n) {
		return true
	}
	if lo, hi, ok := NumberRange(n); ok {
		return s.taken.has(lo) && s.taken.has(hi)
	}
	return false
}

// String is the set as ParseNumberSet reads it.
func (s NumberSet) String() string { return strings.Join(s.items, ", ") }

// beyond is the first whole number above every number taken and prev.
func (t *takenSet) beyond(prev *label) string {
	top := 0
	if prev != nil {
		top = prev.num
	}
	for s := range t.m {
		if l, ok := anchorLabel(s, true); ok && l.num > top {
			top = l.num
		}
	}
	return strconv.Itoa(top + 1)
}
