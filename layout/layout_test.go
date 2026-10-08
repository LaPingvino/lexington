package layout

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
)

const sample = `Title: THE TEST
Credit: Written by
Author: Somebody
Draft date: October 8, 2026

INT. KITCHEN - DAY

Anna opens the **fridge**. It is *empty*, and that is a long sentence that has to wrap around the sixty characters of an action line.

ANNA
(sighing)
Nothing again.

CAROL
Who took the milk?

BRAM
Not me.

DIRK ^
Nor me.

CUT TO:
`

func lay(t *testing.T) []Line {
	t.Helper()
	return Lay(fountain.Parse([]string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "I/E"}, strings.NewReader(sample)), nil)
}

func find(lines []Line, typ lex.ElementType, text string) (Line, bool) {
	for _, l := range lines {
		if l.Type == typ && strings.Contains(l.Text(), text) {
			return l, true
		}
	}
	return Line{}, false
}

func TestColumns(t *testing.T) {
	lines := lay(t)
	for _, c := range []struct {
		typ    lex.ElementType
		text   string
		indent int
		align  byte
	}{
		{lex.TypeScene, "INT. KITCHEN", 0, 'L'},
		{lex.TypeSpeaker, "ANNA", 22, 'L'},
		{lex.TypeParen, "sighing", 16, 'L'},
		{lex.TypeDialog, "Nothing again", 10, 'L'},
		{lex.TypeTrans, "CUT TO", 0, 'R'},
	} {
		l, ok := find(lines, c.typ, c.text)
		if !ok {
			t.Errorf("no %s %q in %+v", c.typ, c.text, lines)
			continue
		}
		if l.Indent != c.indent || l.Align != c.align {
			t.Errorf("%s: indent %d align %c, want %d %c", c.typ, l.Indent, l.Align, c.indent, c.align)
		}
	}
	if l, _ := find(lines, lex.TypeScene, "KITCHEN"); !l.Spans[0].Bold {
		t.Error("scene headings are bold")
	}
	if l, _ := find(lines, lex.TypeTrans, "CUT TO"); !strings.HasSuffix(l.Padded(), "CUT TO:") || len(l.Padded()) != 60 {
		t.Errorf("transition padded %q (%d)", l.Padded(), len(l.Padded()))
	}
}

func TestTitlePage(t *testing.T) {
	lines := lay(t)
	title, ok := find(lines, "Title", "THE TEST")
	if !ok || title.Block != "title" || title.Align != 'C' {
		t.Errorf("title %+v", title)
	}
	if p := title.Padded(); strings.TrimLeft(p, " ") != "THE TEST" || len(p)-len("THE TEST") != 26 {
		t.Errorf("centred title %q", p)
	}
	if d, ok := find(lines, "Draft date", "October"); !ok || d.Block != "meta" || d.Align != 'L' {
		t.Errorf("draft date %+v", d)
	}
	breaks := 0
	for _, l := range lines {
		if l.PageBreak {
			breaks++
		}
	}
	if breaks != 1 {
		t.Errorf("%d page breaks after the title page", breaks)
	}
}

func TestEmphasisAndWrapping(t *testing.T) {
	lines := lay(t)
	var action []Line
	for _, l := range lines {
		if l.Type == lex.TypeAction {
			action = append(action, l)
		}
	}
	if len(action) < 3 {
		t.Fatalf("the long action line is not wrapped: %+v", action)
	}
	for _, l := range action {
		if n := len([]rune(l.Text())); n > l.Width || l.Width != 60 {
			t.Errorf("line of %d in width %d: %q", n, l.Width, l.Text())
		}
	}
	var fridge, empty bool
	for _, s := range action[0].Spans {
		fridge = fridge || (s.Text == "fridge" && s.Bold && !s.Italic)
		empty = empty || (s.Text == "empty" && s.Italic && !s.Bold)
	}
	if !fridge || !empty || strings.Contains(action[0].Text(), "*") {
		t.Errorf("emphasis %+v", action[0].Spans)
	}
}

func TestDualDialogue(t *testing.T) {
	lines := lay(t)
	bram, _ := find(lines, lex.TypeSpeaker, "BRAM")
	dirk, _ := find(lines, lex.TypeSpeaker, "DIRK")
	me, _ := find(lines, lex.TypeDialog, "Nor me")
	// the columns of afterwriting, Better Fountain and screenplain:
	// dialogue at 2" and 5", the speaker 5 characters in
	if bram.Column != 1 || dirk.Column != 2 || bram.Indent != 10 || dirk.Indent != 40 || me.Indent != 35 || me.Width != 25 {
		t.Errorf("dual columns: bram %+v dirk %+v nor me %+v", bram, dirk, me)
	}
	if carol, _ := find(lines, lex.TypeSpeaker, "CAROL"); carol.Column != 0 || carol.Indent != 22 {
		t.Errorf("carol %+v", carol)
	}
}
