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

// (a compressed stream may end in a \r: the line break before endstream
// is left to zlib, which stops at the end of its data)
var pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)

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

// Marker and background over the whole block, and the line with a tint
// under its block: every format shows them on the lines under the
// heading too.
func TestColorBlocks(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	script := "Title: T\n\nINT. PUB - NIGHT [[red]]\n\nRain falls.\n\nANNA\nHi.\n\nEXT. ROAD - DAY\n\nBOB\nBye.\n"
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	export := func(to string, flags ...string) []byte {
		t.Helper()
		out := filepath.Join(dir, "out."+to)
		args := append([]string{"-i", in, "-to", to, "-o", out, "-config", filepath.Join(dir, "none.toml")}, flags...)
		if got := Main(context.Background(), args); got != 0 {
			t.Fatalf("%s %q: exit %d", to, flags, got)
		}
		b, _ := os.ReadFile(out)
		return b
	}
	hex := func(st lex.ColorStyle) (string, string, string) {
		c, _ := lex.ColorFor(st, "red")
		return lex.HexOf(c), strings.ToUpper(lex.HexOf(c)[1:]), fmt.Sprintf("%.3f %.3f %.3f rg", float64(c[0])/255, float64(c[1])/255, float64(c[2])/255)
	}
	for _, c := range []struct {
		flags []string
		style lex.ColorStyle // of the block's fill
	}{
		{[]string{"-colorstyle", "marker", "-colorscope", "block"}, lex.ColorMarker},
		{[]string{"-colorstyle", "background+block"}, lex.ColorBackground},
		{[]string{"-colortint"}, lex.ColorTint},
	} {
		h, w, rg := hex(c.style)
		// html: the marker on a line of the scene, else a band on its block
		html := string(export("html", c.flags...))
		if c.style == lex.ColorMarker {
			if !strings.Contains(html, `<span class="marker" style="background-color: `+h+`">Rain falls.`) {
				t.Errorf("html %q: no marker on the scene's lines", c.flags)
			}
		} else if !regexp.MustCompile(`<div class="colorbar" style="background: [^"]*`+h+`[^"]*">`).MatchString(html) ||
			!strings.Contains(html[strings.Index(html, "colorbar"):], "Hi.") {
			t.Errorf("html %q: no band over the block", c.flags)
		}
		if c.style == lex.ColorTint {
			bar, _, _ := hex(lex.ColorLine)
			if !strings.Contains(html, bar) {
				t.Errorf("html: no bar with the tint")
			}
		}
		// pdf: fills in the color; a band multiplied over the text
		pdf := pdfContent(t, export("pdf", c.flags...))
		if c.style == lex.ColorMarker {
			if n := strings.Count(pdf, rg); n < 3 { // the heading, the action, the speech
				t.Errorf("pdf marker over the block: %d fills", n)
			}
		} else if !strings.Contains(pdf, rg) || !strings.Contains(string(export("pdf", c.flags...)), "/BM /Multiply") {
			t.Errorf("pdf %q: no band", c.flags)
		}
		// docx and odt: on every paragraph of the scene
		docx := zipFile(t, export("docx", c.flags...), "word/document.xml")
		if n := strings.Count(docx, `w:fill="`+w+`"`); n < 3 {
			t.Errorf("docx %q: %d shaded", c.flags, n)
		}
		odt := zipFile(t, export("odt", c.flags...), "content.xml")
		if n := strings.Count(odt, `fo:background-color="`+h+`"`); n < 3 {
			t.Errorf("odt %q: %d with a background", c.flags, n)
		}
		if strings.Contains(docx[strings.Index(docx, "ROAD"):], `w:fill="`+w+`"`) {
			t.Errorf("docx %q: the next scene, without a color, has it", c.flags)
		}
	}
	for _, bad := range [][]string{{"-colorstyle", "text", "-colorscope", "block"}, {"-colorstyle", "marker", "-colortint"}, {"-colorscope", "all"}} {
		args := append([]string{"-i", in, "-to", "html", "-o", filepath.Join(dir, "x.html"), "-config", filepath.Join(dir, "none.toml")}, bad...)
		if got := Main(context.Background(), args); got != 1 {
			t.Errorf("%q: exit %d", bad, got)
		}
	}
}

// Joop's script: colors on sections only (which the default format does
// not print), synopses under headings and sections, a section over
// several pages, a scene of its own color within it. A section's color
// covers it whole as one block, page by page: one bar a page, none
// restarting at its scenes; the scene's own color in its span.
func TestSectionBlocks(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	var b strings.Builder
	b.WriteString("Title: Sections\n\n# Act One [[COLOR BLUE]]\n\n= The first act.\n\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "INT. ROOM %d - DAY\n\n= Scene %d.\n\n", i, i)
		if i == 6 {
			b.WriteString("INT. RED ROOM - NIGHT [[red]]\n\nThe red scene.\n\n")
		}
		for j := 0; j < 8; j++ {
			b.WriteString("Something happens, and then something else.\n\n")
		}
	}
	b.WriteString("# Act Two\n\nINT. PLAIN - DAY\n\nNothing.\n")
	if err := os.WriteFile(in, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "t.pdf")
	conf := []string{"-config", filepath.Join(dir, "none.toml")}
	for _, flags := range [][]string{{"-colors"}, {"-colortint"}} {
		if got := Main(context.Background(), append(append([]string{"-i", in, "-o", out}, conf...), flags...)); got != 0 {
			t.Fatalf("exit %d", got)
		}
		data, _ := os.ReadFile(out)
		pages := len(regexp.MustCompile(`/Type /Page\b[^s]`).FindAll(data, -1))
		content := pdfContent(t, data)
		blue, _ := lex.ColorFor(lex.ColorLine, "blue")
		red, _ := lex.ColorFor(lex.ColorLine, "red")
		bars := func(c [3]uint8) int {
			rg := regexp.QuoteMeta(fmt.Sprintf("%.3f %.3f %.3f rg", float64(c[0])/255, float64(c[1])/255, float64(c[2])/255))
			return len(regexp.MustCompile(rg+`\s*36\.00 [\d.]+ 2\.88 -[\d.]+ re f`).FindAllString(content, -1))
		}
		// one bar of the act's on each page after the title page (not one
		// a scene: there are 12), the red scene's on the pages it is on
		// (it breaks over two here), the act's again after it
		if nb, nr := bars(blue), bars(red); pages < 4 || nr < 1 || nr > 2 || nb != pages-1 {
			t.Errorf("%q: %d pages, %d blue bars, %d red", flags, pages, nb, nr)
		}
	}
}
