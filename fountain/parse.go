// Fountain is a Markdown-like language for screenplays and the main inspiration for this tool.
// Read more at fountain.io
package fountain

import (
	"bufio"
	"github.com/lapingvino/lexington/lex"
	"io"
	"strings"
)

var Scene = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

func CheckScene(row string) (bool, string, string) {
	var scene bool
	row = strings.ToUpper(row)
	for _, prefix := range Scene {
		if strings.HasPrefix(row, prefix+" ") ||
			strings.HasPrefix(row, prefix+".") {
			scene = true
		}
	}
	if strings.HasPrefix(row, ".") {
		row = row[1:]
		scene = true
	}
	return scene, "scene", row
}

func CheckCrow(row string) (bool, string, string) {
	var crow bool
	var el string
	row = strings.ToUpper(row)
	if strings.HasPrefix(row, ">") || strings.HasSuffix(row, " TO:") {
		crow = true
		el = "trans"
	}
	if strings.HasPrefix(row, ">") && strings.HasSuffix(row, "<") {
		el = "center"
	}
	return crow, el, strings.Trim(row, ">< ")
}

func CheckEqual(row string) (bool, string, string) {
	var equal bool
	var el string
	if strings.HasPrefix(row, "=") {
		equal = true
		el = "synopse"
	}
	if len(row) >= 3 && strings.Trim(row, "=") == "" {
		el = "newpage"
	}
	return equal, el, strings.TrimLeft(row, "= ")
}

func CheckSection(row string) (bool, string, string) {
	var section bool
	if strings.HasPrefix(row, "#") {
		section = true
	}
	return section, "section", row
}

// This is a Fountain parser, trying to be as close as possible to the description
// found at https://fountain.io/syntax but it can be incomplete.
// Over time more and more parts should be configurable here, e.g. INT/EXT translatable to other languages.
func Parse(file io.Reader) (out lex.Screenplay) {
	var err error
	var s string
	var toParse []string // Fill with two to avoid out of bounds when backtracking
	f := bufio.NewReader(file)
	for err == nil {
		s, err = f.ReadString('\n')
		toParse = append(toParse, s)
	}
	toParse = append(toParse, "") // Trigger the backtracking also for the last line
	out = make(lex.Screenplay, 2, len(toParse))
	for i, row := range toParse {
		i += 2
		row = strings.TrimRight(row, "\n\r")
		action := "action"
		if row == strings.ToUpper(row) {
			action = "allcaps"
		}
		if row == "" {
			action = "empty"

			// Backtracking for elements that need a following empty line
			checkfuncs := []func(string) (bool, string, string){
				CheckScene,
				CheckCrow,
				CheckEqual,
				CheckSection,
			}
			for _, checkfunc := range checkfuncs {
				check, element, contents := checkfunc(out[i-1].Contents)
				if check && out[i-2].Contents == "" {
					out[i-1].Type = element
					out[i-1].Contents = contents
					break
				}
			}
		}
		if out[i-1].Type != "action" {
			out[i-1].Contents = strings.TrimSpace(out[i-1].Contents)
		}

		// Backtracking to check for dialog sequence
		if row != "" {
			switch out[i-1].Type {
			case "allcaps":
				out[i-1].Type = "speaker"
				fallthrough
			case "paren", "dialog":
				if row[0] == '(' && row[len(row)-1] == ')' {
					action = "paren"
				} else {
					action = "dialog"
				}
			}
		}
		out = append(out, lex.Line{action, row})
	}
	return out[2:] // Remove the safety empty rows
}
