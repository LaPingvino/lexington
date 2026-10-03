package fdx

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
)

var scenes = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

func writeFDX(t *testing.T, src string) (string, FdxFile) {
	t.Helper()
	var buf bytes.Buffer
	if err := (&FDXWriter{}).Write(&buf, fountain.Parse(scenes, strings.NewReader(src))); err != nil {
		t.Fatal(err)
	}
	var doc FdxFile
	if err := xml.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, buf.String())
	}
	return buf.String(), doc
}

// summary renders paragraphs as "Type:text" (styled runs as [Style]text).
func summary(ps []FdxParagraph) []string {
	var out []string
	for _, p := range ps {
		if p.DualDialogue != nil {
			out = append(out, "DUAL{"+strings.Join(summary(p.DualDialogue.Paragraphs), " | ")+"}")
			continue
		}
		var b strings.Builder
		b.WriteString(p.Type)
		if p.Alignment != "" {
			b.WriteString("/" + p.Alignment)
		}
		if p.StartsNewPage != "" {
			b.WriteString("/newpage")
		}
		b.WriteString(":")
		for _, t := range p.Texts {
			if t.Style != "" {
				b.WriteString("[" + t.Style + "]")
			}
			b.WriteString(t.Content)
		}
		out = append(out, b.String())
	}
	return out
}

func TestWriteDocumentHeader(t *testing.T) {
	out, doc := writeFDX(t, "INT. BARN - DAY\n")
	if !strings.HasPrefix(out, `<?xml version="1.0" encoding="UTF-8"`) {
		t.Errorf("missing XML declaration: %q", out[:40])
	}
	if doc.DocumentType != "Script" || doc.Template != "No" || doc.Version == "" {
		t.Errorf("FinalDraft attributes = %q %q %q", doc.DocumentType, doc.Template, doc.Version)
	}
}

func TestWriteScript(t *testing.T) {
	src := strings.Join([]string{
		"Title: The Barn",
		"Credit: Written by",
		"Author: Jane Smith",
		"Contact:",
		"    1 Writer's Lane",
		"",
		"FADE IN:",
		"",
		"INT. BARN - DAY",
		"",
		"Rain *hammers* the **roof** & _the_ <door>.",
		"",
		"",
		"Silence.",
		"",
		"JOHN",
		"(beat)",
		"It's ***coming***.",
		"",
		"> THE END <",
		"",
		"===",
		"",
		"EXT. FIELD - NIGHT",
		"",
		"BOB",
		"Run!",
		"",
		"ALICE ^",
		"Where?",
		"",
		"CUT TO:",
	}, "\n")
	out, doc := writeFDX(t, src)

	if doc.TitlePage == nil {
		t.Fatal("no title page")
	}
	title := strings.Join(summary(doc.TitlePage.Content.Paragraphs), "\n")
	wantTitle := "General/Center:The Barn\nGeneral/Center:\nGeneral/Center:Written by\n" +
		"General/Center:Jane Smith\nGeneral:\nGeneral:1 Writer's Lane"
	if title != wantTitle {
		t.Errorf("title page:\n%s\nwant:\n%s", title, wantTitle)
	}

	body := strings.Join(summary(doc.Content.Paragraphs), "\n")
	wantBody := strings.Join([]string{
		"Action:FADE IN:",
		"Scene Heading:INT. BARN - DAY",
		"Action:Rain [Italic]hammers the [Bold]roof & [Underline]the <door>.",
		"Action:", // the extra blank line
		"Action:Silence.",
		"Character:JOHN",
		"Parenthetical:(beat)",
		"Dialogue:It's [Bold+Italic]coming.",
		"Action/Center:THE END",
		"Scene Heading/newpage:EXT. FIELD - NIGHT",
		"DUAL{Character:BOB | Dialogue:Run! | Character:ALICE | Dialogue:Where?}",
		"Transition:CUT TO:",
	}, "\n")
	if body != wantBody {
		t.Errorf("body:\n%s\nwant:\n%s", body, wantBody)
	}
	if !strings.Contains(out, "&amp; ") || !strings.Contains(out, "&lt;door&gt;") {
		t.Error("special characters are not escaped in the XML")
	}
}

func TestWriteWithTemplate(t *testing.T) {
	tmpl := filepath.Join(t.TempDir(), "t.fdx")
	if err := os.WriteFile(tmpl, []byte(`{{range .Paragraphs}}[{{.Type}}]`+
		`{{range .Texts}}{{.Content}}{{end}}{{end}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	sp := fountain.Parse(scenes, strings.NewReader("INT. BARN - DAY\n\nTom & Jerry.\n"))
	if err := (&FDXWriter{TemplatePath: tmpl}).Write(&buf, sp); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "[Scene Heading]INT. BARN - DAY[Action]Tom &amp; Jerry." {
		t.Errorf("template output = %q", got)
	}
}
