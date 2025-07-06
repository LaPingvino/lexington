package fountain

import (
	"bufio"
	"io"
	"strings"

	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/lex"
)

// Scene contains all the prefixes the scene detection looks for.
// This can be changed with the toml configuration in the rules package.
var Scene = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

// CheckScene determines if a row is a scene heading.
func CheckScene(row string) (bool, string, string) {
	upperRow := strings.ToUpper(row)

	// Check if any scene prefix matches
	_, found := internal.Find(Scene, func(prefix string) bool {
		return strings.HasPrefix(upperRow, prefix+" ") || strings.HasPrefix(upperRow, prefix+".")
	})

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
	row = strings.ToUpper(row)
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
	inDualDialogue        bool
	titletag              string
	consecutiveEmptyLines int
	hasTitlePageContent   bool
	out                   lex.Screenplay
}

// Parse converts a Fountain file into the internal lex.Screenplay format.
func Parse(scenes []string, file io.Reader) (out lex.Screenplay) {
	Scene = scenes

	toParse := readAllLines(file)

	state := &ParseState{
		scenes:    scenes,
		titlepage: true,
		out:       make(lex.Screenplay, 0),
	}

	for i, row := range toParse {
		originalRow := row
		row = strings.TrimRight(originalRow, "\n\r")
		trimmedSpaceRow := strings.TrimSpace(row)

		var currentLine lex.Line
		var isCurrentLineDualSpeakerCandidate bool

		// Handle title page parsing
		if state.titlepage {
			if handled := state.handleTitlePage(row, trimmedSpaceRow, &currentLine); handled {
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

func (state *ParseState) handleTitlePage(row, trimmedSpaceRow string, currentLine *lex.Line) bool {
	isKeyValLine := strings.Contains(row, ":") && !strings.HasPrefix(row, "   ")

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
	} else {
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

func (state *ParseState) parseTitlePageKeyValue(row string, currentLine *lex.Line) {
	split := strings.SplitN(row, ":", 2)
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
	if len(charcheck) > 0 && strings.ToUpper(charcheck[0]) == charcheck[0] && strings.TrimSpace(charcheck[0]) != "" {
		// Speaker name (all caps)
		currentLine.Type = lex.TypeSpeaker
		currentLine.Contents = trimmedSpaceRow
		if strings.HasSuffix(currentLine.Contents, "^") {
			isCurrentLineDualSpeakerCandidate = true
			currentLine.Contents = strings.TrimRight(currentLine.Contents, " ^")
		}
	} else if len(row) > 1 && row[0] == '(' && row[len(row)-1] == ')' {
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

	// Handle dual dialogue opening/next
	if isCurrentLineDualSpeakerCandidate {
		if !state.inDualDialogue {
			state.insertDualDialogueOpen()
			state.inDualDialogue = true
			state.out = append(state.out, lex.Line{Type: lex.TypeDualNext})
		} else {
			// Close current dual dialogue and treat as regular speaker
			state.out = append(state.out, lex.Line{Type: lex.TypeDualClose})
			state.inDualDialogue = false
		}
	}
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

func (state *ParseState) insertDualDialogueOpen() {
	foundOpenInsertPoint := false
	for j := len(state.out) - 1; j >= 0; j-- {
		if state.out[j].Type == lex.TypeSpeaker {
			for k := j; k >= 0; k-- {
				if state.out[k].Type == lex.TypeEmpty {
					dualOpen := []lex.Line{{Type: lex.TypeDualOpen}}
					state.out = append(state.out[:k+1], append(dualOpen, state.out[k+1:]...)...)
					foundOpenInsertPoint = true
					break
				} else if k == 0 {
					state.out = append([]lex.Line{{Type: lex.TypeDualOpen}}, state.out...)
					foundOpenInsertPoint = true
					break
				}
			}
			if foundOpenInsertPoint {
				break
			}
		}
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
	if currentLine.IsDialogueElement() {
		state.inDialogueContext = true
	} else if currentLine.Type == lex.TypeEmpty {
		// Revert speaker to action if followed only by empty line
		if len(state.out) >= 2 && state.out[len(state.out)-2].Type == lex.TypeSpeaker && !state.inDualDialogue {
			state.out[len(state.out)-2].Type = lex.TypeAction
			state.inDialogueContext = false
		} else {
			state.inDialogueContext = false
		}
	} else {
		state.inDialogueContext = false
	}
}
