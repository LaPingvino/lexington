package fountain

import (
	"fmt"
	"io"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// FountainWriter implements the writer.Writer interface for Fountain output.
type FountainWriter struct {
	SceneConfig []string // Configuration for scene headers
}

// WriteState holds the state needed during writing
type WriteState struct {
	titlepage string
	writer    io.Writer
	config    []string
	dualNext  bool   // the next speaker is the second of dual dialogue (^)
	titleKey  string // the title page field written last
	nextBlank bool   // the row after this one is a blank line, or there is none
}

// Write converts the internal lex.Screenplay format to a Fountain file.
// It implements the writer.Writer interface.
func (f *FountainWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// Remove trailing empty line if present
	screenplay = f.trimTrailingEmpty(screenplay)

	state := &WriteState{
		titlepage: "start",
		writer:    &tailWriter{w: w},
		config:    f.SceneConfig,
	}

	for k, line := range screenplay {
		state.nextBlank = k+1 >= len(screenplay) || screenplay[k+1].Type == lex.TypeEmpty
		if err := state.writeLine(line); err != nil {
			return err
		}
	}

	return nil
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
			t = "> " + t
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
	if name, _, _ := strings.Cut(lex.StripNotes(line.Contents), "("); !AllCaps(name) || !state.afterBlank() {
		if _, err := fmt.Fprint(state.writer, "@"); err != nil {
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

func (state *WriteState) writeScene(line lex.Line) error {
	// a heading is one by its word after a blank line
	if !state.isSceneSupported(lex.StripNotes(line.Contents)) || !state.afterBlank() {
		if _, err := fmt.Fprint(state.writer, "."); err != nil {
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

func (state *WriteState) isSceneSupported(contents string) bool {
	for _, prefix := range state.config {
		if StartsScene(contents, prefix) {
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
		if _, err := fmt.Fprint(state.writer, "!"); err != nil {
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
	w    io.Writer
	tail []byte
}

func (t *tailWriter) Write(p []byte) (int, error) {
	t.tail = append(t.tail, p...)
	if len(t.tail) > 2 {
		t.tail = t.tail[len(t.tail)-2:]
	}
	return t.w.Write(p)
}

func (t *tailWriter) blank() bool { return string(t.tail) == "\n\n" }
