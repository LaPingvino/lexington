package trelby

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// theEnd is the centred line of Joop's script.
const theEnd = "THE END OF THE BEGINNING"

// Joop's script as Trelby 2.4's Fountain import made it
// (testdata/lexington-check.trelby): its "> THE END OF THE BEGINNING <"
// is action indented by 18 spaces, (60 - 24) / 2, and Trelby shows it
// centred. It reads as centred, the rest as it is.
func TestTrelbysOwnCentring(t *testing.T) {
	data, err := os.ReadFile("testdata/lexington-check.trelby")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n.."+strings.Repeat(" ", 18)+theEnd+"\n") {
		t.Fatal("the fixture is not as Trelby wrote it")
	}
	s, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	centred := 0
	for _, l := range s {
		switch {
		case l.Type == lex.TypeCenter:
			centred++
			if l.Contents != theEnd {
				t.Errorf("centred: %q", l.Contents)
			}
		case strings.Contains(l.Contents, theEnd):
			t.Errorf("%s: %q", l.Type, l.Contents)
		}
	}
	if centred != 1 {
		t.Errorf("%d centred lines:\n%s", centred, formattest.Dump(s))
	}
	// the scenes, speeches and the rest as they are
	for _, want := range []string{"scene: EXT. GARDEN - CONTINUOUS", "speaker: MARIA (V.O.)", "trans: CUT TO:",
		"action: Jonas alone, holding the letter."} {
		found := false
		for _, r := range formattest.Rows(s, nil) {
			found = found || r == want
		}
		if !found {
			t.Errorf("no %q in:\n%s", want, formattest.Dump(s))
		}
	}
	// and written again, as Trelby wrote it, which reads the same
	var b bytes.Buffer
	if err = (&TrelbyWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "\n.."+strings.Repeat(" ", 18)+theEnd+"\n") {
		t.Errorf("written:\n%s", b.String())
	}
	back, err := Parse(&b)
	if err != nil {
		t.Fatal(err)
	}
	// the script's rows (Trelby's title page is placed otherwise than the
	// writer places one)
	script := func(s lex.Screenplay) []string {
		rows := formattest.Rows(s, nil)
		for i, r := range rows {
			if r == string(lex.TypeNewPage)+": " {
				return rows[i+1:]
			}
		}
		return rows
	}
	formattest.Same(t, "written again", script(s), script(back))
}

// A centred line is written padded as Trelby pads one, by the width of
// the file's config when it reads one; one too wide for that keeps
// Fountain's markers; an indented action line that is not centred so
// stays action.
func TestCentredPadding(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"THE END", strings.Repeat(" ", 26) + "THE END"},
		{"Ünïcödé", strings.Repeat(" ", 26) + "Ünïcödé"},
		{strings.Repeat("x", 57), "> " + strings.Repeat("x", 57) + " <"},
		{strings.Repeat("x", 56), "  " + strings.Repeat("x", 56)},
	} {
		if got := centre(c.in, defaultActionWidth); got != c.want {
			t.Errorf("centre(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	read := func(config, body string) lex.Screenplay {
		t.Helper()
		s, err := Parse(strings.NewReader("#Version 3\n#Begin-Config \n" + config + "#End-Config \n#Start-Script \n" + body))
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, c := range []struct {
		config, body string
		want         lex.ElementType
	}{
		{"", ".." + strings.Repeat(" ", 26) + "THE END\n", lex.TypeCenter},
		{"Element/Action/Width:40\n", ".." + strings.Repeat(" ", 16) + "THE END\n", lex.TypeCenter},
		{"Element/Action/Width:40\n", ".." + strings.Repeat(" ", 26) + "THE END\n", lex.TypeAction},
		{"", "..    An indented line.\n", lex.TypeAction},
		{"", "|." + strings.Repeat(" ", 26) + "THE END\n.." + strings.Repeat(" ", 28) + "FIN\n", lex.TypeCenter},
		{"", "..> THE END <\n", lex.TypeCenter},
	} {
		s := read(c.config, c.body)
		if len(s) == 0 || s[0].Type != c.want {
			t.Errorf("%q with %q:\n%s", c.body, c.config, formattest.Dump(s))
		}
	}
}
