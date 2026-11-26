package osf

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/bundle"
	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// The same short script in each version, as Fade In writes them (written
// here by hand after the OSF SDK's samples): styles with their capitals,
// a numbered heading in lower case, emphasis, a note and a synopsis,
// dual dialogue, a centred line, a page break.

const v40 = `<?xml version="1.0" encoding="UTF-8"?>
<document type="Open Screenplay Format document" version="40">
  <info uuid="1" pagecount="1"/>
  <styles>
    <style name="Normal Text" builtin="1" builtin_index="0"/>
    <style name="Scene Heading" builtin="1" builtin_index="1" basestyle="Normal Text" allcaps="1"/>
    <style name="Action" builtin="1" builtin_index="2" basestyle="Normal Text"/>
    <style name="Character" builtin="1" builtin_index="3" basestyle="Normal Text" allcaps="1"/>
    <style name="Parenthetical" builtin="1" builtin_index="4" basestyle="Normal Text"/>
    <style name="Dialogue" builtin="1" builtin_index="5" basestyle="Normal Text"/>
    <style name="Transition" builtin="1" builtin_index="6" basestyle="Normal Text" allcaps="1"/>
    <style name="Shot" builtin="1" builtin_index="7" basestyle="Normal Text" allcaps="1"/>
    <style name="Insert" basestyle="Action" allcaps="1"/>
  </styles>
  <paragraphs>
    <para number="4" synopsis="They meet.">
      <style basestyle="Scene Heading"/>
      <text>Int. hall - day</text>
    </para>
    <para note="Check the light.">
      <style basestyle="Action"/>
      <text>John walks in, </text>
      <text italic="1">slowly</text>
      <text>, and </text>
      <text bold="1">stops</text>
      <text>.</text>
    </para>
    <para>
      <style basestyle="Character" dualdialogue="1"/>
      <text>John</text>
    </para>
    <para>
      <style basestyle="Parenthetical"/>
      <text>(quietly)</text>
    </para>
    <para>
      <style basestyle="Dialogue"/>
      <text>Hello </text>
      <text underline="1">there</text>
      <text>.
How are you?</text>
    </para>
    <para>
      <style basestyle="Character"/>
      <text>Mary</text>
    </para>
    <para>
      <style basestyle="Dialogue"/>
      <text>Hi &amp; bye.</text>
    </para>
    <para>
      <style basestyle="Insert"/>
      <text>a note on the door</text>
    </para>
    <para>
      <style basestyle="Action" align="center"/>
      <text>THE END</text>
    </para>
    <para>
      <style basestyle="Transition" pagebreakbefore="1"/>
      <text>Cut to:</text>
    </para>
  </paragraphs>
  <titlepage>
    <para bookmark="Title">
      <style basestyle="Normal Text" align="center"/>
      <text underline="1">Brick</text>
    </para>
    <para>
      <style basestyle="Normal Text" align="center"/>
      <text></text>
    </para>
    <para>
      <style basestyle="Normal Text" align="center"/>
      <text>Written by</text>
    </para>
    <para bookmark="Author">
      <style basestyle="Normal Text" align="center"/>
      <text>Jo Doe</text>
    </para>
    <para bookmark="Contact">
      <style basestyle="Normal Text"/>
      <text>Jo Doe
jo@example.com</text>
    </para>
  </titlepage>
</document>
`

// v20 is v40 in 2.0's words: basestylename, scene_number.
var v20 = strings.NewReplacer(`version="40"`, `version="20"`, `basestyle=`, `basestylename=`, `number="4"`, `scene_number="4"`).Replace(v40)

const v12 = `<?xml version="1.0" encoding="utf-8" ?>
<document type="Open Screenplay Format document" version="12">
<info title="&lt;u&gt;Brick&lt;/u&gt;" written_by="Written by&#10;Jo Doe" contact="Jo Doe&#10;jo@example.com" pagecount="1"/>
<styles><style builtin="1" builtin_index="0" name="Normal Text"/><style basestylename="Normal Text" builtin="1" builtin_index="1" effects="65537" name="Scene Heading"/><style basestylename="Normal Text" builtin="1" builtin_index="2" effects="131072" name="Action"/><style basestylename="Normal Text" builtin="1" builtin_index="3" effects="262145" name="Character"/><style basestylename="Normal Text" builtin="1" builtin_index="4" effects="524288" name="Parenthetical"/><style basestylename="Normal Text" builtin="1" builtin_index="5" effects="1048576" name="Dialogue"/><style basestylename="Normal Text" builtin="1" builtin_index="6" effects="2097153" name="Transition"/><style basestylename="Action" effects="1" name="Insert"/></styles>
<paragraphs>
<para scene_number="4" synopsis="They meet."><style basestylename="Scene Heading"/><text>Int. hall - day</text></para>
<para note="Check the light."><style basestylename="Action"/><text>John walks in, &lt;i&gt;slowly&lt;/i&gt;, and &lt;b&gt;stops&lt;/b&gt;.</text></para>
<para><style basestylename="Character" dualdialogue="1"/><text>John</text></para>
<para><style basestylename="Parenthetical"/><text>(quietly)</text></para>
<para><style basestylename="Dialogue"/><text>Hello &lt;u&gt;there&lt;/u&gt;.&lt;br&gt;How are you?</text></para>
<para><style basestylename="Character"/><text>Mary</text></para>
<para><style basestylename="Dialogue"/><text>&lt;font="Times"&gt;Hi&lt;/font&gt; &amp;amp; bye.</text></para>
<para><style basestylename="Insert"/><text>a note on the door</text></para>
<para><style basestylename="Action" align="center"/><text>THE END</text></para>
<para><style basestylename="Transition" pagebreakbefore="1"/><text>Cut to:</text></para>
</paragraphs>
</document>
`

// want is the script each of them is.
var want = []string{
	"titlepage: ",
	"Title: _Brick_",
	"Credit: Written by",
	"Author: Jo Doe",
	"Contact: Jo Doe",
	"Contact: jo@example.com",
	"newpage: ",
	"scene: INT. HALL - DAY #4#",
	"synopse: They meet.",
	"action: John walks in, *slowly*, and **stops**. [[Check the light.]]",
	"dualspeaker_open: ",
	"speaker: JOHN",
	"paren: (quietly)",
	"dialog: Hello _there_.",
	"dialog: How are you?",
	"dualspeaker_next: ",
	"speaker: MARY",
	"dialog: Hi & bye.",
	"dualspeaker_close: ",
	"action: A NOTE ON THE DOOR",
	"center: THE END",
	"newpage: ",
	"trans: CUT TO:",
}

func parse(t *testing.T, doc string) lex.Screenplay {
	t.Helper()
	s, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestReadVersions(t *testing.T) {
	for name, doc := range map[string]string{"4.0": v40, "2.0": v20, "1.2": v12} {
		t.Run(name, func(t *testing.T) {
			formattest.Same(t, name, want, formattest.Rows(parse(t, doc), nil))
		})
	}
}

// TestRoundTrip: each version read, written in each version and read
// back is the same script.
func TestRoundTrip(t *testing.T) {
	for name, doc := range map[string]string{"4.0": v40, "2.0": v20, "1.2": v12} {
		first := parse(t, doc)
		for _, v := range []string{"4.0", "2.0", "1.2"} {
			out := (&OSFWriter{Version: v}).Document(first)
			back := parse(t, string(out))
			wantRows := formattest.Rows(first, nil)
			formattest.Same(t, name+" as "+v, wantRows, formattest.Rows(back, nil))
			if got := Version(attrOf(out, "version")); got != map[string]int{"4.0": 40, "2.0": 20, "1.2": 12}[v] {
				t.Errorf("%s written as %s has version %d", name, v, got)
			}
		}
	}
}

func attrOf(doc []byte, name string) string {
	_, rest, _ := strings.Cut(string(doc), "<document ")
	_, rest, _ = strings.Cut(rest, name+`="`)
	v, _, _ := strings.Cut(rest, `"`)
	return v
}

// TestFountain: Fountain in, OSF out, read back: every printed element
// kept; a section and a synopsis not on a heading are notes, a boneyard
// left out.
func TestFountain(t *testing.T) {
	for name, src := range formattest.Inputs(t) {
		t.Run(name, func(t *testing.T) {
			orig := formattest.Read(src)
			back := parse(t, string((&OSFWriter{}).Document(orig)))
			keep := func(l lex.Line) bool {
				switch l.Type {
				case lex.TypeEmpty, lex.TypeSection, lex.TypeSynopsis, lex.TypeNote, lex.TypeBoneyard:
					return false
				}
				return true
			}
			strip := func(rows []string) []string {
				for i, r := range rows {
					rows[i] = strings.TrimSpace(lex.StripNotes(r))
				}
				return rows
			}
			formattest.Same(t, name, strip(formattest.Rows(orig, keep)), strip(formattest.Rows(back, keep)))
		})
	}
}

func TestScriptNotesAndOutline(t *testing.T) {
	orig := formattest.Read(formattest.Script)
	back := formattest.Rows(parse(t, string((&OSFWriter{}).Document(orig))), nil)
	got := strings.Join(back, "\n")
	for _, w := range []string{
		"scene: INT. HALL - DAY #1# [[# Act One\n= The opening.]]",
		"synopse: A synopsis.",
		"action: John walks in, *slowly*, and **stops**. [[a note]]",
		"speaker: JOHN [[a note of its own]]",
		"lyrics: la la la\nlyrics: and on",
		`action: Rain falls on "quotes" & <brackets>.`,
		"Title: _**BRICK & STEEL**_",
		"Source: Based on a story",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("no %q in\n%s", w, got)
		}
	}
}

func TestFadeIn(t *testing.T) {
	dir := t.TempDir()
	doc := []byte(v40)
	zipped := filepath.Join(dir, "a.fadein")
	if err := WriteFadeIn(zipped, bundle.Auto, doc); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(zipped); err != nil || st.IsDir() {
		t.Fatalf("a.fadein is no file: %v", err)
	}
	// an entry of Fade In's own is kept on a rewrite
	data, _ := os.ReadFile(zipped)
	withExtra := addEntry(t, data, "thumbnail.png", "PNG")
	if err := os.WriteFile(zipped, withExtra, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFadeIn(zipped, bundle.Auto, []byte(v20)); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFadeIn(zipped)
	if err != nil || string(got) != v20 {
		t.Fatalf("read back %q, %v", got, err)
	}
	data, _ = os.ReadFile(zipped)
	if !bytes.Contains(data, []byte("thumbnail.png")) {
		t.Error("the other entry is gone")
	}
	// to a folder and back
	if err := WriteFadeIn(zipped, bundle.Dir, doc); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(zipped, "thumbnail.png")); err != nil {
		t.Errorf("the folder lost the other entry: %v", err)
	}
	if got, err := ReadFadeIn(zipped); err != nil || string(got) != v40 {
		t.Fatalf("folder read back %q, %v", got, err)
	}
	if err := WriteFadeIn(zipped, bundle.Zip, []byte(v12)); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadFadeIn(zipped); err != nil || string(got) != v12 {
		t.Fatalf("zip again read back %q, %v", got, err)
	}
	data, _ = os.ReadFile(zipped)
	if !bytes.Contains(data, []byte("thumbnail.png")) {
		t.Error("the zip from the folder lost the other entry")
	}
	if _, err := FadeInDocument([]byte("not a zip")); err == nil {
		t.Error("no error for a file that is no zip")
	}
}

// addEntry is a zip with one more entry.
func addEntry(t *testing.T, data []byte, name, body string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if err := zw.Copy(f); err != nil {
			t.Fatal(err)
		}
	}
	w, _ := zw.Create(name)
	w.Write([]byte(body))
	zw.Close()
	return buf.Bytes()
}
