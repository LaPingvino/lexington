package celtx

import (
	"bytes"
	"cmp"
	"crypto/sha1"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
)

// Parse reads a Celtx project's first script.
func Parse(r io.Reader) (lex.Screenplay, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseScript(data, "")
}

// ParseScript reads the script pick names (Project.Script: "" the first)
// of a Celtx project's zip.
func ParseScript(data []byte, pick string) (lex.Screenplay, error) {
	s, _, _, err := ParseTags(data, pick)
	return s, err
}

// ParseTags reads the script pick names with its breakdown tags: the set
// (nil if it has none) and the words tagged by row (the text as
// lex.TagPlain counts it), as fdx.ParseTags has them.
func ParseTags(data []byte, pick string) (lex.Screenplay, *lex.TagSet, map[int][]lex.LineTag, error) {
	p, err := ReadProject(data)
	if err != nil {
		return nil, nil, nil, err
	}
	sc, err := p.Script(pick)
	if err != nil {
		return nil, nil, nil, err
	}
	return p.Read(sc)
}

// Read reads one of the project's scripts, with its tags (ParseTags).
func (p *Project) Read(sc Script) (lex.Screenplay, *lex.TagSet, map[int][]lex.LineTag, error) {
	f, ok := p.files[sc.File]
	if !ok {
		return nil, nil, nil, fmt.Errorf("no %s in the project", sc.File)
	}
	data, err := readFile(f)
	if err != nil {
		return nil, nil, nil, err
	}
	doc, err := parseHTML(data)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not read %s: %w", sc.File, err)
	}
	return p.build(sc, doc)
}

// para is a script's paragraph as read: its class and id, its text
// (styled runes; a line break '\n'), notes and tagged words.
type para struct {
	class, id string
	text      []srune
	notes     []string
	tags      []spanTag
}

// srune is a rune of a paragraph with its style and the tag it is in
// (an index in the paragraph's tags, -1 none).
type srune struct {
	r       rune
	b, i, u bool
	tag     int
}

// spanTag is a breakdown span: the item's URI and the span's class.
type spanTag struct{ ref, class string }

// document is a script's HTML as read: its head's fields and its body's
// paragraphs.
type document struct {
	title string
	meta  map[string]string
	paras []para
}

// lineBreak stands for a <br> until the HTML's own white space is
// collapsed.
const lineBreak = ' '

// style is what an element open in a paragraph does to its text.
type style struct {
	b, i, u, skip bool
	tag           int // -1: none of its own
}

// parseHTML reads a script's HTML (HTML 4, not XML: <br>, <meta>,
// &nbsp;) as the decoder's lenient mode reads it.
func parseHTML(data []byte) (*document, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict, d.AutoClose, d.Entity = false, xml.HTMLAutoClose, xml.HTMLEntity
	doc := &document{meta: map[string]string{}}
	var cur *para
	var stack []style
	inTitle, inBody := false, false
	closePara := func() {
		if cur != nil {
			cur.text = collapse(cur.text)
			doc.paras = append(doc.paras, *cur)
			cur, stack = nil, nil
		}
	}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if cur != nil || len(doc.paras) > 0 { // a broken end: what was read
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := strings.ToLower(t.Name.Local)
			switch {
			case name == "title":
				inTitle = true
			case name == "meta":
				if n := attr(t, "", "name"); n != "" {
					doc.meta[n] = attr(t, "", "content")
				}
			case name == "body":
				inBody = true
			case !inBody:
			case name == "p" || name == "div" && cur == nil:
				closePara()
				cur = &para{class: strings.TrimSpace(attr(t, "", "class")), id: attr(t, "", "id")}
			case cur == nil:
			case name == "br":
				cur.text = append(cur.text, srune{r: lineBreak, tag: -1})
			default:
				stack = append(stack, open(cur, t, name))
			}
		case xml.EndElement:
			name := strings.ToLower(t.Name.Local)
			switch {
			case name == "title":
				inTitle = false
			case name == "body":
				closePara()
				inBody = false
			case cur == nil:
			case name == "p" || name == "div":
				closePara()
			case name == "br":
			case len(stack) > 0:
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if inTitle {
				doc.title += string(t)
				continue
			}
			if cur == nil {
				continue
			}
			st := current(stack)
			if st.skip {
				continue
			}
			for _, r := range string(t) {
				cur.text = append(cur.text, srune{r: r, b: st.b, i: st.i, u: st.u, tag: st.tag})
			}
		}
	}
	closePara()
	return doc, nil
}

// open is what an element in a paragraph does: a style, a note, a tag.
func open(p *para, t xml.StartElement, name string) style {
	st := style{tag: -1}
	switch name {
	case "b", "strong":
		st.b = true
	case "i", "em":
		st.i = true
	case "u":
		st.u = true
	case "span", "font":
		css := strings.ToLower(strings.ReplaceAll(attr(t, "", "style"), " ", ""))
		st.b = strings.Contains(css, "font-weight:bold")
		st.i = strings.Contains(css, "font-style:italic")
		st.u = strings.Contains(css, "text-decoration:underline")
		class := strings.ToLower(strings.TrimSpace(attr(t, "", "class")))
		switch {
		case class == "note":
			if n := trimLines(attr(t, "", "text")); n != "" {
				p.notes = append(p.notes, n)
			}
			st.skip = true
		case class == "media" || attr(t, "", "mediares") != "":
			st.skip = true // an attachment
		case attr(t, "", "ref") != "":
			st.tag = len(p.tags)
			p.tags = append(p.tags, spanTag{ref: attr(t, "", "ref"), class: class})
		}
	}
	return st
}

// current is the style the open elements give text: each adds its own.
func current(stack []style) style {
	st := style{tag: -1}
	for _, s := range stack {
		st.b, st.i, st.u, st.skip = st.b || s.b, st.i || s.i, st.u || s.u, st.skip || s.skip
		if s.tag >= 0 {
			st.tag = s.tag
		}
	}
	return st
}

// collapse is a paragraph's text as HTML shows it: a run of white space
// one space, none at a line's ends, a line break where a <br> is (the
// last ones, Mozilla's, left out). A no-break space is a space of its
// own: Mozilla writes two spaces as "&nbsp; ", which stay two.
func collapse(in []srune) []srune {
	var out []srune
	trim := func() {
		for len(out) > 0 && out[len(out)-1].r == ' ' {
			out = out[:len(out)-1]
		}
	}
	for _, c := range in {
		switch {
		case c.r == lineBreak:
			trim()
			c.r = '\n'
			out = append(out, c)
		case c.r != '\u00a0' && unicode.IsSpace(c.r):
			if len(out) == 0 || out[len(out)-1].r == '\n' || out[len(out)-1].r == ' ' {
				continue
			}
			c.r = ' '
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	for i := range out {
		if out[i].r == '\u00a0' {
			out[i].r = ' '
		}
	}
	for {
		trim()
		if len(out) == 0 || out[len(out)-1].r != '\n' {
			break
		}
		out = out[:len(out)-1]
	}
	for len(out) > 0 && out[0].r == ' ' {
		out = out[1:]
	}
	return out
}

// upper are the classes whose text Celtx shows in capitals.
var upper = map[string]bool{"sceneheading": true, "character": true, "transition": true, "shot": true, "act": true}

// classTypes are the paragraph classes Lexington has a type for; any
// other is action.
var classTypes = map[string]lex.ElementType{
	"sceneheading": lex.TypeScene, "action": lex.TypeAction, "character": lex.TypeSpeaker,
	"dialog": lex.TypeDialog, "parenthetical": lex.TypeParen, "transition": lex.TypeTrans,
	"shot": lex.TypeAction, "act": lex.TypeSection,
}

// noteSection and noteSynopsis are the notes a section and a synopsis
// are written as.
var (
	noteSection  = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	noteSynopsis = regexp.MustCompile(`^=\s+(.*)$`)
	centred      = regexp.MustCompile(`^>\s*([^\n]*?)\s*<$`)
)

// titleMetas are the head's fields of the title page, in the order they
// are read, by their Fountain keys.
var titleMetas = []struct{ key, meta string }{
	{"Credit", "CX.byline"}, {"Author", "Author"}, {"Source", "DC.source"},
	{"Copyright", "DC.rights"}, {"Contact", "CX.contact"},
}

// build lays the document out as a screenplay, with its tags.
func (p *Project) build(sc Script, doc *document) (lex.Screenplay, *lex.TagSet, map[int][]lex.LineTag, error) {
	var b lex.Builder
	var title []lex.TitleField
	if t := strings.TrimSpace(doc.title); t != "" {
		title = append(title, lex.TitleField{Key: "Title", Value: lex.EscapeEmphasis(t)})
	}
	for _, m := range titleMetas {
		if v := strings.TrimSpace(doc.meta[m.meta]); v != "" {
			title = append(title, lex.TitleField{Key: m.key, Value: lex.EscapeEmphasis(v)})
		}
	}
	b.TitlePage(title)
	synopses := p.sceneCards(sc)
	tr := newTagReader(p)
	var paras []lex.Paragraph
	var tagged [][]placedTag // the paragraphs' tags
	for _, cp := range doc.paras {
		lp, tags := paragraph(cp, tr)
		if lp == nil {
			continue
		}
		if lp.Type == lex.TypeScene && synopses[cp.id] != "" {
			lp.Synopses = []string{synopses[cp.id]}
		}
		if lp.Type == lex.TypeSpeaker {
			if cue, ok := strings.CutSuffix(lp.Text, "^"); ok {
				lp.Text = strings.TrimSpace(cue)
				if first := dualFirst(paras); first >= 0 {
					paras[first].Dual = lex.DualFirst
					lp.Dual = lex.DualSecond
				}
			}
		}
		paras = append(paras, *lp)
		tagged = append(tagged, tags)
	}
	byRow := map[int][]lex.LineTag{}
	for i, lp := range paras {
		from := b.Len()
		b.Add(lp)
		to := b.Len()
		if len(tagged[i]) > 0 {
			tr.place(b.Rows(), from, to, lp.Type, tagged[i], byRow)
		}
	}
	s := b.Screenplay()
	if len(tr.set.Elements) == 0 {
		return s, nil, nil, nil
	}
	return s, tr.set, byRow, nil
}

// dualFirst is the speaker of the speech just before, if it is not in
// dual dialogue already: -1 if there is none.
func dualFirst(paras []lex.Paragraph) int {
	for i := len(paras) - 1; i >= 0; i-- {
		switch p := paras[i]; p.Type {
		case lex.TypeDialog, lex.TypeParen:
			continue
		case lex.TypeSpeaker:
			if p.Dual == lex.NotDual {
				return i
			}
		}
		return -1
	}
	return -1
}

// sceneCards are the scenes' index card texts, by their headings' ids.
func (p *Project) sceneCards(sc Script) map[string]string {
	out := map[string]string{}
	doc, ok := p.g[sc.res]
	if !ok {
		return out
	}
	for _, seq := range doc.props[nsCX+" scenes"] {
		s, ok := p.g[seq]
		if !ok {
			continue
		}
		for _, it := range s.items {
			if n, ok := p.g[it]; ok {
				if id, d := n.prop(nsCX, "sceneid"), strings.TrimSpace(n.prop(nsDC, "description")); id != "" && d != "" {
					out[id] = d
				}
			}
		}
	}
	return out
}

// placedTag is a tagged word of a paragraph: its line in the paragraph,
// its text and the time it stands in the line before, the elements.
type placedTag struct {
	line    int
	at, all int // the times it stands before in its line, in the paragraph
	text    string
	element string
}

// paragraph is a paragraph as Lexington's, with its tags; nil for one
// with nothing in it.
func paragraph(cp para, tr *tagReader) (*lex.Paragraph, []placedTag) {
	t, ok := classTypes[strings.ToLower(cp.class)]
	if !ok {
		t = lex.TypeAction
	}
	text := cp.text
	if upper[strings.ToLower(cp.class)] {
		for i := range text {
			text[i].r = unicode.ToUpper(text[i].r)
		}
	}
	var lines [][]lex.Run
	var plain []string
	var tags []placedTag
	line, cur := []lex.Run{}, []rune{}
	lastTag := -1
	var tagText []rune
	flushTag := func() {
		if lastTag >= 0 && strings.TrimSpace(string(tagText)) != "" {
			word := strings.TrimSpace(string(tagText))
			before := string(cur[:len(cur)-len(tagText)])
			all := strings.Count(strings.Join(plain, "\n")+"\n"+before, word)
			if len(plain) == 0 {
				all = strings.Count(before, word)
			}
			if e := tr.element(cp.tags[lastTag], word); e != "" {
				tags = append(tags, placedTag{line: len(lines), at: strings.Count(before, word), all: all, text: word, element: e})
			}
		}
		lastTag, tagText = -1, nil
	}
	endLine := func() {
		flushTag()
		lines = append(lines, line)
		plain = append(plain, string(cur))
		line, cur = []lex.Run{}, []rune{}
	}
	for _, c := range text {
		if c.r == '\n' {
			endLine()
			continue
		}
		if c.tag != lastTag {
			flushTag()
			lastTag = c.tag
		}
		cur = append(cur, c.r)
		if c.tag >= 0 {
			tagText = append(tagText, c.r)
		}
		sp := lex.Span{Text: string(c.r), Bold: c.b, Italic: c.i, Underline: c.u}
		if n := len(line); n > 0 && line[n-1].SameStyle(sp) {
			line[n-1].Text += sp.Text
		} else {
			line = append(line, lex.Run{Span: sp})
		}
	}
	endLine()
	var out []string
	for _, l := range lines {
		out = append(out, lex.FountainRuns(l))
	}
	p := &lex.Paragraph{Type: t, Text: strings.TrimSpace(strings.Join(out, "\n")), Notes: cp.notes}
	switch t {
	case lex.TypeSection:
		p.Level, p.Text = 1, lex.EscapeEmphasis(strings.TrimSpace(strings.Join(plain, " ")))
	case lex.TypeAction:
		if p.Text == "" && len(p.Notes) == 1 {
			if m := noteSection.FindStringSubmatch(p.Notes[0]); m != nil {
				return &lex.Paragraph{Type: lex.TypeSection, Level: len(m[1]), Text: lex.EscapeEmphasis(m[2])}, nil
			}
			if m := noteSynopsis.FindStringSubmatch(p.Notes[0]); m != nil {
				return &lex.Paragraph{Type: lex.TypeSynopsis, Text: m[1]}, nil
			}
		}
		ls := strings.Split(p.Text, "\n")
		for i, l := range ls {
			m := centred.FindStringSubmatch(strings.TrimSpace(l))
			if m == nil {
				break
			}
			if ls[i] = m[1]; i == len(ls)-1 {
				p.Type, p.Text = lex.TypeCenter, strings.Join(ls, "\n")
				tags = nil
			}
		}
	}
	if t == lex.TypeDialog || t == lex.TypeAction {
		if lyrics, ok := lyricText(p.Text); ok {
			p.Type, p.Text = lex.TypeLyrics, lyrics
		}
	}
	if p.Text == "" && len(p.Notes) == 0 {
		return nil, nil
	}
	return p, tags
}

// tagReader gathers the tagged items as a breakdown.
type tagReader struct {
	p        *Project
	set      *lex.TagSet
	elements map[string]string // an item's URI → its element's ID
}

func newTagReader(p *Project) *tagReader {
	return &tagReader{p: p, set: &lex.TagSet{}, elements: map[string]string{}}
}

// element is the element of the item a span tags (its words the name,
// when the item has none): its ID.
func (t *tagReader) element(sp spanTag, words string) string {
	if id, ok := t.elements[sp.ref]; ok {
		return id
	}
	dept, name := sp.class, words
	if n, ok := t.p.g[sp.ref]; ok {
		dept = cmp.Or(n.typeIn(nsCX), dept)
		name = cmp.Or(strings.TrimSpace(n.prop(nsDC, "title")), name)
	}
	if dept == "" || name == "" {
		return ""
	}
	cat, ok := lex.TagCategoryOf(dept)
	if !ok {
		if cat, ok = t.set.Category(uuidOf("celtx-department:" + strings.ToLower(dept))); !ok {
			cat = lex.TagCategory{ID: uuidOf("celtx-department:" + strings.ToLower(dept)), Name: dept}
			t.set.Categories = append(t.set.Categories, cat)
		}
	}
	id := uuidOf("celtx-item:" + sp.ref)
	t.elements[sp.ref] = id
	t.set.Elements = append(t.set.Elements, lex.TagElement{ID: id, Category: cat.ID, Name: name})
	return id
}

// place puts a paragraph's tags on the rows it was built as (from, to):
// a tag on its line's row (a speech's lines are rows of their own), the
// time it stands there counted again in the row's text.
func (t *tagReader) place(rows lex.Screenplay, from, to int, typ lex.ElementType, tags []placedTag, byRow map[int][]lex.LineTag) {
	var own []int // the paragraph's rows of its type
	for r := from; r < to && r < len(rows); r++ {
		if rows[r].Type == typ {
			own = append(own, r)
		}
	}
	if len(own) == 0 {
		return
	}
	for _, tg := range tags {
		row := own[0]
		if (typ == lex.TypeDialog || typ == lex.TypeLyrics) && tg.line < len(own) {
			row = own[tg.line]
		}
		plain := lex.TagPlain(rows[row].Contents, rows[row].Type == lex.TypeScene)
		at := tg.at
		if typ != lex.TypeDialog && typ != lex.TypeLyrics {
			at = tg.all // the paragraph's lines in one row
		}
		if lex.TagIndex(plain, tg.text, at) < 0 {
			at = 0
		}
		if lex.TagIndex(plain, tg.text, at) < 0 {
			continue
		}
		byRow[row] = append(byRow[row], lex.LineTag{Text: tg.text, At: at, Elements: []string{tg.element}})
	}
}

// uuidOf is a UUID (version 5's form) made from a name: the same name,
// the same UUID.
func uuidOf(name string) string {
	h := sha1.Sum([]byte(name))
	h[6] = h[6]&0x0f | 0x50
	h[8] = h[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

// trimLines is a note's text without the spaces at its lines' ends.
func trimLines(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// lyricText is a paragraph's text as lyrics, if each of its lines is
// lyrics as Fountain marks them ("~la la la"): without the ~s.
func lyricText(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		rest, ok := strings.CutPrefix(strings.TrimSpace(l), "~")
		if !ok {
			return "", false
		}
		lines[i] = strings.TrimSpace(rest)
	}
	return strings.Join(lines, "\n"), true
}
