package lex

import (
	"io"
	"bufio"
	"strings"
)

func Parse(file io.Reader) (out Screenplay) {
	f := bufio.NewReader(file)
	var err error
	var s string
	for err == nil {
		var line Line
		s, err = f.ReadString('\n')
		split := strings.SplitN(s, ":", 2)
		switch len(split){
		case 0,1:
			line.Type = strings.Trim(s,": \n\r")
		case 2:
			line.Type = split[0]
			line.Contents = strings.TrimSpace(split[1])
		}
		if strings.TrimSpace(split[0]) != "" {
			out = append(out, line)
		}
	}
	return out
}
