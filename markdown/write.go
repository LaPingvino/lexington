package markdown

import (
	"fmt"
	"io"
	"strings"

	"github.com/lapingvino/lexington/lex"
)

// Constants for markdown formatting
const (
	newPageMarker     = "\n\\newpage\n\n"
	metaSeparator     = "---\n\n"
	dualDialogueOpen  = "<div style=\"display: flex; justify-content: space-between;\">\n<div style=\"width: 48%%;\">\n\n"
	dualDialogueNext  = "</div>\n<div style=\"width: 48%%;\">\n\n"
	dualDialogueClose = "</div>\n</div>\n\n"
)

// MarkdownWriter implements the writer.Writer interface for Markdown output.
// This is primarily used as an intermediate format for pandoc conversion.
type MarkdownWriter struct{}

// Write converts the internal lex.Screenplay format to a Markdown file.
// It implements the writer.Writer interface.
func (m *MarkdownWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	state := &markdownState{
		writer:         w,
		inTitlePage:    false,
		inDualDialogue: false,
	}

	for _, line := range screenplay {
		if err := state.processLine(line); err != nil {
			return err
		}
	}

	return nil
}

// markdownState holds the state during markdown conversion
type markdownState struct {
	writer         io.Writer
	inTitlePage    bool
	inDualDialogue bool
}

// processLine processes a single line based on its type
func (s *markdownState) processLine(line lex.Line) error {
	// Handle title page elements
	if s.isTitlePageElement(line.Type) {
		return s.processTitlePageElements(line)
	}

	// Handle dual dialogue markers
	if s.isDualDialogueMarker(line.Type) {
		return s.processDualDialogueMarkers(line)
	}

	// Handle content elements
	return s.processContentElements(line)
}

// isTitlePageElement checks if the line type is a title page element
func (s *markdownState) isTitlePageElement(lineType string) bool {
	return lineType == lex.TypeTitlePage || lineType == "Title" ||
		lineType == "Credit" || lineType == "Author" || lineType == "metasection"
}

// isDualDialogueMarker checks if the line type is a dual dialogue marker
func (s *markdownState) isDualDialogueMarker(lineType string) bool {
	return lineType == lex.TypeDualOpen || lineType == lex.TypeDualNext || lineType == lex.TypeDualClose
}

// processTitlePageElements handles all title page related elements
func (s *markdownState) processTitlePageElements(line lex.Line) error {
	switch line.Type {
	case lex.TypeTitlePage:
		s.inTitlePage = true
		return nil
	case "Title":
		return s.processTitlePageElement(line, "# %s\n\n")
	case "Credit":
		return s.processTitlePageElement(line, "*%s*\n\n")
	case "Author":
		return s.processTitlePageElement(line, "**%s**\n\n")
	case "metasection":
		s.inTitlePage = false
		return s.writeString(metaSeparator)
	}
	return nil
}

// processDualDialogueMarkers handles dual dialogue markers
func (s *markdownState) processDualDialogueMarkers(line lex.Line) error {
	switch line.Type {
	case lex.TypeDualOpen:
		s.inDualDialogue = true
		return s.writeString(dualDialogueOpen)
	case lex.TypeDualNext:
		return s.writeString(dualDialogueNext)
	case lex.TypeDualClose:
		s.inDualDialogue = false
		return s.writeString(dualDialogueClose)
	}
	return nil
}

// processContentElements handles all content elements
func (s *markdownState) processContentElements(line lex.Line) error {
	switch line.Type {
	case lex.TypeNewPage:
		return s.writeString(newPageMarker)
	case lex.TypeScene:
		return s.writeFormatted("## %s\n\n", strings.ToUpper(line.Contents))
	case lex.TypeAction:
		return s.processActionLine(line)
	case lex.TypeSpeaker:
		return s.processSpeakerLine(line)
	case lex.TypeDialog, lex.TypeLyrics:
		return s.processDialogLine(line)
	case lex.TypeParen:
		return s.processParenLine(line)
	case "trans":
		return s.writeFormatted("**%s**\n\n", strings.ToUpper(line.Contents))
	case lex.TypeCenter:
		return s.writeFormatted("<center>%s</center>\n\n", line.Contents)
	case lex.TypeEmpty:
		return s.processEmptyLine()
	case "section":
		return s.processSectionLine(line)
	case "synopse":
		return s.writeFormatted("> %s\n\n", strings.TrimLeft(line.Contents, "= "))
	default:
		return s.processDefaultLine(line)
	}
}

// processTitlePageElement handles title page elements
func (s *markdownState) processTitlePageElement(line lex.Line, format string) error {
	if s.inTitlePage {
		return s.writeFormatted(format, line.Contents)
	}
	return nil
}

// processActionLine handles action lines
func (s *markdownState) processActionLine(line lex.Line) error {
	if strings.TrimSpace(line.Contents) != "" {
		return s.writeFormatted("%s\n\n", line.Contents)
	}
	return nil
}

// processSpeakerLine handles speaker lines
func (s *markdownState) processSpeakerLine(line lex.Line) error {
	if s.inDualDialogue {
		return s.writeFormatted("**%s**  \n", strings.ToUpper(line.Contents))
	}
	return s.writeFormatted("**%s**\n\n", strings.ToUpper(line.Contents))
}

// processDialogLine handles dialog and lyrics lines
func (s *markdownState) processDialogLine(line lex.Line) error {
	if s.inDualDialogue {
		return s.writeFormatted("%s  \n", line.Contents)
	}
	return s.writeFormatted("%s\n\n", line.Contents)
}

// processParenLine handles parenthetical lines
func (s *markdownState) processParenLine(line lex.Line) error {
	if s.inDualDialogue {
		return s.writeFormatted("*%s*  \n", line.Contents)
	}
	return s.writeFormatted("*%s*\n\n", line.Contents)
}

// processEmptyLine handles empty lines
func (s *markdownState) processEmptyLine() error {
	if !s.inDualDialogue {
		return s.writeString("\n")
	}
	return nil
}

// processSectionLine handles section lines
func (s *markdownState) processSectionLine(line lex.Line) error {
	level := strings.Count(line.Contents, "#")
	if level == 0 {
		level = 1
	}
	headerPrefix := strings.Repeat("#", level+2) // +2 because we use ## for scenes
	content := strings.TrimLeft(line.Contents, "# ")
	return s.writeFormatted("%s %s\n\n", headerPrefix, content)
}

// processDefaultLine handles unrecognized line types
func (s *markdownState) processDefaultLine(line lex.Line) error {
	if strings.TrimSpace(line.Contents) != "" {
		return s.writeFormatted("%s\n\n", line.Contents)
	}
	return nil
}

// writeFormatted writes formatted text to the writer
func (s *markdownState) writeFormatted(format string, args ...interface{}) error {
	_, err := fmt.Fprintf(s.writer, format, args...)
	return err
}

// writeString writes a string to the writer
func (s *markdownState) writeString(str string) error {
	_, err := fmt.Fprint(s.writer, str)
	return err
}
