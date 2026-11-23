package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/bundle"
)

// zipOf is a zip of name → data, in the order given (pairs).
func zipOf(t *testing.T, pairs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < len(pairs); i += 2 {
		w, err := zw.Create(pairs[i])
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, pairs[i+1])
	}
	zw.Close()
	return buf.Bytes()
}

// accoladeZip is a synthetic .accolade: Highland's layout, with Accolade's
// key, a draft, a source and a sidecar no one here knows.
func accoladeZip(t *testing.T, text string) []byte {
	return zipOf(t,
		"Barn.textbundle/info.json", `{"version":2,"type":"com.quoteunquoteapps.fountain","eu.kiefte.Accolade":{"version":1,"drafts":"assets/drafts/drafts.json"}}`,
		"Barn.textbundle/text.fountain", text,
		"Barn.textbundle/assets/drafts/drafts.json", `[{"name":"first"}]`,
		"Barn.textbundle/assets/source.md", "# The novel\n",
		"Barn.textbundle/characters.json", `{"ANNA":{}}`,
	)
}

func run(t *testing.T, o Options) {
	t.Helper()
	o.ConfigFile = filepath.Join(t.TempDir(), "none.toml")
	if err := Convert(context.Background(), o); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) (*bundle.Bundle, string) {
	t.Helper()
	b, err := bundle.Read(p)
	if err != nil {
		t.Fatal(err)
	}
	text, err := b.ReadText()
	if err != nil {
		t.Fatal(err)
	}
	return b, string(text)
}

// Every bundle type as output, from Fountain, in both forms: the text as
// it was, byte for byte; and back as input to every format.
func TestBundleOut(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	os.WriteFile(in, []byte(script), 0o644)
	for _, ty := range bundle.Types {
		for _, form := range []string{"", "zip", "folder"} {
			out := filepath.Join(dir, "barn-"+form+ty.Ext)
			run(t, Options{Input: in, Output: out, BundleForm: form})
			b, text := read(t, out)
			want := ty.Form
			if form != "" {
				want, _ = bundle.ParseForm(form)
			}
			if b.Form() != want || text != script || b.Markdown() {
				t.Errorf("%s %q: %s, %q", ty.Ext, form, b.Form(), text)
			}
			if r := bundle.Check(out); !r.OK() {
				t.Errorf("%s: %s", out, r)
			}
			html := filepath.Join(dir, "from-"+form+ty.Ext[1:]+".html")
			run(t, Options{Input: out, Output: html})
			if data, _ := os.ReadFile(html); !strings.Contains(string(data), "coming.") {
				t.Errorf("%s as input: %.200s", out, data)
			}
		}
	}
}

// A bundle rewritten keeps every file it does not write: from a bundle to
// another, and into one that is there.
func TestBundleKeepsTheRest(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "barn.accolade")
	os.WriteFile(src, accoladeZip(t, script), 0o644)
	keep := []string{"info.json", "assets/drafts/drafts.json", "assets/source.md", "characters.json"}
	same := func(what, p string) {
		t.Helper()
		a, _ := bundle.Read(src)
		b, text := read(t, p)
		for _, n := range keep {
			x, _ := a.ReadFile(n)
			y, err := b.ReadFile(n)
			if err != nil || !bytes.Equal(x, y) {
				t.Errorf("%s: %s %q, want %q (%v)", what, n, y, x, err)
			}
		}
		if what != "numbered" && text != script {
			t.Errorf("%s: text %q", what, text)
		}
	}
	// to a folder, a Highland and a TextPack
	for _, out := range []string{"barn-folder.accolade", "barn.highland", "barn.textpack", "barn.textbundle"} {
		run(t, Options{Input: src, Output: filepath.Join(dir, out), BundleForm: map[bool]string{true: "folder"}[strings.Contains(out, "folder")]})
		same(out, filepath.Join(dir, out))
	}
	// Fountain into the bundle that is there: only its text changes
	other := filepath.Join(dir, "other.fountain")
	os.WriteFile(other, []byte("INT. HOUSE - NIGHT\n\nDark.\n"), 0o644)
	run(t, Options{Input: other, Output: src})
	if _, text := read(t, src); text != "INT. HOUSE - NIGHT\n\nDark.\n" {
		t.Errorf("into the bundle: %q", text)
	}
	os.WriteFile(src, accoladeZip(t, script), 0o644)
	// a change (-number): the text written anew, the rest kept
	run(t, Options{Input: src, Output: filepath.Join(dir, "numbered.highland"), Number: "count"})
	same("numbered", filepath.Join(dir, "numbered.highland"))
	if _, text := read(t, filepath.Join(dir, "numbered.highland")); !strings.Contains(text, "#1#") {
		t.Errorf("numbered: %q", text)
	}
	// -keeploose without -o: back into the bundle itself
	o := Options{Input: src, Output: "", KeepLoose: true}
	run(t, o)
	same("keeploose", src)
}

// A Markdown bundle repacks, and is refused where a screenplay is needed.
func TestBundleMarkdown(t *testing.T) {
	dir := t.TempDir()
	tb := filepath.Join(dir, "novel.textbundle")
	if _, err := bundle.Write(nil, tb, bundle.Dir, bundle.New([]byte("# Novel\n"), true, ""), nil); err != nil {
		t.Fatal(err)
	}
	tp := filepath.Join(dir, "novel.textpack")
	run(t, Options{Input: tb, Output: tp})
	if b, text := read(t, tp); !b.Zipped || !b.Markdown() || text != "# Novel\n" {
		t.Errorf("repacked: %v %q", b.Zipped, text)
	}
	err := Convert(context.Background(), Options{Input: tp, Output: filepath.Join(dir, "x.pdf"), ConfigFile: "none.toml"})
	if err == nil || !strings.Contains(err.Error(), "Markdown") {
		t.Errorf("Markdown to PDF: %v", err)
	}
	err = Convert(context.Background(), Options{Input: filepath.Join(dir, "x.fountain"), Output: tp, ConfigFile: "none.toml"})
	if err == nil {
		t.Error("read a missing file")
	}
	os.WriteFile(filepath.Join(dir, "s.fountain"), []byte(script), 0o644)
	err = Convert(context.Background(), Options{Input: filepath.Join(dir, "s.fountain"), Output: tp, ConfigFile: "none.toml"})
	if err == nil || !strings.Contains(err.Error(), "Markdown") {
		t.Errorf("Fountain into a Markdown bundle: %v", err)
	}
}

// An .accolade with only its language's text (no text.fountain) reads it,
// in that language; a plain-text .highland reads as Fountain.
func TestBundleLanguageAndPlain(t *testing.T) {
	dir := t.TempDir()
	eo := filepath.Join(dir, "film.accolade")
	os.WriteFile(eo, zipOf(t, "info.json", `{"version":2}`, "text.eo.fountain", script), 0o644)
	out := filepath.Join(dir, "film.lex")
	run(t, Options{Input: eo, Output: out})
	if data, _ := os.ReadFile(out); !strings.Contains(string(data), "coming") {
		t.Errorf("the language's text: %.200s", data)
	}
	plain := filepath.Join(dir, "plain.highland")
	os.WriteFile(plain, []byte(script), 0o644)
	run(t, Options{Input: plain, Output: filepath.Join(dir, "plain.html")})
}

// Outputs a bundle cannot hold are refused, and nothing is written.
func TestBundleRefused(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	os.WriteFile(in, []byte(script), 0o644)
	for _, o := range []Options{
		{Input: in, Output: filepath.Join(dir, "a.highland"), To: "pdf"},
		{Input: in, Output: filepath.Join(dir, "a.highland"), Report: "scenes"},
		{Input: in, Output: filepath.Join(dir, "a.highland"), BundleForm: "tarball"},
	} {
		o.ConfigFile = "none.toml"
		if err := Convert(context.Background(), o); err == nil {
			t.Errorf("%+v: no error", o)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a.highland")); err == nil {
		t.Error("written all the same")
	}
	notBundle := filepath.Join(dir, "b.accolade")
	os.WriteFile(notBundle, []byte("mine"), 0o644)
	if err := Convert(context.Background(), Options{Input: in, Output: notBundle, ConfigFile: "none.toml"}); err == nil {
		t.Error("over a file that is no bundle")
	}
}

func TestCheckCommand(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.highland")
	os.WriteFile(good, accoladeZip(t, script), 0o644)
	bad := filepath.Join(dir, "bad.textpack")
	os.WriteFile(bad, zipOf(t, "info.json", "{}"), 0o644)
	for args, want := range map[string]int{
		"check " + good: 0, "check " + bad: 1, "check " + good + " " + bad: 1, "check": 2,
	} {
		if got := Main(context.Background(), strings.Fields(args)); got != want {
			t.Errorf("%s: %d, want %d", args, got, want)
		}
	}
}
