package markdown

import (
	"fmt"
	"io"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Constants for markdown formatting
const (
	newPageMarker     = "\n\\newpage\n\n"
	metaSeparator     = "---\n\n"
	dualDialogueOpen  = "<div style=\"display: flex; justify-content: space-between;\">\n<div style=\"width: 48%;\">\n\n"
	dualDialogueNext  = "</div>\n<div style=\"width: 48%;\">\n\n"
	dualDialogueClose = "</div>\n</div>\n\n"

	// Blockquote markers for dialogue blocks
	dialogueBlockStart = "> "
	dialogueBlockEnd   = "\n"
)

// processInlineMarkup turns Fountain's emphasis (lex.Emphasis) into
// Markdown's: ***bold italic***, **bold**, *italic*, and <u> for
// underlining, which Markdown lacks; asterisks and underscores in the
// text are escaped, spaces at a span's ends kept outside its markers.
func processInlineMarkup(text string) string {
	if !strings.ContainsAny(text, `*_\`) {
		return text
	}
	var b strings.Builder
	for _, s := range lex.Emphasis(text) {
		inner := strings.TrimSpace(s.Text)
		if inner == "" || !(s.Bold || s.Italic || s.Underline) {
			b.WriteString(lex.EscapeEmphasis(s.Text))
			continue
		}
		lead := s.Text[:strings.Index(s.Text, inner)]
		trail := s.Text[len(lead)+len(inner):]
		t := lex.EscapeEmphasis(inner)
		if s.Underline {
			t = "<u>" + t + "</u>"
		}
		switch {
		case s.Bold && s.Italic:
			t = "***" + t + "***"
		case s.Bold:
			t = "**" + t + "**"
		case s.Italic:
			t = "*" + t + "*"
		}
		b.WriteString(lead + t + trail)
	}
	return b.String()
}

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

	for _, line := range lex.Printable(screenplay, false) { // notes are not printed
		if err := state.processLine(line); err != nil {
			return err
		}
	}

	return nil
}

// markdownState holds the state during markdown conversion
type markdownState struct {
	writer          io.Writer
	inTitlePage     bool
	inDualDialogue  bool
	inDialogueBlock bool
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
		lineType == "Credit" || lineType == "Author" || lineType == "metasection" ||
		lineType == "Source" || lineType == "Contact" || lineType == "Draft date" ||
		lineType == "Notes" || lineType == "Copyright"
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
	case "Source", "Contact", "Draft date", "Notes", "Copyright":
		return s.processTitlePageElement(line, "%s\n\n")
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
		return s.writeFormatted("## %s\n\n", strings.ToUpper(processInlineMarkup(line.Contents)))
	case lex.TypeAction:
		return s.processActionLine(line)
	case lex.TypeSpeaker:
		return s.processSpeakerLine(line)
	case lex.TypeDialog, lex.TypeLyrics:
		return s.processDialogLine(line)
	case lex.TypeParen:
		return s.processParenLine(line)
	case "trans":
		return s.writeFormatted("**%s**\n\n", strings.ToUpper(processInlineMarkup(line.Contents)))
	case lex.TypeCenter:
		return s.writeFormatted("<center>%s</center>\n\n", processInlineMarkup(line.Contents))
	case lex.TypeEmpty:
		return s.processEmptyLine()
	case "section":
		return s.processSectionLine(line)
	case "synopse":
		return s.writeFormatted("> %s\n\n", processInlineMarkup(strings.TrimLeft(line.Contents, "= ")))
	default:
		return s.processDefaultLine(line)
	}
}

// processTitlePageElement handles title page elements
func (s *markdownState) processTitlePageElement(line lex.Line, format string) error {
	return s.writeFormatted(format, processInlineMarkup(line.Contents))
}

// processActionLine handles action lines
func (s *markdownState) processActionLine(line lex.Line) error {
	if err := s.endDialogueBlock(); err != nil {
		return err
	}
	if strings.TrimSpace(line.Contents) != "" {
		return s.writeFormatted("%s\n\n", processInlineMarkup(line.Contents))
	}
	return nil
}

// processSpeakerLine handles speaker lines
func (s *markdownState) processSpeakerLine(line lex.Line) error {
	if err := s.startDialogueBlock(); err != nil {
		return err
	}
	if s.inDualDialogue {
		return s.writeFormatted("%s**%s**  \n", dialogueBlockStart, strings.ToUpper(processInlineMarkup(line.Contents)))
	}
	return s.writeFormatted("%s**%s**\n\n", dialogueBlockStart, strings.ToUpper(processInlineMarkup(line.Contents)))
}

// processDialogLine handles dialog and lyrics lines
func (s *markdownState) processDialogLine(line lex.Line) error {
	if err := s.startDialogueBlock(); err != nil {
		return err
	}
	if s.inDualDialogue {
		return s.writeFormatted("%s%s  \n", dialogueBlockStart, processInlineMarkup(line.Contents))
	}
	return s.writeFormatted("%s%s\n\n", dialogueBlockStart, processInlineMarkup(line.Contents))
}

// processParenLine handles parenthetical lines
func (s *markdownState) processParenLine(line lex.Line) error {
	if err := s.startDialogueBlock(); err != nil {
		return err
	}
	if s.inDualDialogue {
		return s.writeFormatted("%s*%s*  \n", dialogueBlockStart, processInlineMarkup(line.Contents))
	}
	return s.writeFormatted("%s*%s*\n\n", dialogueBlockStart, processInlineMarkup(line.Contents))
}

// processEmptyLine handles empty lines
func (s *markdownState) processEmptyLine() error {
	if err := s.endDialogueBlock(); err != nil {
		return err
	}
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
	return s.writeFormatted("%s %s\n\n", headerPrefix, processInlineMarkup(content))
}

// processDefaultLine handles unrecognized line types
func (s *markdownState) processDefaultLine(line lex.Line) error {
	if line.Type == "boneyard" || line.Type == lex.TypeNote { // never printed
		return nil
	}
	if strings.TrimSpace(line.Contents) != "" {
		return s.writeFormatted("%s\n\n", processInlineMarkup(line.Contents))
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

// startDialogueBlock starts a dialogue block if not already in one
func (s *markdownState) startDialogueBlock() error {
	if !s.inDialogueBlock {
		s.inDialogueBlock = true
	}
	return nil
}

// endDialogueBlock ends a dialogue block if currently in one
func (s *markdownState) endDialogueBlock() error {
	if s.inDialogueBlock {
		s.inDialogueBlock = false
		return s.writeString(dialogueBlockEnd)
	}
	return nil
}
