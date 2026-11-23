package bundle

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// highlandSamples are the real .highland files: testdata/highland's, and
// those under $LEXINGTON_HIGHLAND_SAMPLES (highwater-too's
// fixtures/generated, say) when it is set.
func highlandSamples(t *testing.T) []string {
	t.Helper()
	ps, _ := filepath.Glob(filepath.Join("testdata", "highland", "*.highland"))
	if dir := os.Getenv("LEXINGTON_HIGHLAND_SAMPLES"); dir != "" {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".highland") {
				ps = append(ps, p)
			}
			return nil
		})
	}
	if len(ps) < 5 {
		t.Fatalf("%d samples", len(ps))
	}
	return ps
}

// noDirEntries: Highland's zips have none.
func noDirEntries(t *testing.T, b *Bundle) {
	t.Helper()
	es, err := b.Entries()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range es {
		if strings.HasSuffix(e.Name, "/") {
			t.Errorf("%s: a folder's entry %s", filepath.Base(b.Path), e.Name)
		}
	}
}

// Every real .highland reads (its text, kind and template), checks clean,
// and goes to another name, to a folder and back, and through a save in
// place with every file kept byte for byte and its folder named for the
// file it is in.
func TestHighlandSamples(t *testing.T) {
	for _, p := range highlandSamples(t) {
		t.Run(filepath.Base(p), func(t *testing.T) {
			b, err := Read(p)
			if err != nil {
				t.Fatal(err)
			}
			if !b.Zipped || !strings.HasSuffix(b.Prefix, ".textbundle/") || strings.Count(b.Prefix, "/") != 1 {
				t.Fatalf("layout: %+v", b)
			}
			uti, _ := InfoString(b.Info, "type")
			want := map[string]struct {
				text string
				kind Kind
			}{FountainUTI: {"text.fountain", Fountain}, MarkdownUTI: {"text.md", Markdown}, PlainUTI: {"text.txt", Plain}}[uti]
			if want.text == "" || b.Text != want.text || b.Kind() != want.kind {
				t.Errorf("type %q: text %s, %s", uti, b.Text, b.Kind())
			}
			if b.Template() == "" {
				t.Errorf("no template in %s", b.Info)
			}
			if r := Check(p); !r.OK() || len(r.Notes) != 0 {
				t.Errorf("check:\n%s", r)
			}
			files := contents(t, b)
			text := files[b.Text]
			dir := t.TempDir()

			// to another name: its folder renamed, all else as it was
			c, err := Write(b, filepath.Join(dir, "Copy.highland"), Auto, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if c.Prefix != "Copy.textbundle/" || !c.Zipped {
				t.Errorf("copy: %q", c.Prefix)
			}
			equal(t, "copy", contents(t, c), files)
			noDirEntries(t, c)

			// its text saved as it is, in place: the zip's files the same
			s, err := SaveText(c, c.Path, Auto, []byte(text), b.Markdown(), "")
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "saved", contents(t, s), files)

			// a folder and back
			d, err := Write(b, filepath.Join(dir, "Folder.highland"), Dir, nil, nil)
			if err != nil || d.Zipped {
				t.Fatal(err)
			}
			equal(t, "folder", contents(t, d), files)
			if r := Check(d.Path); !r.OK() || len(r.Notes) != 1 || !strings.Contains(r.Notes[0], "not known") {
				t.Errorf("check of the folder:\n%s", r)
			}
			z, err := Write(d, filepath.Join(dir, "Back.highland"), Zip, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if z.Prefix != "Back.textbundle/" {
				t.Errorf("back: %q", z.Prefix)
			}
			equal(t, "back", contents(t, z), files)
			noDirEntries(t, z)

			// renamed on disk, then saved in place: its folder follows the name
			moved := filepath.Join(dir, "Moved.highland")
			if err := os.Rename(c.Path, moved); err != nil {
				t.Fatal(err)
			}
			m, err := Read(moved)
			if err != nil {
				t.Fatal(err)
			}
			changed := text + "\nAdded.\n"
			m, err = SaveText(m, moved, Auto, []byte(changed), m.Markdown(), "")
			if err != nil {
				t.Fatal(err)
			}
			if m.Prefix != "Moved.textbundle/" {
				t.Errorf("moved: %q", m.Prefix)
			}
			files[b.Text] = changed
			equal(t, "changed", contents(t, m), files)
			if info, _ := m.ReadFile("info.json"); !bytes.Equal(info, b.Info) {
				t.Errorf("info.json: %s", info)
			}
		})
	}
}

// A .highland's revision cache goes when its text changes (Highland
// builds it again from the text's tags), and stays while it does not.
func TestHighlandRevisionCache(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Rev.highland")
	os.WriteFile(p, makeZip(t, []zipItem{
		{name: "Rev.textbundle/info.json", data: highlandInfo},
		{name: "Rev.textbundle/text.fountain", data: script},
		{name: "Rev.textbundle/revisions/current.json", data: `{"number":1}`},
	}), 0o644)
	b, _ := Read(p)
	b, err := SaveText(b, p, Auto, []byte(script), false, "")
	if err != nil || len(b.Files) != 3 {
		t.Fatalf("unchanged: %v %v", err, b.Files)
	}
	b, err = SaveText(b, p, Auto, []byte(script+"\nMore.\n"), false, "")
	if err != nil || len(b.Files) != 2 {
		t.Errorf("changed: %v %v", err, b.Files)
	}
}
