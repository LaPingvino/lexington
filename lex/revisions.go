package lex

import (
	"strconv"
	"strings"
	"time"
)

// Production revisions: once a script is issued (White), each later
// issue is a revision set in a colour, and its changed lines are marked
// with an asterisk in the right margin, its changed pages with a header
// naming the set and its date. A line's set is in Line.Revision (the
// set's name); which lines changed is for the caller to say (Accolade
// diffs the issued drafts; an FDX gives each text run its RevisionID).
//
// The order of the sets is the WGA's, as Quote-Unquote (Highland's
// makers) gives "The WGA script revision order"
// (https://blog.quoteunquoteapps.com/script-revision-colors-how-to-track-your-screenplay-changes/),
// and Final Draft's knowledge base by a search's snippet of it ("Per the
// WGA west, the standard script revision color sets are: …"; the page
// itself refuses robots: https://kb.finaldraft.com/hc/en-us/articles/15575314119316):
// White, Blue, Pink, Yellow, Green, Goldenrod, Buff, Salmon, Cherry, then
// Second Blue to Second Cherry (no second White), and on: Third Blue ….
// Productions do differ (Wikipedia's Shooting script has tan and ivory
// after cherry, then "double white"; Final Draft's own default sets end
// Cherry, Tan, 2nd White): an FDX's own names are kept as they are.

// RevisionColors are the revision colours in their order, White first.
var RevisionColors = []string{"White", "Blue", "Pink", "Yellow", "Green", "Goldenrod", "Buff", "Salmon", "Cherry"}

// cycleNames are the words before a colour in its second, third … round.
var cycleNames = []string{"", "Second", "Third", "Fourth", "Fifth", "Sixth", "Seventh", "Eighth", "Ninth", "Tenth"}

// RevisionName is the name of the n-th set issued (0: White, the first
// issue; 1: Blue; 8: Cherry; 9: Second Blue … 16: Second Cherry; 17:
// Third Blue).
func RevisionName(n int) string {
	if n <= 0 {
		return RevisionColors[0]
	}
	round, at := (n-1)/8, (n-1)%8
	c := RevisionColors[1+at]
	if round == 0 {
		return c
	}
	if round < len(cycleNames) {
		return cycleNames[round] + " " + c
	}
	return c + " " + strconv.Itoa(round+1)
}

// RevisionIndex is the place of a set's name in the order (RevisionName's
// n), or -1 for a name that is not one of them.
func RevisionIndex(name string) int {
	for n := 0; n < 9*8+1; n++ {
		if strings.EqualFold(strings.TrimSpace(name), RevisionName(n)) {
			return n
		}
	}
	return -1
}

// RevisionColor is the colour a set's name is in ("Second Blue Rev." is
// Blue), "" when it names none.
func RevisionColor(name string) string {
	words := strings.Fields(strings.ToLower(name))
	for _, w := range words {
		w = strings.Trim(w, ".,()")
		for _, c := range RevisionColors {
			if w == strings.ToLower(c) {
				return c
			}
		}
	}
	return ""
}

// revisionPaper are the colours of the paper of each set, as Final Draft
// writes them in its default sets' PageColor (a real FDX's <Revisions>,
// 16 bits a channel taken to 8).
var revisionPaper = map[string][3]uint8{
	"White": {255, 255, 255}, "Blue": {198, 237, 254}, "Pink": {253, 204, 212}, "Yellow": {253, 255, 171},
	"Green": {209, 254, 208}, "Goldenrod": {250, 205, 57}, "Buff": {252, 237, 157}, "Salmon": {248, 174, 142},
	"Cherry": {251, 164, 180},
}

// revisionInk are the colours a set's marks are shown in on a screen:
// darker than the paper, to stand out on white and on a dark theme.
var revisionInk = map[string][3]uint8{
	"White": {160, 160, 160}, "Blue": {52, 120, 220}, "Pink": {226, 92, 160}, "Yellow": {204, 170, 0},
	"Green": {46, 160, 67}, "Goldenrod": {218, 165, 32}, "Buff": {196, 154, 92}, "Salmon": {240, 112, 90},
	"Cherry": {206, 22, 64},
}

// RevisionPaper is the colour of a set's paper, ok false for a name with
// no colour in it.
func RevisionPaper(name string) (rgb [3]uint8, ok bool) {
	rgb, ok = revisionPaper[RevisionColor(name)]
	return
}

// RevisionInk is the colour a set's marks are shown in on a screen.
func RevisionInk(name string) (rgb [3]uint8, ok bool) {
	rgb, ok = revisionInk[RevisionColor(name)]
	return
}

// RevisionSet is one issued (or open) set.
type RevisionSet struct {
	Name string    // "Blue", or a name of the file's own ("Blue Rev. (mm/dd/yy)")
	Date time.Time // when issued (zero: not known)
	Mark string    // the mark in the margin ("": *)
}

// Revisions is how a script's revisions are printed.
type Revisions struct {
	// Sets are the sets, oldest first: a line's Revision names one.
	Sets []RevisionSet
	// Header is the header of a changed page: {set} the set's name,
	// {date} its date (DefaultRevisionHeader when empty; "-" none).
	Header string
	// DateLayout is how the date is written (Go's layout; DefaultDateLayout
	// when empty).
	DateLayout string
	// PagesOnly prints only the pages with a change on them, with their
	// numbers as in the whole script.
	PagesOnly bool
	// PageColors prints a changed page on its set's paper (a tint).
	PageColors bool
}

// DefaultRevisionHeader and DefaultDateLayout are a changed page's header:
// "Blue Revision – 20/11/26".
const (
	DefaultRevisionHeader = "{set} Revision – {date}"
	DefaultDateLayout     = "02/01/06"
)

// Set is the set named name, ok false if none is.
func (r *Revisions) Set(name string) (s RevisionSet, index int, ok bool) {
	if r == nil {
		return s, -1, false
	}
	for i, s := range r.Sets {
		if s.Name == name {
			return s, i, true
		}
	}
	return s, -1, false
}

// HeaderFor is the header of a page whose latest change is in set s.
func (r *Revisions) HeaderFor(s RevisionSet) string {
	h := DefaultRevisionHeader
	layout := DefaultDateLayout
	if r != nil && r.Header != "" {
		h = r.Header
	}
	if r != nil && r.DateLayout != "" {
		layout = r.DateLayout
	}
	if h == "-" {
		return ""
	}
	date := ""
	if !s.Date.IsZero() {
		date = s.Date.Format(layout)
	}
	h = strings.ReplaceAll(h, "{set}", s.Name)
	h = strings.ReplaceAll(h, "{date}", date)
	return strings.TrimRight(strings.TrimSpace(h), " –-")
}

// MarkOf is the mark a set's lines get in the margin.
func (s RevisionSet) MarkOf() string {
	if s.Mark == "" {
		return "*"
	}
	return s.Mark
}

// Revised tells whether any line of s is marked.
func Revised(s Screenplay) bool {
	for _, l := range s {
		if l.Revision != "" {
			return true
		}
	}
	return false
}
