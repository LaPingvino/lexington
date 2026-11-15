package lex

import (
	"fmt"
	"io"
	"strings"
)

// Continued is the type of a lex line that goes on with the text of the
// line before it, after a line break (a Fountain note over several
// lines): "+: " and the next line of the text, as it is.
const Continued = "+"

// LexWriter implements the writer.Writer interface for LEX output.
type LexWriter struct{}

// Write converts the internal lex.Screenplay format to a LEX file.
// It implements the writer.Writer interface.
func (l *LexWriter) Write(w io.Writer, screenplay Screenplay) error {
	for _, line := range screenplay {
		text, rest, more := strings.Cut(line.Contents, "\n")
		if _, err := fmt.Fprintf(w, "%s: %s\n", line.Type, text); err != nil {
			return err
		}
		for more {
			text, rest, more = strings.Cut(rest, "\n")
			if _, err := fmt.Fprintf(w, "%s: %s\n", Continued, text); err != nil {
				return err
			}
		}
	}
	return nil
}
