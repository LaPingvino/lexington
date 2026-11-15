package fountain

import (
	"bufio"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/lex"
)

// Scene contains all the prefixes the scene detection looks for.
// This can be changed with the toml configuration in the rules package.
var Scene = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

// AllCaps reports whether s is written in capitals: it has a capital and
// no small letters. Scripts without capitals (Hebrew, Arabic, Chinese)
// count as small letters, so their character cues are marked with @.
func AllCaps(s string) bool {
	return s == strings.ToUpper(s) && strings.IndexFunc(s, unicode.IsUpper) >= 0
}

// StartsScene reports whether row starts with a scene heading's word,
// followed by a space or a dot, or by anything when the word ends in a
// sign (Japanese headings: ○喫茶店).
func StartsScene(row, word string) bool {
	if strings.HasPrefix(row, word+" ") || strings.HasPrefix(row, word+".") {
		return true
	}
	last, _ := utf8.DecodeLastRuneInString(word)
	return word != "" && !unicode.IsLetter(last) && !unicode.IsDigit(last) && last != '.' && strings.HasPrefix(row, word)
}

// CheckScene determines if a row is a scene heading.
func CheckScene(row string) (bool, string, string) {
	upperRow := strings.ToUpper(row)

	// Check if any scene prefix matches
	_, found := internal.Find(Scene, func(prefix string) bool { return StartsScene(upperRow, prefix) })

	if found {
		return true, internal.ElementScene, upperRow
	}

	// Check for forced scene (starts with .)
	if strings.HasPrefix(row, ".") && !strings.HasPrefix(row, "..") {
		return true, "scene", row[1:]
	}

	return false, "scene", upperRow
}

// CheckCrow determines if a row is a transition or a centered text.
func CheckCrow(row string) (bool, string, string) {
	var crow bool
	var el string
	// leading spaces are not significant: an indented "> FADE OUT." is
	// still a forced transition
	row = strings.ToUpper(strings.TrimSpace(row))
	if strings.HasPrefix(row, ">") || strings.HasSuffix(row, " TO:") {
		crow = true
		el = "trans"
	}
	if strings.HasPrefix(row, ">") && strings.HasSuffix(row, "<") {
		el = "center"
	}
	return crow, el, strings.Trim(row, ">< ")
}

// CheckEqual determines if a row is a synopsis or a page break.
func CheckEqual(row string) (bool, string, string) {
	var equal bool
	var el string
	if strings.HasPrefix(row, "=") {
		equal = true
		el = "synopse"
	}
	if len(row) >= 3 && strings.Trim(row, "=") == "" {
		el = "newpage"
	}
	return equal, el, strings.TrimLeft(row, "= ")
}

// CheckSection determines if a row is a section heading.
func CheckSection(row string) (bool, string, string) {
	var section bool
	if strings.HasPrefix(row, "#") {
		section = true
	}
	return section, "section", row
}

// CheckForce determines if a row is a forced type (e.g., @speaker, ~lyrics, !action).
func CheckForce(row string) (bool, string, string) {
	force := true
	var ftype string
	if len(row) < 1 {
		return false, "", ""
	}
	switch row[0] {
	case '@':
		ftype = lex.TypeSpeaker
	case '~':
		ftype = lex.TypeLyrics
	case '!':
		ftype = lex.TypeAction
	default:
		force = false
	}
	if force {
		row = row[1:]
	}
	return force, ftype, row
}

// ParseState holds the state needed during parsing
type ParseState struct {
	scenes                []string
	titlepage             bool
	inDialogueContext     bool
	inBoneyard            bool // within /* ... */
	inDualDialogue        bool
	titletag              string
	consecutiveEmptyLines int
	hasTitlePageContent   bool
	out                   lex.Screenplay
}

// Parse converts a Fountain file into the internal lex.Screenplay format.
// TypeBoneyard is a line of Fountain's boneyard (/* ... */): kept, not
// printed.
const TypeBoneyard = "boneyard"

func Parse(scenes []string, file io.Reader) (out lex.Screenplay) {
	Scene = scenes

	toParse := readAllLines(file)

	state := &ParseState{
		scenes:    scenes,
		titlepage: true,
		out:       make(lex.Screenplay, 0),
	}

	for i := 0; i < len(toParse); i++ {
		originalRow := toParse[i]
		row := strings.TrimRight(originalRow, "\n\r")
		if !state.inBoneyard && !strings.HasPrefix(strings.TrimSpace(row), "/*") {
			// a note over several lines: one row
			if j, joined := joinNote(toParse, i); j > i {
				i, row, originalRow = j, joined, joined
			}
		}
		trimmedSpaceRow := strings.TrimSpace(row)

		var currentLine lex.Line
		var isCurrentLineDualSpeakerCandidate bool

		// Fountain's boneyard: lines from one starting with /* up to the
		// one with */ are kept as they are (for writing Fountain again)
		// but not printed: the "boneyard" type has no rule, so it is hidden
		if state.inBoneyard || strings.HasPrefix(trimmedSpaceRow, "/*") {
			if !state.inBoneyard {
				state.inBoneyard = !strings.Contains(trimmedSpaceRow[2:], "*/")
			} else {
				state.inBoneyard = !strings.Contains(trimmedSpaceRow, "*/")
			}
			state.out = append(state.out, lex.Line{Type: TypeBoneyard, Contents: row})
			continue
		}

		// Handle title page parsing
		if state.titlepage {
			next := ""
			if i+1 < len(toParse) {
				next = strings.TrimRight(toParse[i+1], "\n\r")
			}
			if handled := state.handleTitlePage(row, trimmedSpaceRow, next, &currentLine); handled {
				continue
			}
		}

		// Parse screenplay body
		currentLine, isCurrentLineDualSpeakerCandidate = state.parseScreenplayLine(originalRow, row, trimmedSpaceRow)

		// Handle dual dialogue logic
		state.handleDualDialogue(currentLine, isCurrentLineDualSpeakerCandidate, i, len(toParse))

		// Append line if appropriate
		if state.shouldAppendLine(currentLine, trimmedSpaceRow, i, len(toParse)) {
			state.out = append(state.out, currentLine)
		}

		// Update dialogue context for next iteration
		state.updateDialogueContext(currentLine)
	}

	return state.out
}

// openNote is where the note ([[) that s opens and does not close (]])
// starts in s, -1 if there is none.
func openNote(s string) int {
	at := 0
	for {
		open := strings.Index(s[at:], "[[")
		if open < 0 {
			return -1
		}
		open += at
		end := strings.Index(s[open+2:], "]]")
		if end < 0 {
			return open
		}
		at = open + 2 + end + 2
	}
}

// joinNote joins rows[i] with the rows after it up to the one that closes
// the note the row opens, with line breaks: Fountain's notes may span
// lines. A note goes on over a line of two spaces, not over a blank line:
// when one comes first, or the script ends, the "[[" is no note, and
// joinNote returns i. (A row that closes the note and opens another that
// is never closed is the last row joined, as a row by itself would be.)
func joinNote(rows []string, i int) (int, string) {
	joined := strings.TrimRight(rows[i], "\n\r")
	if openNote(joined) < 0 {
		return i, joined
	}
	last, lastJoined := i, joined
	for k := i + 1; k < len(rows); k++ {
		r := strings.TrimRight(rows[k], "\n\r")
		if strings.TrimSpace(r) == "" && !strings.HasPrefix(r, "  ") {
			break
		}
		start := len(joined) + 1 // of row k in joined
		joined += "\n" + r
		if open := openNote(joined); open < 0 {
			return k, joined
		} else if open >= start { // the notes before this row are closed
			last, lastJoined = k, joined
		}
	}
	return last, lastJoined
}

// The notes of a row are masked while it is read, so that they do not
// change what it is ("BOB [[note]]" is a speaker, "(beat) [[note]]" a
// parenthetical) and stay as they are written (a scene heading is put in
// capitals, not its notes): each note is a character of Unicode's
// private use area, from maskBase on, in the row.
const maskBase = '\uE000'

// maskNotes masks the notes in row and returns them, in order.
func maskNotes(row string) (string, []string) {
	ns := lex.Notes(row)
	if len(ns) == 0 || strings.ContainsFunc(row, isMask) || len(ns) > 6400 {
		return row, nil
	}
	var b strings.Builder
	var out []string
	rest := row
	for i, n := range ns {
		note := "[[" + n + "]]"
		at := strings.Index(rest, note)
		b.WriteString(rest[:at])
		b.WriteRune(maskBase + rune(i))
		rest = rest[at+len(note):]
		out = append(out, note)
	}
	b.WriteString(rest)
	return b.String(), out
}

func isMask(r rune) bool { return r >= maskBase && r < maskBase+6400 }

// unmask puts the notes back in a masked text.
func unmask(s string, notes []string) string {
	if len(notes) == 0 {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if isMask(r) && int(r-maskBase) < len(notes) {
			b.WriteString(notes[r-maskBase])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isMaskOrSpace(r rune) bool { return isMask(r) || unicode.IsSpace(r) }

func readAllLines(file io.Reader) []string {
	var toParse []string
	f := bufio.NewReader(file)

	for {
		s, err := f.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(s) > 0 {
				toParse = append(toParse, s)
			}
			break
		}
		toParse = append(toParse, s)
	}

	// Add sentinel empty line for final closing logic
	toParse = append(toParse, "")
	return toParse
}

func (state *ParseState) handleTitlePage(row, trimmedSpaceRow, next string, currentLine *lex.Line) bool {
	// a note is no field's value, nor its colon ("Action [[todo: x]]")
	isKeyValLine := isTitleKeyValue(lex.StripNotes(row), lex.StripNotes(next))
	isContinuation := isIndented(row) && trimmedSpaceRow != "" && state.titletag != ""

	if isContinuation {
		// An indented line continues the previous field's value
		// ("Contact:" followed by address lines).
		state.consecutiveEmptyLines = 0
		currentLine.Type = state.titletag
		currentLine.Contents = trimmedSpaceRow
		state.out = append(state.out, *currentLine)
		return true
	}

	// Check for consecutive empty lines
	if trimmedSpaceRow == "" {
		state.consecutiveEmptyLines++
	} else {
		state.consecutiveEmptyLines = 0
	}

	// Check if we should exit title page mode
	if (!isKeyValLine && trimmedSpaceRow != "") || (state.consecutiveEmptyLines >= 2) {
		state.titlepage = false
		if state.hasTitlePageContent {
			state.out = append(state.out, lex.Line{Type: lex.TypeNewPage})
		}
		if trimmedSpaceRow == "" {
			return true // Skip this empty line
		}
		return false // Process as screenplay body
	}

	// Still in title page mode
	if state.titletag == "" && trimmedSpaceRow != "" {
		state.out = append(state.out, lex.Line{Type: lex.TypeTitlePage})
		state.hasTitlePageContent = true
	}

	if isKeyValLine {
		state.parseTitlePageKeyValue(row, currentLine)
	} else if trimmedSpaceRow != "" {
		currentLine.Type = state.titletag
		currentLine.Contents = trimmedSpaceRow
		state.hasTitlePageContent = true
	}

	if currentLine.Contents == "" {
		return true // Skip empty content lines
	}

	state.out = append(state.out, *currentLine)
	return true
}

// isIndented reports whether a line starts with a tab or three spaces,
// which on the title page marks a continuation of the previous value.
func isIndented(row string) bool {
	return strings.HasPrefix(row, "\t") || strings.HasPrefix(row, "   ")
}

// isTitleKeyValue reports whether row is a "Key: value" title page field.
// Script text that happens to contain a colon is not: a scene heading, or
// an uppercase cue with nothing after the colon and no indented value on
// the next line, such as "FADE IN:" opening the script.
func isTitleKeyValue(row, next string) bool {
	if isIndented(row) {
		return false
	}
	key, value, ok := strings.Cut(row, ":")
	if !ok || strings.TrimSpace(key) == "" {
		return false
	}
	// a key is a word ("Draft date"), not Fountain markup: "# Act 1: the
	// kitchen" is a section, ".SCENE 1: NIGHT" a forced scene heading
	if r, _ := utf8.DecodeRuneInString(key); !unicode.IsLetter(r) && !unicode.IsDigit(r) {
		return false
	}
	if isScene, _, _ := CheckScene(row); isScene {
		return false
	}
	if strings.TrimSpace(value) != "" || key != strings.ToUpper(key) {
		return true
	}
	return isIndented(next) && strings.TrimSpace(next) != ""
}

func (state *ParseState) parseTitlePageKeyValue(row string, currentLine *lex.Line) {
	masked, notes := maskNotes(row)
	split := strings.SplitN(masked, ":", 2)
	split[0], split[1] = unmask(split[0], notes), unmask(split[1], notes)
	currentMetaTag := split[0]

	switch strings.ToLower(currentMetaTag) {
	case "title":
		state.titletag = "Title"
	case "credit":
		state.titletag = "Credit"
	case "author", "authors":
		state.titletag = "Author"
	default:
		if state.titletag == "Title" || state.titletag == "Credit" || state.titletag == "Author" {
			state.out = append(state.out, lex.Line{Type: "metasection"})
		}
		state.titletag = currentMetaTag
	}

	currentLine.Type = state.titletag
	currentLine.Contents = strings.TrimSpace(split[1])
	state.hasTitlePageContent = true
}

func (state *ParseState) parseScreenplayLine(originalRow, row, trimmedSpaceRow string) (lex.Line, bool) {
	masked, notes := maskNotes(row)
	if len(notes) == 0 {
		return state.parseLine(originalRow, row, trimmedSpaceRow)
	}
	// notes at the start and the end of the row are set apart, so that
	// they do not hide its markers (">THE END< [[note]]", "BOB ^ [[note]]")
	body := strings.TrimLeftFunc(masked, isMaskOrSpace)
	lead := masked[:len(masked)-len(body)]
	body = strings.TrimRightFunc(body, isMaskOrSpace)
	if body == "" { // only notes
		return lex.Line{Type: lex.TypeNote, Contents: trimmedSpaceRow}, false
	}
	trail := strings.TrimPrefix(masked, lead)[len(body):]
	line, dual := state.parseLine(body, body, body)
	line.Contents = strings.TrimSpace(unmask(strings.TrimLeftFunc(lead, unicode.IsSpace)+line.Contents+
		strings.TrimRightFunc(trail, unicode.IsSpace), notes))
	return line, dual
}

func (state *ParseState) parseLine(originalRow, row, trimmedSpaceRow string) (lex.Line, bool) {
	var currentLine lex.Line
	var isCurrentLineDualSpeakerCandidate bool

	if trimmedSpaceRow == "" {
		currentLine.Type = lex.TypeEmpty
		currentLine.Contents = ""
		return currentLine, false
	}

	// Check forced types first
	if check, ftype, contents := CheckForce(originalRow); check {
		currentLine.Type = ftype
		currentLine.Contents = strings.TrimSpace(contents)
		if ftype == lex.TypeSpeaker && strings.HasSuffix(currentLine.Contents, "^") {
			isCurrentLineDualSpeakerCandidate = true
			currentLine.Contents = strings.TrimRight(currentLine.Contents, " ^")
		}
		return currentLine, isCurrentLineDualSpeakerCandidate
	}

	// Check structural types
	if currentLine = state.checkStructuralTypes(row); currentLine.Type != "" {
		return currentLine, false
	}

	// Check inferred types
	return state.checkInferredTypes(row, trimmedSpaceRow)
}

func (state *ParseState) checkStructuralTypes(row string) lex.Line {
	checkfuncs := []func(string) (bool, string, string){
		CheckScene,
		CheckCrow,
		CheckEqual,
		CheckSection,
	}

	for _, checkfunc := range checkfuncs {
		if check, element, contents := checkfunc(row); check {
			return lex.Line{
				Type:     element,
				Contents: strings.TrimSpace(contents),
			}
		}
	}

	return lex.Line{}
}

func (state *ParseState) checkInferredTypes(row, trimmedSpaceRow string) (lex.Line, bool) {
	var currentLine lex.Line
	var isCurrentLineDualSpeakerCandidate bool

	charcheck := strings.Split(row, "(")
	if !state.inDialogueContext && len(charcheck) > 0 && AllCaps(charcheck[0]) &&
		strings.TrimSpace(charcheck[0]) != "" {
		// Speaker name (all caps); within a speech an all-caps line is
		// dialogue (a shout, a sung lyric, SFX's KRAKOOM!)
		currentLine.Type = lex.TypeSpeaker
		currentLine.Contents = trimmedSpaceRow
		if strings.HasSuffix(currentLine.Contents, "^") {
			isCurrentLineDualSpeakerCandidate = true
			currentLine.Contents = strings.TrimRight(currentLine.Contents, " ^")
		}
	} else if len(trimmedSpaceRow) > 1 && strings.HasPrefix(trimmedSpaceRow, "(") &&
		strings.HasSuffix(trimmedSpaceRow, ")") {
		// Parenthetical
		if state.inDialogueContext {
			currentLine.Type = lex.TypeParen
		} else {
			currentLine.Type = lex.TypeAction
		}
		currentLine.Contents = trimmedSpaceRow
	} else if state.inDialogueContext {
		// Dialogue
		currentLine.Type = lex.TypeDialog
		currentLine.Contents = trimmedSpaceRow
	} else {
		// Action
		currentLine.Type = lex.TypeAction
		currentLine.Contents = trimmedSpaceRow
	}

	return currentLine, isCurrentLineDualSpeakerCandidate
}

func (state *ParseState) handleDualDialogue(currentLine lex.Line, isCurrentLineDualSpeakerCandidate bool,
	i, totalLines int) {
	// Handle dual dialogue closing
	if state.inDualDialogue && state.shouldCloseDualDialogue(currentLine, isCurrentLineDualSpeakerCandidate,
		i, totalLines) {
		state.out = append(state.out, lex.Line{Type: lex.TypeDualClose})
		state.inDualDialogue = false
	}

	// A "NAME ^" speaker shares the screen with the dialogue block right
	// before it. Without such a partner (or when that block is already the
	// second half of a pair) it is ordinary dialogue.
	if isCurrentLineDualSpeakerCandidate {
		if state.inDualDialogue {
			state.out = append(state.out, lex.Line{Type: lex.TypeDualClose})
			state.inDualDialogue = false
			return
		}
		if start, ok := state.previousDialogueBlock(); ok {
			state.out = append(state.out[:start], append(lex.Screenplay{{Type: lex.TypeDualOpen}}, state.out[start:]...)...)
			state.out = append(state.out, lex.Line{Type: lex.TypeDualNext})
			state.inDualDialogue = true
		}
	}
}

// previousDialogueBlock finds the dialogue block the output ends with
// (speaker, then dialogue and parentheticals, then one blank line) and
// returns the index of its speaker.
func (state *ParseState) previousDialogueBlock() (int, bool) {
	j := len(state.out) - 1
	if j < 0 || state.out[j].Type != lex.TypeEmpty {
		return 0, false
	}
	j--
	lines := 0
	for ; j >= 0; j-- {
		switch state.out[j].Type {
		case lex.TypeDialog, lex.TypeParen, lex.TypeLyrics:
			lines++
			continue
		case lex.TypeNote:
			continue
		case lex.TypeSpeaker:
			return j, lines > 0
		}
		break
	}
	return 0, false
}

func (state *ParseState) shouldCloseDualDialogue(currentLine lex.Line, isCurrentLineDualSpeakerCandidate bool,
	i, totalLines int) bool {
	switch currentLine.Type {
	case lex.TypeScene, lex.TypeAction, lex.TypeTrans, lex.TypeCenter, "section", "synopse", lex.TypeNewPage,
		lex.TypeTitlePage, "metasection":
		return true
	case lex.TypeSpeaker:
		return !isCurrentLineDualSpeakerCandidate
	case lex.TypeEmpty:
		return i == totalLines-1 // Last line
	default:
		return false
	}
}

func (state *ParseState) shouldAppendLine(currentLine lex.Line, trimmedSpaceRow string, i, totalLines int) bool {
	// Don't append the final sentinel empty line if it just closed dual dialogue
	if i == totalLines-1 && trimmedSpaceRow == "" && state.inDualDialogue {
		return false
	}
	// Append final sentinel if no dual dialogue was open (for tests)
	if i == totalLines-1 && trimmedSpaceRow == "" && !state.inDualDialogue {
		return true
	}
	return true
}

func (state *ParseState) updateDialogueContext(currentLine lex.Line) {
	if currentLine.Type == lex.TypeNote {
		return // a note between a name and its speech is no break
	}
	if currentLine.IsDialogueElement() {
		state.inDialogueContext = true
	} else if currentLine.Type == lex.TypeEmpty {
		// Revert speaker to action if followed only by empty line
		// (and notes)
		prev := len(state.out) - 2
		for prev >= 0 && state.out[prev].Type == lex.TypeNote {
			prev--
		}
		if prev >= 0 && state.out[prev].Type == lex.TypeSpeaker && !state.inDualDialogue {
			state.out[prev].Type = lex.TypeAction
			state.inDialogueContext = false
		} else {
			state.inDialogueContext = false
		}
	} else {
		state.inDialogueContext = false
	}
}
