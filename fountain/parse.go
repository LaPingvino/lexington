package fountain

import (
	"github.com/lapingvino/lexington/lex"
	"strings"
	"bufio"
	"io"
)

func Parse(file io.Reader) (out lex.Screenplay) {
	var err error
	var s string
	var toParse []string
	f := bufio.NewReader(file)
	for err == nil {
		s, err = f.ReadString('\n')
		toParse = append(toParse, s)
	}
	for i, row := range toParse {
		row = strings.TrimSpace(row)
		action := "action"
		if row == strings.ToUpper(row) {
			action = "allcaps"
		}
		if row == "" {
			action = "empty"
			continue
		}
		if i <= 0 {
			continue
		}
		switch out[i-1].Type {
		case "allcaps":
			out[i-1].Type = "speaker"
			if row[0] == '(' && row[len(row)-1] == ')' {
				action = "paren"
			} else {
				action = "dialog"
			}
		case "paren", "dialog":
			action = "dialog"
		}
		out = append(out, lex.Line{action, row})
	}
	return out
}
