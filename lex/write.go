package lex

import (
	"fmt"
	"io"
)

// LexWriter implements the writer.Writer interface for LEX output.
type LexWriter struct{}

// Write converts the internal lex.Screenplay format to a LEX file.
// It implements the writer.Writer interface.
func (l *LexWriter) Write(w io.Writer, screenplay Screenplay) error {
	for _, line := range screenplay {
		_, err := fmt.Fprintf(w, "%s: %s\n", line.Type, line.Contents)
		if err != nil {
			return err
		}
	}
	return nil
}
