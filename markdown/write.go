package markdown

import (
	"fmt"
	"io"
	"strings"

	"github.com/lapingvino/lexington/lex"
)

// MarkdownWriter implements the writer.Writer interface for Markdown output.
// This is primarily used as an intermediate format for pandoc conversion.
type MarkdownWriter struct{}

// Write converts the internal lex.Screenplay format to a Markdown file.
// It implements the writer.Writer interface.
func (m *MarkdownWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	inTitlePage := false
	inDualDialogue := false

	for _, line := range screenplay {
		switch line.Type {
		case "titlepage":
			inTitlePage = true
			continue
		case "Title":
			if inTitlePage {
				_, err := fmt.Fprintf(w, "# %s\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		case "Credit":
			if inTitlePage {
				_, err := fmt.Fprintf(w, "*%s*\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		case "Author":
			if inTitlePage {
				_, err := fmt.Fprintf(w, "**%s**\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		case "metasection":
			inTitlePage = false
			_, err := fmt.Fprintf(w, "---\n\n")
			if err != nil {
				return err
			}
		case "newpage":
			_, err := fmt.Fprintf(w, "\n\\newpage\n\n")
			if err != nil {
				return err
			}
		case "scene":
			_, err := fmt.Fprintf(w, "## %s\n\n", strings.ToUpper(line.Contents))
			if err != nil {
				return err
			}
		case "action":
			if strings.TrimSpace(line.Contents) != "" {
				_, err := fmt.Fprintf(w, "%s\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		case "speaker":
			if inDualDialogue {
				_, err := fmt.Fprintf(w, "**%s**  \n", strings.ToUpper(line.Contents))
				if err != nil {
					return err
				}
			} else {
				_, err := fmt.Fprintf(w, "**%s**\n\n", strings.ToUpper(line.Contents))
				if err != nil {
					return err
				}
			}
		case "dialog", "lyrics":
			if inDualDialogue {
				_, err := fmt.Fprintf(w, "%s  \n", line.Contents)
				if err != nil {
					return err
				}
			} else {
				_, err := fmt.Fprintf(w, "%s\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		case "paren":
			if inDualDialogue {
				_, err := fmt.Fprintf(w, "*%s*  \n", line.Contents)
				if err != nil {
					return err
				}
			} else {
				_, err := fmt.Fprintf(w, "*%s*\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		case "trans":
			_, err := fmt.Fprintf(w, "**%s**\n\n", strings.ToUpper(line.Contents))
			if err != nil {
				return err
			}
		case "center":
			_, err := fmt.Fprintf(w, "<center>%s</center>\n\n", line.Contents)
			if err != nil {
				return err
			}
		case "empty":
			if !inDualDialogue {
				_, err := fmt.Fprintf(w, "\n")
				if err != nil {
					return err
				}
			}
		case "dualspeaker_open":
			inDualDialogue = true
			_, err := fmt.Fprintf(w, "<div style=\"display: flex; justify-content: space-between;\">\n<div style=\"width: 48%%;\">\n\n")
			if err != nil {
				return err
			}
		case "dualspeaker_next":
			_, err := fmt.Fprintf(w, "</div>\n<div style=\"width: 48%%;\">\n\n")
			if err != nil {
				return err
			}
		case "dualspeaker_close":
			inDualDialogue = false
			_, err := fmt.Fprintf(w, "</div>\n</div>\n\n")
			if err != nil {
				return err
			}
		case "section":
			level := strings.Count(line.Contents, "#")
			if level == 0 {
				level = 1
			}
			headerPrefix := strings.Repeat("#", level+2) // +2 because we use ## for scenes
			content := strings.TrimLeft(line.Contents, "# ")
			_, err := fmt.Fprintf(w, "%s %s\n\n", headerPrefix, content)
			if err != nil {
				return err
			}
		case "synopse":
			_, err := fmt.Fprintf(w, "> %s\n\n", strings.TrimLeft(line.Contents, "= "))
			if err != nil {
				return err
			}
		default:
			// Handle any unrecognized types as plain text
			if strings.TrimSpace(line.Contents) != "" {
				_, err := fmt.Fprintf(w, "%s\n\n", line.Contents)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
