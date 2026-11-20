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
	// ElementSettings are the look of each paragraph type: Final Draft
	// writes them for every type it has (in capitals, hidden, ...).
	ElementSettings []FdxElementSettings `xml:"ElementSettings,omitempty"`
	// Revisions are the revision sets, DeletedText where their deletions
	// were (revisions.go).
	Revisions   *FdxRevisions   `xml:"Revisions,omitempty"`
	DeletedText *FdxDeletedText `xml:"DeletedText,omitempty"`
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
	DualDialogue    *FdxDualDialogue    `xml:"DualDialogue,omitempty"`

	// noteAt is, for each of the ScriptNotes read, the number of Texts
	// before it.
	noteAt []int
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
	Color   string         `xml:"Color,attr,omitempty"`
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
	var fdxFile FdxFile
	if err := xml.NewDecoder(file).Decode(&fdxFile); err != nil {
		return nil, fileRevisions{}, fmt.Errorf("reading FDX: %w", err)
	}
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
			out = append(out, r.parseDualDialogue(p.DualDialogue.Paragraphs)...)
			continue
		}

		line := r.parseParagraph(p)
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
		if line.Type == lex.TypeScene && p.SceneProperties != nil {
			// its color as Beat reads it: a note at the end
			if c := colorName(p.SceneProperties.Color); c != "" && lex.NoteColor(line.Contents) != c {
				line.Contents += " [[" + c + "]]"
			}
		}
		out = append(out, line)
		if line.Type == lex.TypeScene && p.SceneProperties != nil && p.SceneProperties.Summary != nil {
			for _, sp := range p.SceneProperties.Summary.Paragraphs {
				if text := strings.TrimSpace(r.text(sp)); text != "" {
					out = append(out, lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeSynopsis, Contents: text})
				}
			}
		}
	}

	return out, r.revs, nil
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
	if contact != nil {
		out = append(out, lex.Line{Type: "metasection"})
		out = append(out, contact...)
	}
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
// AllCaps) and emphasis, or a note.
type run struct {
	lex.Span
	note bool
}

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
				out = append(out, run{Span: lex.Span{Text: n}, note: true})
			}
		}
	}
	for i, t := range p.Texts {
		addNotes(i)
		styles := strings.Split(t.Style, "+")
		if slices.Contains(styles, "HiddenText") {
			if text := strings.TrimSpace(t.Content); text != "" {
				out = append(out, run{Span: lex.Span{Text: noteText(text)}, note: true})
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
// emphasis; emphasis Fountain cannot write (part of a word underlined)
// is left out rather than printed as markers. Notes are [[notes]] where
// they are among the text; the ones before all of it go after it, as
// Fountain has them (Final Draft writes notes first).
func (r reader) text(p FdxParagraph) string {
	runs := r.runs(p)
	var lead []run
	for len(runs) > 0 && (runs[0].note || strings.TrimSpace(runs[0].Text) == "") && nextText(runs) != "" {
		if runs[0].note {
			lead = append(lead, runs[0])
		}
		runs = runs[1:]
	}
	text := ""
	for _, escape := range []bool{false, true} {
		if s := markup(runs, escape); same(lex.Emphasis(lex.StripNotes(s)), runs) {
			text = s
			break
		}
	}
	if text == "" && nextText(runs) != "" { // plain
		var plain []run
		for _, t := range runs {
			if !t.note {
				t.Bold, t.Italic, t.Underline = false, false, false
			}
			plain = append(plain, t)
		}
		text = markup(plain, true)
	}
	for _, n := range lead {
		text = strings.TrimRightFunc(text, unicode.IsSpace) + " " + n.Text
	}
	return text
}

// markers are Fountain's emphasis markers, in the order they open.
var markers = []struct {
	marker string
	on     func(lex.Span) bool
}{
	{"_", func(s lex.Span) bool { return s.Underline }},
	{"**", func(s lex.Span) bool { return s.Bold }},
	{"*", func(s lex.Span) bool { return s.Italic }},
}

// markup writes runs with Fountain's emphasis markers, escaping the ones
// in their text if escape. A marker opens and closes where the style
// changes, next to the text: the spaces around a change are outside it.
func markup(runs []run, escape bool) string {
	var b strings.Builder
	var open []int // markers open, in order
	space := ""    // spaces not yet written
	change := func(s lex.Span) {
		keep := 0 // the markers open that stay open: up to the first that does not
		for keep < len(open) && markers[open[keep]].on(s) {
			keep++
		}
		for i := len(open) - 1; i >= keep; i-- {
			b.WriteString(markers[open[i]].marker)
		}
		open = open[:keep]
		b.WriteString(space)
		space = ""
		for m := range markers {
			if markers[m].on(s) && !slices.Contains(open, m) {
				b.WriteString(markers[m].marker)
				open = append(open, m)
			}
		}
	}
	for i, t := range runs {
		if t.note {
			// with no space after it, before the spaces in front of
			// it, so that the text without it keeps them ("Rain[[note]]
			// falls.")
			if next := nextText(runs[i+1:]); next == "" || unicode.IsSpace([]rune(next)[0]) {
				b.WriteString(space + t.Text)
				space = ""
			} else {
				b.WriteString(t.Text)
			}
			continue
		}
		text := t.Text
		if escape {
			text = lex.EscapeEmphasis(text)
		}
		content := strings.TrimSpace(text)
		if content == "" {
			space += text
			continue
		}
		lead := text[:strings.Index(text, content)]
		trail := text[len(lead)+len(content):]
		space += lead
		change(t.Span)
		b.WriteString(content)
		space = trail
	}
	change(lex.Span{})
	b.WriteString(space)
	return b.String()
}

// nextText is the text of the first of runs that is not a note.
func nextText(runs []run) string {
	for _, t := range runs {
		if !t.note && t.Text != "" {
			return t.Text
		}
	}
	return ""
}

// same reports whether spans read from Fountain are the runs' text and
// styles (notes aside; spaces have no style that shows).
func same(spans []lex.Span, runs []run) bool {
	var want []lex.Span
	for _, t := range runs {
		if !t.note {
			want = append(want, t.Span)
		}
	}
	return slices.Equal(styledRunes(spans), styledRunes(want))
}

// styledRune is a character and its style, none for a space.
type styledRune struct {
	r                       rune
	bold, italic, underline bool
}

func styledRunes(spans []lex.Span) []styledRune {
	var out []styledRune
	for _, s := range spans {
		for _, r := range s.Text {
			if unicode.IsSpace(r) {
				out = append(out, styledRune{r: r})
			} else {
				out = append(out, styledRune{r, s.Bold, s.Italic, s.Underline})
			}
		}
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
