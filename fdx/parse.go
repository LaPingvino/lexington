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
	"unicode"

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
	// HeaderAndFooter is the page's header and footer: written from
	// Highland's {{HEADER}} and {{FOOTER}} (highland.go); not read.
	HeaderAndFooter *FdxHeaderAndFooter `xml:"HeaderAndFooter,omitempty"`
	// ElementSettings are the look of each paragraph type: Final Draft
	// writes them for every type it has (in capitals, hidden, ...).
	ElementSettings []FdxElementSettings `xml:"ElementSettings,omitempty"`
	// LockedPages are the locked pages (pages.go).
	LockedPages *FdxLockedPages `xml:"LockedPages,omitempty"`
	// Revisions are the revision sets, DeletedText where their deletions
	// were (revisions.go).
	Revisions   *FdxRevisions   `xml:"Revisions,omitempty"`
	DeletedText *FdxDeletedText `xml:"DeletedText,omitempty"`
	// TagData are the breakdown's categories, elements and tags (tags.go).
	TagData *FdxTagData `xml:"TagData,omitempty"`
}

// FdxContent represents a <Content> element, which contains paragraphs.
type FdxContent struct {
	Paragraphs []FdxParagraph `xml:"Paragraph"`
}

// FdxTitlePage represents the <TitlePage> element.
type FdxTitlePage struct {
	Content FdxContent `xml:"Content"`
}

// FdxElementSettings is the look of a paragraph type.
type FdxElementSettings struct {
	Type          string           `xml:"Type,attr"`
	FontSpec      FdxFontSpec      `xml:"FontSpec"`
	ParagraphSpec FdxParagraphSpec `xml:"ParagraphSpec"`
	Behavior      *FdxBehavior     `xml:"Behavior,omitempty"`
}

// FdxBehavior is how Final Draft's editor treats a paragraph type: how it
// paginates, the type after it on Return, its shortcut.
type FdxBehavior struct {
	PaginateAs  string `xml:"PaginateAs,attr,omitempty"`
	ReturnKey   string `xml:"ReturnKey,attr,omitempty"`
	Shortcut    string `xml:"Shortcut,attr,omitempty"`
	WinShortcut string `xml:"WinShortcut,attr,omitempty"`
}

// FdxFontSpec is the type of a paragraph type: its font and its Style
// ("AllCaps", "Bold+Underline+AllCaps", "Italic+HiddenText", ...).
type FdxFontSpec struct {
	AdornmentStyle string `xml:"AdornmentStyle,attr,omitempty"`
	Background     string `xml:"Background,attr,omitempty"`
	Color          string `xml:"Color,attr,omitempty"`
	Font           string `xml:"Font,attr,omitempty"`
	RevisionID     string `xml:"RevisionID,attr,omitempty"`
	Size           string `xml:"Size,attr,omitempty"`
	Style          string `xml:"Style,attr"`
}

// FdxParagraphSpec is the layout of a paragraph type, in inches from the
// page's left edge.
type FdxParagraphSpec struct {
	Alignment     string `xml:"Alignment,attr,omitempty"`
	FirstIndent   string `xml:"FirstIndent,attr,omitempty"`
	Leading       string `xml:"Leading,attr,omitempty"`
	LeftIndent    string `xml:"LeftIndent,attr,omitempty"`
	RightIndent   string `xml:"RightIndent,attr,omitempty"`
	SpaceBefore   string `xml:"SpaceBefore,attr,omitempty"`
	Spacing       string `xml:"Spacing,attr,omitempty"`
	StartsNewPage string `xml:"StartsNewPage,attr,omitempty"`
}

// FdxParagraph represents a <Paragraph> element, which can be a scene
// heading, action, etc. A paragraph wrapping a <DualDialogue> has no type
// (Fade In) or is General (Final Draft).
type FdxParagraph struct {
	XMLName       xml.Name `xml:"Paragraph"`
	Type          string   `xml:"Type,attr,omitempty"`
	Number        string   `xml:"Number,attr,omitempty"` // a scene heading's number: 12A
	Alignment     string   `xml:"Alignment,attr,omitempty"`
	StartsNewPage string   `xml:"StartsNewPage,attr,omitempty"`
	// ScriptNotes are Final Draft's notes on the paragraph: Fountain's
	// [[notes]]. Final Draft writes them before the paragraph's text.
	ScriptNotes     []FdxScriptNote     `xml:"ScriptNote,omitempty"`
	SceneProperties *FdxSceneProperties `xml:"SceneProperties,omitempty"`
	Texts           []FdxText           `xml:"Text"`
	// TrailingNotes are script notes written after the text: the notes
	// in a line's text (ScriptNotes, before it, are a line of notes
	// before the paragraph). Read, every note is in ScriptNotes.
	TrailingNotes []trailingNote   `xml:",any,omitempty"`
	DualDialogue  *FdxDualDialogue `xml:"DualDialogue,omitempty"`

	// noteAt is, for each of the ScriptNotes read, the number of Texts
	// before it.
	noteAt []int
}

// trailingNote is a script note after a paragraph's text.
type trailingNote struct {
	XMLName xml.Name `xml:"ScriptNote"`
	FdxScriptNote
}

// FdxScriptNote is a note on a paragraph, in paragraphs of its own.
type FdxScriptNote struct {
	Color      string         `xml:"Color,attr"`
	Name       string         `xml:"Name,attr"`
	Type       string         `xml:"Type,attr"`
	Paragraphs []FdxParagraph `xml:"Paragraph"`
}

// FdxSceneProperties are a scene heading's properties; its Summary is the
// scene's synopsis (not in our fixtures; as described for Final Draft
// exports by others).
type FdxSceneProperties struct {
	// Color is the scene's color (colors.go), "#000000000000" for none.
	Color string `xml:"Color,attr,omitempty"`
	// Length is the scene's length in eighths of a page ("1 3/8"), Page
	// the page its heading is on, as Final Draft writes them.
	Length  string         `xml:"Length,attr,omitempty"`
	Page    string         `xml:"Page,attr,omitempty"`
	Title   string         `xml:"Title,attr"`
	Summary *FdxScriptNote `xml:"Summary,omitempty"`
}

// UnmarshalXML reads a paragraph keeping where its notes are among its
// text, and skipping what Lexington has no use for (SceneArcBeats, tags,
// DynamicLabels).
func (p *FdxParagraph) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.XMLName = start.Name
	for _, a := range start.Attr {
		switch a.Name.Local {
		case "Type":
			p.Type = a.Value
		case "Number":
			p.Number = a.Value
		case "Alignment":
			p.Alignment = a.Value
		case "StartsNewPage":
			p.StartsNewPage = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			var err error
			switch t.Name.Local {
			case "Text":
				var text FdxText
				err = d.DecodeElement(&text, &t)
				p.Texts = append(p.Texts, text)
			case "ScriptNote":
				var n FdxScriptNote
				err = d.DecodeElement(&n, &t)
				p.ScriptNotes = append(p.ScriptNotes, n)
				p.noteAt = append(p.noteAt, len(p.Texts))
			case "DualDialogue":
				p.DualDialogue = &FdxDualDialogue{}
				err = d.DecodeElement(p.DualDialogue, &t)
			case "SceneProperties":
				p.SceneProperties = &FdxSceneProperties{}
				err = d.DecodeElement(p.SceneProperties, &t)
			default:
				err = d.Skip()
			}
			if err != nil {
				return err
			}
		}
	}
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
	// TagNumber is the breakdown Tag the run is (tags.go).
	TagNumber string `xml:"TagNumber,attr,omitempty"`
}

// Parse reads an .fdx file from an io.Reader and converts it into the
// internal lex.Screenplay format. A file that cannot be decoded gives an
// empty screenplay; use ParseWithError to find out why.
func Parse(file io.Reader) lex.Screenplay {
	out, err := ParseWithError(file)
	if err != nil {
		return nil
	}
	return out
}

// ParseWithError is Parse, reporting files that are not valid FDX.
//
// Paragraphs are read as Final Draft shows them: in capitals where their
// type's settings or their text's style say AllCaps, scene headings with
// their number (#12A#). Types Final Draft hides (HiddenText: Version 6's
// outline elements, Summary, Note) are Fountain's hidden elements:
// Outline 1, 2, ... sections (#, ##, ...), Summary and Outline Body
// synopses (=), Note and other hidden types notes ([[...]]); so are hidden
// text and script notes in a paragraph ([[...]]). Other types without a
// Fountain element (Shot, New Act, End of Act, Sequence, Cast List, ...)
// are action, centred if their type is, after a page break if their type
// starts a page, with their type's bold, italics and underlining.
func ParseWithError(file io.Reader) (lex.Screenplay, error) {
	s, _, err := parse(file)
	return s, err
}

// parse is ParseWithError, with the file's revisions (ParseRevisions).
func parse(file io.Reader) (lex.Screenplay, fileRevisions, error) {
	s, revs, _, _, err := parseFile(file)
	return s, revs, err
}

// parseFile is parse with the file as read and the row each paragraph
// of its body became (a dual dialogue's: the dialogue's first).
func parseFile(file io.Reader) (lex.Screenplay, fileRevisions, *FdxFile, map[*FdxParagraph]int, error) {
	var fdxFile FdxFile
	if err := xml.NewDecoder(file).Decode(&fdxFile); err != nil {
		return nil, fileRevisions{}, nil, nil, fmt.Errorf("reading FDX: %w", err)
	}
	rowOf := map[*FdxParagraph]int{}
	r := reader{types: typeSettings(fdxFile.ElementSettings), revs: newFileRevisions(fdxFile.Revisions)}
	r.deleted = deletedAt(fdxFile.Content, fdxFile.DeletedText)

	var out lex.Screenplay
	if fdxFile.TitlePage != nil {
		out = r.parseTitlePage(fdxFile.TitlePage.Content.Paragraphs)
	}
	bodyStart := len(out)

	for i := range fdxFile.Content.Paragraphs {
		p := fdxFile.Content.Paragraphs[i]
		if p.DualDialogue != nil {
			if len(out) > bodyStart {
				out = append(out, lex.Line{Type: lex.TypeEmpty})
			}
			if notes := r.notes(p.ScriptNotes); notes != "" {
				out = append(out, lex.Line{Type: lex.TypeNote, Contents: notes})
			}
			for j := range p.DualDialogue.Paragraphs {
				rowOf[&p.DualDialogue.Paragraphs[j]] = len(out)
			}
			out = append(out, r.parseDualDialogue(p.DualDialogue.Paragraphs)...)
			continue
		}

		// the notes before its text, as Final Draft writes a paragraph's: a
		// line of notes before it, where Fountain can have one
		before := ""
		line := r.parseParagraph(p)
		if lead := leadNotes(p); lead > 0 {
			rest := p
			rest.ScriptNotes, rest.noteAt = p.ScriptNotes[lead:], p.noteAt[lead:]
			if l := r.parseParagraph(rest); lex.NoteLineBefore(lex.Paragraph{Type: l.Type, Text: lex.StripNotes(l.Contents)}) &&
				!r.types[p.Type].hidden && l.Type != lex.TypeNote {
				before, line = r.notes(p.ScriptNotes[:lead]), l
			}
		}
		line.Revision = r.revisionOf(&fdxFile.Content.Paragraphs[i])
		line.Contents = strings.TrimRightFunc(line.Contents, unicode.IsSpace)
		if line.Type == lex.TypeEmpty {
			line.Contents = ""
		}
		if (p.StartsNewPage == "Yes" || r.types[p.Type].newPage) && len(out) > bodyStart {
			out = append(out, lex.Line{Type: lex.TypeNewPage})
		} else if startsBlock(line.Type) && len(out) > bodyStart && !(line.Type == lex.TypeLyrics && inDialogue(out)) {
			// Final Draft spaces blocks itself; Fountain needs a blank
			// line before each scene heading, action, character and
			// transition.
			out = append(out, lex.Line{Type: lex.TypeEmpty})
		}
		if before != "" {
			out = append(out, lex.Line{Type: lex.TypeNote, Contents: before}, lex.Line{Type: lex.TypeEmpty})
		}
		if line.Type == lex.TypeScene && p.SceneProperties != nil {
			// its color as Beat reads it: a note at the end
			if c := colorName(p.SceneProperties.Color); c != "" && lex.NoteColor(line.Contents) != c {
				line.Contents += " [[" + c + "]]"
			}
		}
		rowOf[&fdxFile.Content.Paragraphs[i]] = len(out)
		out = append(out, line)
		if line.Type == lex.TypeScene && p.SceneProperties != nil && p.SceneProperties.Summary != nil {
			for _, sp := range p.SceneProperties.Summary.Paragraphs {
				if text := strings.TrimSpace(r.text(sp)); text != "" {
					out = append(out, lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeSynopsis, Contents: text})
				}
			}
		}
	}

	return out, r.revs, &fdxFile, rowOf, nil
}

// settings are what Lexington reads of a paragraph type's ElementSettings.
type settings struct {
	allCaps, hidden, center, newPage bool
	style                            []string // Bold, Italic, Underline
}

// defaultTypes are the settings of Final Draft's default script for files
// without ElementSettings (as in every file with them that we have seen):
// headings, characters, transitions and shots in capitals, Version 6's outline
// elements hidden.
var defaultTypes = map[string]settings{
	FDXSceneHeading: {allCaps: true}, FDXCharacter: {allCaps: true}, FDXTransition: {allCaps: true},
	"Shot": {allCaps: true}, "Cast List": {allCaps: true},
	"Summary": {hidden: true}, "Note": {hidden: true}, "Outline Body": {hidden: true},
	"Outline 1": {hidden: true}, "Outline 2": {hidden: true}, "Outline 3": {hidden: true},
	"Outline 4": {hidden: true}, "Outline 5": {hidden: true}, "Outline 6": {hidden: true},
	"New Act":    {allCaps: true, center: true, newPage: true, style: []string{"Bold", "Underline"}},
	"End of Act": {allCaps: true, center: true, style: []string{"Bold", "Underline"}},
}

// typeSettings reads the ElementSettings; without any, Final Draft's
// defaults.
func typeSettings(es []FdxElementSettings) map[string]settings {
	if len(es) == 0 {
		return defaultTypes
	}
	out := map[string]settings{}
	for _, e := range es {
		styles := strings.Split(e.FontSpec.Style, "+")
		s := settings{
			allCaps: slices.Contains(styles, "AllCaps"),
			hidden:  slices.Contains(styles, "HiddenText"),
			center:  e.ParagraphSpec.Alignment == alignCenter,
			newPage: e.ParagraphSpec.StartsNewPage == "Yes",
		}
		for _, m := range fountainMarkup {
			if slices.Contains(styles, m.style) {
				s.style = append(s.style, m.style)
			}
		}
		out[e.Type] = s
	}
	return out
}

// reader reads paragraphs with the file's settings.
type reader struct {
	types   map[string]settings
	revs    fileRevisions
	deleted map[*FdxParagraph][]string
}

// revisionOf is the latest set a paragraph was changed in: of its runs,
// or of a deletion in it.
func (r reader) revisionOf(p *FdxParagraph) string {
	if r.revs.byID == nil {
		return ""
	}
	var ids []string
	for _, t := range p.Texts {
		ids = append(ids, t.RevisionID)
	}
	return r.revs.of(append(ids, r.deleted[p]...)...)
}

// fountainTypes are the paragraph types with an element of their own.
var fountainTypes = map[string]lex.ElementType{
	FDXSceneHeading: lex.TypeScene, FDXCharacter: lex.TypeSpeaker, FDXParenthetical: lex.TypeParen,
	FDXDialogue: lex.TypeDialog, FDXTransition: lex.TypeTrans, FDXLyrics: lex.TypeLyrics, FDXAction: lex.TypeAction,
	FDXGeneral: lex.TypeAction,
}

// parseParagraph maps one paragraph to a script line.
func (r reader) parseParagraph(p FdxParagraph) lex.Line {
	set := r.types[p.Type]
	t, own := fountainTypes[p.Type]
	if !own {
		t = lex.TypeAction
	} else {
		set.style = nil // the element's own look: Lexington's rules
	}
	contents := r.text(p)
	line := lex.Line{Type: t, Contents: contents}
	if set.hidden {
		return hiddenLine(p.Type, lex.StripNotes(contents), lex.Notes(contents))
	}
	switch {
	case strings.TrimSpace(contents) == "" && line.Type == lex.TypeDialog:
		line.Contents = "" // a blank line within a speech (Fountain's two spaces)
	case strings.TrimSpace(lex.StripNotes(contents)) == "" && !lex.HasNotes(contents):
		line.Type = lex.TypeEmpty
	case strings.TrimSpace(lex.StripNotes(contents)) == "":
		line = lex.Line{Type: lex.TypeNote, Contents: strings.TrimSpace(contents)}
	case line.Type == lex.TypeScene && strings.TrimSpace(p.Number) != "" && p.Number != "No":
		// the number before the notes at the end
		at := len(contents)
		if m := trailingNotes.FindStringIndex(contents); m != nil {
			at = m[0]
		}
		line.Contents = strings.TrimRightFunc(contents[:at], unicode.IsSpace) + " #" + strings.TrimSpace(p.Number) + "#" + contents[at:]
	case line.Type == lex.TypeAction && (p.Alignment == alignCenter || (p.Alignment == "" && set.center)):
		line.Type = lex.TypeCenter
	}
	return line
}

// leadNotes is how many of a paragraph's script notes are before its
// first text.
func leadNotes(p FdxParagraph) int {
	n := 0
	for n < len(p.noteAt) && p.noteAt[n] == 0 && len(p.Texts) > 0 {
		n++
	}
	return n
}

// trailingNotes are the notes at the end of a text.
var trailingNotes = regexp.MustCompile(`(?s)(\s*\[\[.*?\]\])+\s*$`)

// hiddenLine is a paragraph of a type Final Draft hides, as the Fountain
// element that is not printed either.
func hiddenLine(fdType, text string, notes []string) lex.Line {
	text = strings.TrimSpace(text)
	if text == "" {
		if len(notes) == 0 {
			return lex.Line{Type: lex.TypeEmpty}
		}
		return lex.Line{Type: lex.TypeNote, Contents: noteText(strings.Join(notes, "\n"))}
	}
	var level int
	if _, err := fmt.Sscanf(fdType, "Outline %d", &level); err == nil && level > 0 {
		return lex.Line{Type: lex.TypeSection, Contents: strings.Repeat("#", level) + " " + text}
	}
	switch fdType {
	case "Summary", "Outline Body":
		return lex.Line{Type: lex.TypeSynopsis, Contents: text}
	}
	return lex.Line{Type: lex.TypeNote, Contents: noteText(text)}
}

// noteText is a Fountain note of a text: [[text]], a blank line in it
// two spaces (so that the note goes on).
func noteText(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i, l := range lines {
		l = strings.ReplaceAll(l, "]]", "] ]")
		if strings.TrimSpace(l) == "" {
			l = "  "
		}
		lines[i] = l
	}
	return "[[" + strings.Join(lines, "\n") + "]]"
}

// notes are a paragraph's script notes as Fountain notes, or "".
func (r reader) notes(ns []FdxScriptNote) string {
	var out []string
	for _, n := range ns {
		var lines []string
		for _, p := range n.Paragraphs {
			lines = append(lines, r.text(p))
		}
		if text := strings.TrimSpace(strings.Join(lines, "\n")); text != "" {
			out = append(out, noteText(text))
		}
	}
	return strings.Join(out, " ")
}

// parseDualDialogue turns the paragraphs of a <DualDialogue> (both
// characters' blocks in order) into the dual dialogue markers the
// Fountain parser produces.
func (r reader) parseDualDialogue(ps []FdxParagraph) lex.Screenplay {
	out := lex.Screenplay{{Type: lex.TypeDualOpen}}
	speakers := 0
	for i, p := range ps {
		line := r.parseParagraph(p)
		if line.Type == lex.TypeEmpty {
			continue
		}
		line.Revision = r.revisionOf(&ps[i])
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
// fields: the first group of centered lines is the title, a line ending in
// "by" the credit, and the other centered lines the author; left-aligned
// lines (contact details, draft date) become Contact fields.
func (r reader) parseTitlePage(ps []FdxParagraph) lex.Screenplay {
	var title, credit, author, contact lex.Screenplay
	inTitle := true
	var lines []FdxParagraph
	for _, p := range ps {
		lines = append(lines, splitLines(p)...)
	}
	for _, p := range lines {
		text := strings.TrimSpace(r.text(p))
		switch {
		case text == "":
			if title != nil {
				inTitle = false
			}
		case p.Alignment != alignCenter:
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
	out = append(out, contact...)
	return append(out, lex.Line{Type: lex.TypeNewPage})
}

// splitLines splits a paragraph at the line breaks in its text: a title
// page's "Draft\ninformation" is two lines. Its notes are left out.
func splitLines(p FdxParagraph) []FdxParagraph {
	line := p
	line.Texts, line.ScriptNotes, line.noteAt = nil, nil, nil
	var out []FdxParagraph
	for _, t := range p.Texts {
		parts := strings.Split(strings.ReplaceAll(t.Content, "\r", ""), "\n")
		for i, part := range parts {
			if i > 0 {
				out = append(out, line)
				line.Texts = nil
			}
			t.Content = part
			line.Texts = append(line.Texts, t)
		}
	}
	return append(out, line)
}

// fountainMarkup are the Final Draft styles that are Fountain emphasis.
var fountainMarkup = []struct{ style, marker string }{
	{"Underline", "_"},
	{"Bold", "**"},
	{"Italic", "*"},
}

// run is a text run as Fountain has it: its text (in capitals if
// AllCaps) and emphasis, or a note (lex.Run).
type run = lex.Run

// runs are a paragraph's text runs and notes, in order, with the type's
// settings: its capitals, and its bold, italics and underlining for types
// without a Fountain element of their own.
func (r reader) runs(p FdxParagraph) []run {
	set := r.types[p.Type]
	if _, own := fountainTypes[p.Type]; own || set.hidden {
		set.style = nil
	}
	var out []run
	note := 0
	addNotes := func(upto int) {
		for ; note < len(p.ScriptNotes) && p.noteAt[note] <= upto; note++ {
			if n := r.notes(p.ScriptNotes[note : note+1]); n != "" {
				out = append(out, run{Span: lex.Span{Text: n}, Note: true})
			}
		}
	}
	for i, t := range p.Texts {
		addNotes(i)
		styles := strings.Split(t.Style, "+")
		if slices.Contains(styles, "HiddenText") {
			if text := strings.TrimSpace(t.Content); text != "" {
				out = append(out, run{Span: lex.Span{Text: noteText(text)}, Note: true})
			}
			continue
		}
		styles = append(styles, set.style...)
		text := t.Content
		if set.allCaps || slices.Contains(styles, "AllCaps") {
			text = strings.ToUpper(text)
		}
		out = append(out, run{Span: lex.Span{Text: text, Bold: slices.Contains(styles, "Bold"),
			Italic: slices.Contains(styles, "Italic"), Underline: slices.Contains(styles, "Underline")}})
	}
	addNotes(len(p.Texts))
	return out
}

// text is a paragraph's text as Fountain: its runs' emphasis as markers,
// the markers that are text escaped where they would be read as
// emphasis; a run's emphasis Fountain cannot write (part of a word
// underlined, styles switching inside a word) is left out rather than
// printed as markers, the rest of the paragraph's kept. Notes are [[notes]] where
// they are among the text; the ones before all of it go after it, as
// Fountain has them (Final Draft writes notes first).
func (r reader) text(p FdxParagraph) string {
	runs := r.runs(p)
	var lead []run
	for len(runs) > 0 && (runs[0].Note || strings.TrimSpace(runs[0].Text) == "") && lex.RunsText(runs) != "" {
		if runs[0].Note {
			lead = append(lead, runs[0])
		}
		runs = runs[1:]
	}
	// the notes after the text, as the writer puts a line's, after a space
	for len(runs) > 0 && runs[len(runs)-1].Note && lex.RunsText(runs[:len(runs)-1]) != "" {
		lead = append([]run{runs[len(runs)-1]}, lead...)
		runs = runs[:len(runs)-1]
	}
	text := lex.FountainRuns(runs)
	for _, n := range lead {
		text = strings.TrimRightFunc(text, unicode.IsSpace) + " " + n.Text
	}
	return text
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
