package celtx

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// The test project is hand-made after what a project Celtx saved shows:
// a project with its tree (a folder in it), two scripts, a scene's
// index card and breakdown items.
const testRDF = `<?xml version="1.0"?>
<RDF:RDF xmlns:NS1="http://celtx.com/project/abc/NS/"
         xmlns:dc="http://purl.org/dc/elements/1.1/"
         xmlns:cx="http://celtx.com/NS/v1/"
         xmlns:RDF="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <cx:Document RDF:about="http://celtx.com/res/stage"
                   cx:localFile="script-b.html"
                   dc:title="Stage Version">
    <cx:doctype RDF:resource="http://celtx.com/NS/v1/TheatreDocument"/>
  </cx:Document>
  <cx:Document RDF:about="http://celtx.com/res/film"
                   dc:title="Film Version"
                   cx:localFile="script-a.html">
    <cx:doctype RDF:resource="http://celtx.com/NS/v1/ScriptDocument"/>
    <cx:scenes RDF:resource="rdf:#$s1"/>
  </cx:Document>
  <RDF:Seq RDF:about="rdf:#$s1">
    <RDF:li RDF:resource="http://celtx.com/res/sc1"/>
  </RDF:Seq>
  <RDF:Description RDF:about="http://celtx.com/res/sc1"
                   cx:sceneid="h1"
                   dc:description="Ann finds the key."/>
  <cx:Cast RDF:about="http://celtx.com/res/ann" dc:title="ANN"/>
  <cx:Props RDF:about="http://celtx.com/res/key">
    <dc:title>Brass Key</dc:title>
  </cx:Props>
  <RDF:Seq RDF:about="http://celtx.com/res/root">
    <RDF:li RDF:resource="http://celtx.com/res/folder"/>
    <RDF:li RDF:resource="http://celtx.com/res/stage"/>
  </RDF:Seq>
  <RDF:Seq RDF:about="http://celtx.com/res/folder">
    <RDF:li RDF:resource="http://celtx.com/res/film"/>
  </RDF:Seq>
  <cx:Folder RDF:about="http://celtx.com/res/folder" dc:title="Drafts"/>
  <RDF:Description RDF:about="http://celtx.com/res/root" cx:projectRoot="true" dc:title="Keys"/>
  <RDF:Seq RDF:about="http://celtx.com/res/comp">
    <RDF:li RDF:resource="http://celtx.com/res/root"/>
  </RDF:Seq>
  <cx:Project RDF:about="http://celtx.com/project/abc" cx:fileVersion="1.4" dc:title="Keys">
    <cx:components RDF:resource="http://celtx.com/res/comp"/>
  </cx:Project>
</RDF:RDF>
`

const testHead = `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">
<html>
<head>
  <title>The Key</title>
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
  <style id="charnumbers" type="text/css">
.character:before { display: none !important; }
  </style>
  <meta content="Ann Writer" name="Author">
  <meta content="Written by" name="CX.byline">
  <meta content="ann@example.com
Somewhere" name="CX.contact">
</head>
<body>
`

const testScript = testHead + `<p scenestr="1" scenenumber="1" id="h1" class="sceneheading">int. attic - day</p>
<p class="action"><span text="Look up.
Then down." class="note" id="n1" date="January 1, 2010 1:00:00 PM"></span><span ref="http://celtx.com/res/ann" class="cast">ANN</span> opens a box: a <b>brass</b>
<span ref="http://celtx.com/res/key" class="props">key</span>, <i>old</i> and <u>bent</u>.&nbsp; She
smiles.</p>
<p class="character">ann</p>
<p class="parenthetical">(softly)</p>
<p class="dialog">There you are.<br>
At last.<br>
</p>
<p class="character">BOB ^</p>
<p class="dialog">Found it?</p>
<p class="character">TOM<span mediares="http://celtx.com/res/m" id="m1" class="media"></span></p>
<p class="dialog">~one more song</p>
<p class="action">&gt; THE END &lt;</p>
<p class=""><span text="## A sequence" class="note" id="n2"></span></p>
<p class="shot">close on the key</p>
<p class="transition">cut to:</p>
<p class="action"><br>
</p>
</body>
</html>
`

const testStage = testHead + `<p id="a1" class="act">ACT I<br>
</p>
<p id="h2" class="sceneheading">A ROOM</p>
<p class="action">Lights up.</p>
</body>
</html>
`

// project is a zip of files.
func project(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w, err := z.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(files[n]))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testProject(t *testing.T) []byte {
	return project(t, map[string]string{ManifestName: testRDF, "script-a.html": testScript, "script-b.html": testStage})
}

func TestScripts(t *testing.T) {
	data := testProject(t)
	if !IsProject(data) {
		t.Fatal("not seen as a project")
	}
	p, err := ReadProject(data)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range p.Scripts {
		got = append(got, s.Title+" "+s.Kind+" "+s.File)
	}
	want := []string{"Film Version Script script-a.html", "Stage Version Theatre script-b.html"} // the tree's order
	formattest.Same(t, "scripts", want, got)
	for pick, title := range map[string]string{"": "Film Version", "2": "Stage Version", "stage version": "Stage Version", "script-a.html": "Film Version"} {
		s, err := p.Script(pick)
		if err != nil || s.Title != title {
			t.Errorf("Script(%q) = %q, %v; want %q", pick, s.Title, err, title)
		}
	}
	for _, pick := range []string{"3", "0", "Radio"} {
		if _, err := p.Script(pick); err == nil {
			t.Errorf("Script(%q): no error", pick)
		}
	}
}

func TestRead(t *testing.T) {
	s, set, tags, err := ParseTags(testProject(t), "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"titlepage: ",
		"Title: The Key",
		"Credit: Written by",
		"Author: Ann Writer",
		"Contact: ann@example.com",
		"Contact: Somewhere",
		"newpage: ",
		"scene: INT. ATTIC - DAY",
		"synopse: Ann finds the key.",
		"action: ANN opens a box: a **brass** key, *old* and _bent_.  She smiles. [[Look up.\nThen down.]]",
		"speaker: ANN",
		"paren: (softly)",
		"dialog: There you are.",
		"dialog: At last.",
		"dualspeaker_next: ",
		"speaker: BOB",
		"dialog: Found it?",
		"dualspeaker_close: ",
		"speaker: TOM",
		"lyrics: one more song",
		"center: THE END",
		"section: ## A sequence",
		"action: CLOSE ON THE KEY",
		"trans: CUT TO:",
	}
	rows := formattest.Rows(s, nil)
	// the first speech opens dual dialogue
	formattest.Same(t, "rows", want, without(rows, "dualspeaker_open: "))
	if !strings.Contains(strings.Join(rows, "\n"), "dualspeaker_open: \nspeaker: ANN") {
		t.Errorf("no dual dialogue opened before ANN:\n%s", strings.Join(rows, "\n"))
	}
	if set == nil || len(set.Elements) != 2 {
		t.Fatalf("elements: %+v", set)
	}
	names := map[string]string{}
	for _, e := range set.Elements {
		c, _ := set.Category(e.Category)
		names[e.Name] = c.Name
	}
	if names["ANN"] != lex.CastCategory || names["Brass Key"] != "Props" {
		t.Errorf("elements and categories: %v", names)
	}
	var placed []string
	for row, ts := range tags {
		for _, tg := range ts {
			plain := lex.TagPlain(s[row].Contents, s[row].Type == lex.TypeScene)
			if lex.TagIndex(plain, tg.Text, tg.At) < 0 {
				t.Errorf("tag %q not in row %d %q", tg.Text, row, plain)
			}
			placed = append(placed, tg.Text)
		}
	}
	sort.Strings(placed)
	formattest.Same(t, "tags", []string{"ANN", "key"}, placed)

	stage, err := ParseScript(testProject(t), "2")
	if err != nil {
		t.Fatal(err)
	}
	formattest.Same(t, "stage", []string{"section: # ACT I", "scene: A ROOM", "action: Lights up."},
		formattest.Rows(stage, func(l lex.Line) bool { return lex.IsBodyType(l.Type) && l.Type != lex.TypeTitlePage }))
}

func without(rows []string, drop string) []string {
	var out []string
	for _, r := range rows {
		if r != drop {
			out = append(out, r)
		}
	}
	return out
}

func write(t *testing.T, w *CeltxWriter, s lex.Screenplay) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := w.Write(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// checkProject checks what the project's catalogue promises: each
// resource it names is a subject of it (or a department or a type of
// Celtx's), each file is in the zip, each scene's id a heading's in its
// script.
func checkProject(t *testing.T, data []byte) {
	t.Helper()
	p, err := ReadProject(data)
	if err != nil {
		t.Fatal(err)
	}
	for res, n := range p.g {
		for k, vs := range n.props {
			for _, v := range vs {
				switch {
				case k == nsCX+" localFile" || k == nsCX+" auxFile" || k == nsCX+" thumbnail":
					if _, ok := p.files[v]; !ok {
						t.Errorf("%s: %s is not in the zip", res, v)
					}
				case strings.HasPrefix(v, nsCX) || strings.HasPrefix(v, "celtx:") || !strings.Contains(v, ":"):
				case strings.HasPrefix(v, "http://celtx.com/res/") || strings.HasPrefix(v, "rdf:#"):
					if _, ok := p.g[v]; !ok && k != nsDC+" source" {
						t.Errorf("%s: %s names %s, which the catalogue lacks", res, k, v)
					}
				}
			}
		}
		for _, it := range n.items {
			if _, ok := p.g[it]; !ok && strings.Contains(it, ":") && !strings.HasPrefix(it, nsCX) {
				t.Errorf("%s holds %s, which the catalogue lacks", res, it)
			}
		}
	}
	for _, sc := range p.Scripts {
		doc, err := parseHTML(mustRead(t, p, sc.File))
		if err != nil {
			t.Fatal(err)
		}
		ids := map[string]bool{}
		for _, cp := range doc.paras {
			ids[cp.id] = true
		}
		for id := range p.sceneCards(sc) {
			if !ids[id] {
				t.Errorf("%s: a scene's id %s is no paragraph's", sc.File, id)
			}
		}
	}
}

func mustRead(t *testing.T, p *Project, name string) []byte {
	t.Helper()
	data, err := readFile(p.files[name])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestFountain: Fountain in, Celtx out, read back: what Celtx holds is
// kept (the text with its emphasis and notes, the outline, dual dialogue,
// lyrics and centred lines as Fountain's markers, the title page's
// fields Celtx has).
func TestFountain(t *testing.T) {
	for name, src := range formattest.Inputs(t) {
		t.Run(name, func(t *testing.T) {
			orig := formattest.Read(src)
			out := write(t, &CeltxWriter{}, orig)
			checkProject(t, out)
			back, err := Parse(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			formattest.Same(t, name, celtxRows(orig), celtxRows(back))
		})
	}
}

// celtxRows are a script's rows as Celtx keeps them: no scene numbers,
// page breaks or boneyard, the title page's fields Celtx has, plain and
// in any order.
func celtxRows(s lex.Screenplay) []string {
	var title, body []string
	inTitle := len(s) > 0 && s[0].Type == lex.TypeTitlePage
	for _, l := range s {
		if inTitle {
			if l.Type == lex.TypeNewPage {
				inTitle = false
			} else if l.Contents != "" && (strings.EqualFold(l.Type, "Title") || titleMeta(l.Type) != "") {
				title = append(title, string(l.Type)+": "+lex.Unstyled(l.Contents))
			}
			continue
		}
		switch l.Type {
		case lex.TypeEmpty, lex.TypeNewPage, lex.TypeBoneyard, lex.TypeTitlePage:
			continue
		}
		text := l.Contents
		if l.Type == lex.TypeScene {
			text, _ = lex.SceneNumber(text)
			text += strings.Join(lex.Notes(l.Contents), "")
		}
		body = append(body, string(l.Type)+": "+strings.TrimSpace(text))
	}
	sort.Strings(title)
	return append(title, body...)
}

// TestTags: the breakdown's Cast, Props and Sound come back on the same
// words; another category's words are plain.
func TestTags(t *testing.T) {
	src := "INT. ATTIC - DAY\n\nANN opens a box: a brass key and a key ring. A bell rings.\n\nANN\nThe key! The key.\n"
	rows := fountain.ParseLines(formattest.Scenes, strings.NewReader(src))
	set := &lex.TagSet{Elements: []lex.TagElement{
		{ID: "e1", Category: "01fc9642-84ff-4366-b37c-a3068dee57e8", Name: "Ann"},
		{ID: "e2", Category: "05c556eb-6bc1-4a3a-b09f-f8b5ba1b6afa", Name: "Brass key"},
		{ID: "e3", Category: "0ce7d308-096d-4603-8fe8-349f72cd89ff", Name: "Bell"},
		{ID: "e4", Category: "0726fa85-1e65-4ab8-87de-bf21d09b01f0", Name: "Ring"},
	}}
	source := func(text string) int {
		for _, r := range rows {
			if strings.HasPrefix(r.Contents, text) {
				return r.Source
			}
		}
		t.Fatalf("no row %q", text)
		return 0
	}
	line := map[int][]lex.LineTag{
		source("ANN opens"): {{Text: "ANN", Elements: []string{"e1"}}, {Text: "key", At: 1, Elements: []string{"e2"}},
			{Text: "bell", Elements: []string{"e3"}}, {Text: "ring", Elements: []string{"e4"}}},
		source("The key!"): {{Text: "key", At: 1, Elements: []string{"e2"}}},
	}
	out := write(t, &CeltxWriter{Tags: set, LineTags: line, Now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}, rows)
	checkProject(t, out)
	p, _ := ReadProject(out)
	html := string(mustRead(t, p, scriptFile))
	for _, w := range []string{`class="cast">ANN</span>`, `a brass key and a <span ref=`, `class="props">key</span> ring`, `class="sound">bell</span>`, `The key! The <span ref=`} {
		if !strings.Contains(html, w) {
			t.Errorf("no %q in\n%s", w, html)
		}
	}
	if strings.Contains(html, `>ring<`) {
		t.Errorf("a wardrobe tag written:\n%s", html)
	}
	s, back, byRow, err := ParseTags(out, "")
	if err != nil {
		t.Fatal(err)
	}
	if back == nil || len(back.Elements) != 3 {
		t.Fatalf("elements back: %+v", back)
	}
	var got []string
	for row, ts := range byRow {
		for _, tg := range ts {
			plain := lex.TagPlain(s[row].Contents, false)
			i := lex.TagIndex(plain, tg.Text, tg.At)
			got = append(got, s[row].Type+": "+plain[:max(i, 0)]+"["+tg.Text+"]")
		}
	}
	sort.Strings(got)
	formattest.Same(t, "tags back", []string{
		"action: ANN opens a box: a brass key and a [key]",
		"action: ANN opens a box: a brass key and a key ring. A [bell]",
		"action: [ANN]",
		"dialog: The key! The [key]",
	}, got)
	man := string(mustRead(t, p, ManifestName))
	for _, w := range []string{`<cx:Cast RDF:about=`, `dc:title="Brass key"`, `NS/v1/Sound"/>`, `cx:sceneid="`, `dc:title="Master Catalog"`} {
		if !strings.Contains(man, w) {
			t.Errorf("no %q in the manifest", w)
		}
	}
}

// TestStored: the files are stored with their sizes in their headers,
// as Celtx's own.
func TestStored(t *testing.T) {
	out := write(t, &CeltxWriter{}, formattest.Read(formattest.Script))
	z, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		if f.Method != zip.Store || f.Flags&0x8 != 0 {
			t.Errorf("%s: method %d, flags %#x", f.Name, f.Method, f.Flags)
		}
	}
	if !IsProject(out) || IsProject([]byte("PK\x03\x04junk")) {
		t.Error("IsProject")
	}
}

// TestOz reads the project Celtx came with, The Wonderful Wizard of Oz,
// where it is (it is Celtx's, and not in this repository).
func TestOz(t *testing.T) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Projects", ".celtx-samples", "Wonderful_Wizard_of_Oz_Film.celtx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("no Celtx sample at", path)
	}
	checkProject(t, data)
	p, err := ReadProject(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Scripts) != 2 || p.Scripts[0].Kind != "Script" || p.Scripts[1].Kind != "Theatre" {
		t.Fatalf("scripts: %+v", p.Scripts)
	}
	for i := range p.Scripts {
		s, set, tags, err := p.Read(p.Scripts[i])
		if err != nil {
			t.Fatal(err)
		}
		count := map[lex.ElementType]int{}
		for _, l := range s {
			count[l.Type]++
		}
		if count[lex.TypeScene] != 4 || count[lex.TypeSpeaker] != 41 || count[lex.TypeParen] != 5 || count[lex.TypeSynopsis] == 0 {
			t.Errorf("%s: %v", p.Scripts[i].Title, count)
		}
		if set == nil || len(set.Elements) < 5 || len(tags) == 0 {
			t.Errorf("%s: tags %+v %v", p.Scripts[i].Title, set, tags)
		}
		// and through the writer again
		out := write(t, &CeltxWriter{}, s)
		checkProject(t, out)
		back, err := Parse(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		formattest.Same(t, p.Scripts[i].Title, celtxRows(s), celtxRows(back))
	}
}
