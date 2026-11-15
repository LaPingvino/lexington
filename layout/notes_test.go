package layout

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

const notesSample = `INT. HOUSE - DAY #12# [[COLOR BLUE]]

Action with [[a note]] inside.

[[A standalone note]]

BOB
Hello [[dialogue note]] there.
`

func layNotes(set rules.Set) []string {
	var out []string
	for _, l := range Lay(fountain.Parse([]string{"INT", "EXT"}, strings.NewReader(notesSample)), set) {
		out = append(out, strings.TrimSpace(l.Text())+"|"+l.SceneNumber)
	}
	return out
}

// Notes are not printed: inside a line they go with the space before
// them, a line of notes goes with one of the blank lines around it.
func TestNotesAreNotLaidOut(t *testing.T) {
	got := strings.Join(layNotes(nil), "\n")
	want := strings.Join([]string{"INT. HOUSE - DAY|12", "|", "Action with inside.|", "|", "BOB|", "Hello there.|", "|"}, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// With a printnotes rule a line of notes is printed in its format, the
// note's text without brackets; notes inside lines are still not.
func TestPrintNotesRule(t *testing.T) {
	set := rules.Set{}
	for k, f := range rules.Default {
		set[k] = f
	}
	set[rules.KeyPrintNotes.String()] = rules.Format{Left: 2, Right: 1, Style: "i"}
	got := strings.Join(layNotes(set), "\n")
	want := strings.Join([]string{"INT. HOUSE - DAY|12", "|", "Action with inside.|", "|", "A standalone note|", "|", "BOB|", "Hello there.|", "|"}, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	for _, p := range Paragraphs(fountain.Parse(nil, strings.NewReader("[[A note]]\n")), set) {
		if p.Text() == "A note" && (p.Key != "printnotes" || !p.Spans[0].Italic) {
			t.Errorf("the note's paragraph: %+v", p)
		}
	}
	if rules.Default.PrintsNotes() {
		t.Error("the default rules print notes")
	}
}
