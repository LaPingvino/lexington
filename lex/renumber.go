package lex

import "strconv"

// Renumbering a script's scenes, as a strategy says: which scenes get a
// new number (the free ones; the others are pinned and keep theirs), what
// becomes of the A-scenes, and of the OMITTED ones.
//
// The free scenes are counted on from the pinned scene before them (1, 2,
// 3 … after none, 8 after a pinned 7), skipping a number that is taken (a
// pinned scene's, an OMITTED one kept, a reserved one: a number that went
// out and must not come back on another scene). When the count would
// reach the next pinned number, or pass it, the scene gets a letter
// between its neighbours instead, as a locked script gives a new scene
// one (SceneNumberBetween): so pinning a number never moves it, and the
// numbers stay in order around it.

// A scene whose label is out of the scheme (labels.go: I-1-A, PROLOGUE)
// is pinned whatever the options say, OMITTED or not: never renumbered,
// never dropped, its label never given to another scene, and no
// neighbour (the free scenes after it count on from the scheme number
// before it).

// OmittedMode is what Renumber does with a free OMITTED scene.
type OmittedMode int

const (
	// OmittedDrop takes its number away: the heading goes and the gap
	// closes (the scene's text is the caller's to keep).
	OmittedDrop OmittedMode = iota
	// OmittedKeep keeps it with its number: the gap stays. A run of
	// OMITTED scenes prints as one heading only when asked
	// (MergeOmittedRuns); the text keeps one a scene.
	OmittedKeep
)

// RenumberScene is a scene as Renumber sees it: its number ("" none) and
// whether its heading is OMITTED.
type RenumberScene struct {
	Number  string
	Omitted bool
}

// RenumberOptions are the strategy.
type RenumberOptions struct {
	// Free are the scenes that get a new number (nil: all); the others
	// keep theirs.
	Free []bool
	// KeepLetters keeps an A-scene lettered: 10, 10A, 11 renumbered
	// after a cut become 9, 9A, 10. Without it every free scene gets a
	// whole number (unless a pinned number forces a letter).
	KeepLetters bool
	Omitted     OmittedMode
	// Reserved are labels never given, though no scene has them now
	// (a number that went out with an issue).
	Reserved []string
	Style    NumberStyle
}

// Renumbered is what becomes of one scene.
type Renumbered struct {
	Number string // its number now ("" none)
	// Drop: an OMITTED scene whose heading goes, with its number.
	Drop bool
}

// Renumber numbers the scenes as o says: one result a scene, in order.
// A pinned scene, a fixed label's and an OMITTED one kept, has its
// number unchanged; no number is given that a pinned scene or o.Reserved
// has, nor twice.
func Renumber(scenes []RenumberScene, o RenumberOptions) []Renumbered {
	out := make([]Renumbered, len(scenes))
	free := make([]bool, len(scenes))
	read := make([]sceneNo, len(scenes))
	for i, sc := range scenes {
		read[i] = readNo(sc.Number, sc.Omitted)
		out[i].Number = sc.Number
		free[i] = (o.Free == nil || (i < len(o.Free) && o.Free[i])) && !read[i].fixed()
		if sc.Omitted && free[i] {
			if o.Omitted == OmittedDrop {
				out[i] = Renumbered{Drop: true}
			}
			free[i] = false // kept: pinned
		}
	}
	taken := newTaken()
	for _, r := range o.Reserved {
		taken.add(r)
	}
	var seq []int // the scenes that stay, in order
	for i := range scenes {
		if out[i].Drop {
			continue
		}
		seq = append(seq, i)
		if !free[i] {
			taken.addNo(read[i])
		}
	}
	lettered := func(n string) bool {
		l, ok := parseLabel(n)
		return ok && (l.lead != "" || l.tail != "")
	}
	// the next pinned scene's number, as a neighbour, after seq[k]
	nextPinned := func(k int) *label {
		for j := k + 1; j < len(seq); j++ {
			if i := seq[j]; !free[i] {
				if l := read[i].next(); l != nil {
					return l
				}
			}
		}
		return nil
	}
	// first pass: the whole numbers
	var later []int // the A-scenes kept lettered, numbered after
	var prev *label
	for k, i := range seq {
		if !free[i] {
			if l := read[i].prev(); l != nil {
				prev = l
			}
			continue
		}
		if o.KeepLetters && lettered(scenes[i].Number) {
			out[i].Number = ""
			later = append(later, k)
			continue
		}
		next := nextPinned(k)
		n := 1
		if prev != nil {
			n = prev.num
			if prev.lead == "" {
				n++
			}
		}
		below := func(n int) bool {
			if next == nil {
				return true
			}
			l, _ := parseLabel(strconv.Itoa(n))
			return compareLabels(l, *next) < 0
		}
		for taken.has(strconv.Itoa(n)) && below(n) {
			n++
		}
		num := strconv.Itoa(n)
		if !below(n) || taken.has(num) {
			num = nextNumber(prev, next, taken, o.Style)
		}
		out[i].Number = num
		taken.add(num)
		if l, ok := parseLabel(num); ok {
			prev = &l
		}
	}
	// then the A-scenes, between their neighbours as numbered now
	now := func(j int) sceneNo {
		i := seq[j]
		if free[i] {
			return readNo(out[i].Number, false)
		}
		return read[i]
	}
	for _, k := range later {
		var p, n *label
		for j := k - 1; j >= 0 && p == nil; j-- {
			p = now(j).prev()
		}
		for j := k + 1; j < len(seq) && n == nil; j++ {
			n = now(j).next()
		}
		num := nextNumber(p, n, taken, o.Style)
		out[seq[k]].Number = num
		taken.add(num)
	}
	return out
}
