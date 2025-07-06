package fdx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"text/template"

	"github.com/LaPingvino/lexington/lex"
)

// Inline markup patterns for FDX output
var (
	bolditalic = regexp.MustCompile(`\*{3}([^\*\n]+)\*{3}`)
	bold       = regexp.MustCompile(`\*{2}([^\*\n]+)\*{2}`)
	italic     = regexp.MustCompile(`\*{1}([^\*\n]+)\*{1}`)
	underline  = regexp.MustCompile(`_{1}([^\*\n]+)_{1}`)
)

// FDXWriter implements the writer.Writer interface for FDX output.
type FDXWriter struct {
	TemplatePath string // Path to a custom FDX template file
}

const defaultFDXTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<FinalDraft Version="1.0">
  <Content>
{{range .Paragraphs}}    <Paragraph Type="{{.Type}}">
{{range .Texts}}      <Text` +
	`{{if .AdornmentStyle}} AdornmentStyle="{{.AdornmentStyle}}"{{end}}` +
	`{{if .Background}} Background="{{.Background}}"{{end}}` +
	`{{if .Color}} Color="{{.Color}}"{{end}}` +
	`{{if .Font}} Font="{{.Font}}"{{end}}` +
	`{{if .RevisionID}} RevisionID="{{.RevisionID}}"{{end}}` +
	`{{if .Size}} Size="{{.Size}}"{{end}}` +
	`{{if .Style}} Style="{{.Style}}"{{end}}>{{.Content}}</Text>
{{end}}    </Paragraph>
{{end}}  </Content>
</FinalDraft>
`

// markupMatch represents a found markup pattern
type markupMatch struct {
	start      int
	end        int
	markupType string
}

// findEarliestMarkup finds the earliest markup pattern in the text
func findEarliestMarkup(text string) *markupMatch {
	var earliest *markupMatch

	patterns := map[string]*regexp.Regexp{
		"bolditalic": bolditalic,
		"bold":       bold,
		"italic":     italic,
		"underline":  underline,
	}

	for markupType, pattern := range patterns {
		if match := pattern.FindStringIndex(text); match != nil {
			if earliest == nil || match[0] < earliest.start {
				earliest = &markupMatch{
					start:      match[0],
					end:        match[1],
					markupType: markupType,
				}
			}
		}
	}

	return earliest
}

// createStyledText creates an FdxText with the appropriate styling
func createStyledText(content, markupType string) FdxText {
	baseText := FdxText{
		Content:    escapeXML(content),
		Background: "#FFFFFFFFFFFF",
		Color:      "#000000000000",
		Font:       "Courier",
		RevisionID: "0",
		Size:       "12",
	}

	switch markupType {
	case "bolditalic", "bold":
		baseText.AdornmentStyle = "0"
		baseText.Style = "Bold"
	case "italic":
		baseText.AdornmentStyle = "-1"
		baseText.Style = ""
	case "underline":
		baseText.AdornmentStyle = "0"
		baseText.Style = "Underline"
	}

	return baseText
}

// extractContent extracts the content from markup based on type
func extractContent(text, markupType string) string {
	switch markupType {
	case "bolditalic":
		return bolditalic.FindStringSubmatch(text)[1]
	case "bold":
		return bold.FindStringSubmatch(text)[1]
	case "italic":
		return italic.FindStringSubmatch(text)[1]
	case "underline":
		return underline.FindStringSubmatch(text)[1]
	default:
		return ""
	}
}

// processInlineMarkup converts fountain-style inline markup to FDX Text elements
func processInlineMarkup(text string) []FdxText {
	if !strings.ContainsAny(text, "*_") {
		return []FdxText{{Content: escapeXML(text)}}
	}

	var result []FdxText
	remaining := text

	for len(remaining) > 0 {
		match := findEarliestMarkup(remaining)
		if match == nil {
			if len(remaining) > 0 {
				result = append(result, FdxText{Content: escapeXML(remaining)})
			}
			break
		}

		// Add text before the markup as normal text
		if match.start > 0 {
			result = append(result, FdxText{Content: escapeXML(remaining[:match.start])})
		}

		// Extract content and create styled text
		matchedText := remaining[match.start:match.end]
		content := extractContent(matchedText, match.markupType)
		styledText := createStyledText(content, match.markupType)
		result = append(result, styledText)

		// Move to the text after this markup
		remaining = remaining[match.end:]
	}

	return result
}

// Write converts the internal lex.Screenplay format to an FDX XML file.
// It implements the writer.Writer interface.
func (f *FDXWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	var fdxFile FdxFile

	for _, line := range screenplay {
		// Skip structural lex types that don't directly map to FDX paragraphs
		switch line.Type {
		case "titlepage", "metasection", "newpage", "section", "synopse":
			continue
		}

		var pType string
		// Map internal lex types to FDX Paragraph types.
		switch line.Type {
		case lex.TypeScene:
			pType = FDXSceneHeading
		case lex.TypeAction, lex.TypeCenter: // Assuming 'center' can be treated as 'Action' for FDX export
			pType = FDXAction
		case lex.TypeEmpty:
			// An empty line in Fountain is often an empty Action paragraph in FDX.
			pType = FDXAction
		case lex.TypeSpeaker:
			pType = FDXCharacter
		case lex.TypeParen:
			pType = FDXParenthetical
		case lex.TypeDialog, lex.TypeLyrics:
			pType = FDXDialogue
		case lex.TypeTrans:
			pType = FDXTransition
		case lex.TypeDualOpen, lex.TypeDualNext, lex.TypeDualClose:
			// Dual dialogue is complex in FDX and might require a more sophisticated
			// transformation than a simple text template can provide.
			// For this basic template, we'll skip these markers for now,
			// or treat them as actions/general text if content is present.
			// A full FDX dual dialogue implementation would involve nested structures.
			continue
		default:
			// Use "General" as a fallback for any unrecognized types.
			pType = FDXGeneral
		}

		// Process inline markup to create multiple text elements
		texts := processInlineMarkup(line.Contents)

		paragraph := FdxParagraph{
			Type:  pType,
			Texts: texts,
		}

		fdxFile.Content.Paragraphs = append(fdxFile.Content.Paragraphs, paragraph)
	}

	var tmpl *template.Template
	var err error

	if f.TemplatePath != "" {
		tmpl, err = template.ParseFiles(f.TemplatePath)
		if err != nil {
			return fmt.Errorf("failed to parse FDX template file %s: %w", f.TemplatePath, err)
		}
	} else {
		tmpl, err = template.New("fdxScreenplay").Parse(defaultFDXTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse default FDX template: %w", err)
		}
	}

	// Execute the template with the constructed FdxFile data.
	// We need to pass fdxFile.Content as the top-level data for the template
	// because the template expects a slice of Paragraphs.
	return tmpl.Execute(w, fdxFile.Content)
}

// escapeXML escapes characters that have special meaning in XML.
func escapeXML(s string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		// xml.EscapeText should not fail for valid strings, but handle error just in case
		return s
	}
	return b.String()
}
