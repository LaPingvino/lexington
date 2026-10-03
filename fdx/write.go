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

// markupMatch represents a found markup pattern
type markupMatch struct {
	start      int
	end        int
	markupType string
}

// findEarliestMarkup finds the earliest markup pattern in the text
func findEarliestMarkup(text string) *markupMatch {
	var earliest *markupMatch

	// checked in order, so on a tie the longer marker wins
	patterns := []struct {
		markupType string
		pattern    *regexp.Regexp
	}{
		{"bolditalic", bolditalic},
		{"bold", bold},
		{"italic", italic},
		{"underline", underline},
	}

	for _, p := range patterns {
		if match := p.pattern.FindStringIndex(text); match != nil {
			if earliest == nil || match[0] < earliest.start {
				earliest = &markupMatch{
					start:      match[0],
					end:        match[1],
					markupType: p.markupType,
				}
			}
		}
	}

	return earliest
}

// fdxStyles maps Fountain emphasis to Final Draft's Style attribute.
var fdxStyles = map[string]string{
	"bolditalic": "Bold+Italic",
	"bold":       "Bold",
	"italic":     "Italic",
	"underline":  "Underline",
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

// processInlineMarkup converts fountain-style inline markup to FDX Text
// elements. The text is not XML-escaped; the XML encoder does that.
func processInlineMarkup(text string) []FdxText {
	if !strings.ContainsAny(text, "*_") {
		return []FdxText{{Content: text}}
	}

	var result []FdxText
	remaining := text

	for len(remaining) > 0 {
		match := findEarliestMarkup(remaining)
		if match == nil {
			result = append(result, FdxText{Content: remaining})
			break
		}

		if match.start > 0 {
			result = append(result, FdxText{Content: remaining[:match.start]})
		}

		matchedText := remaining[match.start:match.end]
		result = append(result, FdxText{
			Content: extractContent(matchedText, match.markupType),
			Style:   fdxStyles[match.markupType],
		})

		remaining = remaining[match.end:]
	}

	return result
}

// paragraphType maps a lex element type to a Final Draft paragraph type.
// ok is false for lines that have no paragraph of their own.
func paragraphType(t lex.ElementType) (pType string, ok bool) {
	switch t {
	case lex.TypeScene:
		return FDXSceneHeading, true
	case lex.TypeAction, lex.TypeCenter:
		return FDXAction, true
	case lex.TypeSpeaker:
		return FDXCharacter, true
	case lex.TypeParen:
		return FDXParenthetical, true
	case lex.TypeDialog:
		return FDXDialogue, true
	case lex.TypeLyrics:
		return FDXLyrics, true
	case lex.TypeTrans:
		return FDXTransition, true
	case lex.TypeEmpty, lex.TypeTitlePage, lex.TypeNewPage, "metasection", "section", "synopse", "note":
		// Final Draft spaces elements itself; blank lines (handled in
		// buildDocument) and structure markers have no paragraph.
		return "", false
	}
	return FDXGeneral, true
}

// titleRole returns where a title page field goes: "title", "credit" or
// "author" lines are centered; anything else (contact details, draft
// date, source) is "other" and goes below them.
func titleRole(t lex.ElementType) string {
	switch strings.ToLower(string(t)) {
	case "title":
		return "title"
	case "credit":
		return "credit"
	case "author", "authors":
		return "author"
	}
	return "other"
}

// alignCenter is Final Draft's Alignment value for centered paragraphs.
const alignCenter = "Center"

// buildDocument converts a screenplay into the FDX document structure.
func buildDocument(screenplay lex.Screenplay) FdxFile {
	doc := FdxFile{DocumentType: "Script", Template: "No", Version: "5"}

	i := 0
	if len(screenplay) > 0 && screenplay[0].Type == lex.TypeTitlePage {
		doc.TitlePage, i = buildTitlePage(screenplay)
	}

	b := docBuilder{doc: &doc}
	for _, line := range screenplay[i:] {
		b.add(line)
	}
	b.closeDual()
	return doc
}

// docBuilder adds screenplay lines to an FDX document, keeping track of
// blank lines, pending page breaks and open dual dialogue.
type docBuilder struct {
	doc     *FdxFile
	dual    *FdxDualDialogue
	newPage bool
	blanks  int
}

func (b *docBuilder) add(line lex.Line) {
	if line.Type == lex.TypeEmpty {
		b.blanks++
		return
	}
	b.flushBlanks()
	if b.structural(line.Type) {
		return
	}

	pType, ok := paragraphType(line.Type)
	if !ok {
		return
	}
	p := FdxParagraph{Type: pType, Texts: processInlineMarkup(line.Contents)}
	if line.Type == lex.TypeCenter {
		p.Alignment = alignCenter
	}
	if b.dual != nil {
		b.dual.Paragraphs = append(b.dual.Paragraphs, p)
		return
	}
	if b.newPage {
		p.StartsNewPage = "Yes"
		b.newPage = false
	}
	b.doc.Content.Paragraphs = append(b.doc.Content.Paragraphs, p)
}

// flushBlanks writes the blank lines before a block: one blank line
// separates blocks; each further one is an empty paragraph the writer put
// there on purpose.
func (b *docBuilder) flushBlanks() {
	for ; b.blanks > 1 && b.dual == nil; b.blanks-- {
		b.doc.Content.Paragraphs = append(b.doc.Content.Paragraphs, FdxParagraph{Type: FDXAction, Texts: []FdxText{{}}})
	}
	b.blanks = 0
}

// structural handles page breaks and dual dialogue markers, reporting
// whether the line was one.
func (b *docBuilder) structural(t lex.ElementType) bool {
	switch t {
	case lex.TypeNewPage:
		b.newPage = true
	case lex.TypeDualOpen:
		b.dual = &FdxDualDialogue{}
	case lex.TypeDualNext:
	case lex.TypeDualClose:
		b.closeDual()
	default:
		return false
	}
	return true
}

// closeDual ends dual dialogue, adding it if it has any paragraphs.
func (b *docBuilder) closeDual() {
	if b.dual != nil && len(b.dual.Paragraphs) > 0 {
		b.doc.Content.Paragraphs = append(b.doc.Content.Paragraphs, FdxParagraph{DualDialogue: b.dual})
	}
	b.dual = nil
}

// buildTitlePage turns the title page lines at the start of the screenplay
// into a Final Draft title page: title fields centered, other fields (contact
// details, draft date) left-aligned below them. It returns the index of the
// first line after the title page.
func buildTitlePage(screenplay lex.Screenplay) (*FdxTitlePage, int) {
	roles := map[string][]FdxParagraph{}
	i := 1
	for ; i < len(screenplay); i++ {
		line := screenplay[i]
		if line.Type == lex.TypeNewPage {
			i++
			break
		}
		if line.Type == "metasection" || line.Contents == "" {
			continue
		}
		role := titleRole(line.Type)
		p := FdxParagraph{Type: FDXGeneral, Texts: processInlineMarkup(line.Contents)}
		if role != "other" {
			p.Alignment = alignCenter
		}
		roles[role] = append(roles[role], p)
	}

	// Title, a blank line, then credit and author: the usual layout, and
	// the one ParseWithError reads back.
	tp := &FdxTitlePage{}
	add := func(ps ...FdxParagraph) { tp.Content.Paragraphs = append(tp.Content.Paragraphs, ps...) }
	add(roles["title"]...)
	if len(roles["credit"])+len(roles["author"]) > 0 {
		if len(roles["title"]) > 0 {
			add(FdxParagraph{Type: FDXGeneral, Alignment: alignCenter})
		}
		add(roles["credit"]...)
		add(roles["author"]...)
	}
	if len(roles["other"]) > 0 {
		if len(tp.Content.Paragraphs) > 0 {
			add(FdxParagraph{Type: FDXGeneral})
		}
		add(roles["other"]...)
	}
	return tp, i
}

// Write converts the internal lex.Screenplay format to an FDX XML file.
// It implements the writer.Writer interface.
func (f *FDXWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	doc := buildDocument(screenplay)

	if f.TemplatePath != "" {
		return f.writeTemplate(w, doc)
	}

	if _, err := io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>`+"\n"); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("failed to encode FDX: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// writeTemplate renders a custom template. Templates get the document's
// Content with XML-escaped text; dual dialogue is flattened into the
// paragraph list, as templates cannot recurse.
func (f *FDXWriter) writeTemplate(w io.Writer, doc FdxFile) error {
	tmpl, err := template.ParseFiles(f.TemplatePath)
	if err != nil {
		return fmt.Errorf("failed to parse FDX template file %s: %w", f.TemplatePath, err)
	}

	var content FdxContent
	for _, p := range doc.Content.Paragraphs {
		if p.DualDialogue != nil {
			for _, dp := range p.DualDialogue.Paragraphs {
				content.Paragraphs = append(content.Paragraphs, escapeParagraph(dp))
			}
			continue
		}
		content.Paragraphs = append(content.Paragraphs, escapeParagraph(p))
	}
	return tmpl.Execute(w, content)
}

func escapeParagraph(p FdxParagraph) FdxParagraph {
	texts := make([]FdxText, len(p.Texts))
	for i, t := range p.Texts {
		t.Content = escapeXML(t.Content)
		texts[i] = t
	}
	p.Texts = texts
	return p
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
