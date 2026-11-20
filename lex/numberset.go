package lex

import (
	"fmt"
	"strings"
)

// NumberSet is a list of scenes as a writer types it in a dialog: "5,
// 12A, 20-24". A range is list syntax only, never a scene's number: X-Y
// is every scene from the one labelled X to the one labelled Y as the
// script has them, whatever their labels ("I-1-A – PROLOGUE" too, and
// "4-2" where the scene labelled 4 comes before the one labelled 2), the
// scenes without a number between them included. Labels may hold a dash
// (I-1-A), so:
//
//   - items are split by commas, semicolons and new lines; inside one,
//     spaces split too;
//   - "–", "—", ".." and "…" join two labels into a range, and so does a
//     "-" with spaces around it ("20 - 24");
//   - a lone "-" inside a word ("20-24") is a range only when the word is
//     no label of the script and it splits at exactly one of its dashes
//     into two labels the script has ("I-1-A-I-2-A" splits once, at the
//     dash between them, if those are the labels); split more ways it is
//     an error, to be written with "–" or "..". A word that splits at
//     one dash into a label the script has and a scheme number it has
//     not ("1-99") is a range with an end missing: an error. Otherwise a
//     word no scene has is an unknown label (3-x2k9q).
//
// A range's ends are found in the script's order: X is the first scene
// labelled X, Y the first labelled Y at or after it. If Y only comes
// before X, the range is an error (Y comes before X in the script), not
// read backwards. A single label no scene has is Unknown (a keep list
// still reserves it: it is never given); a range with an end no scene
// has is an error.
type NumberSet struct {
	Items   []SetItem
	Unknown []string // single labels no scene has
	Errors  []string // items not read, each saying why
	scenes  []bool
}

// SetItem is one item of a list: the scenes First…Last (indices into the
// labels, in order; -1 for an unknown label or an error).
type SetItem struct {
	Text        string
	First, Last int
	Range       bool
}

// isRangeOp tells whether a field joins two labels into a range.
func isRangeOp(f string) bool {
	switch f {
	case "-", "–", "—", "..", "…":
		return true
	}
	return false
}

// ParseNumberSet reads a list against the script's scene labels, in its
// order ("" for a scene without a number).
func ParseNumberSet(s string, labels []string) NumberSet {
	set := NumberSet{scenes: make([]bool, len(labels))}
	first := func(l string, from int) int {
		for i := from; i < len(labels); i++ {
			if labels[i] == l && l != "" {
				return i
			}
		}
		return -1
	}
	has := func(l string) bool { return first(l, 0) >= 0 }
	spacer := strings.NewReplacer("–", " – ", "—", " — ", "..", " .. ", "…", " … ")
	addRange := func(text, x, y string) {
		it := SetItem{Text: text, First: -1, Last: -1, Range: true}
		switch {
		case !has(x) && !has(y):
			set.Errors = append(set.Errors, fmt.Sprintf("%s: no scene %s or %s", text, x, y))
		case !has(x):
			set.Errors = append(set.Errors, fmt.Sprintf("%s: no scene %s", text, x))
		case !has(y):
			set.Errors = append(set.Errors, fmt.Sprintf("%s: no scene %s", text, y))
		default:
			a := first(x, 0)
			b := first(y, a)
			if b < 0 {
				set.Errors = append(set.Errors, fmt.Sprintf("%s: scene %s comes before scene %s in the script", text, y, x))
				break
			}
			it.First, it.Last = a, b
			for i := a; i <= b; i++ {
				set.scenes[i] = true
			}
		}
		set.Items = append(set.Items, it)
	}
	addOne := func(f string) {
		if i := first(f, 0); i >= 0 {
			set.Items = append(set.Items, SetItem{Text: f, First: i, Last: i})
			for k, l := range labels {
				if l == f {
					set.scenes[k] = true
				}
			}
			return
		}
		var splits [][2]string
		for k := 0; k < len(f); k++ {
			if f[k] == '-' && has(f[:k]) && has(f[k+1:]) {
				splits = append(splits, [2]string{f[:k], f[k+1:]})
			}
		}
		if len(splits) == 0 { // one side a scene's: meant as a range
			for k := 1; k < len(f)-1; k++ {
				a, b := f[:k], f[k+1:]
				if f[k] == '-' && (has(a) && InNumberScheme(b) || has(b) && InNumberScheme(a)) {
					splits = append(splits, [2]string{f[:k], f[k+1:]})
				}
			}
			if len(splits) != 1 {
				splits = nil
			}
		}
		switch len(splits) {
		case 0:
			set.Items = append(set.Items, SetItem{Text: f, First: -1, Last: -1})
			set.Unknown = append(set.Unknown, f)
		case 1:
			addRange(f, splits[0][0], splits[0][1])
		default:
			var ways []string
			for _, sp := range splits {
				ways = append(ways, sp[0]+" – "+sp[1])
			}
			set.Errors = append(set.Errors, fmt.Sprintf("%s: a range read more than one way (%s): write it with – or ..", f, strings.Join(ways, " or ")))
			set.Items = append(set.Items, SetItem{Text: f, First: -1, Last: -1, Range: true})
		}
	}
	for _, chunk := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' || r == '\n' }) {
		fields := strings.Fields(spacer.Replace(chunk))
		for k := 0; k < len(fields); k++ {
			f := fields[k]
			switch {
			case isRangeOp(f):
				set.Errors = append(set.Errors, fmt.Sprintf("%s: a range needs a scene before it", strings.TrimSpace(chunk)))
			case k+2 < len(fields) && isRangeOp(fields[k+1]) && !isRangeOp(fields[k+2]):
				addRange(f+" "+fields[k+1]+" "+fields[k+2], f, fields[k+2])
				k += 2
			case k+1 < len(fields) && isRangeOp(fields[k+1]):
				set.Errors = append(set.Errors, fmt.Sprintf("%s: a range needs a scene after it", strings.TrimSpace(chunk)))
				k = len(fields)
			default:
				addOne(f)
			}
		}
	}
	return set
}

// Has tells whether the scene at index i (into the labels the set was
// read against) is in the set.
func (s NumberSet) Has(i int) bool { return i >= 0 && i < len(s.scenes) && s.scenes[i] }

// Empty tells whether the set names nothing.
func (s NumberSet) Empty() bool { return len(s.Items) == 0 }

// String is the items as typed, joined by commas.
func (s NumberSet) String() string {
	var out []string
	for _, it := range s.Items {
		out = append(out, it.Text)
	}
	return strings.Join(out, ", ")
}

// Read is how the set was read, for under the field it was typed in:
// "5 · 12A · 20 to 24 (5 scenes) · 99 (no such scene)".
func (s NumberSet) Read(labels []string) string {
	name := func(i int) string {
		if labels[i] == "" {
			return fmt.Sprintf("scene %d (no number)", i+1)
		}
		return labels[i]
	}
	var out []string
	for _, it := range s.Items {
		switch {
		case it.First < 0 && it.Range:
			// an error: said apart
		case it.First < 0:
			out = append(out, it.Text+" (no such scene)")
		case it.Range && it.First == it.Last:
			out = append(out, name(it.First))
		case it.Range:
			out = append(out, fmt.Sprintf("%s to %s (%d scenes)", name(it.First), name(it.Last), it.Last-it.First+1))
		default:
			out = append(out, it.Text)
		}
	}
	return strings.Join(out, " · ")
}
