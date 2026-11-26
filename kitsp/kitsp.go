// Package kitsp reads and writes KIT Scenarist's projects (.kitsp): an
// SQLite database whose scenario table holds the script as XML, a block a
// line, and whose scenario_data table holds the title page. Its format is
// as KIT Scenarist's own code has it (github.com/dimkanovikov/
// KITScenaristCore: DataLayer/Database, BusinessLayer/ScenarioDocument/
// ScenarioXml.cpp); no code of KIT's is in this package, and the database
// is read and written by Lexington's own internal/sqlite.
//
// What maps to what, as KIT's own Fountain import and export have it:
// folders are sections, a scene description a synopsis, review comments
// notes, non-printing text a boneyard; Lexington adds a note element for
// a line of notes alone, KIT's title style for a centred line, and its
// scene colours for colour notes ([[#ff0000]]). KIT has no page break.
package kitsp

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/LaPingvino/lexington/internal/sqlite"
	"github.com/LaPingvino/lexington/lex"
)

// The block types of KIT's scenario XML, by their element's name.
var blockTypes = map[string]lex.ElementType{
	"scene_heading":     lex.TypeScene,
	"time_and_place":    lex.TypeScene, // older files'
	"scene_characters":  lex.TypeAction,
	"action":            lex.TypeAction,
	"character":         lex.TypeSpeaker,
	"parenthetical":     lex.TypeParen,
	"dialog":            lex.TypeDialog,
	"transition":        lex.TypeTrans,
	"note":              lex.TypeNote,
	"title_header":      lex.TypeCenter,
	"title":             lex.TypeCenter,
	"noprintable_text":  lex.TypeBoneyard,
	"folder_header":     lex.TypeSection,
	"folder_footer":     "folder_footer",
	"scene_description": lex.TypeSynopsis,
	"lyrics":            lex.TypeLyrics,
	"undefined":         lex.TypeAction,
}

// The title page's keys in scenario_data, as KIT's Fountain import maps
// Fountain's.
var titleKeys = []struct{ data, key string }{
	{"name", "Title"},
	{"genre", "Credit"},
	{"author", "Author"},
	{"additional_info", "Source"},
	{"year", "Draft date"},
	{"contacts", "Contact"},
	{"logline", "Logline"},
	{"synopsis", "Synopsis"},
}

// Parse reads a KIT Scenarist project.
func Parse(r io.Reader) (lex.Screenplay, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	db, err := sqlite.Open(data)
	if err != nil {
		return nil, fmt.Errorf("not a KIT Scenarist project: %w", err)
	}
	rows, err := db.Rows("scenario")
	if err != nil {
		return nil, fmt.Errorf("not a KIT Scenarist project: %w", err)
	}
	var text string
	for _, r := range rows { // the script, not its draft
		if draft, _ := r["is_draft"].(int64); draft == 0 || text == "" {
			if t, ok := r["text"].(string); ok {
				text = t
				if draft == 0 {
					break
				}
			}
		}
	}
	s, err := ParseXML(strings.NewReader(text))
	if err != nil {
		return nil, err
	}
	var title []lex.TitleField
	if data, err := db.Rows("scenario_data"); err == nil {
		values := map[string]string{}
		for _, r := range data {
			if n, ok := r["data_name"].(string); ok {
				v, _ := r["data_value"].(string)
				values[n] = v
			}
		}
		for _, k := range titleKeys {
			if v := strings.TrimSpace(values[k.data]); v != "" {
				title = append(title, lex.TitleField{Key: k.key, Value: lex.EscapeEmphasis(v)})
			}
		}
	}
	if len(title) > 0 {
		var b lex.Builder
		b.TitlePage(title)
		s = append(b.Screenplay(), s...)
	}
	return s, nil
}

// block is a block of KIT's scenario XML as read.
type block struct {
	name    string
	attrs   []xml.Attr
	text    string
	formats []format
	notes   []string
}

type format struct {
	from, length            int
	bold, italic, underline bool
}

// ParseXML reads KIT's scenario XML.
func ParseXML(r io.Reader) (lex.Screenplay, error) {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	var b lex.Builder
	depth := 0     // the folders open
	dual := 0      // 0 none, 1 before the first speaker of a column, 2 the first column's, 3 before the second's
	var cur *block // the block being read
	inValue := false
	started := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("KIT Scenarist's script: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			switch {
			case name == "scenario":
				started = true
			case !started:
				return nil, errors.New("KIT Scenarist's script: no <scenario>")
			case cur == nil && name == "splitter_start":
				dual = 1
			case cur == nil && name == "splitter":
				dual = 3
			case cur == nil && name == "splitter_end":
				dual = 0
				b.DualClose()
			case cur == nil:
				cur = &block{name: name, attrs: t.Attr}
			case name == "v":
				inValue = true
			case name == "format":
				f := format{from: atoi(attr(t.Attr, "from")), length: atoi(attr(t.Attr, "length")),
					bold: attr(t.Attr, "bold") == "true", italic: attr(t.Attr, "italic") == "true",
					underline: attr(t.Attr, "underline") == "true"}
				cur.formats = append(cur.formats, f)
			case name == "review_comment":
				if c := strings.TrimSpace(unescape(attr(t.Attr, "comment"))); c != "" {
					cur.notes = append(cur.notes, c)
				}
			}
		case xml.CharData:
			if cur != nil && inValue {
				cur.text += string(t)
			}
		case xml.EndElement:
			switch {
			case t.Name.Local == "v":
				inValue = false
			case cur != nil && t.Name.Local == cur.name:
				p, ok := cur.paragraph(&depth)
				cur = nil
				if !ok {
					continue
				}
				if p.Type == lex.TypeSpeaker {
					switch dual {
					case 1:
						p.Dual, dual = lex.DualFirst, 2
					case 3:
						p.Dual, dual = lex.DualSecond, 4
					}
				}
				b.Add(p)
			}
		}
	}
	return b.Screenplay(), nil
}

func attr(as []xml.Attr, name string) string {
	for _, a := range as {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// unescape reads KIT's HTML escapes (TextEditHelper::toHtmlEscaped), which
// it writes inside the XML's own.
var unescape = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&amp;", "&").Replace

// paragraph is the block as a paragraph; ok is false for a folder's end.
func (bl *block) paragraph(depth *int) (lex.Paragraph, bool) {
	t, known := blockTypes[bl.name]
	if !known {
		t = lex.TypeAction
	}
	text := unescape(bl.text)
	switch t {
	case "folder_footer":
		*depth = max(*depth-1, 0)
		return lex.Paragraph{}, false
	case lex.TypeBoneyard:
		return lex.Paragraph{Type: lex.TypeBoneyard, Text: "/*\n" + text + "\n*/"}, strings.TrimSpace(text) != ""
	case lex.TypeNote:
		return lex.Paragraph{Type: lex.TypeAction, Before: append([]string{text}, bl.notes...)}, true
	case lex.TypeSynopsis:
		return lex.Paragraph{Type: t, Text: lex.EscapeEmphasis(text), Notes: bl.notes}, true
	}
	p := lex.Paragraph{Type: t, Text: styled(text, bl.formats), Notes: bl.notes}
	switch t {
	case lex.TypeSection:
		*depth++
		p.Level = *depth
	case lex.TypeScene, lex.TypeSpeaker, lex.TypeTrans:
		p.Text = strings.ToUpper(p.Text)
	}
	if t == lex.TypeScene {
		p.Number = attr(bl.attrs, "number")
	}
	if t == lex.TypeScene || t == lex.TypeSection {
		for _, c := range strings.Split(attr(bl.attrs, "color"), ";") {
			if c = strings.TrimSpace(c); c != "" {
				p.Notes = append([]string{c}, p.Notes...)
			}
		}
	}
	return p, true
}

// styled is a block's text as Fountain, its formats' ranges (in UTF-16
// units, as Qt counts) as emphasis.
func styled(text string, fs []format) string {
	runes := []rune(text)
	style := make([]lex.Span, len(runes))
	if len(fs) > 0 {
		// the rune each UTF-16 unit is in
		var unit []int
		for i, r := range runes {
			unit = append(unit, i)
			if utf16.RuneLen(r) == 2 {
				unit = append(unit, i)
			}
		}
		for _, f := range fs {
			for u := f.from; u < f.from+f.length && u < len(unit); u++ {
				if u < 0 {
					continue
				}
				s := &style[unit[u]]
				s.Bold, s.Italic, s.Underline = s.Bold || f.bold, s.Italic || f.italic, s.Underline || f.underline
			}
		}
	}
	var runs []lex.Run
	for i, r := range runes {
		sp := style[i]
		sp.Text = string(r)
		if n := len(runs); n > 0 && runs[n-1].Span.SameStyle(sp) {
			runs[n-1].Text += sp.Text
			continue
		}
		runs = append(runs, lex.Run{Span: sp})
	}
	return lex.FountainRuns(runs)
}

// IsProject tells whether data is an SQLite database: a .kitsp's form.
func IsProject(data []byte) bool { return bytes.HasPrefix(data, []byte(sqlite.Magic)) }
