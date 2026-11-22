package fountain

import (
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// A script read with one language's scene words (rules' Scenes) is a
// dialect of Fountain: another language's reader, the standard one among
// them, may read some of its lines as other elements. Force writes such a
// text so that it reads as the same elements with the other words, and
// Unforce takes that back. Both decide by parsing, line by line (the
// rows' Source), so whatever the language's words change is covered, and
// they change only Fountain's force marks: the lines stay as written
// (no Fountain writer round trip, which would put headings in capitals
// and rewrite the title page).
//
// The marks: "." for a scene heading, "!" for action, "@" for a
// character. Dialogue and parentheticals have none: such a line that the
// other words read as another element stays as it is.

// forceMarks are the marks Force adds, by the element they force.
var forceMarks = map[string]byte{lex.TypeScene: '.', lex.TypeAction: '!', lex.TypeSpeaker: '@', lex.TypeTrans: '>'}

// markedType is the element a line's force mark forces, "" for none (a
// line starting ".." is no forced heading).
func markedType(line string) string {
	if len(line) < 2 {
		return ""
	}
	switch line[0] {
	case '.':
		if line[1] == '.' {
			return ""
		}
		return lex.TypeScene
	case '!':
		return lex.TypeAction
	case '@':
		return lex.TypeSpeaker
	case '>':
		if strings.HasSuffix(strings.TrimSpace(line), "<") {
			return "" // centred
		}
		return lex.TypeTrans
	}
	return ""
}

// forced tells whether a line starts with a force mark of any element.
func forced(line string) bool {
	return line != "" && strings.ContainsRune(".!@~>", rune(line[0]))
}

// elements are the elements text is read as with the scene words lang,
// by line (from 1): each line's first row; the rows the parser adds
// (dual dialogue's, the title page's) left out.
func elements(text string, lang rules.Language) map[int]string {
	return elementsOf(text, lang, false)
}

// Force is text, written for the scene words from, as Fountain that the
// scene words to read as the same elements: a force mark added to each
// line to would read otherwise, where Fountain has one.
func Force(text string, from, to []string) string {
	return ForceIn(text, rules.Words(from), rules.Words(to))
}

// ForceIn is Force with languages' rules: a line one reads as a cue or
// transition by its rules that the other would not gets its mark too.
func ForceIn(text string, from, to rules.Language) string {
	want := elements(text, from)
	lines := strings.Split(text, "\n")
	// a mark changes what the lines after it are read as (a heading read
	// as a character makes the next line dialogue): only the first line
	// of each run that reads otherwise is marked, then read again, until
	// none changes
	for range lines {
		got := elements(strings.Join(lines, "\n"), to)
		changed, inRun := false, false
		for i, line := range lines {
			w, g := want[i+1], got[i+1]
			if w == g {
				inRun = false
				continue
			}
			mark, ok := forceMarks[w]
			if inRun || !ok || forced(line) {
				continue
			}
			lines[i] = string(mark) + line
			changed, inRun = true, true
		}
		if !changed {
			break
		}
	}
	return strings.Join(lines, "\n")
}

// Unforce is Force's text back as the scene words from read it: each
// force mark Force would add taken off, where without it from reads the
// line as the element it forces and to does not. A mark either needs —
// or one to needs none for (".INT. HOUSE" with English's words) — stays.
func Unforce(text string, from, to []string) string {
	return UnforceIn(text, rules.Words(from), rules.Words(to))
}

// UnforceIn is Unforce with languages' rules.
func UnforceIn(text string, from, to rules.Language) string {
	lines := strings.Split(text, "\n")
	now := elements(text, to)
	cands := map[int]string{}
	for i, line := range lines {
		if t := markedType(line); t != "" && now[i+1] == t {
			cands[i] = t
		}
	}
	for len(cands) > 0 {
		try := append([]string(nil), lines...)
		for i := range cands {
			try[i] = lines[i][1:]
		}
		joined := strings.Join(try, "\n")
		mine, theirs := elements(joined, from), elements(joined, to)
		kept := false
		for i, t := range cands {
			if mine[i+1] != t || theirs[i+1] == t {
				delete(cands, i)
				kept = true
			}
		}
		if !kept {
			return joined
		}
	}
	return strings.Join(lines, "\n")
}
