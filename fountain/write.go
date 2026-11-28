package fountain

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// FountainWriter implements the writer.Writer interface for Fountain output.
type FountainWriter struct {
	SceneConfig []string // Configuration for scene headers
	// Language is what the text written is read with (its scene words,
	// cue and transition rules); nil: rules.Words(SceneConfig), or, with
	// no SceneConfig either, English as the default configuration has it.
	Language *rules.Language
}

// language is what the text written is read with, and the scene words
// a heading is written with: a zero FountainWriter writes English
// Fountain, whose INT. heading needs no dot.
func (f *FountainWriter) language() (rules.Language, []string) {
	switch {
	case f.Language != nil && f.SceneConfig == nil:
		return *f.Language, f.Language.Scenes
	case f.Language != nil:
		return *f.Language, f.SceneConfig
	case f.SceneConfig == nil:
		en := rules.DefaultLanguages()["en"]
		return en, en.Scenes
	}
	return rules.Words(f.SceneConfig), f.SceneConfig
}

// WriteState holds the state needed during writing
type WriteState struct {
	titlepage string
	writer    io.Writer
	config    []string
	dualNext  bool   // the next speaker is the second of dual dialogue (^)
	titleKey  string // the title page field written last
	nextBlank bool   // the row after this one is a blank line, or there is none
	marks     []forceMark
}

// forceMark is a force mark the writer put before a line (from 0) of
// what it wrote: n bytes ("!", "> "); a kept one stays whatever the
// reading needs.
type forceMark struct {
	line, n int
	kept    bool
}

// Write converts the internal lex.Screenplay format to a Fountain file.
// It implements the writer.Writer interface.
func (f *FountainWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// Remove trailing empty line if present
	screenplay = f.trimTrailingEmpty(screenplay)

	lang, scenes := f.language()
	var buf bytes.Buffer
	state := &WriteState{
		titlepage: "start",
		writer:    &tailWriter{w: &buf},
		config:    scenes,
	}

	for k, line := range screenplay {
		state.nextBlank = k+1 >= len(screenplay) || screenplay[k+1].Type == lex.TypeEmpty
		if err := state.writeLine(line); err != nil {
			return err
		}
	}

	_, err := io.WriteString(w, minimalMarks(buf.String(), state.marks, lang))
	return err
}

// mark writes the force mark m before the line being started.
func (state *WriteState) mark(m string) error { return state.markKept(m, false) }

// markKept is mark, the mark kept by minimalMarks when kept.
func (state *WriteState) markKept(m string, kept bool) error {
	if t, ok := state.writer.(*tailWriter); ok {
		state.marks = append(state.marks, forceMark{t.lines, len(m), kept})
	}
	_, err := fmt.Fprint(state.writer, m)
	return err
}

// minimalMarks is text, written with the force marks marks, with each
// of them taken off that the reading lang does not need: text without it
// reads, line by line, as the same rows (elements and contents). A mark
// is needed where the line in its place would read otherwise ("!BOB"
// with text below it, which would be a cue; "!CUT TO:" a transition);
// "FADE IN:" before a blank line is action by itself. All are taken off
// and the text read again; where a run of lines reads otherwise, the
// mark nearest before its first line goes back on, until none does.
func minimalMarks(text string, marks []forceMark, lang rules.Language) string {
	if len(marks) == 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	want := rowsByLine(text, lang)
	off := make([]bool, len(marks)) // marks[k] taken off
	for k, m := range marks {
		off[k] = !m.kept
	}
	for {
		try := append([]string(nil), lines...)
		for k, m := range marks {
			if off[k] {
				try[m.line] = try[m.line][m.n:]
			}
		}
		joined := strings.Join(try, "\n")
		got := rowsByLine(joined, lang)
		restored, prevDiff := false, false
		for i := 1; i <= len(lines); i++ {
			diff := !sameRows(want[i], got[i])
			if diff && !prevDiff {
				if k := markFor(marks, off, i-1); k >= 0 {
					off[k], restored = false, true
				}
			}
			prevDiff = diff
		}
		if !restored {
			return joined
		}
	}
}

// markFor is the mark taken off nearest at or before line (from 0), or
// else nearest after it; -1 for none.
func markFor(marks []forceMark, off []bool, line int) int {
	best := -1
	for k, m := range marks {
		if !off[k] {
			continue
		}
		if m.line <= line {
			best = k
		} else {
			if best < 0 {
				best = k
			}
			break
		}
	}
	return best
}

// rowsByLine are the rows text is read as with lang, by the line (from
// 1) they were read from.
func rowsByLine(text string, lang rules.Language) map[int][]lex.Line {
	out := map[int][]lex.Line{}
	for _, row := range ParseLinesIn(lang, strings.NewReader(text)) {
		out[row.Source] = append(out[row.Source], lex.Line{Type: row.Type, Contents: row.Contents})
	}
	return out
}

func sameRows(a, b []lex.Line) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (f *FountainWriter) trimTrailingEmpty(screenplay lex.Screenplay) lex.Screenplay {
	if len(screenplay) > 0 && screenplay[len(screenplay)-1].Type == lex.TypeEmpty {
		return screenplay[:len(screenplay)-1]
	}
	return screenplay
}

func (state *WriteState) writeLine(line lex.Line) error {
	element := line.Type
	if state.titlepage == "start" && line.Type != lex.TypeTitlePage {
		state.titlepage = ""
	}
	if state.titlepage != "" {
		element = state.titlepage
	}

	switch element {
	case "start":
		state.titlepage = lex.TypeTitlePage
		return nil
	case lex.TypeTitlePage:
		return state.writeTitlePageLine(line)
	case lex.TypeNewPage:
		return state.writeNewPage()
	case lex.TypeEmpty:
		return state.writeEmpty()
	case lex.TypeSpeaker:
		return state.writeSpeaker(line)
	case lex.TypeScene:
		return state.writeScene(line)
	case lex.TypeLyrics:
		return state.writeLyrics(line)
	case lex.TypeAction:
		return state.writeAction(line)
	case lex.TypeDualOpen, lex.TypeDualClose:
		state.dualNext = false
		return nil
	case lex.TypeDualNext:
		// a blank line between the two speeches, if there is none yet
		state.dualNext = true
		if t, ok := state.writer.(*tailWriter); ok && t.blank() {
			return nil
		}
		return state.writeEmpty()
	case lex.TypeSynopsis:
		_, err := fmt.Fprintf(state.writer, "= %s\n", strings.TrimSpace(line.Contents))
		return err
	case lex.TypeCenter:
		_, err := fmt.Fprintf(state.writer, ">%s<\n", strings.TrimSpace(line.Contents))
		return err
	case lex.TypeTrans:
		// only "... TO:" in capitals with blank lines around it is a
		// transition by itself; others need the ">"
		t := strings.TrimSpace(line.Contents)
		if bare := strings.TrimSpace(lex.StripNotes(t)); !strings.HasPrefix(bare, ">") &&
			(!strings.HasSuffix(bare, "TO:") || !AllCaps(bare) || !state.afterBlank() || !state.nextBlank) {
			if err := state.mark("> "); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintln(state.writer, t)
		return err
	default:
		return state.writeDefault(line)
	}
}

func (state *WriteState) writeTitlePageLine(line lex.Line) error {
	if line.Type == lex.TypeMetaSection {
		return nil
	}
	if line.Type == lex.TypeNewPage {
		if _, err := fmt.Fprintln(state.writer, ""); err != nil {
			return err
		}
		state.titlepage = ""
		return nil
	}
	// a field over several lines (Contact: name, address) goes on with
	// indented lines
	if line.Type == state.titleKey {
		_, err := fmt.Fprintf(state.writer, "    %s\n", line.Contents)
		return err
	}
	state.titleKey = line.Type
	_, err := fmt.Fprintf(state.writer, "%s: %s\n", line.Type, line.Contents)
	return err
}

func (state *WriteState) writeNewPage() error {
	_, err := fmt.Fprintln(state.writer, "===")
	return err
}

func (state *WriteState) writeEmpty() error {
	_, err := fmt.Fprintln(state.writer, "")
	return err
}

func (state *WriteState) writeSpeaker(line lex.Line) error {
	// the notes are no part of the name ("BOB [[note]]" is in capitals)
	// a name is one by itself in capitals after a blank line
	// a name in a script without capitals (Hebrew, Arabic, Chinese)
	// keeps its @ always, as such scripts write it: a cue the reading
	// takes by its context alone is one an edit can lose
	bare := lex.StripNotes(line.Contents)
	if name, _, _ := strings.Cut(bare, "("); !AllCaps(name) || !state.afterBlank() {
		own, _, _ := strings.Cut(name, "（")
		if err := state.markKept("@", caseless(own)); err != nil {
			return err
		}
	}
	if state.dualNext {
		state.dualNext = false
		_, err := fmt.Fprintln(state.writer, line.Contents+" ^")
		return err
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

// caseless tells whether s has letters and none of them has a case.
func caseless(s string) bool {
	letters := false
	for _, r := range s {
		if unicode.IsUpper(r) || unicode.IsLower(r) {
			return false
		}
		letters = letters || unicode.IsLetter(r)
	}
	return letters
}

func (state *WriteState) writeScene(line lex.Line) error {
	// a heading is one by its word after a blank line
	if !state.isSceneSupported(lex.StripNotes(line.Contents)) || !state.afterBlank() {
		if err := state.mark("."); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

// afterBlank tells whether what is written so far ends with a blank line,
// or is nothing yet.
func (state *WriteState) afterBlank() bool {
	t, ok := state.writer.(*tailWriter)
	return !ok || len(t.tail) == 0 || t.blank()
}

// isSceneSupported tells whether contents start with a scene word, in
// any case, as the parser reads them (Dialect.Heading).
func (state *WriteState) isSceneSupported(contents string) bool {
	upper := strings.ToUpper(strings.TrimSpace(contents))
	for _, prefix := range state.config {
		if StartsScene(upper, strings.ToUpper(prefix)) {
			return true
		}
	}
	return false
}

func (state *WriteState) writeLyrics(line lex.Line) error {
	_, err := fmt.Fprintf(state.writer, "~%s\n", line.Contents)
	return err
}

func (state *WriteState) writeAction(line lex.Line) error {
	// action that would read as a name, a transition or a heading
	if bare := lex.StripNotes(line.Contents); AllCaps(bare) || (state.afterBlank() && state.isSceneSupported(bare)) || markStart(bare) {
		if err := state.mark("!"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

// markStart: text that starts with one of Fountain's marks, so that it
// would read as another element (". .. and then" a heading, "> …" a
// transition, "# …" a section).
func markStart(s string) bool {
	switch {
	case strings.HasPrefix(s, ".") && !strings.HasPrefix(s, ".."),
		strings.HasPrefix(s, "@"), strings.HasPrefix(s, ">"), strings.HasPrefix(s, "~"),
		strings.HasPrefix(s, "="), strings.HasPrefix(s, "#"), strings.HasPrefix(s, "!"):
		return true
	}
	return false
}

func (state *WriteState) writeDefault(line lex.Line) error {
	if line.Type == lex.TypeDialog && strings.TrimSpace(line.Contents) == "" {
		// a blank line within a speech: two spaces, as Fountain has it
		_, err := fmt.Fprintln(state.writer, "  ")
		return err
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

// tailWriter remembers the end of what was written, to tell whether it
// ends with a blank line.
type tailWriter struct {
	w     io.Writer
	tail  []byte
	lines int // the newlines written
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.lines += bytes.Count(p, []byte("\n"))
	t.tail = append(t.tail, p...)
	if len(t.tail) > 2 {
		t.tail = t.tail[len(t.tail)-2:]
	}
	return t.w.Write(p)
}

func (t *tailWriter) blank() bool { return string(t.tail) == "\n\n" }
