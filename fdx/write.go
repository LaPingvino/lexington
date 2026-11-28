package fdx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"

	"github.com/LaPingvino/lexington/lex"
)

// FDXWriter implements the writer.Writer interface for FDX output.
type FDXWriter struct {
	TemplatePath string // Path to a custom FDX template file
	// Revisions are the script's revision sets: written as <Revisions>,
	// each line's Revision as its runs' RevisionID (revisions.go).
	Revisions *lex.Revisions
	// Lock is the script's locked pages: written as <LockedPages>, each
	// page found at its Position (pages.go).
	Lock *lex.PageLock
	// Tags are the breakdown's categories and elements: written as
	// <TagData> (tags.go). nil: no TagData.
	Tags *lex.TagSet
	// LineTags are the words tagged, by the row's Source (its line of the
	// Fountain text, fountain.ParseLines): written as TagNumbers on the
	// paragraph's runs.
	LineTags map[int][]lex.LineTag
	// Scenes are the scene headings' pages and lengths, in order (as
	// pdf.PDFWriter.Measure gives them): written in their
	// SceneProperties. nil: none written.
	Scenes []SceneProperty
	// LastPage is the last page's number, for a Highland header's %pp
	// ("": left out); Final Draft has no label for it.
	LastPage string
}

// processInlineMarkup turns Fountain's emphasis (lex.Emphasis) into
// FDX Text runs with their Style. The text is not XML-escaped; the XML
// encoder does that.
func processInlineMarkup(text string) []FdxText {
	var out []FdxText
	for _, s := range lex.Emphasis(text) {
		var style []string
		if s.Bold {
			style = append(style, "Bold")
		}
		if s.Italic {
			style = append(style, "Italic")
		}
		if s.Underline {
			style = append(style, "Underline")
		}
		out = append(out, FdxText{Content: s.Text, Style: strings.Join(style, "+")})
	}
	return out
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
	case lex.TypeEmpty, lex.TypeTitlePage, lex.TypeNewPage, lex.TypeMetaSection, "section", "synopse", "note", lex.TypeBoneyard:
		// Final Draft spaces elements itself; blank lines (handled in
		// buildDocument), structure markers and Fountain's boneyard
		// (never printed: notes blocks, an omitted scene) have no
		// paragraph.
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
// Notes become script notes on the paragraph they are in, or on the next
// one for a line of notes.
//
// The Version is 5, as a Final Draft file of ours has it (screenplay-tools'
// TestFDX-FD.fdx); which Final Draft writes which Version is not known
// for sure (finaler-draft's README has Final Draft 13 write 6, whose ids on every
// paragraph and notes kept apart by character ranges Lexington does not
// write). Script notes in the paragraph are as Final Draft files of
// Versions 2 and 4 have them, without their dates. No file written so has
// been opened in Final Draft itself.
func buildDocument(screenplay lex.Screenplay) FdxFile {
	return buildRevised(screenplay, nil, nil, nil, nil, nil)
}

// buildRevised is buildDocument with the revision sets and the locked
// pages.
func buildRevised(screenplay lex.Screenplay, revs *lex.Revisions, lock *lex.PageLock, tags *lex.TagSet, lineTags map[int][]lex.LineTag, scenes []SceneProperty) FdxFile {
	doc := FdxFile{DocumentType: "Script", Template: "No", Version: "5", ElementSettings: elementSettings}
	ids := revisionIDs(revs)
	doc.Revisions = buildRevisions(revs, ids)

	i := 0
	if len(screenplay) > 0 && screenplay[0].Type == lex.TypeTitlePage {
		// (a title page has no lines of notes: the lines are the same)
		doc.TitlePage, i = buildTitlePage(lex.Printable(screenplay, nil))
	}

	screenplay = lex.ExpandValues(screenplay) // {{SERIES}}, {{TIMESTAMP}}
	b := docBuilder{doc: &doc, ids: ids, seqOf: map[int]int{}, tags: newTagWriter(tags), lineTags: lineTags}
	heading := 0 // the scene headings so far
	for k := i; k < len(screenplay); k++ {
		line := screenplay[k]
		b.row = k
		// Highland's directives that print nothing: Final Draft would print them
		directives := lex.IsDirectiveLine(line)
		if toc := lex.TOCLines(screenplay, line); toc != nil { // Highland's table of contents: its sections
			for _, e := range toc {
				b.add(e, nil)
			}
			continue
		}
		line.Contents = lex.StripDirectives(line.Contents)
		if directives || lex.IsNoteLine(line) {
			b.notes = append(b.notes, lex.Notes(line.Contents)...)
			if b.blanks > 0 && k+1 < len(screenplay) && screenplay[k+1].Type == lex.TypeEmpty {
				b.blanks-- // one blank line around it goes, as in print
			}
			continue
		}
		notes := lex.Notes(line.Contents)
		color := ""
		if line.Type == lex.TypeScene {
			// its color in its SceneProperties, not as a note as well
			if color = fdColor(lex.NoteColor(line.Contents)); color != "" {
				notes = withoutColor(notes)
			}
		}
		line.Contents = lex.StripNotes(line.Contents)
		b.add(line, notes)
		if line.Type != lex.TypeScene {
			continue
		}
		var props *FdxSceneProperties
		if color != "" {
			props = &FdxSceneProperties{Color: color}
		}
		if heading < len(scenes) {
			sc := scenes[heading]
			if props == nil {
				props = &FdxSceneProperties{}
			}
			if _, omitted := lex.OmittedNumber(line.Contents); !omitted { // an OMITTED scene has no length
				props.Length = sc.Length.String()
			}
			if sc.Page > 0 {
				props.Page = strconv.Itoa(sc.Page)
			}
		}
		heading++
		if ps := doc.Content.Paragraphs; props != nil && len(ps) > 0 && ps[len(ps)-1].Type == FDXSceneHeading {
			ps[len(ps)-1].SceneProperties = props
		}
	}
	b.closeDual()
	if ps := doc.Content.Paragraphs; len(b.notes) > 0 && len(ps) > 0 {
		// notes after the last paragraph: after its text
		ps[len(ps)-1].TrailingNotes = append(ps[len(ps)-1].TrailingNotes, trailing(b.notes)...)
	}
	doc.LockedPages = buildLockedPages(&doc, screenplay, lock, b.seqOf)
	doc.TagData = b.tags.tagData()
	return doc
}

// withoutColor are notes without the one that gives the color (the last
// color note).
func withoutColor(notes []string) []string {
	for i := len(notes) - 1; i >= 0; i-- {
		if lex.NoteColor("[["+notes[i]+"]]") != "" {
			return append(notes[:i:i], notes[i+1:]...)
		}
	}
	return notes
}

// scriptNotes are Fountain notes as Final Draft's: a paragraph for each
// of their lines.
func scriptNotes(notes []string) []FdxScriptNote {
	var out []FdxScriptNote
	for _, n := range notes {
		sn := FdxScriptNote{Color: "#000000000000"}
		for _, l := range strings.Split(n, "\n") {
			sn.Paragraphs = append(sn.Paragraphs, FdxParagraph{Texts: []FdxText{{Content: strings.TrimSpace(l)}}})
		}
		out = append(out, sn)
	}
	return out
}

// trailing are notes as script notes after a paragraph's text.
func trailing(notes []string) []trailingNote {
	var out []trailingNote
	for _, n := range scriptNotes(notes) {
		out = append(out, trailingNote{FdxScriptNote: n})
	}
	return out
}

// docBuilder adds screenplay lines to an FDX document, keeping track of
// blank lines, pending page breaks, notes for the next paragraph and open
// dual dialogue.
type docBuilder struct {
	doc      *FdxFile
	dual     *FdxDualDialogue
	newPage  bool
	blanks   int
	notes    []string
	ids      map[string]string     // the revision sets' IDs by name
	row      int                   // the row being added
	seq      int                   // the paragraphs so far, in the order places counts them
	seqOf    map[int]int           // each row's paragraph
	tags     *tagWriter            // the breakdown's (nil: none)
	lineTags map[int][]lex.LineTag // the words tagged, by Source
}

func (b *docBuilder) add(line lex.Line, notes []string) {
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
	p := FdxParagraph{Type: pType}
	text := line.Contents
	if line.Type == lex.TypeScene {
		text, p.Number = lex.SceneNumber(text)
	}
	p.Texts = processInlineMarkup(text)
	if line.Source > 0 {
		p.Texts = b.tags.tagRuns(p.Texts, b.lineTags[line.Source])
	}
	if id := b.ids[line.Revision]; id != "" && id != "0" {
		for i := range p.Texts {
			p.Texts[i].RevisionID = id
		}
	}
	if line.Type == lex.TypeCenter {
		p.Alignment = alignCenter
	}
	// a line of notes before the paragraph, before its text; the notes
	// in its text after it
	if len(b.notes) > 0 {
		p.ScriptNotes, b.notes = scriptNotes(b.notes), nil
	}
	p.TrailingNotes = trailing(notes)
	b.seqOf[b.row] = b.seq
	b.seq++
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

// elementSettings are the paragraph types Lexington writes, as Final
// Draft 13 has them in a new script (fdx-mcp-server's NewFile.fdx: type,
// indents and spacing in inches and points) with the behaviour of a
// Version 5 file (screenplay-tools' TestFDX-FD.fdx): headings,
// characters, transitions and shots in capitals; lyrics, which Final
// Draft's screenplays lack, in italics, indented and paginated as
// dialogue. Without them Final Draft would use the settings of whatever
// script it has as its default.
var elementSettings = []FdxElementSettings{
	setting(FDXGeneral, "", "Left", "0.00", "1.50", "7.50", "0", FDXGeneral, FDXGeneral, "0"),
	setting(FDXSceneHeading, "AllCaps", "Left", "0.00", "1.50", "7.50", "24", FDXSceneHeading, FDXAction, "1"),
	setting(FDXAction, "", "Left", "0.00", "1.50", "7.50", "12", FDXAction, FDXAction, "2"),
	setting(FDXCharacter, "AllCaps", "Left", "0.00", "3.50", "7.25", "12", FDXCharacter, FDXDialogue, "3"),
	setting(FDXParenthetical, "", "Left", "-0.10", "3.00", "5.50", "0", FDXParenthetical, FDXDialogue, "4"),
	setting(FDXDialogue, "", "Left", "0.00", "2.50", "6.00", "0", FDXDialogue, FDXAction, "5"),
	setting(FDXTransition, "AllCaps", "Right", "0.00", "5.50", "7.10", "12", FDXTransition, FDXSceneHeading, "6"),
	setting("Shot", "AllCaps", "Left", "0.00", "1.50", "7.50", "12", FDXSceneHeading, FDXAction, "7"),
	setting(FDXLyrics, "Italic", "Left", "0.00", "2.50", "6.00", "0", FDXDialogue, FDXLyrics, ""),
}

func setting(t, style, align, first, left, right, before, paginate, next, key string) FdxElementSettings {
	return FdxElementSettings{
		Type: t,
		FontSpec: FdxFontSpec{AdornmentStyle: "0", Background: "#FFFFFFFFFFFF", Color: "#000000000000",
			Font: "Courier Final Draft", RevisionID: "0", Size: "12", Style: style},
		ParagraphSpec: FdxParagraphSpec{Alignment: align, FirstIndent: first, Leading: "Regular", LeftIndent: left,
			RightIndent: right, SpaceBefore: before, Spacing: "1", StartsNewPage: "No"},
		Behavior: &FdxBehavior{PaginateAs: paginate, ReturnKey: next, Shortcut: key, WinShortcut: key},
	}
}

// flushBlanks writes the blank lines before a block: one blank line
// separates blocks; each further one is an empty paragraph the writer put
// there on purpose.
func (b *docBuilder) flushBlanks() {
	for ; b.blanks > 1 && b.dual == nil; b.blanks-- {
		b.doc.Content.Paragraphs = append(b.doc.Content.Paragraphs, FdxParagraph{Type: FDXAction, Texts: []FdxText{{}}})
		b.seq++
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
		if line.Type == lex.TypeMetaSection || line.Contents == "" {
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
	doc := buildRevised(screenplay, f.Revisions, f.Lock, f.Tags, f.LineTags, f.Scenes)
	header, footer := lex.PageTexts(lex.Printable(screenplay, nil))
	doc.HeaderAndFooter = headerAndFooter(header, footer, f.LastPage)

	if f.TemplatePath != "" {
		return f.writeTemplate(w, doc)
	}

	if _, err := io.WriteString(w, `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>`+"\n"); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("failed to encode FDX: %w", err)
	}
	buf.WriteString("\n")
	_, err := w.Write(plainQuotes(buf.Bytes()))
	return err
}

// plainQuotes writes the text's quotes and apostrophes as themselves, as
// Final Draft does ("John's", not "John&#39;s", which encoding/xml
// writes): a reader that does not undo character references would show
// them, or worse. In tags (attributes) they stay as they are; Go's
// encoder escapes every < and > that is not markup, so what is between >
// and < is text.
func plainQuotes(doc []byte) []byte {
	out := make([]byte, 0, len(doc))
	inTag := false
	for i := 0; i < len(doc); i++ {
		switch c := doc[i]; {
		case c == '<':
			inTag = true
		case c == '>':
			inTag = false
		case c == '&' && !inTag && bytes.HasPrefix(doc[i:], []byte("&#39;")):
			out, i = append(out, '\''), i+4
			continue
		case c == '&' && !inTag && bytes.HasPrefix(doc[i:], []byte("&#34;")):
			out, i = append(out, '"'), i+4
			continue
		}
		out = append(out, doc[i])
	}
	return out
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

// escapeXML escapes characters that have special meaning in XML text
// (quotes and apostrophes are written as themselves: plainQuotes).
func escapeXML(s string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		// xml.EscapeText should not fail for valid strings, but handle error just in case
		return s
	}
	return string(plainQuotes(b.Bytes()))
}
