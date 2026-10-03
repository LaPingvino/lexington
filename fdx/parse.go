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
	XMLName      xml.Name      `xml:"FinalDraft"`
	DocumentType string        `xml:"DocumentType,attr,omitempty"`
	Template     string        `xml:"Template,attr,omitempty"`
	Version      string        `xml:"Version,attr,omitempty"`
	Content      FdxContent    `xml:"Content"`
	TitlePage    *FdxTitlePage `xml:"TitlePage,omitempty"`
}

// FdxContent represents a <Content> element, which contains paragraphs.
type FdxContent struct {
	Paragraphs []FdxParagraph `xml:"Paragraph"`
}

// FdxTitlePage represents the <TitlePage> element.
type FdxTitlePage struct {
	Content FdxContent `xml:"Content"`
}

// FdxParagraph represents a <Paragraph> element, which can be a scene
// heading, action, etc. A paragraph wrapping a <DualDialogue> has no type.
type FdxParagraph struct {
	XMLName       xml.Name         `xml:"Paragraph"`
	Type          string           `xml:"Type,attr,omitempty"`
	Alignment     string           `xml:"Alignment,attr,omitempty"`
	StartsNewPage string           `xml:"StartsNewPage,attr,omitempty"`
	Texts         []FdxText        `xml:"Text"`
	DualDialogue  *FdxDualDialogue `xml:"DualDialogue,omitempty"`
}

// FdxDualDialogue holds the paragraphs of two characters speaking at once:
// the first character's paragraphs followed by the second's.
type FdxDualDialogue struct {
	Paragraphs []FdxParagraph `xml:"Paragraph"`
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
		// Final Draft spaces blocks itself; Fountain needs a blank line
		// before each scene heading, action, character and transition.
		if startsBlock(line.Type) && len(out) > 0 {
			out = append(out, lex.Line{Type: lex.TypeEmpty})
		}
		out = append(out, line)
	}

	return out
}

// startsBlock reports whether an element begins a new block, as opposed to
// continuing a character's dialogue (or being an empty paragraph itself).
func startsBlock(t lex.ElementType) bool {
	switch t {
	case lex.TypeParen, lex.TypeDialog, lex.TypeEmpty:
		return false
	}
	return true
}
