package rules

import (
	"slices"
	"strings"
)

// A heading pattern's grammar (Language.Headings, Language.NewHeading):
// text with slots in braces. The reader (fountain.NewDialect) reads a
// line by it; a program that writes headings (Accolade's New Scene) fills
// it in (FillPattern); one that edits patterns takes it apart and puts it
// together again (ParsePattern, JoinPattern) without changing a byte.

// The slots of a heading pattern.
const (
	SlotNumber = "n"     // a scene number: 12, 12A, A12, ١٢, 十二
	SlotIO     = "io"    // inside or outside: one of Settings or Scenes
	SlotTime   = "time"  // the time of day: one of Times
	SlotPlace  = "place" // where: any text
	SlotAny    = "any"   // any text
)

// Slots are the slots, in the order they are described.
var Slots = []string{SlotNumber, SlotIO, SlotPlace, SlotTime, SlotAny}

// SceneForm is Fountain's own heading as a pattern: a scene word, the
// place, a dash and the time ("INT. KITCHEN - DAY"). It is the form a
// new heading is written in for a language with no NewHeading and no
// Headings (HeadingForm); its {io} is one of the Matchup words (INT.),
// and a line written so is read by its scene word, not as a pattern.
const SceneForm = "{io} {place} - {time}"

// A PatternPart is a slot or the text between slots.
type PatternPart struct {
	Slot string // one of Slots; "" for text
	Text string // the text, for a part that is no slot
}

// ParsePattern takes a pattern apart: its slots and the text between
// them, in order. Braces around anything else ("{x}", "{") are text.
// JoinPattern puts the parts together as they were.
func ParsePattern(p string) []PatternPart {
	var out []PatternPart
	text := func(s string) {
		if s == "" {
			return
		}
		if n := len(out); n > 0 && out[n-1].Slot == "" {
			out[n-1].Text += s
			return
		}
		out = append(out, PatternPart{Text: s})
	}
	for p != "" {
		i := strings.IndexByte(p, '{')
		if i < 0 {
			text(p)
			break
		}
		text(p[:i])
		p = p[i:]
		j := strings.IndexByte(p, '}')
		if j > 0 && slices.Contains(Slots, p[1:j]) {
			out = append(out, PatternPart{Slot: p[1:j]})
			p = p[j+1:]
			continue
		}
		text("{")
		p = p[1:]
	}
	return out
}

// JoinPattern is the pattern of parts (ParsePattern's again).
func JoinPattern(parts []PatternPart) string {
	var b strings.Builder
	for _, pt := range parts {
		if pt.Slot != "" {
			b.WriteString("{" + pt.Slot + "}")
		} else {
			b.WriteString(pt.Text)
		}
	}
	return b.String()
}

// PatternSlots are a pattern's slots, in order.
func PatternSlots(p string) []string {
	var out []string
	for _, pt := range ParsePattern(p) {
		if pt.Slot != "" {
			out = append(out, pt.Slot)
		}
	}
	return out
}

// FillPattern is a heading written by a pattern: each slot's value from
// values, the text between the slots as it is. A slot with no value is
// left out with the text that joins it to the slot before it (the first
// slot with the text after it, and a number with its label before it; the last with the text
// after it too; one between two others with the text after it where
// that is plainer or the text before ends the slot before: "INT.
// KITCHEN", "INT. DAY", "12. 日 内"), so "{n}. {io} {any}" with no {any}
// is "12. INT":
// whether what is left reads as a heading is the language's to say
// (fountain.Dialect.Heading).
func FillPattern(p string, values map[string]string) string {
	return FillParts(ParsePattern(p), values)
}

// FillParts is FillPattern for a pattern taken apart, whose text parts
// may be other text than the pattern's: a heading's own, as it was typed
// between its slots ("场景12  厨房", two spaces), so a slot left out or
// put in changes nothing else of it (fountain.Dialect.WithHeadingNumber).
func FillParts(parts []PatternPart, values map[string]string) string {
	keep := FillKept(parts, values)
	var b strings.Builder
	for i, pt := range parts {
		switch {
		case !keep[i]:
		case pt.Slot != "":
			b.WriteString(strings.TrimSpace(values[pt.Slot]))
		default:
			b.WriteString(pt.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

// FillKept are the parts FillParts writes, part by part: the slots with
// a value and the text it keeps beside them.
func FillKept(parts []PatternPart, values map[string]string) []bool {
	keep := make([]bool, len(parts))
	for i, pt := range parts {
		keep[i] = pt.Slot == "" || strings.TrimSpace(values[pt.Slot]) != ""
	}
	first, last := -1, -1
	for i, pt := range parts {
		if pt.Slot != "" {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	for i, pt := range parts {
		if pt.Slot == "" || keep[i] {
			continue
		}
		switch {
		case i == first:
			// the text before a number is its label (场景, SÉQUENCE, مشهد):
			// it goes with it; before another slot it says what the line
			// is (○), and stays
			number := pt.Slot == SlotNumber
			if i > 0 && number {
				keep[i-1] = false
			}
			if i+1 < len(parts) && parts[i+1].Slot == "" && i+1 != len(parts)-1 &&
				(number || closes(parts[i+1].Text) || strings.TrimSpace(parts[i+1].Text) == "") {
				keep[i+1] = false
			}
		case i != last && i > 0 && parts[i-1].Slot == "" && i+1 < len(parts) && parts[i+1].Slot == "" && keep[i-1] &&
			(closes(parts[i-1].Text) || !closes(parts[i+1].Text) && len(strings.TrimSpace(parts[i-1].Text)) <= len(strings.TrimSpace(parts[i+1].Text))):
			// between two kept: the text that ends the slot before stays
			// ("12. 日 内"), else the plainer ("INT. DAY", not "INT. - DAY")
			keep[i+1] = false
		default:
			if i > 0 && parts[i-1].Slot == "" {
				keep[i-1] = false
			}
		}
		if i == last && i+1 < len(parts) && parts[i+1].Slot == "" {
			keep[i+1] = false
		}
	}
	return keep
}

// closes tells whether text between slots starts with a sign that ends
// the slot before it (the dot of "12. ", the slash of "/"), not a space,
// a dash or a bracket that starts the one after.
func closes(text string) bool {
	for _, r := range text {
		return !strings.ContainsRune(" \t-–—([（【", r)
	}
	return false
}

// HeadingForm is the pattern a new scene heading in the language is
// written by: NewHeading, else the first of Headings, else SceneForm.
func (l Language) HeadingForm() string {
	switch {
	case strings.TrimSpace(l.NewHeading) != "":
		return l.NewHeading
	case len(l.Headings) > 0 && strings.TrimSpace(l.Headings[0]) != "":
		return l.Headings[0]
	}
	return SceneForm
}

// IOWords are the words for inside and outside a heading written by form
// takes in its {io}, the commonest first: for a form that starts with
// {io} (SceneForm), the Matchup words, which are scene words (INT.,
// EXT., INT./EXT., EXT./INT., EST.); for a form with {io} after other
// slots, Settings (内, 外) and then the Matchup words, which the
// pattern's {io} reads too. The scene words if neither has any; nil for
// a form without {io}.
func (l Language) IOWords(form string) []string {
	parts := ParsePattern(form)
	at := slices.IndexFunc(parts, func(p PatternPart) bool { return p.Slot == SlotIO })
	if at < 0 {
		return nil
	}
	var out []string
	seen := map[string]bool{} // a word once, with its dot or without (अंदर, अंदर.)
	add := func(ws []string) {
		for _, w := range ws {
			w = strings.TrimSpace(w)
			if k := strings.ToUpper(strings.TrimSuffix(w, ".")); k != "" && !seen[k] {
				seen[k] = true
				out = append(out, w)
			}
		}
	}
	if at > 0 {
		add(l.Settings)
	}
	add(l.Matchup)
	if len(out) == 0 {
		add(l.Scenes)
	}
	return out
}

// PatternVariants are a pattern and the patterns it is with slots left
// out (Joop: "the new format keeps working when used partially"), as
// FillPattern leaves an empty slot out with the text joining it: "{n}.
// {place} {io}/{time}" is also "{n}. {place} {io}" and "{place}
// {io}/{time}". A pattern with slots left out must still say it is a
// heading: a word of the language's ({io} or {time}) or the text it
// starts with ("场景", "مشهد"), and besides it a second sign — another
// such, a number, or a sign between the slots kept ("/", ".", "（") — so
// "{place} {io}" (any sentence ending in INNEN) and "{n}. {place}" (a
// numbered list) are no headings. The pattern itself comes first, as
// it is, whatever it has.
func PatternVariants(p string) []string {
	out := []string{p}
	slots := PatternSlots(p)
	if len(slots) > 8 {
		return out
	}
	for mask := 1; mask < 1<<len(slots); mask++ {
		vals := map[string]string{}
		parts := ParsePattern(p)
		// a slot's own placeholder; slots twice in a pattern go together
		k := 0
		for _, pt := range parts {
			if pt.Slot == "" {
				continue
			}
			if mask&(1<<k) == 0 {
				vals[pt.Slot] = "{" + pt.Slot + "}"
			}
			k++
		}
		if len(vals) == 0 {
			continue
		}
		v := FillPattern(p, vals)
		if v != p && signs(v) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// signs tells whether a pattern with slots left out still says it is a
// heading (PatternVariants).
func signs(p string) bool {
	parts := ParsePattern(p)
	word, n := false, 0
	for i, pt := range parts {
		switch {
		case pt.Slot == SlotIO || pt.Slot == SlotTime:
			word = true
			n++
		case pt.Slot == SlotNumber:
			n++
		case i == 0 && strings.TrimSpace(pt.Text) != "": // the text it starts with
			word = true
			n++
		case strings.TrimFunc(pt.Text, func(r rune) bool { return r == ' ' || r == '-' || r == '–' || r == '—' }) != "":
			n++
		}
	}
	return word && n >= 2
}
