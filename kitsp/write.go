package kitsp

import (
	"crypto/sha1"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/LaPingvino/lexington/internal/sqlite"
	"github.com/LaPingvino/lexington/lex"
)

// AppVersion is the KIT Scenarist a project is written as from: its last
// release, which opens it without updating it.
const AppVersion = "0.7.2 rc 15"

// KITWriter writes a screenplay as a KIT Scenarist project: the database
// a new project of KIT's has (its tables, its lists of places, times,
// character states and transitions), with the script and its title page.
type KITWriter struct {
	// Date is the first draft's date, as KIT writes it ("2026-11-26
	// 12:00:00:000"); "" a fixed one, so that a script is written the same
	// every time.
	Date string
}

// Write writes the project.
func (w *KITWriter) Write(out io.Writer, s lex.Screenplay) error {
	title, paras := lex.Paragraphs(s)
	date := w.Date
	if date == "" {
		date = "2000-01-01 00:00:00:000"
	}
	var data [][]sqlite.Value
	written := map[string]bool{}
	for _, f := range title {
		for _, k := range titleKeys {
			if strings.EqualFold(f.Key, k.key) || (k.data == "year" && strings.EqualFold(f.Key, "Copyright")) ||
				(k.data == "author" && strings.EqualFold(f.Key, "Authors")) {
				if !written[k.data] {
					written[k.data] = true
					data = append(data, []sqlite.Value{nil, k.data, lex.Unstyled(f.Value)})
				}
			}
		}
	}
	list := func(names ...string) [][]sqlite.Value {
		var rows [][]sqlite.Value
		for _, n := range names {
			rows = append(rows, []sqlite.Value{nil, n})
		}
		return rows
	}
	tables := []sqlite.TableDef{
		{Name: "_database_history", SQL: "CREATE TABLE _database_history ( id TEXT PRIMARY KEY, query TEXT NOT NULL, query_values TEXT NOT NULL, username TEXT NOT NULL, datetime TEXT NOT NULL )",
			Unique: [][]string{{"id"}}},
		{Name: "system_variables", SQL: "CREATE TABLE system_variables ( variable TEXT PRIMARY KEY ON CONFLICT REPLACE, value TEXT NOT NULL )",
			Rows:   [][]sqlite.Value{{"application-version-on-create", AppVersion}, {"application-version", AppVersion}},
			Unique: [][]string{{"variable"}}},
		{Name: "places", SQL: "CREATE TABLE places ( id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL )",
			Rows: list("INT", "EXT", "INT./EXT"), Unique: [][]string{{"name"}}},
		{Name: "scenary_days", SQL: "CREATE TABLE scenary_days ( id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL )",
			Unique: [][]string{{"name"}}},
		{Name: "times", SQL: "CREATE TABLE times ( id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL )",
			Rows:   list("DAY", "NIGHT", "MORNING", "AFTERNOON", "EVENING", "LATER", "MOMENTS LATER", "CONTINUOUS", "THE NEXT DAY"),
			Unique: [][]string{{"name"}}},
		{Name: "character_states", SQL: "CREATE TABLE character_states ( id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL )",
			Rows: list("V.O.", "O.S.", "O.C.", "SUBTITLE", "CONT'D"), Unique: [][]string{{"name"}}},
		{Name: "transitions", SQL: "CREATE TABLE transitions ( id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL )",
			Rows:   list("CUT TO:", "FADE IN:", "FADE OUT", "FADE TO:", "DISSOLVE TO:", "BACK TO:", "MATCH CUT TO:", "JUMP CUT TO:", "FADE TO BLACK"),
			Unique: [][]string{{"name"}}},
		{Name: "scenario", SQL: "CREATE TABLE scenario ( id INTEGER PRIMARY KEY AUTOINCREMENT, scheme TEXT NOT NULL, text TEXT NOT NULL, is_draft INTEGER NOT NULL DEFAULT(0) )",
			Rows: [][]sqlite.Value{{nil, defaultCards, ScenarioXML(paras), int64(0)}}},
		{Name: "scenario_changes", SQL: "CREATE TABLE scenario_changes (id INTEGER PRIMARY KEY AUTOINCREMENT, uuid TEXT NOT NULL, datetime TEXT NOT NULL, username TEXT NOT NULL, undo_patch TEXT NOT NULL, redo_patch TEXT NOT NULL, is_draft INTEGER NOT NULL DEFAULT(0) )"},
		{Name: "scenario_data", SQL: "CREATE TABLE scenario_data (id INTEGER PRIMARY KEY AUTOINCREMENT, data_name TEXT NOT NULL UNIQUE, data_value TEXT DEFAULT(NULL) )",
			Rows: data, Unique: [][]string{{"data_name"}}},
		{Name: "research", SQL: "CREATE TABLE research (id INTEGER PRIMARY KEY AUTOINCREMENT, parent_id INTEGER DEFAULT(NULL), type INTEGER NOT NULL DEFAULT(0), name TEXT NOT NULL, description TEXT DEFAULT(NULL), url TEXT DEFAULT(NULL), image BLOB DEFAULT(NULL), color TEXT DEFAULT(NULL), sort_order INTEGER NOT NULL DEFAULT(0) )"},
		{Name: "script_versions", SQL: "CREATE TABLE script_versions ( id INTEGER PRIMARY KEY AUTOINCREMENT, fk_script_id INTEGER NOT NULL, username TEXT NOT NULL, datetime TEXT NOT NULL, color TEXT DEFAULT(NULL), name TEXT UNIQUE NOT NULL, description TEXT DEFAULT(NULL), script_text TEXT NOT NULL)",
			Rows: [][]sqlite.Value{{nil, int64(0), "", date, nil, "First draft", nil, ""}}, Unique: [][]string{{"name"}}},
	}
	db, err := sqlite.Write(tables)
	if err != nil {
		return err
	}
	_, err = out.Write(db)
	return err
}

// defaultCards is the cards' scheme of a new project, as KIT writes it.
const defaultCards = `<?xml version="1.0"?>
<cards x="0" y="0" width="0" height="0" scale="1" >
<Card id="{000000-0000000-000000}" is_folder="false" title="" description="" stamp="" colors="" x="0" y="0" />
</cards_xml>`

// hexColor is a colour note KIT keeps as a scene's colour.
var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ScenarioXML is the paragraphs as KIT's scenario XML.
func ScenarioXML(paras []lex.Paragraph) string {
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\"?>\n<scenario version=\"1.0\">\n")
	depth := 0
	n := 0
	inDual := false
	for _, p := range paras {
		for _, note := range p.Before {
			writeBlock(&b, "note", "", note, nil)
		}
		switch p.Type {
		case lex.TypeSection:
			for ; depth >= p.Level && depth > 0; depth-- {
				writeBlock(&b, "folder_footer", "", "", nil)
			}
			for depth < p.Level-1 { // a ### under a #: the folders between
				depth++
				writeBlock(&b, "folder_header", ` uuid="`+uuid(n, "")+`"`, "", nil)
				n++
			}
			depth++
			attrs, notes := colorAttr(p.Notes)
			writeBlock(&b, "folder_header", ` uuid="`+uuid(n, p.Text)+`"`+attrs, p.Text, notes)
			n++
			for _, s := range p.Synopses {
				writeBlock(&b, "scene_description", "", s, nil)
			}
			continue
		case lex.TypeBoneyard:
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(p.Text), "/*"), "*/"))
			writeBlock(&b, "noprintable_text", "", lex.EscapeEmphasis(text), nil)
			continue
		case lex.TypeSynopsis:
			writeBlock(&b, "scene_description", "", p.Text, p.Notes)
			continue
		}
		if p.Type == lex.TypeSpeaker && p.Dual == lex.DualFirst {
			if inDual {
				b.WriteString("<splitter_end/>\n")
			}
			b.WriteString("<splitter_start/>\n")
			inDual = true
		} else if p.Type == lex.TypeSpeaker && p.Dual == lex.DualSecond && inDual {
			b.WriteString("<splitter/>\n")
		} else if inDual && !inSpeech(p.Type) {
			b.WriteString("<splitter_end/>\n")
			inDual = false
		}
		name := map[lex.ElementType]string{
			lex.TypeScene: "scene_heading", lex.TypeSpeaker: "character", lex.TypeParen: "parenthetical",
			lex.TypeDialog: "dialog", lex.TypeTrans: "transition", lex.TypeLyrics: "lyrics", lex.TypeCenter: "title",
		}[p.Type]
		if name == "" {
			name = "action"
		}
		attrs, notes := "", p.Notes
		if p.Type == lex.TypeScene {
			attrs, notes = colorAttr(p.Notes)
			attrs = ` uuid="` + uuid(n, p.Text) + `"` + attrs
			n++
			if p.Number != "" {
				attrs += ` number="` + xmlEscape(p.Number) + `" number_fix_nesting="0" number_suffix="0"`
			}
		}
		lines := strings.Split(p.Text, "\n")
		for i, l := range lines {
			if i == len(lines)-1 {
				writeBlock(&b, name, attrs, l, notes)
			} else {
				writeBlock(&b, name, attrs, l, nil)
				attrs = ""
			}
		}
		for _, s := range p.Synopses {
			writeBlock(&b, "scene_description", "", s, nil)
		}
	}
	if inDual {
		b.WriteString("<splitter_end/>\n")
	}
	for ; depth > 0; depth-- {
		writeBlock(&b, "folder_footer", "", "", nil)
	}
	b.WriteString("</scenario>\n")
	return b.String()
}

func inSpeech(t lex.ElementType) bool {
	return t == lex.TypeParen || t == lex.TypeDialog || t == lex.TypeLyrics
}

// colorAttr takes a heading's colour notes out as KIT's color attribute.
func colorAttr(notes []string) (string, []string) {
	var colors, rest []string
	for _, n := range notes {
		if hexColor.MatchString(strings.TrimSpace(n)) {
			colors = append(colors, strings.ToLower(strings.TrimSpace(n)))
		} else {
			rest = append(rest, n)
		}
	}
	if len(colors) == 0 {
		return "", notes
	}
	return ` color="` + strings.Join(colors, ";") + `"`, rest
}

func uuid(n int, text string) string {
	sum := sha1.Sum([]byte(strconv.Itoa(n) + "\x00" + text))
	return fmt.Sprintf("{%x-%x-%x-%x-%x}", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

// writeBlock writes a block: its text HTML-escaped in a CDATA section, as KIT
// writes it; its emphasis as formats, its notes as a review's comments.
func writeBlock(b *strings.Builder, name, attrs, text string, notes []string) {
	spans := lex.Emphasis(text)
	var plain strings.Builder
	type fmtRange struct {
		from, length int
		span         lex.Span
	}
	var fs []fmtRange
	at := 0
	for _, s := range spans {
		n := len(utf16.Encode([]rune(s.Text)))
		if s.Bold || s.Italic || s.Underline {
			fs = append(fs, fmtRange{at, n, s})
		}
		plain.WriteString(s.Text)
		at += n
	}
	fmt.Fprintf(b, "<%s%s>\n<v><![CDATA[%s]]></v>\n", name, attrs, htmlEscape(plain.String()))
	if len(notes) > 0 {
		fmt.Fprintf(b, `<reviews>`+"\n"+`<review from="0" length="%d" color="#000000" bgcolor="#ffff00" is_highlight="true" done="false">`+"\n", max(at, 1))
		for _, n := range notes {
			fmt.Fprintf(b, `<review_comment comment="%s" author="" date=""/>`+"\n", xmlEscape(htmlEscape(n)))
		}
		b.WriteString("</review>\n</reviews>\n")
	}
	if len(fs) > 0 {
		b.WriteString("<formatting>\n")
		for _, f := range fs {
			fmt.Fprintf(b, `<format from="%d" length="%d" bold="%t" italic="%t" underline="%t"/>`+"\n",
				f.from, f.length, f.span.Bold, f.span.Italic, f.span.Underline)
		}
		b.WriteString("</formatting>\n")
	}
	fmt.Fprintf(b, "</%s>\n", name)
}

// htmlEscape is Qt's toHtmlEscaped: what KIT writes in the CDATA.
var htmlEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace

// xmlEscape escapes an attribute's value.
var xmlEscape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\n", "&#10;").Replace
