package lex

import (
	"fmt"
	"io"
)

func Write(s Screenplay, out io.Writer) {
	for _, line := range s {
		fmt.Fprintf(out, "%s: %s\n", line.Type, line.Contents)
	}
}
