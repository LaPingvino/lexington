package cli

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/pdfin"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// pdfText is a PDF's text, its lines joined by "|".
func pdfText(t *testing.T, p string) string {
	t.Helper()
	pages, err := pdfin.Pages(p)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, pg := range pages {
		for _, l := range pg {
			out = append(out, strings.TrimSpace(l.Text))
		}
	}
	return strings.Join(out, "|")
}

// Includes on the disk: each relative to the file that includes it,
// nested, a cycle and a missing file each a line in print; Fountain
// written keeps the directive.
func TestIncludesPrinted(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"main.fountain":              "INT. MAIN - DAY\n\nOne.\n\n{{INCLUDE: acts/act.fountain}}\n\n{{INCLUDE: gone.fountain}}\n\nFour.\n",
		"acts/act.fountain":          "Title: Not printed\n\nTwo.\n\n{{INCLUDE: scenes/three.fountain}}\n",
		"acts/scenes/three.fountain": "Three.\n\n{{INCLUDE: ../act.fountain}}\n",
	})
	out := filepath.Join(dir, "main.pdf")
	logged := captureLog(t, func() { run(t, Options{Input: filepath.Join(dir, "main.fountain"), Output: out}) })
	got := pdfText(t, out)
	want := "INT. MAIN - DAY|One.|Two.|Three.|Include cycle: main.fountain → act.fountain → three.fountain → act.fountain|Included file not found: \"gone.fountain\"|Four."
	if strings.ReplaceAll(got, "→|", "→ ") != want { // (the cycle's line broken)
		t.Errorf("printed\n%s\nwant\n%s", got, want)
	}
	if !strings.Contains(logged, `Included file not found: "gone.fountain"`) {
		t.Errorf("not logged: %s", logged)
	}
	ft := filepath.Join(dir, "out.fountain")
	run(t, Options{Input: filepath.Join(dir, "main.fountain"), Output: ft})
	if data, _ := os.ReadFile(ft); !strings.Contains(string(data), "{{INCLUDE: acts/act.fountain}}") || strings.Contains(string(data), "Two.") {
		t.Errorf("Fountain:\n%s", data)
	}
	// `check` warns about them, with their lines
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	code := checkBundles([]string{filepath.Join(dir, "main.fountain")})
	w.Close()
	os.Stdout = stdout
	report, _ := io.ReadAll(r)
	for _, want := range []string{"warning: line 7: Included file not found: \"gone.fountain\"",
		"warning: line 5: Include cycle: main.fountain → act.fountain → three.fountain → act.fountain", "ok"} {
		if !strings.Contains(string(report), want) {
			t.Errorf("check: no %q in\n%s", want, report)
		}
	}
	if code != 0 {
		t.Errorf("check's code %d", code)
	}
}

// The page setup's header and footer, for a script without its own.
func TestPageTextsFromOptions(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"plain.fountain": "INT. ROOM - DAY\n\nText.\n",
		"own.fountain":   "{{HEADER: Its own}}\n\nINT. ROOM - DAY\n\nText.\n",
	})
	for name, want := range map[string]string{"plain": "Set up|INT. ROOM - DAY|Text.|Footer 1", "own": "Its own|INT. ROOM - DAY|Text.|Footer 1"} {
		out := filepath.Join(dir, name+".pdf")
		run(t, Options{Input: filepath.Join(dir, name+".fountain"), Output: out, Header: "Set up", Footer: "Footer %p"})
		if got := pdfText(t, out); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

// A real Highland file's include (ScreenplayEdgeCases's IncludedScene,
// beside it) prints after its INCLUDE line.
func TestHighlandSampleInclude(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sec.pdf")
	run(t, Options{Input: filepath.Join("..", "bundle", "testdata", "highland", "ScreenplayEdgeCases.highland"), Output: out})
	got := pdfText(t, out)
	if !strings.Contains(got, "INT. INCLUDED SCENE - DAY|This scene comes from an included file.") {
		t.Errorf("not included: %.400s", got)
	}
	if !strings.Contains(got, "Left|Centre|Right") && !strings.Contains(got, "Left Centre Right") {
		t.Errorf("no footer: %.400s", got)
	}
}

// captureLog is what f logs.
func captureLog(t *testing.T, f func()) string {
	t.Helper()
	var b strings.Builder
	old := log.Writer()
	log.SetOutput(&b)
	defer log.SetOutput(old)
	f()
	return b.String()
}
