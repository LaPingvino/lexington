package parse

import (
	"strings"
)

func (t *Tree) ParseString(play string) {
	toParse := strings.Split(play, "\n")
	for i, row := range toParse {
		action := "action"
		if row == strings.ToUpper(row) {
			action = "allcaps"
		}
		if row == "" {
			action = "empty"
		} else {
			if i > 0 {
				switch t.F[i-1].Format {
				case "allcaps":
					t.F[i-1].Format = "speaker"
					if row[0] == '(' && row[len(row)-1] == ')' {
						action = "paren"
					} else {
						action = "dialog"
					}
				case "paren", "dialog":
					action = "dialog"
				}
			}
		}
		t.F = append(t.F, struct{ Format, Text string }{action, row})
	}
}
