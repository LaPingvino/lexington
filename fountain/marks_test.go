package fountain

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
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
	if err := (&FountainWriter{SceneConfig: Scene[:2]}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"\nFADE IN:\n", "\n!BOB\nwalks in.\n", "\n!CUT TO:\nthe hall.\n", "\n!!Bang.\n", "\nTHE END\n",
		"\n> SMASH CUT:\n", "\nFADE OUT.\n",
	} {
		if !strings.Contains("\n"+out, want) {
			t.Errorf("no %q in\n%s", want, out)
		}
	}
	var got, wantRows []string
	for _, l := range s {
		wantRows = append(wantRows, string(l.Type)+": "+l.Contents)
	}
	for _, l := range Parse(Scene[:2], strings.NewReader(out)) {
		got = append(got, string(l.Type)+": "+l.Contents)
	}
	got = got[:len(got)-1] // the blank line the parser ends with
	if strings.Join(got, "\n") != strings.Join(wantRows, "\n") {
		t.Errorf("reads back as\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(wantRows, "\n"))
	}
}

// A FountainWriter with neither scene words nor a language writes
// English Fountain: an INT. heading needs no dot, OMITTED and Esperanto's
// ENE. KUIREJO do; with Esperanto's scene words ENE. needs none, and
// with ones that do not have INT, INT. needs its dot.
func TestWriterDefaultLanguage(t *testing.T) {
	empty := lex.Line{Type: lex.TypeEmpty}
	scene := func(s string) lex.Line { return lex.Line{Type: lex.TypeScene, Contents: s} }
	s := lex.Screenplay{
		scene("INT. OFFICE KITCHEN - DAY"), empty,
		scene("OMITTED"), empty,
		scene("ENE. KUIREJO - TAGO"), empty,
		{Type: lex.TypeAction, Contents: "Int. the pipe bursts."},
	}
	for _, c := range []struct {
		w    *FountainWriter
		want []string
	}{
		{&FountainWriter{}, []string{
			"INT. OFFICE KITCHEN - DAY", ".OMITTED", ".ENE. KUIREJO - TAGO", "!Int. the pipe bursts.",
		}},
		{&FountainWriter{SceneConfig: []string{"ENE", "EKST"}}, []string{
			".INT. OFFICE KITCHEN - DAY", ".OMITTED", "ENE. KUIREJO - TAGO", "Int. the pipe bursts.",
		}},
	} {
		var b strings.Builder
		if err := c.w.Write(&b, s); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, line := range strings.Split(b.String(), "\n") {
			if line != "" {
				got = append(got, line)
			}
		}
		if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%v: got\n%s\nwant\n%s", c.w.SceneConfig, strings.Join(got, "\n"), strings.Join(c.want, "\n"))
		}
	}
}

// A cue in a script without capitals keeps its @ even where the reading
// would take it as one by its context (before a parenthetical, with an
// extension); a Latin cue in capitals needs none; a full-width
// parenthetical is written as it is.
func TestWriterCaselessCues(t *testing.T) {
	empty := lex.Line{Type: lex.TypeEmpty}
	act := func(s string) lex.Line { return lex.Line{Type: lex.TypeAction, Contents: s} }
	cue := func(s string) lex.Line { return lex.Line{Type: lex.TypeSpeaker, Contents: s} }
	par := func(s string) lex.Line { return lex.Line{Type: lex.TypeParen, Contents: s} }
	say := func(s string) lex.Line { return lex.Line{Type: lex.TypeDialog, Contents: s} }
	const latin = "ANNA"
	for lang, s := range map[string]lex.Screenplay{
		langZh: {act("下雨。"), empty, cue("玛丽亚"), par("（轻声）"), say("你好。"), empty,
			cue("玛丽亚（画外音）"), say("再见。"), empty, cue(latin), say("Hi.")},
		hebrew: {act("גשם."), empty, cue("מריה (V.O.)"), say("שלום.")},
		langAr: {act("مطر."), empty, cue("سمير"), par("(لنفسه)"), say("لا.")},
	} {
		l := rules.DefaultConf().Language(lang)
		var b strings.Builder
		if err := (&FountainWriter{SceneConfig: l.Scenes, Language: &l}).Write(&b, s); err != nil {
			t.Fatal(err)
		}
		out := b.String()
		for _, row := range s {
			if row.Type != lex.TypeSpeaker {
				continue
			}
			want := "\n@" + row.Contents + "\n"
			if row.Contents == latin {
				want = "\n" + latin + "\n"
			}
			if !strings.Contains("\n"+out, want) {
				t.Errorf("%s: no %q in\n%s", lang, want, out)
			}
		}
		if lang == langZh && !strings.Contains(out, "\n（轻声）\n") {
			t.Errorf("parenthetical rewritten:\n%s", out)
		}
	}
}
