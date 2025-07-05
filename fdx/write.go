package fdx

import (
	"encoding/xml"
	"io"

	"github.com/lapingvino/lexington/lex"
)

// Write converts the internal lex.Screenplay format to an FDX XML file.
// Note: This is a basic implementation and does not handle all FDX features,
// such as title pages or complex text adornments.
func Write(f io.Writer, screenplay lex.Screenplay) {
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
	f.Write([]byte(xml.Header))

	// Use an encoder to get indented, human-readable XML output.
	encoder := xml.NewEncoder(f)
	encoder.Indent("", "  ")
	err := encoder.Encode(fdxFile)
	if err != nil {
		// In case of an error, write a comment into the file.
		f.Write([]byte("<!-- Error encoding FDX file: " + err.Error() + " -->"))
	}
}
