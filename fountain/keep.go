package fountain

import (
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Lexington before 1.19 read Fountain more loosely than the spec
// (ParseLoose): a scene word started a heading anywhere, a line in
// capitals right after action was a character cue, any line ending in
// " TO:" a transition, and a line of spaces ended a speech. KeepLoose
// writes a script that relied on that so that Parse reads it as
// ParseLoose did, the lines otherwise as written:
//
//   - a heading or a cue without a blank line before it gets one (or,
//     where a blank line would change the line before, its force mark,
//     "." or "@");
//   - a transition the spec does not see gets ">";
//   - a line of spaces that ended a speech is emptied;
//   - another line that reads otherwise is forced where Fountain has a
//     mark for it ("!" action, "." heading, "@" character).
//
// So too a heading by a scene word that needs its dot from 1.67.3 (ENE.,
// INNEN.; rules.Undotted) written without it: "ENE KUIREJO" is forced,
// ".ENE KUIREJO", as is a line the old reading took for one ("Ene de la
// domo…" after a blank line, which the old reading made a heading).
//
// It decides by parsing both ways, line by line, as Force does.

// KeepLoose is text, read with the scene words lang, written so that
// Parse reads each of its lines as ParseLoose reads them; places is how
// many lines it added or changed (0: the text is as it was, both read it
// alike).
func KeepLoose(text string, scenes []string) (kept string, places int) {
	return KeepLooseIn(text, rules.Words(scenes))
}

// KeepLooseIn is KeepLoose with a language's rules.
func KeepLooseIn(text string, lang rules.Language) (kept string, places int) {
	want := elementsOf(text, lang, true)
	if sameElements(want, elementsOf(text, lang, false)) {
		return text, 0
	}
	was := strings.Split(text, "\n")
	lines := make([]keptLine, len(was))
	for i, l := range was {
		lines[i] = keptLine{text: l, origin: i + 1}
	}
	// each change can change what the lines after it are: only the first
	// line of each run that reads otherwise is changed, then all is read
	// again, until nothing changes
	for range len(lines) + 1 {
		got := elementsOf(joinKept(lines), lang, false)
		changed, inRun := false, false
		next := make([]keptLine, 0, len(lines)+1)
		for k := 0; k < len(lines); k++ {
			l := lines[k]
			if l.origin == 0 || want[l.origin] == got[k+1] {
				inRun = inRun && l.origin == 0
				next = append(next, l)
				continue
			}
			if inRun {
				next = append(next, l)
				continue
			}
			inRun = true
			w := want[l.origin]
			switch {
			case k+2 < len(lines) && lines[k+1].origin == 0:
				// the blank line added before the next line changed this
				// one ("BOB" before a heading is a name only without
				// it): that line gets its force mark instead
				after := lines[k+2]
				after.mark(want[after.origin])
				next = append(next, l, after)
				k += 2
				changed = true
			case w == lex.TypeEmpty && strings.TrimSpace(l.text) == "":
				next = append(next, keptLine{origin: l.origin}) // spaces that ended a speech
				changed = true
			case (w == lex.TypeScene || w == lex.TypeSpeaker) && !l.marked &&
				len(next) > 0 && strings.TrimSpace(next[len(next)-1].text) != "":
				next = append(next, keptLine{}, l) // the blank line the spec wants
				changed = true
			default:
				if l.mark(w) {
					changed = true
				}
				next = append(next, l)
			}
		}
		lines = next
		if !changed {
			break
		}
	}
	for _, l := range lines {
		if l.origin == 0 || l.text != was[l.origin-1] {
			places++
		}
	}
	return joinKept(lines), places
}

// keptLine is a line of KeepLoose's text: its line in the text it was
// given (from 1; 0 for a blank line added), and whether a force mark was
// tried on it.
type keptLine struct {
	text   string
	origin int
	marked bool
}

// mark puts the force mark for t before the line, if Fountain has one
// and it has none yet.
func (l *keptLine) mark(t string) bool {
	m, ok := keepMarks[t]
	if !ok || l.marked || forced(strings.TrimSpace(l.text)) {
		return false
	}
	l.text, l.marked = string(m)+strings.TrimLeft(l.text, " \t"), true
	return true
}

func joinKept(lines []keptLine) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return strings.Join(out, "\n")
}

// keepMarks are the marks KeepLoose adds, by the element they force.
var keepMarks = map[string]byte{lex.TypeScene: '.', lex.TypeAction: '!', lex.TypeSpeaker: '@', lex.TypeTrans: '>'}

// elementsOf are the elements text is read as with the scene words
// lang, by line (from 1): each line's first row; the rows the parser
// adds (dual dialogue's, the title page's) left out. loose: as ParseLoose
// reads it.
func elementsOf(text string, lang rules.Language, loose bool) map[int]string {
	out := map[int]string{}
	for _, row := range parse(lang, strings.NewReader(text), true, loose) {
		switch row.Type {
		case lex.TypeDualOpen, lex.TypeDualNext, lex.TypeDualClose, lex.TypeTitlePage, lex.TypeNewPage, lex.TypeMetaSection:
			continue
		}
		if _, ok := out[row.Source]; !ok && row.Source > 0 {
			out[row.Source] = row.Type
		}
	}
	return out
}

func sameElements(a, b map[int]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
