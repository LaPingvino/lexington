package pdf_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
)

// Emphasis prints without its markers, escapes as the characters, and
// text that only looks like markup (a < sign, a lone asterisk) as it is.
func TestEmphasisPrints(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.pdf")
	s := lex.Screenplay{
		{Type: lex.TypeAction, Contents: `Steel enters the code on the keypad: **\*9765\***`},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeCenter, Contents: "_**THE END**_"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeAction, Contents: "He dialed *69 and then *23, and *a < b* & c."},
	}
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	back, err := pdfin.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, l := range back {
		b.WriteString(l.Contents + "\n")
	}
	got := b.String()
	for _, want := range []string{"Steel enters the code on the keypad: *9765*", "THE END", "He dialed *69 and then *23, and a < b & c."} {
		if !strings.Contains(lex.Unstyled(got), want) && !strings.Contains(got, want) {
			t.Errorf("lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "_") || strings.Contains(got, `\`) {
		t.Errorf("markers printed:\n%s", got)
	}
}
