package cli

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/bundle"
	"github.com/LaPingvino/lexington/pdfin"
)

// highlandSamples are the real .highland files (bundle/testdata/highland,
// and $LEXINGTON_HIGHLAND_SAMPLES's when set).
func highlandSamples(t *testing.T) []string {
	t.Helper()
	ps, _ := filepath.Glob(filepath.Join("..", "bundle", "testdata", "highland", "*.highland"))
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

// Every real .highland goes through the CLI: copied to another .highland
// and to a folder and back with its text byte for byte; a screenplay
// printed to PDF and HTML without Highland's directives showing; Markdown
// and plain text refused for print, by their kind.
func TestHighlandSamplesCLI(t *testing.T) {
	for _, p := range highlandSamples(t) {
		t.Run(filepath.Base(p), func(t *testing.T) {
			src, text := read(t, p)
			dir := t.TempDir()
			cp := filepath.Join(dir, "Copy.highland")
			run(t, Options{Input: p, Output: cp})
			if b, got := read(t, cp); got != text || b.Prefix != "Copy.textbundle/" {
				t.Errorf("copy: %q, text changed: %v", b.Prefix, got != text)
			}
			folder := filepath.Join(dir, "Folder.highland")
			run(t, Options{Input: cp, Output: folder, BundleForm: "folder"})
			back := filepath.Join(dir, "Back.highland")
			run(t, Options{Input: folder, Output: back, BundleForm: "zip"})
			if b, got := read(t, back); got != text || !b.Zipped || b.Prefix != "Back.textbundle/" {
				t.Errorf("back: %q, text changed: %v", b.Prefix, got != text)
			}
			pdf := filepath.Join(dir, "out.pdf")
			err := Convert(context.Background(), Options{Input: p, Output: pdf, ConfigFile: filepath.Join(dir, "none.toml")})
			if k := src.Kind(); k != bundle.Fountain {
				if err == nil || !strings.Contains(err.Error(), k.String()) {
					t.Errorf("%s printed: %v", k, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			pages, err := pdfin.Pages(pdf)
			if err != nil || len(pages) == 0 {
				t.Fatal(err)
			}
			for _, pg := range pages {
				for _, l := range pg {
					if strings.Contains(l.Text, "{{") {
						t.Errorf("printed: %q", l.Text)
					}
				}
			}
			html := filepath.Join(dir, "out.html")
			run(t, Options{Input: p, Output: html})
			if data, _ := os.ReadFile(html); strings.Contains(string(data), "{{") {
				i := strings.Index(string(data), "{{")
				t.Errorf("in HTML: %.80s", data[i:])
			}
			// Fountain written through Lexington keeps the directives
			ft := filepath.Join(dir, "out.fountain")
			run(t, Options{Input: p, Output: ft, Character: ""})
			if data, _ := os.ReadFile(ft); strings.Count(string(data), "{{") != strings.Count(text, "{{") {
				t.Errorf("directives lost in Fountain: %d of %d", strings.Count(string(data), "{{"), strings.Count(text, "{{"))
			}
			// and so does a bundle whose text Lexington rewrites (scenes numbered)
			num := filepath.Join(dir, "Numbered.highland")
			run(t, Options{Input: p, Output: num, Number: "count"})
			if b, got := read(t, num); strings.Count(got, "{{") != strings.Count(text, "{{") || len(b.Files) != len(src.Files) {
				t.Errorf("numbered: %d of %d directives, files %v", strings.Count(got, "{{"), strings.Count(text, "{{"), b.Files)
			}
		})
	}
}
