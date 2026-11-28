package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// The formats of other programs, through the command line: written from
// Fountain, read back by their names and by their content alone.
func TestOtherProgramsFormats(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "script.fountain")
	if err := os.WriteFile(src, []byte(formattest.Script), 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := ReadScreenplay(context.Background(), src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"fadein", "osf", "trelby", "kitsp", "fdx", "celtx"} {
		t.Run(ext, func(t *testing.T) {
			out := filepath.Join(dir, "script."+ext)
			if code := Main(context.Background(), []string{"-i", src, "-o", out}); code != 0 {
				t.Fatalf("-o %s: exit %d", ext, code)
			}
			back, err := ReadScreenplay(context.Background(), out, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if speakers(back) != speakers(want) || headings(back) != headings(want) {
				t.Errorf("read back from %s:\n%s", ext, formattest.Dump(back))
			}
			// by its content, under a name that says nothing
			data, _ := os.ReadFile(out)
			bare := filepath.Join(dir, "bare-"+ext)
			if err := os.WriteFile(bare, data, 0o644); err != nil {
				t.Fatal(err)
			}
			again, err := ReadScreenplay(context.Background(), bare, Options{})
			if err != nil {
				t.Fatal(err)
			}
			formattest.Same(t, ext+" by content", formattest.Rows(back, nil), formattest.Rows(again, nil))
			// and through -i, by its content alone
			fromBare := filepath.Join(dir, "bare-"+ext+".fountain")
			if code := Main(context.Background(), []string{"-i", bare, "-o", fromBare}); code != 0 {
				t.Fatalf("-i %s by content: exit %d", ext, code)
			}
			viaI, err := ReadScreenplay(context.Background(), fromBare, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if speakers(viaI) != speakers(want) || headings(viaI) != headings(want) {
				t.Errorf("-i %s by content:\n%s", ext, formattest.Dump(viaI))
			}
		})
	}
}

func speakers(s lex.Screenplay) string {
	var out []string
	for _, l := range s {
		if l.Type == lex.TypeSpeaker {
			out = append(out, lex.StripNotes(l.Contents))
		}
	}
	return strings.Join(out, "|")
}

func headings(s lex.Screenplay) string {
	var out []string
	for _, l := range s {
		if l.Type == lex.TypeScene {
			h, _ := lex.SceneNumber(l.Contents)
			out = append(out, h)
		}
	}
	return strings.Join(out, "|")
}

// TestFadeInFolder: a .fadein is written as a folder with -form folder,
// and read back from it with -i.
func TestFadeInFolder(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.fountain")
	if err := os.WriteFile(src, []byte(formattest.Script), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "a.fadein")
	if code := Main(context.Background(), []string{"-osfversion", "2.0", "-i", src, "-o", out, "-form", "folder"}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	doc, err := os.ReadFile(filepath.Join(out, "document.xml"))
	if err != nil {
		t.Fatalf("no folder: %v", err)
	}
	if !strings.Contains(string(doc), `version="20"`) {
		t.Errorf("not OSF 2.0: %.200s", doc)
	}
	back := filepath.Join(dir, "back.fountain")
	if code := Main(context.Background(), []string{"-i", out, "-o", back}); code != 0 {
		t.Fatalf("back: exit %d", code)
	}
	text, _ := os.ReadFile(back)
	if !strings.Contains(string(text), "MARY ^") || !strings.Contains(string(text), "INT. HALL - DAY #1#") {
		t.Errorf("back from the folder:\n%s", text)
	}
}

// TestPlainFiles: IN [OUT] without -i and -o, the flags anywhere; mixed
// with -i or -o, or more than two, an error.
func TestPlainFiles(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.fountain")
	if err := os.WriteFile(src, []byte(formattest.Script), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "a.trelby")
	if code := Main(context.Background(), []string{src, "-e", "default", out}); code != 0 {
		t.Fatalf("exit %d", code)
	}
	bare := filepath.Join(dir, "bare")
	data, _ := os.ReadFile(out)
	if err := os.WriteFile(bare, data, 0o644); err != nil {
		t.Fatal(err)
	}
	back := filepath.Join(dir, "back.txt")
	if code := Main(context.Background(), []string{"-to", "fountain", bare, back}); code != 0 {
		t.Fatalf("back: exit %d", code)
	}
	if text, _ := os.ReadFile(back); !strings.Contains(string(text), "MARY") {
		t.Errorf("back by content:\n%s", text)
	}
	o, ok, err := parseFlags([]string{src})
	if err != nil || !ok || o.Input != src || o.Output != "-" {
		t.Errorf("one file: %+v %v %v", o.Input+" "+o.Output, ok, err)
	}
	for _, args := range [][]string{{"-i", src, out}, {src, "-o", out}, {src, out, back}} {
		if _, _, err := parseFlags(args); err == nil {
			t.Errorf("%q: no error", args)
		}
	}
}

func TestSniff(t *testing.T) {
	for want, head := range map[string]string{
		"kitsp":  "SQLite format 3\x00rest",
		"pdf":    "%PDF-1.7",
		"trelby": "\ufeff#Version 3\n",
		"fdx":    `<?xml version="1.0"?><FinalDraft DocumentType="Script">`,
		"osf":    `<?xml version="1.0"?>` + "\n" + `<document type="Open Screenplay Format document" version="40">`,
		"":       "INT. HALL - DAY\n\nJohn walks.",
	} {
		if got := Sniff([]byte(head)); got != want {
			t.Errorf("Sniff(%.20q) = %q, want %q", head, got, want)
		}
	}
}

func TestInputs(t *testing.T) {
	for name, key := range map[string]string{"a.fadein": "fadein", "a.FadeIn/": "fadein", "b.kitsp": "kitsp", "c.trelby": "trelby", "d.osf": "osf", "e.fdx": "fdx", "f.celtx": "celtx"} {
		if f, ok := InputFor(name); !ok || f.Key != key {
			t.Errorf("InputFor(%q) = %v, %v", name, f, ok)
		}
	}
	for _, f := range Inputs {
		if f.Key == "fountain" || f.Key == "pdf" || f.Key == "lex" {
			continue
		}
		if _, ok := FormatFor(f.Key); !ok {
			t.Errorf("%s is read but not written", f.Key)
		}
	}
}

// TestCeltxScript: -script picks a Celtx project's other script, and a
// project is known by its content.
func TestCeltxScript(t *testing.T) {
	const rdf = `<?xml version="1.0"?>
<RDF:RDF xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:cx="http://celtx.com/NS/v1/" xmlns:RDF="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <cx:Document RDF:about="http://celtx.com/res/a" dc:title="Film" cx:localFile="script-a.html"/>
  <cx:Document RDF:about="http://celtx.com/res/b" dc:title="Stage" cx:localFile="script-b.html"/>
</RDF:RDF>
`
	page := func(heading string) string {
		return "<html><head><title></title></head><body>\n<p class=\"sceneheading\">" + heading + "</p>\n</body></html>\n"
	}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range map[string]string{"project.rdf": rdf, "script-a.html": page("INT. FILM - DAY"), "script-b.html": page("INT. STAGE - NIGHT")} {
		w, _ := z.Create(name)
		w.Write([]byte(data))
	}
	z.Close()
	if Sniff(buf.Bytes()) != "celtx" {
		t.Errorf("a Celtx project not sniffed as one")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "two.celtx")
	if err := os.WriteFile(in, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for pick, want := range map[string]string{"": "INT. FILM - DAY", "2": "INT. STAGE - NIGHT", "Stage": "INT. STAGE - NIGHT"} {
		out := filepath.Join(dir, "out"+pick+".fountain")
		args := []string{in, out}
		if pick != "" {
			args = append(args, "-script", pick)
		}
		if code := Main(context.Background(), args); code != 0 {
			t.Fatalf("%q: exit %d", args, code)
		}
		if data, _ := os.ReadFile(out); !strings.Contains(string(data), want) {
			t.Errorf("-script %q: %s", pick, data)
		}
	}
	if code := Main(context.Background(), []string{in, filepath.Join(dir, "x.fountain"), "-script", "Radio"}); code == 0 {
		t.Error("-script of no script: exit 0")
	}
}
