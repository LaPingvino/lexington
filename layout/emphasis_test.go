package layout

import (
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// Emphasis on top of an element's own style: a bold heading's italic
// words are bold italic; escapes and nesting as lex.Emphasis reads them.
func TestEmphasisOnBase(t *testing.T) {
	got := emphasis(`_Steel's face FILLS the *Leupold Mark 4* scope_ **\*9765\***`, Span{Bold: true})
	want := []Span{
		{Text: "Steel's face FILLS the ", Bold: true, Underline: true},
		{Text: "Leupold Mark 4", Bold: true, Italic: true, Underline: true},
		{Text: " scope", Bold: true, Underline: true},
		{Text: " *9765*", Bold: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if lines := Lay(lex.Screenplay{{Type: lex.TypeAction, Contents: "He dialed *69 and then *23, and then hung up."}}, nil); lines[0].Text() != "He dialed *69 and then *23, and then hung up." {
		t.Errorf("%q", lines[0].Text())
	}
}
