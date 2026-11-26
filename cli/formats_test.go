package cli

import (
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
	for _, ext := range []string{"fadein", "osf", "trelby", "kitsp", "fdx"} {
		t.Run(ext, func(t *testing.T) {
			out := filepath.Join(dir, "script."+ext)
			if code := Main(context.Background(), []string{"convert", src, out}); code != 0 {
				t.Fatalf("convert to %s: exit %d", ext, code)
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

// TestConvertArgs: convert's flags go before, between or after its two
// files; a .fadein is written as a folder with -form folder, and read
// back from it.
func TestConvertArgs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.fountain")
	if err := os.WriteFile(src, []byte(formattest.Script), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "a.fadein")
	if code := Main(context.Background(), []string{"convert", "-osfversion", "2.0", src, out, "-form", "folder"}); code != 0 {
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
	if code := Main(context.Background(), []string{"convert", out, back}); code != 0 {
		t.Fatalf("back: exit %d", code)
	}
	text, _ := os.ReadFile(back)
	if !strings.Contains(string(text), "MARY ^") || !strings.Contains(string(text), "INT. HALL - DAY #1#") {
		t.Errorf("back from the folder:\n%s", text)
	}
	if code := Main(context.Background(), []string{"convert", src}); code == 0 {
		t.Error("convert with one file did not fail")
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
	for name, key := range map[string]string{"a.fadein": "fadein", "a.FadeIn/": "fadein", "b.kitsp": "kitsp", "c.trelby": "trelby", "d.osf": "osf", "e.fdx": "fdx"} {
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
