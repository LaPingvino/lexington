package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal"
)

// flagIn and flagOut are the input's and the output's flags.
const flagIn, flagOut = "-i", "-o"

// Fountain through each other program's format and back to Fountain: a
// line of notes before a centred line, a transition and a heading comes
// back as a line of notes before it, never in its text, while a
// heading's colour, a link to the story adapted and a note in a speech
// stay on their line; a centred line
// stays centred; and the Fountain written forces only what needs it
// ("FADE IN:" and "FADE OUT." are action by themselves; capitals with
// text right below them, TestWriterMinimalMarks).
func TestNotesAndMarksThroughFormats(t *testing.T) {
	const src = `Title: Notes

FADE IN:

INT. HALL - DAY

Bob waits.

[[Note before centred.]]

> THE END <

[[Note before transition.]]

CUT TO:

[[Note before heading.]]

EXT. GARDEN - DAY [[blue]]

Rain. [[from: #:~:text=It rained]]

BOB
Hi. [[speech note]]

FADE OUT.
`
	dir := t.TempDir()
	in := filepath.Join(dir, "script.fountain")
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Inputs { // the other programs' formats
		ext := f.Key
		if ext == internal.FormatFountain || ext == internal.FormatPDF || ext == internal.FormatLex {
			continue
		}
		t.Run(ext, func(t *testing.T) {
			mid := filepath.Join(dir, "script."+ext)
			back := filepath.Join(dir, ext+".fountain")
			if code := Main(context.Background(), []string{flagIn, in, flagOut, mid}); code != 0 {
				t.Fatalf("-o %s: exit %d", ext, code)
			}
			if code := Main(context.Background(), []string{flagIn, mid, flagOut, back}); code != 0 {
				t.Fatalf("-i %s: exit %d", ext, code)
			}
			if ext == "osf" {
				doc, err := os.ReadFile(mid)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(doc), `<document type="Open Screenplay Format document" version="50">`) {
					t.Errorf("not OSF 5.0:\n%.300s", doc)
				}
			}
			data, err := os.ReadFile(back)
			if err != nil {
				t.Fatal(err)
			}
			got := "\n" + string(data)
			for _, w := range []string{
				"\nFADE IN:\n", "\nFADE OUT.\n",
				"\n[[Note before centred.]]\n\n>THE END<\n",
				"\n[[Note before transition.]]\n\nCUT TO:\n",
				"\n[[Note before heading.]]\n\nEXT. GARDEN - DAY",
			} {
				if !strings.Contains(got, w) {
					t.Errorf("no %q in\n%s", w, got)
				}
			}
			if ext == "trelby" {
				return // Trelby's notes are elements of their own, after the line they were in
			}
			for _, w := range []string{
				"\nEXT. GARDEN - DAY [[blue]]\n", "\nRain. [[from: #:~:text=It rained]]\n", "\nBOB\nHi. [[speech note]]\n",
			} {
				if !strings.Contains(got, w) {
					t.Errorf("no %q in\n%s", w, got)
				}
			}
		})
	}
}
