package fdx

import (
	"encoding/xml"
	"io"

	"github.com/lapingvino/lexington/lex"
)

// Write converts the internal lex.Screenplay format to an FDX XML file.
// Note: This is a basic implementation and does not handle all FDX features,
// such as title pages or complex text adornments.
// FDXWriter implements the writer.Writer interface for FDX output.
type FDXWriter struct{}

// Write converts the internal lex.Screenplay format to an FDX XML file.
// It implements the writer.Writer interface.
func (f *FDXWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	var fdxFile FdxFile
	fdxFile.XMLName = xml.Name{Local: "FinalDraft"}

	for _, line := range screenplay {
		// Title page elements have a separate structure in FDX and are skipped for now.
		// Other non-paragraph elements are also skipped.
		switch line.Type {
		case "titlepage", "metasection", "newpage", "section", "synopse":
			continue
		}

		var pType string
		// Map internal lex types to FDX Paragraph types.
		switch line.Type {
		case "scene":
			pType = "Scene Heading"
		case "action", "center":
			pType = "Action"
		case "empty":
			// An empty line in Fountain is an empty Action paragraph in FDX.
			pType = "Action"
		case "speaker":
			pType = "Character"
		case "paren":
			pType = "Parenthetical"
		case "dialog", "lyrics":
			pType = "Dialogue"
		case "trans":
			pType = "Transition"
		default:
			// Use "General" as a fallback for any unrecognized types.
			pType = "General"
		}

		paragraph := FdxParagraph{
			Type: pType,
			// FDX can have multiple <Text> elements for styling, but for now
			// we'll just use one per paragraph.
			Texts: []FdxText{
				{Content: line.Contents},
			},
		}

		fdxFile.Content.Paragraphs = append(fdxFile.Content.Paragraphs, paragraph)
	}

	// Write the standard XML header to the file.
	_, err := w.Write([]byte(xml.Header))
	if err != nil {
		return err
	}

	// Use an encoder to get indented, human-readable XML output.
	encoder := xml.NewEncoder(w)
	encoder.Indent("", "  ")
	err = encoder.Encode(fdxFile)
	if err != nil {
		return err
	}
	return nil
}
