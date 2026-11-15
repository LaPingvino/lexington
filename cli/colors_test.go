package cli

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Every output format that can show a color shows a red heading's color
// with -colors, and none without: html, pdf, docx, odt; fdx keeps it in
// SceneProperties either way (Final Draft doesn't print it).
func TestColorsInEveryFormat(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	script := "Title: T\n\nINT. PUB - NIGHT [[red]]\n\nANNA\nHi.\n\nEXT. ROAD - DAY\n\nBOB\nBye.\n"
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := []string{"-config", filepath.Join(dir, "none.toml")}
	export := func(to string, colors bool) []byte {
		t.Helper()
		out := filepath.Join(dir, "out."+to)
		args := append([]string{"-i", in, "-to", to, "-o", out}, conf...)
		if colors {
			args = append(args, "-colors")
		}
		if got := Main(context.Background(), args); got != 0 {
			t.Fatalf("%s: exit %d", to, got)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	// Beat's red, #ef0049, as each format writes a color
	red := map[string]func([]byte) bool{
		"html": func(b []byte) bool { return bytes.Contains(b, []byte(`color: #ef0049`)) },
		"pdf":  func(b []byte) bool { return strings.Contains(pdfContent(t, b), "0.937 0.000 0.286 rg") },
		"docx": func(b []byte) bool {
			return strings.Contains(zipFile(t, b, "word/document.xml"), `w:color w:val="EF0049"`)
		},
		"odt": func(b []byte) bool { return strings.Contains(zipFile(t, b, "content.xml"), `fo:color="#ef0049"`) },
	}
	for to, has := range red {
		if !has(export(to, true)) {
			t.Errorf("%s with -colors: no red heading", to)
		}
		if has(export(to, false)) {
			t.Errorf("%s without -colors: a red heading", to)
		}
	}
	if fdx := export("fdx", false); !bytes.Contains(fdx, []byte(`Color="#`)) {
		t.Errorf("fdx: no SceneProperties Color")
	}
}

var pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)

// pdfContent is a PDF's streams, inflated where they are compressed.
func pdfContent(t *testing.T, b []byte) string {
	t.Helper()
	var all strings.Builder
	for _, m := range pdfStream.FindAllSubmatch(b, -1) {
		if r, err := zlib.NewReader(bytes.NewReader(m[1])); err == nil {
			if d, err := io.ReadAll(r); err == nil {
				all.Write(d)
				continue
			}
		}
		all.Write(m[1])
	}
	return all.String()
}

// zipFile is one file of a zip (docx, odt).
func zipFile(t *testing.T, b []byte, name string) string {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := z.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := io.ReadAll(f)
	return string(d)
}
