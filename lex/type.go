package lex

import (
	"io"
	"fmt"
)

type Screenplay []Line

type Line struct{
	Type string
	Contents string
}

func Parse(file io.Reader) (out Screenplay) {
	var err error
	line := Line{}
	for err == nil {
		_, err = fmt.Fscanf(file, "%s: %s", &line.Type, &line.Contents)
		out = append(out, line)
	}
	return out
}
