package fountain

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

var en = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

// body parses a script without a title page, without its blank lines.
func body(script string) lex.Screenplay {
	var out lex.Screenplay
	for _, l := range Parse(en, strings.NewReader(script)) {
		if l.Type != lex.TypeEmpty {
			out = append(out, l)
		}
	}
	return out
}

// Notes are kept in the text as they are written (a heading's capitals
// leave them alone) and do not change what a line is.
func TestParseNotes(t *testing.T) {
	for _, c := range []struct {
		script string
		want   lex.Line
	}{
		{"INT. HOUSE - DAY [[COLOR BLUE]]\n", lex.Line{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY [[COLOR BLUE]]"}},
		{"int. house - day [[red]]\n", lex.Line{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY [[red]]"}},
		{"EXT. STREET - NIGHT #12# [[red]]\n", lex.Line{Type: lex.TypeScene, Contents: "EXT. STREET - NIGHT #12# [[red]]"}},
		{".MONTAGE OF THINGS [[COLOR RED]]\n", lex.Line{Type: lex.TypeScene, Contents: "MONTAGE OF THINGS [[COLOR RED]]"}},
		{"..MONTAGE OF THINGS [[COLOR RED]]\n", lex.Line{Type: lex.TypeAction, Contents: "..MONTAGE OF THINGS [[COLOR RED]]"}},
		{"= A synopsis [[blue]]\n", lex.Line{Type: lex.TypeSynopsis, Contents: "A synopsis [[blue]]"}},
		{"# War 6 [[red]]\n", lex.Line{Type: lex.TypeSection, Contents: "# War 6 [[red]]"}},
		{"Action with [[a note]] inside.\n", lex.Line{Type: lex.TypeAction, Contents: "Action with [[a note]] inside."}},
		{"[[A standalone note]]\n", lex.Line{Type: lex.TypeNote, Contents: "[[A standalone note]]"}},
		{"[[one]] [[two]]\n", lex.Line{Type: lex.TypeNote, Contents: "[[one]] [[two]]"}},
		{"CUT TO: [[later]]\n", lex.Line{Type: lex.TypeTrans, Contents: "CUT TO: [[later]]"}},
		{">THE END< [[x]]\n", lex.Line{Type: lex.TypeCenter, Contents: "THE END [[x]]"}},
		{"> fade out. [[x]]\n", lex.Line{Type: lex.TypeTrans, Contents: "FADE OUT. [[x]]"}},
		{"Action [[todo: fix]] first.\n", lex.Line{Type: lex.TypeAction, Contents: "Action [[todo: fix]] first."}},
		{"Unterminated [[note\n\nINT. HOUSE - DAY\n", lex.Line{Type: lex.TypeAction, Contents: "Unterminated [[note"}},
	} {
		got := body(c.script)
		if len(got) == 0 || got[0] != c.want {
			t.Errorf("%q: %q, want %q", c.script, got, c.want)
		}
	}
}

func TestParseNotesInDialogue(t *testing.T) {
	got := body("BOB [[the lead]]\n(beat) [[or not]]\nHello [[dialogue note]] there.\n[[a note in the speech]]\nAnd more.\n")
	want := lex.Screenplay{
		{Type: lex.TypeSpeaker, Contents: "BOB [[the lead]]"},
		{Type: lex.TypeParen, Contents: "(beat) [[or not]]"},
		{Type: lex.TypeDialog, Contents: "Hello [[dialogue note]] there."},
		{Type: lex.TypeNote, Contents: "[[a note in the speech]]"},
		{Type: lex.TypeDialog, Contents: "And more."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	// a name with only a note after it is action, as without the note
	got = body("BOB\n[[a note]]\n\nRain.\n")
	if got[0].Type != lex.TypeAction {
		t.Errorf("a name with only a note after it: %q", got)
	}
}

// A note over several lines is one element; a line of two spaces keeps
// it going, a blank line ends it, and a "[[" that is not closed before a
// blank line is no note.
func TestParseNoteOverLines(t *testing.T) {
	script := "INT. HOUSE - DAY\n\nScott pauses.[[This section needs work.\nEither that, or I need coffee.\n  \nDefinitely coffee.]] He looks around.\n\n[[A note\nover two lines]]\n\nBOB\nHi.\n"
	got := body(script)
	want := lex.Screenplay{
		{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY"},
		{Type: lex.TypeAction, Contents: "Scott pauses.[[This section needs work.\nEither that, or I need coffee.\n  \nDefinitely coffee.]] He looks around."},
		{Type: lex.TypeNote, Contents: "[[A note\nover two lines]]"},
		{Type: lex.TypeSpeaker, Contents: "BOB"},
		{Type: lex.TypeDialog, Contents: "Hi."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
	if s := lex.StripNotes(got[1].Contents); s != "Scott pauses. He looks around." {
		t.Errorf("printed: %q", s)
	}

	// not closed before a blank line: the lines are what they would be
	got = body("Rain [[falls\nhard.\n\nINT. HOUSE - DAY\n\nBOB\nHi.]]\n")
	want = lex.Screenplay{
		{Type: lex.TypeAction, Contents: "Rain [[falls"},
		{Type: lex.TypeAction, Contents: "hard."},
		{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY"},
		{Type: lex.TypeSpeaker, Contents: "BOB"},
		{Type: lex.TypeDialog, Contents: "Hi.]]"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("unclosed: %q", got)
	}
}

// Fountain written from a script keeps every note as it is, and reads
// back the same.
func TestNotesRoundTrip(t *testing.T) {
	script := `Title: Test [[working title]]

INT. HOUSE - DAY [[COLOR BLUE]]

Action with an inline [[note here]] in it.

[[A standalone note]]

BOB [[lead]]
Hello [[dialogue note]] there.

BANG [[sound]]

.MONTAGE OF THINGS [[red]]

= A synopsis [[blue]]

# Section [[green]]

EXT. STREET - NIGHT #12# [[red]]

Scott pauses.[[This section needs work.
Either that, or I need coffee.

Definitely coffee.]] He looks around.

CUT TO: [[later]]
`
	first := Parse(en, strings.NewReader(script))
	var b bytes.Buffer
	if err := (&FountainWriter{SceneConfig: en}).Write(&b, first); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, n := range lex.Notes(script) {
		if !strings.Contains(out, "[["+n+"]]") {
			t.Errorf("the note %q is lost:\n%s", n, out)
		}
	}
	if strings.Count(out, "[[") != strings.Count(script, "[[") {
		t.Errorf("%d notes, want %d:\n%s", strings.Count(out, "[["), strings.Count(script, "[["), out)
	}
	second := Parse(en, strings.NewReader(out))
	if !reflect.DeepEqual(first, second) {
		t.Errorf("read back differently:\n%q\n%q\n%s", first, second, out)
	}
}
