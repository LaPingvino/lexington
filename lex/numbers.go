package lex

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Scene numbers, as a shooting script has them. Before the script is
// locked, scenes are numbered 1, 2, 3 … Once it is locked, the numbers
// stay: a scene written between 10 and 11 gets a number of its own
// between them, and a cut scene keeps its number as OMITTED, so that no
// later scene takes it.
//
// Two ways of lettering are in use (https://en.wikipedia.org/wiki/Shooting_script):
//
//   - NumberAfter (the default): the letter follows the scene before. 10,
//     10A, 10B, 11; between 10A and 10B comes 10AA. Final Draft gives
//     inserted scenes this form ("28A",
//     https://kb.finaldraft.com/hc/en-us/articles/27810301418132-How-do-I-number-scenes),
//     as WriterDuet does (2A, 2B). There is no letter between 10 and 10A;
//     Wikipedia's form for that place is 10aA, a small a before the letter.
//   - NumberBefore: the letter goes before the scene after. 10, A11, B11,
//     11; between 10 and A11 comes AA11, between A11 and B11 AB11. This is
//     Movie Magic Screenwriter's "Flipped - A1". Wikipedia calls it the US
//     way (in American slating, 10A would read as set-up A of scene 10)
//     and the other the UK way; the software above, American, defaults to
//     10A.
//
// In both, a scene before the first numbered one gets A1 (B1 after it),
// and one after the last numbered one the next number: 88 after 87, as
// Final Draft letters only scenes "inserted between previously numbered
// scene headings".

// NumberStyle is how a locked script letters a new scene's number.
type NumberStyle int

const (
	NumberAfter  NumberStyle = iota // 10A: the scene before's number, a letter after it
	NumberBefore                    // A11: a letter before the scene after's number
)

// String is the style as an example: "10A" or "A11".
func (s NumberStyle) String() string {
	if s == NumberBefore {
		return "A11"
	}
	return "10A"
}

// ParseNumberStyle reads a style as String writes it (or "after",
// "before"); anything else is NumberAfter.
func ParseNumberStyle(s string) NumberStyle {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "a11", "a1", "before", "flipped":
		return NumberBefore
	}
	return NumberAfter
}

// NumberOptions say how NumberScenes numbers.
type NumberOptions struct {
	// Locked keeps the numbers there are: a scene between two numbered
	// ones gets a letter (Style), one after the last the next number.
	// Unlocked, the scenes without a number count on from the last whole
	// number before them: 1, 2, 3, or 8 after a scene numbered 7.
	Locked bool
	Style  NumberStyle
}

// NumberScenes gives each scene heading without a number (Fountain's
// #12#) one, as the options say; the others keep theirs.
func NumberScenes(s Screenplay, o NumberOptions) Screenplay {
	out := make(Screenplay, len(s))
	copy(out, s)
	var at []int
	var nums []string
	for i, l := range out {
		if l.Type == TypeScene {
			_, n := SceneNumber(l.Contents)
			at, nums = append(at, i), append(nums, n)
		}
	}
	for k, n := range NumberLabels(nums, o) {
		if nums[k] == "" {
			out[at[k]].Contents = WithSceneNumber(out[at[k]].Contents, n)
		}
	}
	return out
}

// NumberLabels are the scenes' numbers, in the script's order, with the
// empty ones ("": no number yet) filled in as NumberScenes does.
func NumberLabels(nums []string, o NumberOptions) []string {
	out := append([]string(nil), nums...)
	if !o.Locked {
		n := 0
		for i, num := range out {
			if num != "" {
				if v, err := strconv.Atoi(num); err == nil {
					n = v
				}
				continue
			}
			n++
			out[i] = strconv.Itoa(n)
		}
		return out
	}
	taken := newTaken()
	anchors := false
	for _, num := range nums {
		if num != "" {
			taken.add(num)
			if _, ok := anchorLabel(num, true); ok {
				anchors = true
			}
		}
	}
	if !anchors { // nothing to keep: count
		n := 0
		for i, num := range out {
			if num != "" {
				continue
			}
			for n++; taken.has(strconv.Itoa(n)); n++ {
			}
			out[i] = strconv.Itoa(n)
			taken.add(out[i])
		}
		return out
	}
	for i, num := range out {
		if num != "" {
			continue
		}
		var prev, next *label
		for j := i - 1; j >= 0 && prev == nil; j-- {
			if l, ok := anchorLabel(out[j], true); ok {
				prev = &l
			}
		}
		for j := i + 1; j < len(nums) && next == nil; j++ {
			if l, ok := anchorLabel(nums[j], false); ok {
				next = &l
			}
		}
		out[i] = nextNumber(prev, next, taken, o.Style)
		taken.add(out[i])
	}
	return out
}

// label is a scene number taken apart: A11B is lead "A", number 11,
// tail "B".
type label struct {
	lead, tail string
	num        int
	digits     string
}

var labelForm = regexp.MustCompile(`^([A-Za-z]*)([0-9]+)([A-Za-z]*)$`)

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

// String is the label as written: A11B.
func (l label) String() string { return l.lead + l.digits + l.tail }

// head is the label up to its tail's letters (A11 of A11B); rest from
// its number on (11B).
func (l label) head() string { return l.lead + l.digits }
func (l label) rest() string { return l.digits + l.tail }

// nextNumber is the number for a new scene between the numbered scenes
// prev and next (nil: none that side), not one of taken: see NumberStyle.
func nextNumber(prev, next *label, taken *takenSet, style NumberStyle) string {
	if n := between(prev, next, taken, style); n != "" {
		return n
	}
	// nothing free between them (a range takes it all): after prev, else
	// past every number taken
	if prev != nil {
		if n := after(*prev, nil, taken); n != "" {
			return n
		}
	}
	return taken.beyond(prev)
}

// between is nextNumber's number, "" if every one there is taken.
func between(prev, next *label, taken *takenSet, style NumberStyle) string {
	switch {
	case prev == nil && next == nil:
		for n := 1; ; n++ {
			if !taken.has(strconv.Itoa(n)) {
				return strconv.Itoa(n)
			}
		}
	case prev == nil: // before the first: A1
		return before(nil, *next, taken)
	case next == nil: // after the last: the next number
		if n := strconv.Itoa(prev.num + 1); !taken.has(n) {
			return n
		}
		return after(*prev, nil, taken)
	case style == NumberBefore:
		if prev.rest() == next.rest() && beforeKey(prev.lead) < beforeKey(next.lead) {
			return before(&prev.lead, *next, taken)
		}
		return before(nil, *next, taken)
	case prev.lead != "" && prev.rest() == next.rest() && beforeKey(prev.lead) < beforeKey(next.lead):
		return before(&prev.lead, *next, taken) // after A1, before 1: B1
	default:
		if prev.head() == next.head() && afterKey(prev.tail) < afterKey(next.tail) {
			return after(*prev, &next.tail, taken)
		}
		return after(*prev, nil, taken)
	}
}

// SceneNumberBetween is the number for a new scene between the scenes
// numbered prev and next in a locked script ("": none that side), not
// one of taken: see NumberStyle. A number that is not digits with
// letters (Storyboarder's 3-x2k9q) counts as none; a range (12-14, an
// OMITTED run's) is its last number before, its first after, and takes
// every number in it (NumberRange).
func SceneNumberBetween(prev, next string, taken map[string]bool, style NumberStyle) string {
	var p, n *label
	if l, ok := anchorLabel(prev, true); ok {
		p = &l
	}
	if l, ok := anchorLabel(next, false); ok {
		n = &l
	}
	t := newTaken()
	for k, v := range taken {
		if v {
			t.add(k)
		}
	}
	return nextNumber(p, n, t, style)
}

// rank orders the letters: the small a, Wikipedia's filler for "before
// the A", below A … Z.
func rank(r rune) int {
	if r == 'a' {
		return 0
	}
	return int(unicode.ToUpper(r)-'A') + 1
}

const top = 27 // above every letter

// afterKey orders tails: 10 < 10A < 10AA < 10B, the shorter first.
func afterKey(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		b = append(b, byte('0'+rank(r)))
	}
	return string(b)
}

// beforeKey orders leads: AA11 < A11 < AB11 < B11 < 11. The letter next
// to the number counts first; a letter put before a lead goes before it.
func beforeKey(s string) string {
	r := []rune(s)
	b := make([]byte, 0, len(r)+1)
	for i := len(r) - 1; i >= 0; i-- {
		b = append(b, byte('0'+rank(r[i])))
	}
	return string(append(b, '0'+top))
}

// letters calls f with the strings of n letters, in order of their
// ranks; filler allows the small a but not at the end. It stops when f
// does.
func letters(n int, filler bool, f func(string) bool) bool {
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var rec func(prefix string, left int) bool
	rec = func(prefix string, left int) bool {
		set := alphabet
		if filler && left > 1 {
			set = "a" + alphabet
		}
		for _, r := range set {
			s := prefix + string(r)
			if left == 1 {
				if f(s) {
					return true
				}
			} else if rec(s, left-1) {
				return true
			}
		}
		return false
	}
	return rec("", n)
}

// moreLetters is how many letters after and before look at past the
// longest of their neighbours': past it they give up ("").
const moreLetters = 2

// after is the first free number after prev (and before its tail upper,
// if given), with the fewest letters: prev's head and a tail.
func after(prev label, upper *string, taken *takenSet) string {
	lo := afterKey(prev.tail)
	most := len(prev.tail)
	if upper != nil {
		most = max(most, len(*upper))
	}
	most += moreLetters
	var found string
	for n := 1; found == "" && n <= most; n++ {
		letters(n, true, func(s string) bool {
			k := afterKey(s) // the whole tail: 10B after 10A
			if k <= lo || (upper != nil && k >= afterKey(*upper)) || taken.has(prev.head()+s) {
				return false
			}
			found = prev.head() + s
			return true
		})
	}
	return found
}

// before is the first free number before next (and after the lead lower,
// if given), with the fewest letters: a lead and next's number and tail.
func before(lower *string, next label, taken *takenSet) string {
	hi := beforeKey(next.lead)
	most := len(next.lead)
	if lower != nil {
		most = max(most, len(*lower))
	}
	most += moreLetters
	var found string
	for n := 1; found == "" && n <= most; n++ {
		// the letter next to the number first, as the key orders them
		letters(n, false, func(rev string) bool {
			r := []rune(rev)
			for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
				r[i], r[j] = r[j], r[i]
			}
			lead := string(r)
			k := beforeKey(lead)
			if k >= hi || (lower != nil && k <= beforeKey(*lower)) || taken.has(lead+next.rest()) {
				return false
			}
			found = lead + next.rest()
			return true
		})
	}
	return found
}

// WithSceneNumber is a heading with the number n (#n#): in place of the
// one it has, else at its end, before the notes there ("" takes it out).
func WithSceneNumber(heading, n string) string {
	body, notes := heading, ""
	if m := trailingNotes.FindStringIndex(heading); m != nil {
		body, notes = heading[:m[0]], heading[m[0]:]
	}
	if m := sceneNumber.FindStringIndex(body); m != nil {
		body = body[:m[0]]
	}
	body = strings.TrimRightFunc(body, unicode.IsSpace)
	if n != "" {
		body += " #" + n + "#"
	}
	if notes != "" {
		notes = " " + strings.TrimLeftFunc(notes, unicode.IsSpace)
	}
	return body + notes
}

var trailingNotes = regexp.MustCompile(`(?s)(\s*\[\[.*?\]\])+\s*$`)

// Omitted is the heading text of an omitted scene, OMITTED, as the
// shooting script prints it: in Fountain ".OMITTED #12#", the old scene
// kept below it in boneyard (fountain.OmitScene).
const Omitted = "OMITTED"

// OmittedNumber is the number of an omitted scene's heading ("OMITTED
// #12#", with or without Fountain's dot), and whether it is one.
func OmittedNumber(heading string) (string, bool) {
	text, n := SceneNumber(heading)
	text = strings.TrimPrefix(strings.TrimSpace(text), ".")
	return n, strings.EqualFold(strings.TrimSpace(text), Omitted)
}
