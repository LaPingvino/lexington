package fountain

import "strings"

// Accolade's end blocks: what Accolade keeps with a script, at its end,
// in Fountain's boneyard, so that it travels with the file and no
// Fountain app prints it:
//
//	INT. HOUSE - DAY
//
//	Anna waits.
//
//	/* Characters
//	ANNA: the lead
//	*/
//
//	/* Source
//	The story the script is adapted from, as Markdown.
//	*/
//
//	/* Notes
//	The writer's notepad.
//	*/
//
// The notes block is the last; the others come before it, in any order
// (EndBlockNames). A "*/" written inside a block would close it, so it
// is kept as "* /" (BlockMarks.Escaped). A plain file of a script in
// another language than the standard one ends with one more line after
// them, the language line ("/* Language: eo */", LanguageLine).
//
// This is the one place that says what they are: Accolade finds its
// blocks with EndBlocks, and Strip drops them.

// The end blocks, by name: "/* Notes" opens the notes block.
const (
	EndNotes      = "Notes"
	EndSource     = "Source"
	EndAdaptation = "Adaptation"
	EndProduction = "Production"
	EndCharacters = "Characters"
	// EndFixUp holds the pattern fixer's rules: how an imported script's
	// lines were set right ("heading: Scene <number> - <text> → <text>
	// #<number>#"), one a line, to apply again after a re-import
	EndFixUp = "Fix-Up"
)

// EndBlockNames are the blocks before the notes block, which come in any
// order.
var EndBlockNames = []string{EndSource, EndAdaptation, EndProduction, EndCharacters, EndFixUp}

// BlockMarks are how a block at the end of a text is written: the line
// that opens it, the mark that closes it, and that mark as it is kept
// inside the block (escaped).
type BlockMarks struct {
	Open, Close, Escaped string
}

// EndBlockMarks are the marks of the end block of that name.
func EndBlockMarks(name string) BlockMarks {
	return BlockMarks{Open: "/* " + name, Close: "*/", Escaped: "* /"}
}

// At finds the block at the very end of text: the last line that opens
// it, if the block closes at the end of the text (only white space after
// it) and holds no other close mark (an earlier boneyard of the same name
// is the script's, not this block). It is where the block starts, in
// bytes, with the blank lines before it, and what is inside it as written
// (escaped, less the new line before the close mark); -1 if there is
// none.
//
// It is a plain scan of the text's end: Accolade asks it several times
// per keystroke, of a text that may hold a whole story in its source
// block.
func (m BlockMarks) At(text string) (int, string) {
	c := len(text)
	for c > 0 && blankByte(text[c-1]) {
		c--
	}
	if !strings.HasSuffix(text[:c], m.Close) {
		return -1, ""
	}
	c -= len(m.Close)
	// forward scans (strings.Index is fast, LastIndex is not): the last
	// close before it, and the last opening after that
	from := 0
	for off := 0; ; {
		j := strings.Index(text[off:c], m.Close)
		if j < 0 {
			break
		}
		from = off + j + len(m.Close)
		off = from
	}
	open := m.Open + "\n"
	i := -1
	for off := from; ; {
		j := strings.Index(text[off:c], open)
		if j < 0 {
			break
		}
		i = off + j
		off = i + 1
	}
	if i < 0 || (i > 0 && text[i-1] != '\n') {
		return -1, ""
	}
	inside := strings.TrimSuffix(text[i+len(open):c], "\n")
	for i > 0 && text[i-1] == '\n' {
		i--
	}
	return i, inside
}

// Unescape is what is inside a block as it was meant: its escaped close
// marks closing again.
func (m BlockMarks) Unescape(inside string) string {
	return strings.ReplaceAll(inside, m.Escaped, m.Close)
}

// blankByte reports whether b is white space as a regexp's \s has it.
func blankByte(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\f', '\r':
		return true
	}
	return false
}

// EndBlock is one of the end blocks a text has: its name, where it is in
// bytes (from the blank lines before it up to the next block, or the end
// of the text), and what is inside it as written (BlockMarks.At).
type EndBlock struct {
	Name       string
	Start, End int
	Inside     string
}

// EndBlocks are the end blocks of text, in the order they are found:
// the notes block first, then the others from the last back. A block not
// known here hides the ones before it. Body is where the script proper
// ends, in bytes: before the first of them (the text's length if it has
// none).
func EndBlocks(text string) (blocks []EndBlock, body int) {
	end := len(text)
	if i, inside := EndBlockMarks(EndNotes).At(text); i >= 0 {
		blocks = append(blocks, EndBlock{EndNotes, i, end, inside})
		end = i
	}
	found := map[string]bool{}
	for {
		before, more := text[:end], false
		for _, name := range EndBlockNames {
			if found[name] {
				continue
			}
			if i, inside := EndBlockMarks(name).At(before); i >= 0 {
				blocks = append(blocks, EndBlock{name, i, end, inside})
				found[name], end, more = true, i, true
				break
			}
		}
		if !more {
			return blocks, end
		}
	}
}

// LanguageLinePrefix starts the language line: the last line of a plain
// file of a script in another language than the standard one, written in
// standard Fountain (Force), "/* Language: eo */".
const LanguageLinePrefix = "/* Language: "

// LanguageLine is the language line at the end of text: where it starts
// in bytes (with the blank line written before it) and the language it
// names, as written; -1 if the text has none.
func LanguageLine(text string) (int, string) {
	body := strings.TrimRight(text, "\r\n")
	nl := strings.LastIndexByte(body, '\n')
	if nl < 0 {
		return -1, ""
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(body[nl+1:]), LanguageLinePrefix)
	if !ok {
		return -1, ""
	}
	lang, ok := strings.CutSuffix(rest, " */")
	if lang = strings.TrimSpace(lang); !ok || lang == "" || strings.ContainsAny(lang, " \t*/") {
		return -1, ""
	}
	at := nl + 1
	if strings.HasSuffix(body[:at], "\n\n") { // the blank line written before it
		at--
	}
	return at, lang
}

// IsSourceLink reports whether a note's inside (what is between "[[" and
// "]]") is one of Accolade's links to the story a script is adapted
// from: it starts with "from:" ("[[from: #:~:text=She opened the door]]",
// the old form "[[from: "She opened…"]]").
func IsSourceLink(inside string) bool {
	t := strings.TrimSpace(inside)
	return len(t) >= 5 && strings.EqualFold(t[:5], "from:")
}
