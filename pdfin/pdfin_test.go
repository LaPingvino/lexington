package pdfin

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

var scenes = rules.DefaultConf().Scenes["en"]

// fountainOf writes a screenplay as Fountain, without what a PDF cannot
// give back: sections, page breaks and the blank lines around them.
func fountainOf(t *testing.T, s lex.Screenplay) string {
	t.Helper()
	var b strings.Builder
	if err := (&fountain.FountainWriter{SceneConfig: scenes}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	out := regexp.MustCompile(`(?m)^(#.*|===)\n\n?`).ReplaceAllString(b.String(), "")
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(strings.TrimSpace(out), "\n\n")
}

// A script printed by Lexington reads back as the same script.
func TestRoundTrip(t *testing.T) {
	src, err := os.ReadFile("../examples/tv-episode.fountain")
	if err != nil {
		t.Fatal(err)
	}
	orig := fountain.Parse(scenes, strings.NewReader(string(src)))
	file := filepath.Join(t.TempDir(), "tv.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, orig); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := fountainOf(t, got), fountainOf(t, orig); a != b {
		t.Errorf("read back:\n%s\n\nwant:\n%s", a, b)
	}
}

func ln(page int, y float64, runs ...run) line { return line{Page: page, Y: y, Runs: runs} }
func at(x float64, text string) run {
	return run{X: x, End: x + 0.1*float64(len([]rune(text))), Text: text}
}

// What other programs print at page breaks: page numbers, (MORE), the
// name again with (CONT'D), scene numbers in both margins.
func TestPageBreaks(t *testing.T) {
	const step = 1.0 / 6
	page1 := []line{
		ln(1, 0.5, at(7.0, "1.")),
		ln(1, 1.0, at(1.0, "12"), at(1.5, "INT. HOUSE - DAY"), at(7.6, "12")),
		ln(1, 1.0+2*step, at(1.5, "Anna sits.")),
		ln(1, 1.0+4*step, at(3.7, "ANNA")),
		ln(1, 1.0+5*step, at(2.5, "I have been waiting")),
		ln(1, 1.0+6*step, at(2.5, "all morning")),
		ln(1, 1.0+7*step, at(3.7, "(MORE)")),
	}
	page2 := []line{
		ln(2, 0.5, at(7.0, "2.")),
		ln(2, 1.0, at(3.7, "ANNA (CONT'D)")),
		ln(2, 1.0+step, at(2.5, "for this biscuit.")),
		ln(2, 1.0+3*step, at(1.5, "She eats it.")),
	}
	s := classify([][]line{page1, page2}, 8.5)
	var got []string
	for _, l := range s {
		got = append(got, string(l.Type)+": "+l.Contents)
	}
	want := []string{
		"scene: INT. HOUSE - DAY #12#",
		"empty: ",
		"action: Anna sits.",
		"empty: ",
		"speaker: ANNA",
		"dialog: I have been waiting all morning for this biscuit.",
		"empty: ",
		"action: She eats it.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTitlePage(t *testing.T) {
	page := []line{
		ln(1, 4.0, at(3.75, "BISCUITS")),
		ln(1, 4.5, at(3.65, "Written by")),
		ln(1, 4.8, at(3.6, "Anna Smith")),
		ln(1, 9.0, at(1.5, "anna@example.com")),
		ln(1, 9.2, at(1.5, "Draft, May 2026")),
	}
	s, ok := titlePage(page, 8.5)
	if !ok {
		t.Fatal("no title page")
	}
	var got []string
	for _, l := range s {
		got = append(got, string(l.Type)+"="+l.Contents)
	}
	want := "titlepage=|Title=BISCUITS|Credit=Written by|Author=Anna Smith|metasection=|Contact=anna@example.com|Draft date=Draft, May 2026|newpage="
	if strings.Join(got, "|") != want {
		t.Errorf("got %s", strings.Join(got, "|"))
	}
	// a first page with a scene is no title page
	if _, ok := titlePage([]line{ln(1, 1, at(1.5, "INT. HOUSE - DAY"))}, 8.5); ok {
		t.Error("scene page taken for the title page")
	}
}
