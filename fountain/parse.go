// Fountain is a Markdown-like language for screenplays and the main inspiration for this tool.
// Read more at fountain.io
package fountain

import (
	"bufio"
	"github.com/lapingvino/lexington/lex"
	"io"
	"strings"
)

// Scene contains all the prefixes the scene detection looks for.
// This can be changed with the toml configuration in the rules package.
var Scene = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

func last(out *lex.Screenplay, i int) *lex.Line {
	if len(*out) >= i {
		return &(*out)[len(*out)-i]
	} else {
		line := lex.Line{Type: "empty"}
		return &line
	}
}

func CheckScene(row string) (bool, string, string) {
	var scene bool
	row = strings.ToUpper(row)
	for _, prefix := range Scene {
		if strings.HasPrefix(row, prefix+" ") ||
			strings.HasPrefix(row, prefix+".") {
			scene = true
		}
	}
	if strings.HasPrefix(row, ".") && !strings.HasPrefix(row, "..") {
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

func CheckForce(row string) (bool, string, string) {
	var force = true
	var ftype string
	if len(row) < 1 {
		return false, "", ""
	}
	switch row[0] {
	case '@':
		ftype = "speaker"
	case '~':
		ftype = "lyrics"
	case '!':
		ftype = "action"
	default:
		force = false
	}
	if force {
		row = row[1:]
	}
	return force, ftype, row
}

// This is a Fountain parser, trying to be as close as possible to the description
// found at https://fountain.io/syntax but it can be incomplete.
func Parse(scenes []string, file io.Reader) (out lex.Screenplay) {
	Scene = scenes
	var err error
	var titlepage, dialog bool = true, false
	var s, titletag string
	var toParse []string
	f := bufio.NewReader(file)
	for err == nil {
		s, err = f.ReadString('\n')
		toParse = append(toParse, s)
	}
	toParse = append(toParse, "") // Trigger the backtracking also for the last line
	for _, row := range toParse {
		row = strings.TrimRight(row, "\n\r")
		action := "action"
		if row == "" {
			action = "empty"
			if titlepage {
				titlepage = false
				out = append(out, lex.Line{Type: "newpage"})
				continue
			}
		}
		if last(&out, 1).Type != "action" {
			last(&out, 1).Contents = strings.TrimSpace(last(&out, 1).Contents)
		}

		// Backtracking to check for dialog sequence
		if dialog {
			if row == "" {
				dialog = false
				action = "empty"
				if last(&out, 1).Type == "speaker" {
					last(&out, 1).Type = "action"
				}
			} else {
				if row[0] == '(' && row[len(row)-1] == ')' {
					action = "paren"
				} else {
					action = "dialog"
				}
			}
		}
		charcheck := strings.Split(row, "(")
		if charcheck[0] == strings.ToUpper(charcheck[0]) && action == "action" {
			action = "speaker"
			dialog = true
		}

		checkfuncs := []func(string) (bool, string, string){
			CheckScene, // should actually check for empty lines, but doing that creates more problems than it solves
			CheckCrow,
			CheckEqual,
			CheckSection,
			CheckForce,
		}
		for _, checkfunc := range checkfuncs {
			check, element, contents := checkfunc(row)
			if check {
				action = element
				row = contents
				if action == "speaker" {
					dialog = true
				}
				break
			}
		}

		if titlepage {
			if titletag == "" {
				out = append(out, lex.Line{Type: "titlepage"})
			}
			split := strings.SplitN(row, ":", 2)
			if len(split) == 2 {
				action = split[0]
				switch strings.ToLower(action) {
				case "title", "credit", "author", "authors":
					titletag = "title"
				default:
					if titletag == "title" {
						out = append(out, lex.Line{Type: "metasection"})
					}
					titletag = action
				}
				row = strings.TrimSpace(split[1])
				if row == "" {
					continue
				}
			} else {
				action = titletag
				row = strings.TrimSpace(row)
			}
		}
		out = append(out, lex.Line{action, row})
	}
	return out
}
