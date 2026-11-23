package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakePrograms puts stand-ins for wkhtmltopdf, pdflatex and ebook-convert
// first on the PATH: each copies the HTML or LaTeX it is given to its
// output, so a test reads what Lexington handed it.
func fakePrograms(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-ins are shell scripts")
	}
	bin := t.TempDir()
	scripts := map[string]string{
		// wkhtmltopdf [options] in.html out.pdf
		"wkhtmltopdf": `for a; do in=$out; out=$a; done; cp "$in" "$out"`,
		// pdflatex -interaction=nonstopmode -output-directory dir file.tex
		"pdflatex": `cp "$4" "$3/script.pdf"`,
		// ebook-convert in.html out.ext [options]
		"ebook-convert": `cp "$1" "$2"`,
	}
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// The formats made with other programs print the title page with the
// chosen template, as Lexington's own writers do: the HTML and LaTeX
// handed to wkhtmltopdf, LaTeX and Calibre have it.
func TestExternalFormatsTitlePage(t *testing.T) {
	fakePrograms(t)
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	const titled = "Title: The Barn\nAuthor: Joop\nLogline: A storm comes.\n\nINT. BARN - DAY\n\nRain.\n"
	writeFile(t, in, []byte(titled))
	conf := filepath.Join(dir, "lexington.toml")
	writeFile(t, conf, []byte("[TitlePages.loud]\n[[TitlePages.loud.Keys]]\nKey = \"Logline\"\nPlace = \"top\"\n"+
		"[[TitlePages.loud.Keys]]\nKey = \"Title\"\nPlace = \"centre\"\n"))
	for _, to := range []string{"htmlpdf", "latexpdf", "azw3"} {
		f, ok := FormatFor(to)
		if !ok || f.Needs == "pandoc" {
			continue
		}
		out := func(tpl string) string {
			p := filepath.Join(dir, to+"-"+tpl+"."+f.Extension())
			o := Options{Input: in, Output: p, To: to, ConfigFile: conf, TitlePage: tpl}
			if err := Convert(t.Context(), o); err != nil {
				t.Fatalf("%s: %v", to, err)
			}
			return readFile(t, p)
		}
		loud, plain := out("loud"), out("standard")
		if loud == plain {
			t.Errorf("%s: the template made no difference", to)
		}
		if i, j := strings.Index(loud, "A storm comes."), strings.LastIndex(loud, "The Barn"); i < 0 || j < 0 || i > j {
			t.Errorf("%s: the Logline is not above the title:\n%s", to, loud)
		}
	}
}
