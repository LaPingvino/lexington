package fdx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"text/template"

	"github.com/lapingvino/lexington/lex"
)

// FDXWriter implements the writer.Writer interface for FDX output.
type FDXWriter struct {
	TemplatePath string // Path to a custom FDX template file
}

const defaultFDXTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<FinalDraft Version="1.0">
  <Content>
{{range .Paragraphs}}    <Paragraph Type="{{.Type}}">
      <Text>{{range .Texts}}{{.Content}}{{end}}</Text>
    </Paragraph>
{{end}}  </Content>
</FinalDraft>
`

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
		case "scene":
			pType = "Scene Heading"
		case "action", "center": // Assuming 'center' can be treated as 'Action' for FDX export
			pType = "Action"
		case "empty":
			// An empty line in Fountain is often an empty Action paragraph in FDX.
			pType = "Action"
		case "speaker":
			pType = "Character"
		case "paren":
			pType = "Parenthetical"
		case "dialog", "lyrics":
			pType = "Dialogue"
		case "trans":
			pType = "Transition"
		case "dualspeaker_open", "dualspeaker_next", "dualspeaker_close":
			// Dual dialogue is complex in FDX and might require a more sophisticated
			// transformation than a simple text template can provide.
			// For this basic template, we'll skip these markers for now,
			// or treat them as actions/general text if content is present.
			// A full FDX dual dialogue implementation would involve nested structures.
			continue
		default:
			// Use "General" as a fallback for any unrecognized types.
			pType = "General"
		}

		// Escape XML special characters
		escapedContent := escapeXML(line.Contents)

		paragraph := FdxParagraph{
			Type: pType,
			Texts: []FdxText{
				{Content: escapedContent},
			},
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
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
