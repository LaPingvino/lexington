package fountain

import (
	"github.com/LaPingvino/lexington/rules"

	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// kinds is what each line of text is read as (line from 1), "" for one
// with no row of its own.
func kinds(text string, loose bool) []string {
	got := elementsOf(text, rules.Words(Scene), loose)
	out := make([]string, strings.Count(text, "\n")+1)
	for i := range out {
		out[i] = got[i+1]
	}
	// the end: the line after the last newline, the blank row read after it
	for len(out) > 0 && (out[len(out)-1] == "" || out[len(out)-1] == lex.TypeEmpty) {
		out = out[:len(out)-1]
	}
	return out
}

func TestStrictReading(t *testing.T) {
	for _, c := range []struct {
		name, text    string
		strict, loose []string
	}{
		// 1. a scene heading needs a blank line before it
		{"heading after action", "Bob waits.\nINT. HOUSE - DAY\n",
			[]string{"action", "action"}, []string{"action", "scene"}},
		{"heading in a speech", "BOB\nHello.\nINT. HOUSE - DAY\n",
			[]string{"speaker", "dialog", "dialog"}, []string{"speaker", "dialog", "scene"}},
		{"heading after a blank line", "Bob waits.\n\nINT. HOUSE - DAY\n",
			[]string{"action", "empty", "scene"}, nil},
		{"heading first", "INT. HOUSE - DAY\n", []string{"scene"}, nil},
		{"heading after a note line", "Bob waits.\n\n[[todo]]\nINT. HOUSE - DAY\n",
			[]string{"action", "empty", "note", "action"}, []string{"action", "empty", "note", "scene"}},
		{"forced heading anywhere", "BOB\nHello.\n.THE ROOF\n",
			[]string{"speaker", "dialog", "scene"}, nil},
		// 2. a character cue needs a blank line before it
		{"cue after action", "Bob waits.\nBOB\nHello.\n",
			[]string{"action", "action", "action"}, []string{"action", "speaker", "dialog"}},
		{"cue after a heading", "INT. HOUSE\nBOB\nHello.\n",
			[]string{"scene", "action", "action"}, []string{"scene", "speaker", "dialog"}},
		{"cue after a blank line", "Bob waits.\n\nBOB\nHello.\n",
			[]string{"action", "empty", "speaker", "dialog"}, nil},
		{"forced cue anywhere", "Bob waits.\n@BOB\nHello.\n",
			[]string{"action", "speaker", "dialog"}, nil},
		// 3. a transition: capitals, TO:, blank lines around it
		{"transition", "Bob waits.\n\nCUT TO:\n\nINT. HOUSE\n",
			[]string{"action", "empty", "trans", "empty", "scene"}, nil},
		{"transition in small letters", "Bob waits.\n\ncut to:\n\nINT. HOUSE\n",
			[]string{"action", "empty", "action", "empty", "scene"},
			[]string{"action", "empty", "trans", "empty", "scene"}},
		{"transition at the end of action", "INT. HOUSE\n\nBob waits and DISSOLVE TO:\n\nINT. GARDEN\n",
			[]string{"scene", "empty", "action", "empty", "scene"}, []string{"scene", "empty", "trans", "empty", "scene"}},
		// (the leniency: one ending in TO: needs no blank line after it;
		// the heading right below it still needs its blank line before)
		{"transition without a blank line after", "Bob waits.\n\nCUT TO:\nINT. HOUSE\n",
			[]string{"action", "empty", "trans", "action"},
			[]string{"action", "empty", "trans", "scene"}},
		{"transition right above action", "INT. HOUSE\n\nCUT TO:\nJonas alone, holding the letter.\n",
			[]string{"scene", "empty", "trans", "action"}, nil},
		{"transition without a blank line before", "Bob waits.\nCUT TO:\n\nINT. HOUSE\n",
			[]string{"action", "action", "empty", "scene"},
			[]string{"action", "trans", "empty", "scene"}},
		{"cue below a transition", "Bob waits.\n\nCUT TO:\nBOB\nHello.\n",
			[]string{"action", "empty", "trans", "action", "action"},
			[]string{"action", "empty", "trans", "speaker", "dialog"}},
		{"TO: in a speech", "BOB\nHello.\nCUT TO:\nGoodbye.\n",
			[]string{"speaker", "dialog", "dialog", "dialog"},
			[]string{"speaker", "dialog", "trans", "action"}},
		// cues the leniency leaves alone
		{"cue TOBIAS", "Bob waits.\n\nTOBIAS\nHello.\n",
			[]string{"action", "empty", "speaker", "dialog"}, nil},
		{"cue with an extension", "Bob waits.\n\nAMOS (CONT'D)\nHello.\n",
			[]string{"action", "empty", "speaker", "dialog"}, nil},
		{"cue ending in TO", "Bob waits.\n\nOTTO\nHello.\n",
			[]string{"action", "empty", "speaker", "dialog"}, nil},
		{"dialogue under a cue", "BOB\nHello.\nWHERE TO?\n",
			[]string{"speaker", "dialog", "dialog"}, nil},
		{"transition at the end", "Bob waits.\n\nCUT TO:", []string{"action", "empty", "trans"}, nil},
		{"forced transition anywhere", "Bob waits.\n> fade out.\nINT. HOUSE\n",
			[]string{"action", "trans", "action"}, []string{"action", "trans", "scene"}},
		// 4. two spaces keep a speech going
		{"two spaces in a speech", "BOB\nHello.\n  \nGoodbye.\n",
			[]string{"speaker", "dialog", "dialog", "dialog"},
			[]string{"speaker", "dialog", "empty", "action"}},
		{"more spaces in a speech", "BOB\nHello.\n    \nGoodbye.\n",
			[]string{"speaker", "dialog", "dialog", "dialog"},
			[]string{"speaker", "dialog", "empty", "action"}},
		{"two spaces after a name", "BOB\n  \nGoodbye.\n",
			[]string{"action", "empty", "action"}, nil},
		{"two spaces in action", "Bob waits.\n  \nBOB\nHello.\n",
			[]string{"action", "empty", "speaker", "dialog"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.loose == nil {
				c.loose = c.strict
			}
			if got := kinds(c.text, false); strings.Join(got, " ") != strings.Join(c.strict, " ") {
				t.Errorf("Parse: %v, want %v", got, c.strict)
			}
			if got := kinds(c.text, true); strings.Join(got, " ") != strings.Join(c.loose, " ") {
				t.Errorf("ParseLoose: %v, want %v", got, c.loose)
			}
			// what KeepLoose writes reads, the spec's way, as the old way
			kept, n := KeepLoose(c.text, Scene)
			if (n == 0) != (strings.Join(c.strict, " ") == strings.Join(c.loose, " ")) {
				t.Errorf("KeepLoose: %d places", n)
			}
			assertKept(t, c.text, kept)
		})
	}
}

// assertKept checks that kept reads as text read loosely, line for line
// (the blank lines KeepLoose added passed over).
func assertKept(t *testing.T, text, kept string) {
	t.Helper()
	want := elementsOf(text, rules.Words(Scene), true)
	got := elementsOf(kept, rules.Words(Scene), false)
	was := strings.Split(text, "\n")
	j := 0
	for i, l := range strings.Split(kept, "\n") {
		if j >= len(was) {
			t.Errorf("kept has more lines than blank ones added\n%s", kept)
			return
		}
		if strings.TrimSpace(l) == "" && strings.TrimSpace(was[j]) != "" {
			continue // a blank line added
		}
		if got[i+1] != want[j+1] {
			t.Errorf("kept line %d %q is %s, was %s\n%s", i+1, l, got[i+1], want[j+1], kept)
			return
		}
		j++
	}
}

func TestKeepLooseEdits(t *testing.T) {
	for _, c := range []struct {
		text, kept string
		places     int
	}{
		{"Bob waits.\nINT. HOUSE - DAY\n\nHe sits.", "Bob waits.\n\nINT. HOUSE - DAY\n\nHe sits.", 1},
		{"Bob waits.\nBOB\nHello.", "Bob waits.\n\nBOB\nHello.", 1},
		{"Bob waits.\n\ncut to:\n\nINT. HOUSE", "Bob waits.\n\n>cut to:\n\nINT. HOUSE", 1},
		{"INT. HOUSE\n\nWe fade and DISSOLVE TO:\n\nINT. GARDEN", "INT. HOUSE\n\n>We fade and DISSOLVE TO:\n\nINT. GARDEN", 1},
		{"BOB\nHello.\n  \nHe goes.", "BOB\nHello.\n\nHe goes.", 1},
		// a blank line before the heading would make BOB action: a mark
		{"BOB\nINT. HOUSE - DAY", "BOB\n.INT. HOUSE - DAY", 1},
		{"INT. HOUSE\nBOB\nHello.\nINT. GARDEN\n\nBob waits.", "INT. HOUSE\n\nBOB\nHello.\n\nINT. GARDEN\n\nBob waits.", 2},
		{"INT. HOUSE\n\nBOB\nHello.", "INT. HOUSE\n\nBOB\nHello.", 0},
	} {
		kept, n := KeepLoose(c.text, Scene)
		if kept != c.kept || n != c.places {
			t.Errorf("KeepLoose(%q) = %q, %d; want %q, %d", c.text, kept, n, c.kept, c.places)
		}
	}
}

// The scripts in the repository read the same both ways, and Fountain
// written from them reads as they do.
func TestStrictCorpus(t *testing.T) {
	files, _ := filepath.Glob("../testdata/input/*.fountain")
	more, _ := filepath.Glob("../examples/*.fountain")
	files = append(files, more...)
	if len(files) == 0 {
		t.Skip("no scripts")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		kept, _ := KeepLoose(text, Scene)
		assertKept(t, text, kept)
		// written as Fountain and read again: the same rows
		rows := Parse(Scene, strings.NewReader(text))
		var b bytes.Buffer
		if err := (&FountainWriter{SceneConfig: Scene}).Write(&b, rows); err != nil {
			t.Fatal(err)
		}
		again := Parse(Scene, &b)
		if len(rows) > 0 && rows[len(rows)-1].Type == lex.TypeEmpty && (len(again) == 0 || again[len(again)-1].Type != lex.TypeEmpty) {
			rows = rows[:len(rows)-1]
		}
		for i := range min(len(rows), len(again)) {
			if rows[i].Type != again[i].Type || rows[i].Contents != again[i].Contents {
				t.Errorf("%s: row %d %v, written and read again %v", f, i, rows[i], again[i])
				break
			}
		}
	}
}

// Fountain written from rows that the spec would read otherwise (from
// Final Draft, a PDF) has the marks it needs.
func TestWriteSpecValid(t *testing.T) {
	rows := lex.Screenplay{
		{Type: lex.TypeAction, Contents: "Bob waits."},
		{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY"},
		{Type: lex.TypeSpeaker, Contents: "BOB"},
		{Type: lex.TypeDialog, Contents: "Hello."},
		{Type: lex.TypeDialog, Contents: ""},
		{Type: lex.TypeDialog, Contents: "Goodbye."},
		{Type: lex.TypeTrans, Contents: "CUT TO:"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeAction, Contents: "INT. is written on the wall."},
	}
	var b bytes.Buffer
	if err := (&FountainWriter{SceneConfig: Scene}).Write(&b, rows); err != nil {
		t.Fatal(err)
	}
	want := "Bob waits.\n.INT. HOUSE - DAY\n@BOB\nHello.\n  \nGoodbye.\n> CUT TO:\n\n!INT. is written on the wall.\n"
	if b.String() != want {
		t.Errorf("written:\n%q\nwant\n%q", b.String(), want)
	}
	again := Parse(Scene, strings.NewReader(b.String()))
	for i, r := range rows {
		if again[i].Type != r.Type || again[i].Contents != r.Contents {
			t.Errorf("row %d read again as %v, was %v", i, again[i], r)
		}
	}
}

// The leniency is for a line ending in TO: only: a transition of the
// language's word list (SMASH CUT here) with blank lines around is none
// without the one after it, as before, nor is FADE TO TITLE:.
func TestTransitionLeniencyOnlyTO(t *testing.T) {
	lang := rules.Words(Scene)
	lang.Transitions = append(lang.Transitions, "SMASH CUT")
	if got := elementsOf("Bob waits.\n\nSMASH CUT\n", lang, false); got[3] != lex.TypeTrans {
		t.Fatalf("SMASH CUT with blank lines around: %v, want a transition", got)
	}
	if got := elementsOf("Bob waits.\n\nSMASH CUT\nThe end.\n", lang, false); got[3] == lex.TypeTrans {
		t.Errorf("SMASH CUT with no blank line after: %v, want no transition", got)
	}
	if got := elementsOf("Bob waits.\n\nFADE TO TITLE:\nHOPE TALKS\n", lang, false); got[3] == lex.TypeTrans {
		t.Errorf("FADE TO TITLE: with no blank line after: %v, want no transition", got)
	}
}
