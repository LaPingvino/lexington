// Package osf reads and writes the Open Screenplay Format, Fade In's own:
// an XML document (a .osf file), which a .fadein file holds zipped as its
// document.xml (fadein.go).
//
// Its versions are each read here: 1.2 (version="12", its emphasis as
// tags in the text: <b>, <i>, <u>, <br>), 2.x and 3.0 (version="20",
// "21", "30": text runs with attributes, the title page in <titlepage>)
// and 4.0 (version="40": basestyle for basestylename, number for
// scene_number).
// The writer writes any of them (OSFWriter.Version), 4.0 unless told.
//
// The format is known from the OSF SDK's sample documents
// (github.com/OpenScreenplayFormat/osf-sdk, one per version) and from
// Fade In's own files; no other program's code is in this package.
package osf

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// DocumentType is the <document> type attribute of every OSF file.
const DocumentType = "Open Screenplay Format document"

// The built-in styles' names (builtin_index 0 to 7).
const (
	styleNormal = "Normal Text"
	styleScene  = "Scene Heading"
	styleAction = "Action"
	styleChar   = "Character"
	styleParen  = "Parenthetical"
	styleDialog = "Dialogue"
	styleTrans  = "Transition"
	styleShot   = "Shot"
	styleLyrics = "Lyrics" // Lexington's own, for Fountain's ~lyrics
)

// attrs are an element's attributes, read by name whatever its case or
// underscores (basestylename, baseStyleName; scene_number, sceneNumber).
type attrs []xml.Attr

func norm(name string) string { return strings.ToLower(strings.ReplaceAll(name, "_", "")) }

func (as attrs) get(names ...string) string {
	for _, n := range names {
		for _, a := range as {
			if norm(a.Name.Local) == norm(n) {
				return a.Value
			}
		}
	}
	return ""
}

func (as attrs) on(names ...string) bool {
	v := strings.ToLower(as.get(names...))
	return v == "1" || v == "true" || v == "yes"
}

type xdoc struct {
	Attrs  attrs   `xml:",any,attr"`
	Info   *xelem  `xml:"info"`
	Styles []xelem `xml:"styles>style"`
	Paras  []xpara `xml:"paragraphs>para"`
	Title  []xpara `xml:"titlepage>para"`
}

type xelem struct {
	Attrs attrs `xml:",any,attr"`
}

type xpara struct {
	Attrs attrs   `xml:",any,attr"`
	Style xelem   `xml:"style"`
	Texts []xtext `xml:"text"`
}

type xtext struct {
	Attrs attrs  `xml:",any,attr"`
	Text  string `xml:",chardata"`
}

// style is what a paragraph style is in a script.
type style struct {
	typ  lex.ElementType
	caps bool
}

// Version is a document's OSF version as its attribute has it ("12",
// "20", "40"), as a number (12, 20, 40); 0 when it has none.
func Version(v string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(strings.ReplaceAll(v, ".", "")))
	if n > 0 && n < 10 { // "4" for 4.0
		n *= 10
	}
	return n
}

// Parse reads an OSF document.
func Parse(r io.Reader) (lex.Screenplay, error) {
	var d xdoc
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("not an Open Screenplay Format document: %w", err)
	}
	version := Version(d.Attrs.get("version"))
	styles := readStyles(d.Styles)
	var b lex.Builder
	inDual := false // since a speaker marked dualdialogue, before the second's
	for _, p := range d.Paras {
		name := p.Style.Attrs.get("basestyle", "basestylename")
		st, ok := styles[name]
		if !ok {
			st = builtin(name)
		}
		par := lex.Paragraph{Type: st.typ}
		par.Text = paraText(p, version, st.caps)
		if strings.EqualFold(p.Style.Attrs.get("align"), "center") && par.Type == lex.TypeAction {
			par.Type = lex.TypeCenter
		}
		par.NewPage = p.Style.Attrs.on("pagebreakbefore")
		if n := p.Attrs.get("note"); strings.TrimSpace(n) != "" {
			par.Notes = []string{n}
		}
		if s := p.Attrs.get("synopsis"); strings.TrimSpace(s) != "" {
			par.Synopses = strings.Split(s, "\n")
		}
		switch par.Type {
		case lex.TypeScene:
			par.Number = p.Attrs.get("number", "scene_number")
		case lex.TypeSpeaker:
			switch {
			case p.Style.Attrs.on("dualdialogue"):
				par.Dual, inDual = lex.DualFirst, true
			case inDual:
				par.Dual, inDual = lex.DualSecond, false
			}
		}
		b.Add(par)
	}
	b.TitlePage(titlePage(d, version))
	return b.Screenplay(), nil
}

// readStyles are the document's styles, by name: each the element its
// base is (a style of one's own on Action is action), in capitals as its
// allcaps (4.0) or effects (1.2, its lowest bit) has it.
func readStyles(xs []xelem) map[string]style {
	base := map[string]string{}
	for _, s := range xs {
		base[s.Attrs.get("name")] = s.Attrs.get("basestyle", "basestylename")
	}
	out := map[string]style{}
	for _, s := range xs {
		name := s.Attrs.get("name")
		st := builtin(name)
		for n, seen := name, 0; seen < 10 && st.typ == lex.TypeAction && !isBuiltin(n); seen++ {
			n = base[n]
			if n == "" {
				break
			}
			st = builtin(n)
		}
		if c := s.Attrs.get("allcaps"); c != "" {
			st.caps = c == "1" || strings.EqualFold(c, "true")
		} else if e, err := strconv.Atoi(s.Attrs.get("effects")); err == nil {
			st.caps = e&1 == 1
		}
		out[name] = st
	}
	return out
}

func isBuiltin(name string) bool {
	switch name {
	case styleNormal, styleScene, styleAction, styleChar, styleParen, styleDialog, styleTrans, styleShot:
		return true
	}
	return false
}

// builtin is the element a style is by its name, with the built-in
// styles' capitals.
func builtin(name string) style {
	switch strings.ToLower(name) {
	case "scene heading":
		return style{lex.TypeScene, true}
	case "character":
		return style{lex.TypeSpeaker, true}
	case "parenthetical":
		return style{lex.TypeParen, false}
	case "dialogue":
		return style{lex.TypeDialog, false}
	case "transition":
		return style{lex.TypeTrans, true}
	case "shot":
		return style{lex.TypeAction, true}
	case "lyrics":
		return style{lex.TypeLyrics, false}
	}
	return style{lex.TypeAction, false}
}

// paraText is a paragraph's text as Fountain: its runs' emphasis as
// markers, in capitals for a style in capitals.
func paraText(p xpara, version int, caps bool) string {
	var runs []lex.Run
	for _, t := range p.Texts {
		if version > 0 && version < 20 {
			runs = append(runs, inlineRuns(t.Text)...)
			continue
		}
		runs = append(runs, lex.Run{Span: lex.Span{Text: t.Text,
			Bold: t.Attrs.on("bold"), Italic: t.Attrs.on("italic"), Underline: t.Attrs.on("underline")}})
	}
	for i := range runs {
		runs[i].Text = strings.ReplaceAll(runs[i].Text, "\r", "")
		if caps {
			runs[i].Text = strings.ToUpper(runs[i].Text)
		}
	}
	return strings.TrimSpace(lex.FountainRuns(runs))
}

// inlineTag is one of OSF 1.2's tags in a text: <b>, </i>, <br>,
// <font="Courier">, <rev="1">.
var inlineTag = regexp.MustCompile(`<(/?)(b|i|u|s|br|font|size|bgcolor|color|rev)(=[^<>]*)?>`)

// inlineRuns are an OSF 1.2 text's runs: its emphasis tags read, <br> a
// line break, the other tags (font, size, colours, revisions) left out;
// < and > in its text are escaped (&lt;), as they would be tags (no 1.2
// file seen has one: the escape is a guess).
func inlineRuns(text string) []lex.Run {
	var out []lex.Run
	var cur lex.Span
	at := 0
	add := func(s string) {
		if s == "" {
			return
		}
		r := cur
		r.Text = inlineUnescape.Replace(s)
		out = append(out, lex.Run{Span: r})
	}
	for _, m := range inlineTag.FindAllStringSubmatchIndex(text, -1) {
		add(text[at:m[0]])
		at = m[1]
		closing, tag := text[m[2]:m[3]] == "/", text[m[4]:m[5]]
		switch tag {
		case "b":
			cur.Bold = !closing
		case "i":
			cur.Italic = !closing
		case "u":
			cur.Underline = !closing
		case "br":
			add("\n")
		}
	}
	add(text[at:])
	return out
}

// infoText is a 1.2 <info> value as Fountain: its tags read.
func infoText(v string) string { return lex.FountainRuns(inlineRuns(v)) }

// creditLine is a credit's line ("Written by", "A Short Film by").
var creditLine = regexp.MustCompile(`(?i)\bby:?$`)

// inlineUnescape reads the characters a 1.2 text escapes, as they are
// markup in it.
var inlineUnescape = strings.NewReplacer("&lt;", "<", "&gt;", ">")

// Fade In's placeholders in a new 1.2 document's <info>.
var placeholders = map[string]bool{
	"untitled screenplay": true, "untitled": true, "written by": true,
	"copyright (c)": true, "first draft": true,
}

// titlePage is the document's title page: 2.x and 4.0's <titlepage>,
// its paragraphs marked by their bookmarks (Title, Author, Copyright,
// Draft, Contact); 1.2's <info>, unless all of it is Fade In's
// placeholders.
func titlePage(d xdoc, version int) []lex.TitleField {
	if len(d.Title) == 0 && d.Info != nil {
		var out []lex.TitleField
		real := false
		add := func(key, raw string) {
			v := strings.TrimSpace(infoText(raw))
			if v == "" {
				return
			}
			real = real || !placeholders[strings.ToLower(v)]
			out = append(out, lex.TitleField{Key: key, Value: v})
		}
		info := d.Info.Attrs
		title := info.get("title")
		if !strings.Contains(title, "<") && title != "" {
			title = info.get("title_format") + title // the title's own look, as tags ("<u>")
		}
		add("Title", title)
		if wb := strings.TrimSpace(infoText(info.get("written_by"))); wb != "" {
			credit, author, _ := strings.Cut(wb, "\n")
			real = real || !placeholders[strings.ToLower(credit)]
			out = append(out, lex.TitleField{Key: "Credit", Value: strings.TrimSpace(credit)})
			if author = strings.TrimSpace(author); author != "" {
				out = append(out, lex.TitleField{Key: "Author", Value: author})
			}
		}
		add("Source", info.get("subtitle"))
		add("Draft date", info.get("drafts"))
		add("Copyright", info.get("copyright"))
		add("Contact", info.get("contact"))
		if !real {
			return nil
		}
		return out
	}
	var out []lex.TitleField
	seenTitle, seenAuthor := false, false
	for _, p := range d.Title {
		text := strings.TrimSpace(paraText(p, version, false))
		if text == "" {
			continue
		}
		key := ""
		switch strings.ToLower(p.Attrs.get("bookmark")) {
		case "title":
			key, seenTitle = "Title", true
		case "author":
			key, seenAuthor = "Author", true
		case "credit":
			key = "Credit"
		case "copyright":
			key = "Copyright"
		case "draft":
			key = "Draft date"
		case "contact":
			key = "Contact"
		case "source", "based on":
			key = "Source"
		case "notes":
			key = "Notes"
		default:
			switch {
			case creditLine.MatchString(text):
				key = "Credit"
			case strings.EqualFold(p.Style.Attrs.get("align"), "center") && !seenAuthor:
				if !seenTitle {
					key, seenTitle = "Title", true
				} else {
					key = "Credit"
				}
			case !strings.EqualFold(p.Style.Attrs.get("align"), "center"):
				key = "Contact"
			default:
				key = "Notes"
			}
		}
		if n := len(out); n > 0 && out[n-1].Key == key {
			out[n-1].Value += "\n" + text
			continue
		}
		out = append(out, lex.TitleField{Key: key, Value: text})
	}
	return out
}
