package bundle

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type zipItem struct {
	name, data string
	stored     bool
}

func makeZip(t *testing.T, items []zipItem) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	when := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	for _, it := range items {
		h := &zip.FileHeader{Name: it.name, Method: zip.Deflate, Modified: when}
		if it.stored || strings.HasSuffix(it.name, "/") {
			h.Method = zip.Store
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, it.data)
	}
	zw.SetComment("made for a test")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// highlandInfo is an info.json as Highland writes one (a teardown of
// Highland 2's own samples: Accolade's docs/HIGHLAND.md).
const highlandInfo = `{
  "com.quoteunquoteapps.highland2" : { "lastScrollPosition": 190.5, "cursorPosition": 0, "templateName": "Screenplay" },
  "transient" : false,
  "creatorURL" : "file:\/\/\/Applications\/Highland 2.app\/",
  "type" : "com.quoteunquoteapps.fountain",
  "version" : 2
}`

const script = "INT. KITCHEN - DAY\n\nBram cooks.\n"

var png = string([]byte{0x89, 'P', 'N', 'G', 0, 1, 2, 3, 0xff, 0xfe})

// highlandItems are a synthetic .highland as Highland's documented layout
// has it: one Name.textbundle/ folder (prefix; "" for files at the root)
// with text.fountain, info.json, Highland's sidecars, assets/, a file no
// one knows, and macOS's junk beside it.
func highlandItems(prefix string) []zipItem {
	var items []zipItem
	if prefix != "" {
		items = append(items, zipItem{name: prefix})
	}
	items = append(items,
		zipItem{name: prefix + "info.json", data: highlandInfo},
		zipItem{name: prefix + "characters.json", data: `{"BRAM":{"color":"#ff0000"}}`},
		zipItem{name: prefix + "text.fountain", data: script},
		zipItem{name: prefix + "assets/"},
		zipItem{name: prefix + "assets/cover.png", data: png, stored: true},
		zipItem{name: prefix + "unknown.bin", data: "\x00\x01 keep me"},
		zipItem{name: prefix + "sprints.json", data: "[]"},
	)
	if prefix != "" {
		items = append(items, zipItem{name: "__MACOSX/" + prefix + "._text.fountain", data: "junk"})
	}
	return items
}

// contents are a bundle's files by name relative to it, junk left out.
func contents(t *testing.T, b *Bundle) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, n := range b.Files {
		if IsJunk(n) {
			continue
		}
		data, err := b.ReadFile(n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		m[n] = string(data)
	}
	return m
}

func hasDir(t *testing.T, b *Bundle, name string) bool {
	t.Helper()
	es, err := b.Entries()
	if err != nil {
		t.Fatal(err)
	}
	return slices.ContainsFunc(es, func(e Entry) bool {
		return strings.TrimPrefix(e.Name, b.Prefix) == name
	})
}

func equal(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %d files, want %d: %v", what, len(got), len(want), keys(got))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %s = %q, want %q", what, k, got[k], v)
		}
	}
}

func keys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

// Every type goes zip → folder → zip with every file kept (empty assets/
// too), the layout and the text as they were, and the text rewritten.
func TestRoundTrips(t *testing.T) {
	for _, ty := range Types {
		for _, prefix := range []string{"", "Film.textbundle/"} {
			t.Run(ty.Name+"/"+prefix, func(t *testing.T) {
				dir := t.TempDir()
				zp := filepath.Join(dir, "Film"+ty.Ext)
				items := highlandItems(prefix)
				items = append(items, zipItem{name: prefix + "empty/"})
				if err := os.WriteFile(zp, makeZip(t, items), 0o644); err != nil {
					t.Fatal(err)
				}
				b, err := Read(zp)
				if err != nil || !b.Zipped || b.Prefix != prefix || b.Text != "text.fountain" || b.Markdown() {
					t.Fatalf("read: %v %+v", err, b)
				}
				want := contents(t, b)
				// the folder form, by its own name
				dp := filepath.Join(dir, "Folder"+ty.Ext)
				d, err := Write(b, dp, Dir, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if st, err := os.Stat(dp); err != nil || !st.IsDir() || d.Zipped {
					t.Fatalf("not a folder: %v", err)
				}
				equal(t, "folder", contents(t, d), want)
				if !hasDir(t, d, "empty/") || !hasDir(t, d, "assets/") {
					t.Error("the folder lost its empty folders")
				}
				if _, err := os.Stat(filepath.Join(dp, "__MACOSX")); err == nil {
					t.Error("macOS's junk unzipped")
				}
				// read again with Auto: a folder; written in place
				d2, err := Read(dp)
				if err != nil || d2.Zipped {
					t.Fatal(err)
				}
				d3, err := SaveText(d2, dp, Auto, []byte(script+"\nBram eats.\n"), false, "")
				if err != nil || d3.Zipped {
					t.Fatal(err)
				}
				want["text.fountain"] = script + "\nBram eats.\n"
				equal(t, "folder saved", contents(t, d3), want)
				// back to a zip: in Name.textbundle/
				zp2 := filepath.Join(dir, "Again"+ty.Ext)
				z, err := Write(d3, zp2, Zip, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if z.Prefix != "Again.textbundle/" {
					t.Errorf("prefix %q", z.Prefix)
				}
				equal(t, "zip again", contents(t, z), want)
				if !hasDir(t, z, "empty/") {
					t.Error("the zip lost the empty folder")
				}
			})
		}
	}
}

// A zip saved in place keeps every other entry byte for byte (raw), its
// order and the archive's comment.
func TestZipInPlace(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Film.highland")
	orig := makeZip(t, highlandItems("Film.textbundle/"))
	os.WriteFile(p, orig, 0o644)
	b, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	nb, err := SaveText(b, p, Auto, []byte("NEW"), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if nb.Changed() {
		t.Error("changed right after writing")
	}
	after, _ := os.ReadFile(p)
	a, _ := zip.NewReader(bytes.NewReader(orig), int64(len(orig)))
	z, _ := zip.NewReader(bytes.NewReader(after), int64(len(after)))
	if z.Comment != a.Comment || len(z.File) != len(a.File) {
		t.Fatalf("comment %q, %d entries", z.Comment, len(z.File))
	}
	for i, f := range a.File {
		g := z.File[i]
		if g.Name != f.Name {
			t.Errorf("entry %d: %s, want %s", i, g.Name, f.Name)
		}
		if strings.HasSuffix(f.Name, "text.fountain") {
			continue
		}
		if g.CRC32 != f.CRC32 || g.Method != f.Method || g.CompressedSize64 != f.CompressedSize64 {
			t.Errorf("%s changed", f.Name)
		}
	}
	if got, _ := nb.ReadText(); string(got) != "NEW" {
		t.Errorf("text %q", got)
	}
}

// A bundle changes form at its own path: a zip becomes a folder and back.
func TestFormInPlace(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Film.accolade")
	os.WriteFile(p, makeZip(t, highlandItems("")), 0o644)
	b, _ := Read(p)
	want := contents(t, b)
	d, err := Write(b, p, Dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); !st.IsDir() {
		t.Fatal("still a file")
	}
	equal(t, "folder", contents(t, d), want)
	z, err := Write(d, p, Zip, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(p); st.IsDir() {
		t.Fatal("still a folder")
	}
	equal(t, "zip", contents(t, z), want)
	if ents, _ := os.ReadDir(filepath.Dir(p)); len(ents) != 1 {
		t.Errorf("left behind: %v", ents)
	}
}

// A folder that is no bundle, or a file that is no zip, is never replaced.
func TestNotReplaced(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "Film.accolade")
	os.Mkdir(folder, 0o755)
	os.WriteFile(filepath.Join(folder, "mine.txt"), []byte("mine"), 0o644)
	if _, err := Write(nil, folder, Zip, New([]byte(script), false, ""), nil); err == nil {
		t.Error("a folder that is no bundle replaced by a zip")
	}
	if _, err := Write(nil, folder, Dir, New([]byte(script), false, ""), nil); err == nil {
		t.Error("a folder that is no bundle replaced by a bundle folder")
	}
	file := filepath.Join(dir, "Film.textbundle")
	os.WriteFile(file, []byte("plain"), 0o644)
	if _, err := Write(nil, file, Dir, New([]byte(script), false, ""), nil); err == nil {
		t.Error("a file that is no zip replaced")
	}
	// an empty file (a save dialog's) is replaced
	empty := filepath.Join(dir, "New.textbundle")
	os.WriteFile(empty, nil, 0o644)
	if b, err := Write(nil, empty, Auto, New([]byte(script), true, "x"), nil); err != nil || b.Zipped || !b.Markdown() {
		t.Errorf("over an empty file: %v", err)
	}
}

// Names: the type's form for a new bundle, what is on disk for one there.
func TestFormOf(t *testing.T) {
	dir := t.TempDir()
	for name, want := range map[string]Form{
		"a.textbundle": Dir, "a.textpack": Zip, "a.highland": Zip, "a.accolade": Zip,
		"a.accolade/": Dir, "a.txt": Zip,
	} {
		if got := FormOf(dir + "/" + name); got != want {
			t.Errorf("%s: %s, want %s", name, got, want)
		}
	}
	os.Mkdir(filepath.Join(dir, "b.textpack"), 0o755)
	if FormOf(filepath.Join(dir, "b.textpack")) != Dir {
		t.Error("a folder named .textpack")
	}
	os.WriteFile(filepath.Join(dir, "c.textbundle"), []byte("PK"), 0o644)
	if FormOf(filepath.Join(dir, "c.textbundle")) != Zip {
		t.Error("a file named .textbundle")
	}
}

// The text: text.fountain over a language's text.eo.fountain, which is
// taken when it is the only one.
func TestPickText(t *testing.T) {
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{[]string{"text.eo.fountain", "text.fountain"}, "text.fountain"},
		{[]string{"text.md", "text.eo.fountain"}, "text.md"},
		{[]string{"text.eo.fountain", "info.json"}, "text.eo.fountain"},
		{[]string{"text.txt", "text.markdown"}, "text.markdown"},
		{[]string{"TEXT.FOUNTAIN"}, "TEXT.FOUNTAIN"},
		{[]string{"info.json", "texts.md"}, ""},
	} {
		if got := PickText(tc.names, nil); got != tc.want {
			t.Errorf("%v: %q, want %q", tc.names, got, tc.want)
		}
	}
}

func TestTextMarkdown(t *testing.T) {
	if TextMarkdown("text.txt", nil, "a.textbundle") != true || TextMarkdown("text.txt", nil, "a.highland") != false {
		t.Error("type defaults")
	}
	if TextMarkdown("text.txt", []byte(`{"type":"`+FountainUTI+`"}`), "a.textpack") {
		t.Error("info's type")
	}
}

// info.json is edited as text: only the member asked for changes.
func TestSetInfoValue(t *testing.T) {
	for _, tc := range []struct{ in, val, want string }{
		{`{}`, `1`, "{\n  \"k\": 1\n}"},
		{`{"a":1}`, `2`, `{"a":1, "k": 2}`},
		{"{\n    \"a\":\"x\\/y\",\n}", `2`, "{\n    \"a\":\"x\\/y\",\n    \"k\": 2,\n}"},
		{"{\n  \"a\": 1,\n  \"k\": {\"old\": [1, \"}\"]},\n  \"b\": 2\n}", `3`, "{\n  \"a\": 1,\n  \"k\": 3,\n  \"b\": 2\n}"},
		{"{\n  \"a\": 1,\n  \"k\": 0\n}", ``, "{\n  \"a\": 1\n}"},
		{"{\n  \"k\": 0,\n  \"a\": 1\n}", ``, "{\n  \"a\": 1\n}"},
	} {
		var val []byte
		if tc.val != "" {
			val = []byte(tc.val)
		}
		got, ok := SetInfoValue([]byte(tc.in), "k", val)
		if !ok || string(got) != tc.want {
			t.Errorf("%q with %q: %q (%v), want %q", tc.in, tc.val, got, ok, tc.want)
		}
	}
	if _, ok := SetInfoValue([]byte(`not json`), "k", []byte("1")); ok {
		t.Error("not JSON, edited")
	}
}

// A new bundle: info.json, the text, assets/, in Name.textbundle/ when
// zipped.
func TestNew(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Film.highland")
	b, err := SaveText(nil, p, Auto, []byte(script), false, "org.example")
	if err != nil {
		t.Fatal(err)
	}
	if !b.Zipped || b.Prefix != "Film.textbundle/" || b.Text != "text.fountain" || !hasDir(t, b, "assets/") {
		t.Errorf("%+v", b)
	}
	if c, _ := InfoString(b.Info, "creatorIdentifier"); c != "org.example" {
		t.Errorf("info %s", b.Info)
	}
	if r := Check(p); !r.OK() || len(r.Notes) != 0 {
		t.Errorf("check:\n%s", r)
	}
}

// Check on TextBundle's own samples and on broken bundles.
func TestCheck(t *testing.T) {
	sample := func(n string) string { return filepath.Join("testdata", "textbundle", n) }
	if r := Check(sample("textpack sample.textpack")); !r.OK() {
		t.Errorf("the sample:\n%s", r)
	}
	r := Check(sample("only text.textbundle"))
	if !r.OK() || !strings.Contains(r.String(), "not strict JSON") {
		t.Errorf("trailing comma:\n%s", r)
	}
	if r := Check(sample("invalid no text.textbundle")); r.OK() {
		t.Errorf("no text:\n%s", r)
	}
	if r := Check(sample("invalid no info.textbundle")); !r.OK() || !strings.Contains(r.String(), "no info.json") {
		t.Errorf("no info:\n%s", r)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.textpack")
	os.WriteFile(bad, makeZip(t, []zipItem{{name: "../evil", data: "x"}, {name: "text.md", data: "# x"}}), 0o644)
	if r := Check(bad); r.OK() {
		t.Errorf("a name outside:\n%s", r)
	}
	two := filepath.Join(dir, "two.textpack")
	os.WriteFile(two, makeZip(t, []zipItem{{name: "a/text.md", data: "a"}, {name: "b/text.md", data: "b"}}), 0o644)
	if r := Check(two); r.OK() {
		t.Errorf("two folders:\n%s", r)
	}
	if r := Check(filepath.Join(dir, "none.highland")); r.OK() {
		t.Error("a missing file")
	}
	plain := filepath.Join(dir, "plain.highland")
	os.WriteFile(plain, []byte(script), 0o644)
	if _, err := Read(plain); err != ErrNotZip {
		t.Errorf("plain text: %v", err)
	}
}

// ReadFile never reads outside the bundle.
func TestReadFileOutside(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "secret"), []byte("x"), 0o644)
	b, err := Write(nil, filepath.Join(dir, "a.textbundle"), Dir, New([]byte("# a"), true, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.ReadFile("../secret"); err == nil {
		t.Error("read outside")
	}
	if _, err := Write(b, b.Path, Auto, []File{{"../x", []byte("x")}}, nil); err == nil {
		t.Error("wrote outside")
	}
}
