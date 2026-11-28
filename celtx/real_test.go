package celtx

import (
	"bytes"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// The projects in testdata are Lexington's, opened and saved by Celtx
// 2.x itself (testdata/README.md).
const chameleon, biscuits = "a-chameleon", "biscuits"

func realProject(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name + ".celtx")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rowsOf(s lex.Screenplay, typ lex.ElementType) []string {
	var out []string
	for _, l := range s {
		if l.Type == typ {
			out = append(out, l.Contents)
		}
	}
	return out
}

// TestRealScenes: the scenes Celtx saved read back, OMITTED among them.
func TestRealScenes(t *testing.T) {
	for name, want := range map[string][]string{
		chameleon: {
			"EXT. MARKET SQUARE - DAY", "EXT. GENERAL ZHIGALOV'S YARD - DAY",
			"EXT. PITCHUGIN'S TIMBER-YARD - DAY", "OMITTED", "EXT. MARKET SQUARE - DAY",
			"EXT. PITCHUGIN'S TIMBER-YARD - DAY",
		},
		biscuits: {"INT. OFFICE KITCHEN - DAY", "INT. OPEN-PLAN OFFICE - CONTINUOUS", "INT. OFFICE KITCHEN - LATER"},
	} {
		s, err := Parse(bytes.NewReader(realProject(t, name)))
		if err != nil {
			t.Fatal(err)
		}
		formattest.Same(t, name+" scenes", want, rowsOf(s, lex.TypeScene))
	}
	s, err := Parse(bytes.NewReader(realProject(t, biscuits)))
	if err != nil {
		t.Fatal(err)
	}
	formattest.Same(t, "biscuits centred", []string{"COLD OPEN", "MAIN TITLES", "ACT ONE", "END OF ACT ONE", "ACT TWO"},
		rowsOf(s, lex.TypeCenter))
	formattest.Same(t, "biscuits transitions", []string{"SMASH CUT TO:"}, rowsOf(s, lex.TypeTrans))
	if !slices.ContainsFunc(s, func(l lex.Line) bool { return l.Type == lex.TypeDualOpen }) {
		t.Error("biscuits: no dual dialogue")
	}
}

// TestRealCatalogue: the Cast and Props Celtx made its Master Catalog of
// from Lexington's tags, and the words they are on.
func TestRealCatalogue(t *testing.T) {
	s, set, byRow, err := ParseTags(realProject(t, chameleon), "")
	if err != nil {
		t.Fatal(err)
	}
	if set == nil {
		t.Fatal("no catalogue")
	}
	var cast, props []string
	for _, e := range set.Elements {
		c, _ := set.Category(e.Category)
		switch c.Name {
		case lex.CastCategory:
			cast = append(cast, e.Name)
		case "Props":
			props = append(props, e.Name)
		default:
			t.Errorf("%s in %q", e.Name, c.Name)
		}
	}
	sort.Strings(cast)
	sort.Strings(props)
	formattest.Same(t, "cast", []string{
		"HRYUKIN", "OTCHUMYELOV", "PROHOR", "SOMEONE IN THE CROWD", "VOICE", "VOICE IN THE CROWD", "YELDYRIN",
	}, cast)
	formattest.Same(t, "props", []string{"parcel", "sieve full of confiscated gooseberries"}, props)
	var tagged []string
	for row, ts := range byRow {
		for _, tg := range ts {
			if s[row].Type != lex.TypeSpeaker {
				tagged = append(tagged, s[row].Type+": "+tg.Text)
			}
		}
	}
	sort.Strings(tagged)
	formattest.Same(t, "props on", []string{"action: parcel", "action: sieve full of confiscated gooseberries"}, tagged)
	speakers := 0
	for row := range byRow {
		if s[row].Type == lex.TypeSpeaker {
			speakers++
		}
	}
	if n := len(rowsOf(s, lex.TypeSpeaker)); speakers != n {
		t.Errorf("%d of %d cues tagged", speakers, n)
	}
}

// TestRealNotes: all 25 of A Chameleon's notes come back, each a link to
// the story ([[from: …]]).
func TestRealNotes(t *testing.T) {
	s, err := Parse(bytes.NewReader(realProject(t, chameleon)))
	if err != nil {
		t.Fatal(err)
	}
	var notes []string
	for _, l := range s {
		notes = append(notes, lex.Notes(l.Contents)...)
	}
	if len(notes) != 25 {
		t.Fatalf("%d notes: %q", len(notes), notes)
	}
	for _, n := range notes {
		if !strings.HasPrefix(n, "from: #:~:text=") {
			t.Errorf("note %q", n)
		}
	}
}

// TestRealTitlePage: the title page's fields, Source among them, and the
// cue extensions.
func TestRealTitlePage(t *testing.T) {
	for name, want := range map[string][]string{
		chameleon: {"Title: A Chameleon", "Credit: Adapted from the story by", "Author: Anton Chekhov",
			"Source: Translated by Constance Garnett. Accolade's sample script."},
		biscuits: {"Title: BISCUITS", `Credit: Episode 101: "The Last One"`, "Author: Lexington's examples"},
	} {
		s, err := Parse(bytes.NewReader(realProject(t, name)))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, l := range s[1:] {
			if l.Type == lex.TypeNewPage {
				break
			}
			got = append(got, l.Type+": "+l.Contents)
		}
		formattest.Same(t, name+" title page", want, got)
	}
	s, err := Parse(bytes.NewReader(realProject(t, chameleon)))
	if err != nil {
		t.Fatal(err)
	}
	for _, cue := range []string{"VOICE (O.S.)", "OTCHUMYELOV (CONT'D)"} {
		if !slices.Contains(rowsOf(s, lex.TypeSpeaker), cue) {
			t.Errorf("no cue %q", cue)
		}
	}
}

// TestRealRoundTrip: what Celtx saved, as Fountain, written as Celtx and
// read back is the same Fountain; its headings need no force mark but
// OMITTED's.
func TestRealRoundTrip(t *testing.T) {
	for _, name := range []string{chameleon, biscuits} {
		t.Run(name, func(t *testing.T) {
			s, err := Parse(bytes.NewReader(realProject(t, name)))
			if err != nil {
				t.Fatal(err)
			}
			first := fountainOf(t, s)
			for _, line := range strings.Split(first, "\n") {
				if strings.HasPrefix(line, ".") && line != ".OMITTED" {
					t.Errorf("forced heading %q", line)
				}
			}
			again := fountain.Parse(formattest.Scenes, strings.NewReader(first))
			back, err := Parse(bytes.NewReader(write(t, &CeltxWriter{}, again)))
			if err != nil {
				t.Fatal(err)
			}
			formattest.Same(t, name, strings.Split(first, "\n"), strings.Split(fountainOf(t, back), "\n"))
		})
	}
}

func fountainOf(t *testing.T, s lex.Screenplay) string {
	t.Helper()
	var b bytes.Buffer
	if err := (&fountain.FountainWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
