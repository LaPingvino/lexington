package fountain

import (
	"fmt"
	"io"
	"strings"

	"github.com/lapingvino/lexington/lex"
)

// FountainWriter implements the writer.Writer interface for Fountain output.
type FountainWriter struct {
	SceneConfig []string // Configuration for scene headers
}

// Write converts the internal lex.Screenplay format to a Fountain file.
// It implements the writer.Writer interface.
func (f *FountainWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// The parser adds an empty line at the end which we don't want to write out
	// if it would create a superfluous trailing newline. This makes the writer
	// the inverse of the parser.
	if len(screenplay) > 0 && screenplay[len(screenplay)-1].Type == "empty" {
		screenplay = screenplay[:len(screenplay)-1]
	}

	var titlepage = "start"
	for _, line := range screenplay {
		element := line.Type
		if titlepage == "start" && line.Type != "titlepage" {
			titlepage = ""
		}
		if titlepage != "" {
			element = titlepage
		}
		switch element {
		case "start":
			titlepage = "titlepage"
		case "titlepage":
			if line.Type == "metasection" {
				continue
			}
			if line.Type == "newpage" {
				_, err := fmt.Fprintln(w, "")
				if err != nil {
					return err
				}
				titlepage = ""
				continue
			}
			_, err := fmt.Fprintf(w, "%s: %s\n", line.Type, line.Contents)
			if err != nil {
				return err
			}
		case "newpage":
			_, err := fmt.Fprintln(w, "===")
			if err != nil {
				return err
			}
		case "empty":
			_, err := fmt.Fprintln(w, "")
			if err != nil {
				return err
			}
		case "speaker":
			if line.Contents != strings.ToUpper(line.Contents) {
				_, err := fmt.Fprint(w, "@")
				if err != nil {
					return err
				}
			}
			_, err := fmt.Fprintln(w, line.Contents)
			if err != nil {
				return err
			}
		case "scene":
			var supported bool
			for _, prefix := range f.SceneConfig { // Use f.SceneConfig
				if strings.HasPrefix(line.Contents, prefix+" ") ||
					strings.HasPrefix(line.Contents, prefix+".") {
					supported = true
				}
			}
			if !supported {
				_, err := fmt.Fprint(w, ".")
				if err != nil {
					return err
				}
			}
			_, err := fmt.Fprintln(w, line.Contents)
			if err != nil {
				return err
			}
		case "lyrics":
			_, err := fmt.Fprintf(w, "~%s\n", line.Contents)
			if err != nil {
				return err
			}
		case "action":
			if line.Contents == strings.ToUpper(line.Contents) {
				_, err := fmt.Fprint(w, "!")
				if err != nil {
					return err
				}
			}
			_, err := fmt.Fprintln(w, line.Contents)
			if err != nil {
				return err
			}
		default:
			_, err := fmt.Fprintln(w, line.Contents)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
