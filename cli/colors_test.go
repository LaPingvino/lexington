package cli

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// Every output format that can show a color shows a red heading's color
// with -colors, in each style, and none without: html, pdf, docx, odt;
// fdx keeps it in SceneProperties either way (Final Draft doesn't print
// it).
func TestColorsInEveryFormat(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	script := "Title: T\n\nINT. PUB - NIGHT [[red]]\n\nANNA\nHi.\n\nEXT. ROAD - DAY\n\nBOB\nBye.\n"
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := []string{"-config", filepath.Join(dir, "none.toml")}
	export := func(to string, flags ...string) []byte {
		t.Helper()
		out := filepath.Join(dir, "out."+to)
		args := append(append([]string{"-i", in, "-to", to, "-o", out}, conf...), flags...)
		if got := Main(context.Background(), args); got != 0 {
			t.Fatalf("%s %q: exit %d", to, flags, got)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, style := range []lex.ColorStyle{lex.ColorLine, lex.ColorMarker, lex.ColorBackground, lex.ColorText} {
		rgb, _ := lex.ColorFor(style, "red")
		hex, word := lex.HexOf(rgb), strings.ToUpper(lex.HexOf(rgb)[1:])
		rg := fmt.Sprintf("%.3f %.3f %.3f", float64(rgb[0])/255, float64(rgb[1])/255, float64(rgb[2])/255)
		shows := map[string]func([]byte) bool{
			"html": map[lex.ColorStyle]func([]byte) bool{
				lex.ColorText: func(b []byte) bool { return bytes.Contains(b, []byte(`scene-heading" dir="auto" style="color: `+hex)) },
				lex.ColorMarker: func(b []byte) bool {
					return bytes.Contains(b, []byte(`<span class="marker" style="background-color: `+hex))
				},
				lex.ColorBackground: func(b []byte) bool {
					return bytes.Contains(b, []byte(`scene-heading" dir="auto" style="background-color: `+hex))
				},
				lex.ColorLine: func(b []byte) bool {
					return bytes.Contains(b, []byte(`<div class="colorbar"`)) && bytes.Contains(b, []byte(hex))
				},
			}[style],
			"pdf": func(b []byte) bool {
				c := pdfContent(t, b)
				if style == lex.ColorText {
					return strings.Contains(c, rg+" rg")
				}
				// a filled rectangle in the color; the bar at the margin
				re := regexp.MustCompile(regexp.QuoteMeta(rg+" rg") + `(?s).*?([\d.]+) [\d.]+ [\d.]+ -?[\d.]+ re f`)
				m := re.FindStringSubmatch(c)
				return m != nil && (style != lex.ColorLine || m[1] == "36.00")
			},
			"docx": func(b []byte) bool {
				x := zipFile(t, b, "word/document.xml")
				return strings.Contains(x, map[lex.ColorStyle]string{
					lex.ColorText:       `<w:color w:val="` + word + `"/>`,
					lex.ColorMarker:     `<w:shd w:val="clear" w:color="auto" w:fill="` + word + `"/></w:rPr>`,
					lex.ColorBackground: `<w:shd w:val="clear" w:color="auto" w:fill="` + word + `"/></w:pPr>`,
					lex.ColorLine:       `w:color="` + word + `"/></w:pBdr>`,
				}[style])
			},
			"odt": func(b []byte) bool {
				x := zipFile(t, b, "content.xml")
				return strings.Contains(x, map[lex.ColorStyle]string{
					lex.ColorText:       `<style:text-properties fo:color="` + hex + `"/>`,
					lex.ColorMarker:     `<style:text-properties fo:background-color="` + hex + `"/>`,
					lex.ColorBackground: `fo:background-color="` + hex + `"></style:paragraph-properties>`,
					lex.ColorLine:       `solid ` + hex + `"`,
				}[style])
			},
		}
		for to, has := range shows {
			if !has(export(to, "-colorstyle", string(style))) {
				t.Errorf("%s, %s: no red heading", to, style)
			}
			if has(export(to)) {
				t.Errorf("%s without colors: a red heading (%s)", to, style)
			}
		}
		if style == lex.ColorLine && !shows["html"](export("html", "-colors")) {
			t.Error("-colors is not line")
		}
	}
	if fdx := export("fdx"); !bytes.Contains(fdx, []byte(`Color="#`)) {
		t.Errorf("fdx: no SceneProperties Color")
	}
	if got := Main(context.Background(), append([]string{"-i", in, "-to", "html", "-o", filepath.Join(dir, "x.html"), "-colorstyle", "neon"}, conf...)); got != 1 {
		t.Errorf("a style that is none: exit %d", got)
	}
}

// The bar of a scene over several pages is drawn on each of them, at the
// margin, from the top of the text to its foot; a section's color colors
// its scenes, though sections are not printed (Joop's script).
func TestColorBarOverPages(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	var b strings.Builder
	b.WriteString("# Act One [[blue]]\n\nINT. LONG SCENE\n\n")
	for i := 0; i < 120; i++ {
		b.WriteString("Something happens, and then something else.\n\n")
	}
	b.WriteString("# Act Two\n\nINT. PLAIN\n\nNothing.\n")
	if err := os.WriteFile(in, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "t.pdf")
	if got := Main(context.Background(), []string{"-i", in, "-o", out, "-colors", "-config", filepath.Join(dir, "none.toml")}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	data, _ := os.ReadFile(out)
	pages := len(regexp.MustCompile(`/Type /Page\b[^s]`).FindAll(data, -1))
	bars := regexp.MustCompile(`36\.00 [\d.]+ 2\.88 -[\d.]+ re f`).FindAllString(pdfContent(t, data), -1)
	if pages < 4 || len(bars) != pages {
		t.Errorf("%d pages, %d bars: %q", pages, len(bars), bars)
	}
	// and in html, the default format, which does not print sections
	html := filepath.Join(dir, "t.html")
	if got := Main(context.Background(), []string{"-i", in, "-o", html, "-colorstyle", "background", "-config", filepath.Join(dir, "none.toml")}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	h, _ := os.ReadFile(html)
	rgb, _ := lex.ColorFor(lex.ColorBackground, "blue")
	if n := strings.Count(string(h), "background-color: "+lex.HexOf(rgb)); n != 1 {
		t.Errorf("%d headings in the section's color, want 1", n)
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
