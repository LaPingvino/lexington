package fountain

import (
	"github.com/lapingvino/lexington/lex"
	"io"
	"fmt"
	"strings"
)

func Write(f io.Writer, scene []string, screenplay lex.Screenplay) {
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
				fmt.Fprintln(f, "")
				titlepage = ""
				continue
			}
			fmt.Fprintf(f, "%s: %s\n", line.Type, line.Contents)
		case "newpage":
			fmt.Fprintln(f, "===")
		case "empty":
			fmt.Fprintln(f, "")
		case "speaker":
			if line.Contents != strings.ToUpper(line.Contents) {
				fmt.Fprint(f, "@")
			}
			fmt.Fprintln(f, line.Contents)
		case "scene":
			var supported bool
			for _, prefix := range scene {
				if strings.HasPrefix(line.Contents, prefix+" ") ||
				strings.HasPrefix(line.Contents, prefix+".") {
					supported = true
				}
			}
			if !supported {
				fmt.Fprint(f, ".")
			}
			fmt.Fprintln(f, line.Contents)
		case "lyrics":
			fmt.Fprintf(f, "~%s\n", line.Contents)
		case "action":
			if line.Contents == strings.ToUpper(line.Contents) {
				fmt.Fprint(f, "!")
			}
			fmt.Fprintln(f, line.Contents)
		default:
			fmt.Fprintln(f, line.Contents)
		}
	}
}
