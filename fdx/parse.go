// FDX is the format used by Final Draft, a popular screenwriting software.
// This package handles parsing of the .fdx XML format.
package fdx

import (
	"encoding/xml"
	"io"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// FdxFile represents the top-level <FinalDraft> element.
type FdxFile struct {
	XMLName xml.Name   `xml:"FinalDraft"`
	Content FdxContent `xml:"Content"`
}

// FdxContent represents the <Content> element, which contains paragraphs.
type FdxContent struct {
	XMLName    xml.Name       `xml:"Content"`
	Paragraphs []FdxParagraph `xml:"Paragraph"`
}

// FdxParagraph represents a <Paragraph> element, which can be a scene heading, action, etc.
type FdxParagraph struct {
	XMLName xml.Name  `xml:"Paragraph"`
	Type    string    `xml:"Type,attr"`
	Texts   []FdxText `xml:"Text"`
}

// FdxText represents a <Text> element which contains the actual script content.
// A paragraph can have multiple text elements for styling purposes.
type FdxText struct {
	Content        string `xml:",chardata"`
	AdornmentStyle string `xml:"AdornmentStyle,attr,omitempty"`
	Background     string `xml:"Background,attr,omitempty"`
	Color          string `xml:"Color,attr,omitempty"`
	Font           string `xml:"Font,attr,omitempty"`
	RevisionID     string `xml:"RevisionID,attr,omitempty"`
	Size           string `xml:"Size,attr,omitempty"`
	Style          string `xml:"Style,attr,omitempty"`
}

// Parse reads an .fdx file from an io.Reader and converts it into the internal lex.Screenplay format.
func Parse(file io.Reader) (out lex.Screenplay) {
	var fdxFile FdxFile
	decoder := xml.NewDecoder(file)
	err := decoder.Decode(&fdxFile)
	if err != nil {
		// In a real-world scenario, you'd want better error handling.
		// For now, we'll return what we have if a decoding error occurs mid-stream.
		return
	}

	for _, p := range fdxFile.Content.Paragraphs {
		var line lex.Line
		var contents []string
		for _, t := range p.Texts {
			contents = append(contents, t.Content)
		}
		fullContent := strings.Join(contents, "")

		// Map FDX types to internal lex types
		switch p.Type {
		case FDXSceneHeading:
			line.Type = lex.TypeScene
		case FDXAction, FDXGeneral:
			if fullContent == "" {
				line.Type = lex.TypeEmpty
			} else {
				line.Type = lex.TypeAction
			}
		case FDXCharacter:
			line.Type = lex.TypeSpeaker
		case FDXParenthetical:
			line.Type = lex.TypeParen
		case FDXDialogue:
			line.Type = lex.TypeDialog
		case FDXTransition:
			line.Type = lex.TypeTrans
		default:
			// If we don't recognize the type, treat it as a generic action.
			line.Type = lex.TypeAction
		}

		line.Contents = fullContent
		out = append(out, line)
	}

	return out
}
