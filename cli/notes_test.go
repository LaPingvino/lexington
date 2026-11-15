package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/markdown"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
)

// The script of the report on Fountain's notes, laid out as Fountain.
const notesScript = `Title: Test

INT. HOUSE - DAY [[COLOR BLUE]]

Action with an inline [[note here]] in it.

[[A standalone note]]

BOB
Hello [[dialogue note]] there.

.MONTAGE OF THINGS [[red]]

= A synopsis [[blue]]

# Section [[green]]

EXT. STREET - NIGHT #12# [[red]]

Done.
`

// The shape of a real script with notes: every heading ends in a color
// note, synopses under each heading and section, notes inside the text
// and over several lines.
const colorScript = `Title: Colors
Author: Someone

# Act One [[COLOR RED]]

= The first act. [[blue]]

INT. KITCHEN - DAY [[COLOR BLUE]]

= Anna cooks. [[remark]]

Anna stirs the pot.[[Is it soup?
` + "  " + `
Or stew.]] She tastes it.

ANNA
(to herself) [[beat]]
Too salty. [[fix the line]]

EXT. GARDEN - NIGHT #2# [[COLOR GREEN]]

= Anna goes out. [[#3366ff]]

[[Cut this scene?]]

Rain falls.

## Kitchen again [[COLOR GREEN]]

= Back inside.

INT. KITCHEN - LATER [[some note]] [[COLOR RED]]

= Anna sits.

ANNA [[lead]]
Done.

CUT TO: [[later]]

>THE END< [[end note]]
`

// written writes a script in a format and returns the text that is
// printed: the file, the XML of a document's text, the text a PDF reads
// back as and its outline.
func written(t *testing.T, script, format string) string {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "t."+format)
	o := Options{Input: in, Output: out, To: format, ConfigFile: filepath.Join(dir, "none.toml")}
	if err := Convert(context.Background(), o); err != nil {
		t.Fatalf("%s: %v", format, err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	switch format {
	case "pdf":
		s, err := pdfin.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, l := range s {
			b.WriteString(l.Contents + "\n")
		}
		// the outline's titles (UTF-16: without its byte order mark and
		// zero bytes)
		for _, m := range regexp.MustCompile(`/Title \(([^)]*)\)`).FindAllSubmatch(data, -1) {
			title := bytes.ReplaceAll(bytes.TrimPrefix(m[1], []byte{0xfe, 0xff}), []byte{0}, nil)
			b.WriteString("outline: " + string(title) + "\n")
		}
		return b.String()
	case "docx", "odt":
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, f := range z.File {
			if strings.HasSuffix(f.Name, ".xml") {
				r, _ := f.Open()
				x, _ := io.ReadAll(r)
				r.Close()
				b.Write(x)
			}
		}
		// the text without its markup
		return regexp.MustCompile(`<[^>]*>`).ReplaceAllString(b.String(), " ")
	}
	return string(data)
}

// No output prints a note: html, pdf (its text and its outline), latex,
// fdx, docx, odt, markdown. Fountain and lex keep them all.
func TestNotesAreNeverPrinted(t *testing.T) {
	for name, c := range map[string]struct {
		script string
		shown  []string // printed, in every output
	}{
		"report": {notesScript, []string{"INT. HOUSE - DAY", "Action with an inline in it.", "Hello there.",
			"MONTAGE OF THINGS", "EXT. STREET - NIGHT"}},
		"colors": {colorScript, []string{"INT. KITCHEN - DAY", "Anna stirs the pot. She tastes it.", "Too salty.",
			"EXT. GARDEN - NIGHT", "Rain falls.", "INT. KITCHEN - LATER", "THE END"}},
	} {
		for _, format := range []string{"html", "pdf", "latex", "fdx", "docx", "odt", "markdown"} {
			var out string
			if format == "markdown" { // with pandoc it is an intermediate; the writer itself
				var b bytes.Buffer
				s := fountain.Parse(rules.DefaultConf().Scenes["en"], strings.NewReader(c.script))
				if err := (&markdown.MarkdownWriter{}).Write(&b, s); err != nil {
					t.Fatal(err)
				}
				out = b.String()
			} else {
				out = written(t, c.script, format)
			}
			if format == "fdx" { // where notes are Final Draft's script notes, not printed
				out = regexp.MustCompile(`(?s)<ScriptNote[ >].*?</ScriptNote>`).ReplaceAllString(out, "")
			}
			bads := []string{"[[", "]]", "note", "COLOR", "Cut this", "stew"}
			if format != "markdown" { // which prints synopses, as quotes
				bads = append(bads, "A synopsis", "The first act", "Anna cooks", "Back inside")
			}
			for _, bad := range bads {
				if i := strings.Index(out, bad); i >= 0 {
					t.Errorf("%s, %s: prints %q: …%s…", name, format, bad, out[max(0, i-60):min(len(out), i+60)])
				}
			}
			for _, s := range c.shown {
				// LaTeX writes its dashes and apostrophes its own way
				if format == "latex" {
					s = strings.ReplaceAll(s, "'", "`")
				}
				if !strings.Contains(out, s) {
					t.Errorf("%s, %s: lacks %q", name, format, s)
				}
			}
		}
		for _, format := range []string{"fountain", "lex"} {
			out := written(t, c.script, format)
			if format == "lex" { // its text, read back
				var b strings.Builder
				for _, l := range lex.Parse(strings.NewReader(out)) {
					b.WriteString(l.Contents + "\n")
				}
				out = b.String()
			}
			for _, n := range lex.Notes(c.script) {
				if !strings.Contains(out, n) {
					t.Errorf("%s, %s: lost the note %q:\n%s", name, format, n, out)
				}
			}
		}
	}
	// the PDF's outline: headings and sections without their notes
	out := written(t, colorScript, "pdf")
	for _, s := range []string{"outline: Act One\n", "outline: INT. KITCHEN - DAY\n", "outline: Kitchen again\n",
		"outline: INT. KITCHEN - LATER\n"} {
		if !strings.Contains(out, s) {
			t.Errorf("pdf outline lacks %q:\n%s", s, out)
		}
	}
}

// The colors of the headings, sections and synopses of the script, for a
// navigator (Accolade's), from the parsed script.
func TestNoteColorsOfAScript(t *testing.T) {
	var got []string
	for _, l := range fountain.Parse(rules.DefaultConf().Scenes["en"], strings.NewReader(colorScript)) {
		switch l.Type {
		case lex.TypeScene:
			h, n := lex.SceneNumber(l.Contents)
			got = append(got, "scene "+h+" "+n+" "+lex.NoteColor(l.Contents))
		case lex.TypeSection:
			got = append(got, "section "+lex.SectionText(l.Contents)+" "+lex.NoteColor(l.Contents))
		case lex.TypeSynopsis:
			got = append(got, "synopsis "+lex.StripNotes(l.Contents)+" "+lex.NoteColor(l.Contents))
		}
	}
	want := []string{
		"section Act One red",
		"synopsis The first act. blue",
		"scene INT. KITCHEN - DAY  blue",
		"synopsis Anna cooks. ",
		"scene EXT. GARDEN - NIGHT 2 green",
		"synopsis Anna goes out. #3366ff",
		"section Kitchen again green",
		"synopsis Back inside. ",
		"scene INT. KITCHEN - LATER  red",
		"synopsis Anna sits. ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
