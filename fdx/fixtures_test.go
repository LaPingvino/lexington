package fdx

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// fixture parses a file of testdata/fdx.
func fixture(t *testing.T, name string) (lex.Screenplay, FdxFile) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../testdata/fdx", name))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseWithError(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var f FdxFile
	if err := xml.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return s, f
}

func lines(s lex.Screenplay, types ...lex.ElementType) []string {
	var out []string
	for _, l := range s {
		if slices.Contains(types, l.Type) {
			out = append(out, l.Contents)
		}
	}
	return out
}

// One file of each Version we have: what Final Draft shows of it.
func TestFixtures(t *testing.T) {
	t.Run("Big Fish, Version 1", func(t *testing.T) {
		s, f := fixture(t, "big-fish/Big-Fish.fdx")
		if f.Version != "1" {
			t.Errorf("Version %q", f.Version)
		}
		headings := lines(s, lex.TypeScene)
		if len(headings) != 202 || headings[0] != "A RIVER. #1#" || headings[1] != "INT.  WILL’S BEDROOM - NIGHT (1973) #2#" {
			t.Errorf("%d headings: %q", len(headings), headings[:2])
		}
		for _, h := range headings {
			if _, n := lex.SceneNumber(h); n == "" {
				t.Errorf("no number: %q", h)
			}
		}
		for _, c := range lines(s, lex.TypeSpeaker, lex.TypeScene, lex.TypeTrans) {
			if c != strings.ToUpper(c) {
				t.Errorf("not in capitals, as Final Draft shows it: %q", c)
			}
		}
		if sp := lines(s, lex.TypeSpeaker); !slices.Contains(sp, "EDWARD (V.O.)") {
			t.Error("no EDWARD (V.O.)")
		}
		if tr := lines(s, lex.TypeTrans); !slices.Contains(tr, "FLASHCUTS TO:") {
			t.Errorf("transitions: %q", tr)
		}
		if c := lines(s, lex.TypeCenter); !slices.Contains(c, "_**THE END**_") {
			t.Errorf("centered: %q", c)
		}
	})
	t.Run("Final Draft, Version 5", func(t *testing.T) {
		s, f := fixture(t, "screenplay-tools/TestFDX-FD.fdx")
		if f.Version != "5" {
			t.Errorf("Version %q", f.Version)
		}
		if h := lines(s, lex.TypeScene); !slices.Equal(h, []string{"INT. RADIO STUDIO #1#", "EXT. OUTSIDE THE FOOD STORE #2#"}) {
			t.Errorf("headings %q", h)
		}
		if ti := lines(s, "Title"); !slices.Equal(ti, []string{"_FDX Test Script_"}) {
			t.Errorf("title %q", ti)
		}
		if c := lines(s, "Contact"); !slices.Equal(c, []string{"Copyright (c) 2026", "Draft", "information", "Contact", "information"}) {
			t.Errorf("contact %q", c)
		}
	})
	t.Run("Fade In, Version 2", func(t *testing.T) {
		s, f := fixture(t, "screenplay-tools/TestFDX-FI.fdx")
		if f.Version != "2" {
			t.Errorf("Version %q", f.Version)
		}
		if h := lines(s, lex.TypeScene); !slices.Equal(h, []string{"INT. RADIO STUDIO #1#", "EXT. OUTSIDE THE FOOD STORE #2#"}) {
			t.Errorf("headings %q", h)
		}
		if sp := lines(s, lex.TypeSpeaker); !slices.Equal(sp, []string{"DJ", "DAVE", "JIM (O.S.)", "KAY"}) {
			t.Errorf("speakers %q", sp)
		}
	})
	t.Run("Version 3", func(t *testing.T) {
		s, f := fixture(t, "open-fdx-toolkit/sample-03.fdx")
		if f.Version != "3" || len(lines(s, lex.TypeScene)) != 1 || len(lines(s, lex.TypeDialog)) != 1 {
			t.Errorf("Version %q: %q", f.Version, s)
		}
	})
	t.Run("Version 4", func(t *testing.T) {
		s, f := fixture(t, "jumpcut/title-pages-multi.fdx")
		if f.Version != "4" {
			t.Errorf("Version %q", f.Version)
		}
		if ti := lines(s, "Title"); len(ti) == 0 || ti[0] != "_**WHEN WE WERE VIKINGS**_" {
			t.Errorf("title %q", ti)
		}
		for _, l := range s {
			if strings.Contains(l.Contents, "\n") {
				t.Errorf("a line break in %q", l)
			}
		}
	})
	t.Run("Final Draft 13, Version 6", func(t *testing.T) {
		s, f := fixture(t, "fdx-mcp-server/NewFile.fdx")
		if f.Version != "6" {
			t.Errorf("Version %q", f.Version)
		}
		if ti := lines(s, "Title"); !slices.Equal(ti, []string{"_SCRIPT TITLE_"}) {
			t.Errorf("title %q", ti)
		}
		if b := lines(s, lex.TypeScene, lex.TypeAction); len(b) != 0 { // its one, empty, heading
			t.Errorf("body %q", b)
		}
	})
}

// Every fixture written as FDX by Lexington reads back the same.
func TestFixturesRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("../testdata/fdx/*/*.fdx")
	if len(files) < 6 {
		t.Fatalf("fixtures: %q", files)
	}
	for _, file := range files {
		name, _ := filepath.Rel("../testdata/fdx", file)
		t.Run(name, func(t *testing.T) {
			first, _ := fixture(t, name)
			var b bytes.Buffer
			if err := (&FDXWriter{}).Write(&b, first); err != nil {
				t.Fatal(err)
			}
			second, err := ParseWithError(&b)
			if err != nil {
				t.Fatal(err)
			}
			if w, g := significant(first), significant(second); !slices.Equal(w, g) {
				for i := range min(len(w), len(g)) {
					if w[i] != g[i] {
						t.Fatalf("element %d: %q, back %q", i, w[i], g[i])
					}
				}
				t.Errorf("%d elements, back %d", len(w), len(g))
			}
		})
	}
}

// The Fountain text of every paragraph in the fixtures reads (with
// lex.Emphasis) as the paragraph's runs: their text and their styles.
func TestFixtureEmphasis(t *testing.T) {
	files, _ := filepath.Glob("../testdata/fdx/*/*.fdx")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var f FdxFile
		if err := xml.Unmarshal(data, &f); err != nil {
			t.Fatal(err)
		}
		r := reader{types: typeSettings(f.ElementSettings)}
		ps := f.Content.Paragraphs
		if f.TitlePage != nil {
			ps = append(ps, f.TitlePage.Content.Paragraphs...)
		}
		for _, p := range ps {
			if text := r.text(p); !same(lex.Emphasis(lex.StripNotes(text)), r.runs(p)) {
				t.Errorf("%s: %q reads otherwise than its runs %+v", filepath.Base(file), text, r.runs(p))
			}
		}
	}
}
