// FDX is the format used by Final Draft, a popular screenwriting software.
// This package handles parsing of the .fdx XML format.
package fdx

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"slices"
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

// Parse reads an .fdx file from an io.Reader and converts it into the
// internal lex.Screenplay format. A file that cannot be decoded gives an
// empty screenplay; use ParseWithError to find out why.
func Parse(file io.Reader) lex.Screenplay {
	out, _ := ParseWithError(file)
	return out
}

// ParseWithError is Parse, reporting files that are not valid FDX.
func ParseWithError(file io.Reader) (lex.Screenplay, error) {
	var fdxFile FdxFile
	if err := xml.NewDecoder(file).Decode(&fdxFile); err != nil {
		return nil, fmt.Errorf("reading FDX: %w", err)
	}

	var out lex.Screenplay
	if fdxFile.TitlePage != nil {
		out = parseTitlePage(fdxFile.TitlePage.Content.Paragraphs)
	}
	bodyStart := len(out)

	for _, p := range fdxFile.Content.Paragraphs {
		if p.DualDialogue != nil {
			if len(out) > bodyStart {
				out = append(out, lex.Line{Type: lex.TypeEmpty})
			}
			out = append(out, parseDualDialogue(p.DualDialogue.Paragraphs)...)
			continue
		}

		line := parseParagraph(p)
		if p.StartsNewPage == "Yes" && len(out) > bodyStart {
			out = append(out, lex.Line{Type: lex.TypeNewPage})
		} else if startsBlock(line.Type) && len(out) > bodyStart && !(line.Type == lex.TypeLyrics && inDialogue(out)) {
			// Final Draft spaces blocks itself; Fountain needs a blank
			// line before each scene heading, action, character and
			// transition.
			out = append(out, lex.Line{Type: lex.TypeEmpty})
		}
		out = append(out, line)
	}

	return out, nil
}

// parseParagraph maps one paragraph to a script line.
func parseParagraph(p FdxParagraph) lex.Line {
	contents := textWithMarkup(p.Texts)
	line := lex.Line{Contents: contents}

	switch p.Type {
	case FDXSceneHeading:
		line.Type = lex.TypeScene
	case FDXCharacter:
		line.Type = lex.TypeSpeaker
	case FDXParenthetical:
		line.Type = lex.TypeParen
	case FDXDialogue:
		line.Type = lex.TypeDialog
	case FDXTransition:
		line.Type = lex.TypeTrans
	case FDXLyrics:
		line.Type = lex.TypeLyrics
	default:
		// Action, General, Shot, Cast List, act breaks, ...
		line.Type = lex.TypeAction
	}

	switch {
	case contents == "" && line.Type == lex.TypeAction:
		line.Type = lex.TypeEmpty
	case p.Alignment == "Center" && line.Type == lex.TypeAction:
		line.Type = lex.TypeCenter
	}
	return line
}

// parseDualDialogue turns the paragraphs of a <DualDialogue> (both
// characters' blocks in order) into the dual dialogue markers the
// Fountain parser produces.
func parseDualDialogue(ps []FdxParagraph) lex.Screenplay {
	out := lex.Screenplay{{Type: lex.TypeDualOpen}}
	speakers := 0
	for _, p := range ps {
		line := parseParagraph(p)
		if line.Type == lex.TypeEmpty {
			continue
		}
		if line.Type == lex.TypeSpeaker {
			speakers++
			if speakers == 2 {
				out = append(out, lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeDualNext})
			}
		}
		out = append(out, line)
	}
	return append(out, lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeDualClose})
}

// creditLine matches the credit between title and author ("Written by",
// "A Short Film by").
var creditLine = regexp.MustCompile(`(?i)\bby:?$`)

// parseTitlePage reads a Final Draft title page back into Fountain title
// fields: the first group of centred lines is the title, a line ending in
// "by" the credit, and the other centred lines the author; left-aligned
// lines (contact details, draft date) become Contact fields.
func parseTitlePage(ps []FdxParagraph) lex.Screenplay {
	var title, credit, author, contact lex.Screenplay
	inTitle := true
	for _, p := range ps {
		text := strings.TrimSpace(textWithMarkup(p.Texts))
		switch {
		case text == "":
			if title != nil {
				inTitle = false
			}
		case p.Alignment != "Center":
			contact = append(contact, lex.Line{Type: "Contact", Contents: text})
		case creditLine.MatchString(text):
			inTitle = false
			credit = append(credit, lex.Line{Type: "Credit", Contents: text})
		case inTitle:
			title = append(title, lex.Line{Type: "Title", Contents: text})
		default:
			author = append(author, lex.Line{Type: "Author", Contents: text})
		}
	}
	if title == nil && credit == nil && author == nil && contact == nil {
		return nil
	}

	out := lex.Screenplay{{Type: lex.TypeTitlePage}}
	out = append(out, title...)
	out = append(out, credit...)
	out = append(out, author...)
	if contact != nil {
		out = append(out, lex.Line{Type: "metasection"})
		out = append(out, contact...)
	}
	return append(out, lex.Line{Type: lex.TypeNewPage})
}

// fountainMarkup maps Final Draft styles to Fountain emphasis markers.
var fountainMarkup = []struct{ style, marker string }{
	{"Underline", "_"},
	{"Bold", "**"},
	{"Italic", "*"},
}

// textWithMarkup joins a paragraph's text runs, turning bold, italic and
// underlined runs into Fountain emphasis.
func textWithMarkup(texts []FdxText) string {
	var b strings.Builder
	for _, t := range texts {
		styles := strings.Split(t.Style, "+")
		open, close := "", ""
		for _, m := range fountainMarkup {
			if slices.Contains(styles, m.style) && strings.TrimSpace(t.Content) != "" {
				open += m.marker
				close = m.marker + close
			}
		}
		// keep surrounding spaces outside the markers
		content := strings.TrimSpace(t.Content)
		lead := t.Content[:strings.Index(t.Content, content)]
		trail := t.Content[len(lead)+len(content):]
		if open == "" {
			b.WriteString(t.Content)
			continue
		}
		b.WriteString(lead + open + content + close + trail)
	}
	return b.String()
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

// inDialogue reports whether the screenplay so far ends inside a
// character's dialogue, where sung lines continue the block.
func inDialogue(out lex.Screenplay) bool {
	if len(out) == 0 {
		return false
	}
	switch out[len(out)-1].Type {
	case lex.TypeSpeaker, lex.TypeDialog, lex.TypeParen, lex.TypeLyrics:
		return true
	}
	return false
}
