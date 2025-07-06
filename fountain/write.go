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
}

// Write converts the internal lex.Screenplay format to a Fountain file.
// It implements the writer.Writer interface.
func (f *FountainWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// Remove trailing empty line if present
	screenplay = f.trimTrailingEmpty(screenplay)

	state := &WriteState{
		titlepage: "start",
		writer:    w,
		config:    f.SceneConfig,
	}

	for _, line := range screenplay {
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
	default:
		return state.writeDefault(line)
	}
}

func (state *WriteState) writeTitlePageLine(line lex.Line) error {
	if line.Type == "metasection" {
		return nil
	}
	if line.Type == lex.TypeNewPage {
		if _, err := fmt.Fprintln(state.writer, ""); err != nil {
			return err
		}
		state.titlepage = ""
		return nil
	}
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
	if line.Contents != strings.ToUpper(line.Contents) {
		if _, err := fmt.Fprint(state.writer, "@"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

func (state *WriteState) writeScene(line lex.Line) error {
	if !state.isSceneSupported(line.Contents) {
		if _, err := fmt.Fprint(state.writer, "."); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

func (state *WriteState) isSceneSupported(contents string) bool {
	for _, prefix := range state.config {
		if strings.HasPrefix(contents, prefix+" ") ||
			strings.HasPrefix(contents, prefix+".") {
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
	if line.Contents == strings.ToUpper(line.Contents) {
		if _, err := fmt.Fprint(state.writer, "!"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}

func (state *WriteState) writeDefault(line lex.Line) error {
	_, err := fmt.Fprintln(state.writer, line.Contents)
	return err
}
