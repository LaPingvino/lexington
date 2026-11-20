package pdf_test

import (
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
)

// revisedScript is three pages or so: scenes of action and dialogue,
// the lines of some marked as revised (by their Fountain line).
func revisedScript(t *testing.T, marks map[int]string) lex.Screenplay {
	t.Helper()
	var b strings.Builder
	b.WriteString("Title: Revised\nAuthor: Someone\n\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "INT. ROOM %d - DAY #%d#\n\nAnna walks in and looks around room %d, slowly, as if she had never seen it before.\n\nANNA\nThis is room %d. I remember it.\n\n", i, i, i, i)
	}
	s := fountain.ParseLines(fountain.Scene, strings.NewReader(b.String()))
	for i := range s {
		if set, ok := marks[s[i].Source]; ok {
			s[i].Revision = set
		}
	}
	return s
}

// lineOf is the Fountain line (from 1) of revisedScript's text with text.
func lineOf(t *testing.T, text string) int {
	t.Helper()
	s := revisedScript(t, nil)
	for _, l := range s {
		if strings.Contains(l.Contents, text) {
			return l.Source
		}
	}
	t.Fatalf("no line %q", text)
	return 0
}

var testSets = &lex.Revisions{Sets: []lex.RevisionSet{
	{Name: "White", Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
	{Name: "Blue", Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)},
	{Name: "Pink", Date: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)},
}}

func writeRevised(t *testing.T, s lex.Screenplay, r *lex.Revisions) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "rev.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Revisions: r}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	return file
}

// marked are the lines of each page that have the mark at the right
// margin, and the page's first line (the header, if any).
func readRevised(t *testing.T, file string) (pages [][]pdfin.PageLine) {
	t.Helper()
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

// asterisks are the marks on a page: the lines ending in " *" (pdfin joins
// the mark to its line) or standing alone at the margin.
func asterisks(page []pdfin.PageLine) (n int, lines []string) {
	for _, l := range page {
		if strings.HasSuffix(l.Text, "*") {
			n++
			lines = append(lines, l.Text)
		}
	}
	return n, lines
}

func header(page []pdfin.PageLine) string {
	for _, l := range page {
		if strings.Contains(l.Text, "Revision") {
			return strings.TrimSpace(pageNumberAfter.ReplaceAllString(l.Text, ""))
		}
	}
	return ""
}

func TestRevisionMarksAndHeader(t *testing.T) {
	anna3 := lineOf(t, "room 3, slowly")
	room9 := lineOf(t, "INT. ROOM 9")
	s := revisedScript(t, map[int]string{anna3: "Blue", room9: "Pink"})
	pages := readRevised(t, writeRevised(t, s, testSets))
	if len(pages) < 3 {
		t.Fatalf("%d pages", len(pages))
	}
	changed := map[int]string{}
	for i, p := range pages {
		n, lines := asterisks(p)
		h := header(p)
		if n > 0 || h != "" {
			changed[i] = h
			t.Logf("page %d: header %q, marks %q", i, h, lines)
		}
		if n > 0 && h == "" || n == 0 && h != "" {
			t.Errorf("page %d: %d marks but header %q", i, n, h)
		}
	}
	if len(changed) != 2 {
		t.Fatalf("changed pages %v, want two", changed)
	}
	found := map[string]bool{}
	for _, h := range changed {
		found[h] = true
	}
	if !found["Blue Revision – 20/11/26"] || !found["Pink Revision – 25/11/26"] {
		t.Errorf("headers %v", changed)
	}
	// the wrapped action's every line has its mark; the heading its one
	for i, p := range pages {
		n, lines := asterisks(p)
		switch changed[i] {
		case "Blue Revision – 20/11/26":
			if n != 2 || !strings.Contains(lines[0], "room 3") {
				t.Errorf("Blue page's marks %q, want the action's two lines", lines)
			}
		case "Pink Revision – 25/11/26":
			if n != 1 || !strings.Contains(lines[0], "ROOM 9") {
				t.Errorf("Pink page's marks %q, want the heading's", lines)
			}
		}
	}
	// without Revisions nothing is marked
	for i, p := range readRevised(t, writeRevised(t, s, nil)) {
		if n, _ := asterisks(p); n > 0 || header(p) != "" {
			t.Errorf("page %d marked without revisions", i)
		}
	}
}

func TestRevisionMarkPlace(t *testing.T) {
	room9 := lineOf(t, "INT. ROOM 9")
	anna := lineOf(t, "room 3, slowly")
	s := revisedScript(t, map[int]string{room9: "Blue", anna: "Blue"})
	// on Letter at 7.75", right of a numbered heading's number
	file := writeRevised(t, s, testSets)
	words := wordsOf(t, file)
	var stars []float64
	for _, w := range words {
		if w.text == "*" {
			stars = append(stars, w.x)
		}
	}
	if len(stars) != 3 {
		t.Fatalf("%d marks, want 3: %v", len(stars), stars)
	}
	heading, action := 0, 0
	for _, x := range stars {
		switch {
		case x > 7.74 && x < 7.76:
			action++
		case x > 7.76:
			heading++
		}
	}
	if action != 2 || heading != 1 {
		t.Errorf("marks at %v: want two at 7.75\" and the heading's past its number", stars)
	}
}

type word struct {
	text string
	x    float64
}

// wordsOf are the words of a PDF with their left edge in inches
// (pdftotext -bbox).
func wordsOf(t *testing.T, file string) []word {
	t.Helper()
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("no pdftotext")
	}
	out, err := exec.Command("pdftotext", "-bbox", file, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	var ws []word
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "<word ") {
			continue
		}
		var x float64
		fmt.Sscanf(line[strings.Index(line, `xMin="`)+6:], "%f", &x)
		text := line[strings.Index(line, ">")+1 : strings.LastIndex(line, "</word>")]
		ws = append(ws, word{text, x / 72})
	}
	return ws
}

func TestRevisedPagesOnly(t *testing.T) {
	anna3 := lineOf(t, "room 3, slowly")
	anna11 := lineOf(t, "room 11, slowly")
	s := revisedScript(t, map[int]string{anna3: "Blue", anna11: "Blue"})
	all := readRevised(t, writeRevised(t, s, testSets))
	want := (&pdf.PDFWriter{Revisions: testSets}).RevisedPages(s)
	if len(want) != 2 {
		t.Fatalf("revised pages %v, want 2", want)
	}
	r := *testSets
	r.PagesOnly = true
	only := readRevised(t, writeRevised(t, s, &r))
	if len(only) != len(want) || len(all) <= len(only) {
		t.Fatalf("%d pages of %d, want %d", len(only), len(all), len(want))
	}
	for i, p := range only {
		if n, _ := asterisks(p); n == 0 || header(p) == "" {
			t.Errorf("page %d of the revised: no mark or header", i)
		}
		// numbered as in the whole script (its first page has none)
		num := ""
		for _, l := range p {
			if m := pageNumberAfter.FindString(l.Text); m != "" {
				num = strings.TrimSuffix(strings.TrimSpace(m), ".")
			}
		}
		if num == "" {
			num = "1"
		}
		if n, _ := strconv.Atoi(num); want[n] == "" {
			t.Errorf("page %d is numbered %s; the revised pages are %v", i, num, want)
		}
	}
	// no change at all: an error
	plain := revisedScript(t, nil)
	file := filepath.Join(t.TempDir(), "none.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Revisions: &r}).Write(nil, plain); err == nil {
		t.Error("revised pages only of a script without changes: no error")
	}
}

func TestRevisionPageColors(t *testing.T) {
	anna3 := lineOf(t, "room 3, slowly")
	s := revisedScript(t, map[int]string{anna3: "Blue"})
	r := *testSets
	r.PageColors = true
	file := writeRevised(t, s, &r)
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("no pdftoppm")
	}
	dir := t.TempDir()
	if err := exec.Command("pdftoppm", "-r", "10", "-png", file, filepath.Join(dir, "p")).Run(); err != nil {
		t.Fatal(err)
	}
	names, _ := filepath.Glob(filepath.Join(dir, "p*.png"))
	if len(names) < 3 {
		t.Fatalf("%d pages rendered", len(names))
	}
	tinted := 0
	for _, n := range names {
		if pixelAt(t, n) != [3]uint8{255, 255, 255} {
			tinted++
		}
	}
	if tinted != 1 {
		t.Errorf("%d pages tinted, want the one changed", tinted)
	}
}

func TestFountainParseLines(t *testing.T) {
	s := fountain.ParseLines(fountain.Scene, strings.NewReader("INT. A - DAY\n\nShe runs.\n\nBOB\nHi.\n"))
	want := map[string]int{"INT. A - DAY": 1, "She runs.": 3, "BOB": 5, "Hi.": 6}
	for _, l := range s {
		if n, ok := want[l.Contents]; ok && l.Source != n {
			t.Errorf("%q from line %d, want %d", l.Contents, l.Source, n)
		}
	}
	for _, l := range fountain.Parse(fountain.Scene, strings.NewReader("INT. A - DAY\n")) {
		if l.Source != 0 {
			t.Error("Parse gives sources")
		}
	}
}

// pageNumberAfter is the page number at a header's end ("… 12.").
var pageNumberAfter = regexp.MustCompile(`\s*\d+[A-Z]?\.$`)

// pixelAt is the colour of a PNG's pixel near its top left corner.
func pixelAt(t *testing.T, name string) [3]uint8 {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(2, 2).RGBA()
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

// A marked speech in dual dialogue, and a page whose last text is styled
// (bold, italic lyrics): the marks and the header in the plain font.
func TestRevisionMarksDualAndStyled(t *testing.T) {
	text := "INT. ROOM - DAY #1#\n\nANNA\nWe go now.\n\nBOB ^\nWe stay.\n\n**A bold line of action.**\n\n~*la la la, sung in italics*\n"
	s := fountain.ParseLines(fountain.Scene, strings.NewReader(text))
	for i := range s {
		if s[i].Source == 7 || s[i].Source == 9 { // Bob's line; the bold action
			s[i].Revision = "Blue"
		}
	}
	file := writeRevised(t, s, testSets)
	words := wordsOf(t, file)
	stars := 0
	for _, w := range words {
		if w.text == "*" {
			stars++
		}
	}
	if stars != 3 { // the dual dialogue's two lines (a block marked whole), the action's one
		t.Errorf("%d marks, want the dual dialogue's two and the action's", stars)
	}
	pages := readRevised(t, file)
	if header(pages[0]) != "Blue Revision – 20/11/26" {
		t.Errorf("header %q", header(pages[0]))
	}
	if dir := os.Getenv("REVISION_SHOT"); dir != "" {
		data, _ := os.ReadFile(file)
		os.WriteFile(filepath.Join(dir, "styled.pdf"), data, 0o644)
	}
}
