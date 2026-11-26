package kitsp

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/internal/sqlite"
	"github.com/LaPingvino/lexington/lex"
)

// sampleXML is a script as KIT Scenarist writes it (by hand here, after
// its ScenarioXml.cpp): a folder, a heading with its colour and number,
// formats counted in UTF-16 units (the emoji is two), a review's comments,
// dual dialogue in a table's columns, the elements KIT has of its own.
const sampleXML = `<?xml version="1.0"?>
<scenario version="1.0">
<folder_header uuid="{1}">
<v><![CDATA[Act one]]></v>
</folder_header>
<scene_heading uuid="{2}" color="#ff0000" number="4" number_fix_nesting="0" number_suffix="0">
<v><![CDATA[int. hall - day]]></v>
</scene_heading>
<scene_description>
<v><![CDATA[They meet.]]></v>
</scene_description>
<action>
<v><![CDATA[🙂 John walks &amp; stops.]]></v>
<reviews>
<review from="0" length="5" color="#000000" bgcolor="#ffff00" is_highlight="true">
<review_comment comment="Check &amp;quot;this&amp;quot;." author="Jo" date="2020"/>
<review_comment comment="And this." author="Mo" date="2020"/>
</review>
</reviews>
<formatting>
<format from="3" length="4" bold="true"/>
<format from="16" length="5" italic="true" underline="true"/>
</formatting>
</action>
<note>
<v><![CDATA[A note of its own]]></v>
</note>
<splitter_start/>
<character>
<v><![CDATA[john]]></v>
</character>
<dialog>
<v><![CDATA[Hello.]]></v>
</dialog>
<dialog>
<v><![CDATA[How are you?]]></v>
</dialog>
<splitter/>
<character>
<v><![CDATA[MARY]]></v>
</character>
<dialog>
<v><![CDATA[Hi.]]></v>
</dialog>
<splitter_end/>
<title>
<v><![CDATA[THE END]]></v>
</title>
<noprintable_text>
<v><![CDATA[cut scene]]></v>
</noprintable_text>
<transition>
<v><![CDATA[cut to:]]></v>
</transition>
<folder_footer>
<v><![CDATA[]]></v>
</folder_footer>
</scenario>
`

var sampleRows = []string{
	"titlepage: ",
	"Title: Brick",
	"Credit: Written by",
	"Author: Jo Doe",
	"Draft date: 2026",
	"Contact: jo@example.com",
	"newpage: ",
	"section: # Act one",
	"scene: INT. HALL - DAY #4# [[#ff0000]]",
	"synopse: They meet.",
	`action: 🙂 **John** walks & _*stops*_. [[Check "this".]] [[And this.]]`,
	"note: [[A note of its own]]",
	"dualspeaker_open: ",
	"speaker: JOHN",
	"dialog: Hello.",
	"dialog: How are you?",
	"dualspeaker_next: ",
	"speaker: MARY",
	"dialog: Hi.",
	"dualspeaker_close: ",
	"center: THE END",
	"boneyard: /*\ncut scene\n*/",
	"trans: CUT TO:",
}

func project(t *testing.T, xml string) []byte {
	t.Helper()
	db, err := sqlite.Write([]sqlite.TableDef{
		{Name: "scenario", SQL: "CREATE TABLE scenario ( id INTEGER PRIMARY KEY AUTOINCREMENT, scheme TEXT NOT NULL, text TEXT NOT NULL, is_draft INTEGER NOT NULL DEFAULT(0) )",
			Rows: [][]sqlite.Value{{nil, "", "<scenario version=\"1.0\"></scenario>", int64(1)}, {nil, "", xml, int64(0)}}},
		{Name: "scenario_data", SQL: "CREATE TABLE scenario_data (id INTEGER PRIMARY KEY AUTOINCREMENT, data_name TEXT NOT NULL UNIQUE, data_value TEXT DEFAULT(NULL) )",
			Rows: [][]sqlite.Value{{nil, "name", "Brick"}, {nil, "author", "Jo Doe"}, {nil, "genre", "Written by"},
				{nil, "year", "2026"}, {nil, "contacts", "jo@example.com"}, {nil, "scene_numbers_prefix", ""}},
			Unique: [][]string{{"data_name"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func parse(t *testing.T, data []byte) lex.Screenplay {
	t.Helper()
	s, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func write(t *testing.T, s lex.Screenplay) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := (&KITWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRead(t *testing.T) {
	formattest.Same(t, "sample", sampleRows, formattest.Rows(parse(t, project(t, sampleXML)), nil))
}

func TestRoundTrip(t *testing.T) {
	first := parse(t, project(t, sampleXML))
	out := write(t, first)
	formattest.Same(t, "again", formattest.Rows(first, nil), formattest.Rows(parse(t, out), nil))
	if !IsProject(out) {
		t.Error("not a database")
	}
	checkSQLite(t, out)
}

// checkSQLite has sqlite3 check a project, as KIT reads it (when sqlite3
// is installed).
func checkSQLite(t *testing.T, data []byte) {
	t.Helper()
	sqlite3, err := exec.LookPath("sqlite3")
	if err != nil {
		return
	}
	p := filepath.Join(t.TempDir(), "p.kitsp")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(sqlite3, p, "PRAGMA integrity_check; "+
		"SELECT value FROM system_variables WHERE variable = 'application-version'; "+
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index'; "+
		"INSERT INTO system_variables VALUES ('application-version', 'x'); SELECT COUNT(*) FROM system_variables;").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if got := string(out); got != "ok\n"+AppVersion+"\n9\n2\n" {
		t.Errorf("sqlite3 says %q", got)
	}
}

// TestFountain: Fountain in, a KIT project out, read back: all kept but
// the page breaks.
func TestFountain(t *testing.T) {
	for name, src := range formattest.Inputs(t) {
		t.Run(name, func(t *testing.T) {
			orig := formattest.Read(src)
			out := write(t, orig)
			back := parse(t, out)
			formattest.Same(t, name, kitRows(orig), kitRows(back))
			checkSQLite(t, out)
		})
	}
}

// kitRows are a script's rows as KIT keeps them: no page breaks, a
// line of notes alone a note, the title page's fields KIT has, in its
// order.
func kitRows(s lex.Screenplay) []string {
	var title, body []string
	inTitle := len(s) > 0 && s[0].Type == lex.TypeTitlePage
	order := map[string]int{}
	for i, k := range titleKeys {
		order[k.key] = i
	}
	for _, l := range s {
		if inTitle {
			if l.Type == lex.TypeNewPage {
				inTitle = false
				continue
			}
			if _, ok := order[string(l.Type)]; ok && l.Contents != "" {
				title = append(title, string(l.Type)+": "+lex.Unstyled(l.Contents))
			}
			continue
		}
		if l.Type == lex.TypeEmpty || l.Type == lex.TypeNewPage || l.Type == lex.TypeTitlePage {
			continue
		}
		text := l.Contents
		if l.Type == lex.TypeBoneyard {
			text = strings.Join(strings.Fields(text), " ")
		}
		body = append(body, string(l.Type)+": "+text)
	}
	// one value a key: the first, in KIT's order
	seen := map[string]bool{}
	var first []string
	for _, k := range titleKeys {
		for _, r := range title {
			if strings.HasPrefix(r, k.key+": ") && !seen[k.key] {
				seen[k.key] = true
				first = append(first, r)
			}
		}
	}
	return append(first, body...)
}
