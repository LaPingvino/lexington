package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Highland's header and footer as the printed page's margin boxes: %p
// and %pp the page counters, on the first page too, none on a title page.
func TestHighlandPageBoxes(t *testing.T) {
	s := lex.Screenplay{
		{Type: lex.TypeAction, Contents: `{{HEADER: "Draft" | %p of %pp}}`},
		{Type: lex.TypeAction, Contents: "{{FOOTER: Centre}}"},
		{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
	}
	var buf bytes.Buffer
	if err := (&HTMLWriter{Elements: rules.Default}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`@top-left { content: "\"Draft\"";`,
		`@top-right { content: counter(page) " of " counter(pages);`,
		`@bottom-center { content: "Centre";`,
		`@page :first { @top-left`,
		`@page title { @top-left { content: none; }`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s", want)
		}
	}
	if strings.Contains(out, "HEADER") {
		t.Error("the directive printed")
	}
}
