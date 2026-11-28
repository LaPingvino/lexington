package fountain

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// The writer forces a line only where the strict reading of what it
// wrote would read it otherwise: "FADE IN:" before a blank line is
// action by itself, capitals with text below them would be a cue, a
// line ending in "TO:" a transition.
func TestWriterMinimalMarks(t *testing.T) {
	act := func(s string) lex.Line { return lex.Line{Type: lex.TypeAction, Contents: s} }
	empty := lex.Line{Type: lex.TypeEmpty}
	s := lex.Screenplay{
		act("FADE IN:"), empty,
		{Type: lex.TypeScene, Contents: "INT. HALL - DAY"}, empty,
		act("BOB"), act("walks in."), empty,
		act("CUT TO:"), act("the hall."), empty,
		act("!Bang."), empty,
		act("THE END"), empty,
		{Type: lex.TypeTrans, Contents: "SMASH CUT:"}, empty,
		act("FADE OUT."),
	}
	var b strings.Builder
	if err := (&FountainWriter{SceneConfig: []string{"INT", "EXT"}}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"\nFADE IN:\n", "\n!BOB\nwalks in.\n", "\n!CUT TO:\nthe hall.\n", "\n!!Bang.\n", "\nTHE END\n", "\n> SMASH CUT:\n", "\nFADE OUT.\n"} {
		if !strings.Contains("\n"+out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	var got, wantRows []string
	for _, l := range s {
		wantRows = append(wantRows, string(l.Type)+": "+l.Contents)
	}
	for _, l := range Parse([]string{"INT", "EXT"}, strings.NewReader(out)) {
		got = append(got, string(l.Type)+": "+l.Contents)
	}
	got = got[:len(got)-1] // the blank line the parser ends with
	if strings.Join(got, "\n") != strings.Join(wantRows, "\n") {
		t.Errorf("reads back as\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantRows, "\n"))
	}
}
