// Package formattest is what the tests of the formats Lexington reads and
// writes share: the synthetic scripts, and a comparison of two scripts
// that says where they part.
package formattest

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
)

// Scenes are the scene words the scripts are read with.
var Scenes = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "I/E"}

// Script is every element a format may hold: a title page, a section and
// synopses, numbered headings, notes, emphasis, dual dialogue, lyrics, a
// centred line, a page break and a transition.
const Script = `Title: _**BRICK & STEEL**_
Credit: Written by
Author: Jo Doe
Source: Based on a story
Draft date: 1 May 2026
Contact:
    Jo Doe
    jo@example.com

# Act One

= The opening.

INT. HALL - DAY #1#

= A synopsis.

John walks in, *slowly*, and **stops**. [[a note]]

[[a note of its own]]

JOHN
(quietly)
Hello _there_.
How are you?

MARY ^
Hi.

JOHN (V.O.)
Fine.

> THE END <

===

EXT. STREET - NIGHT #2A#

Rain falls on "quotes" & <brackets>.

~la la la
~and on

CUT TO:
`

// Read reads Fountain.
func Read(src string) lex.Screenplay {
	return fountain.Parse(Scenes, strings.NewReader(src))
}

// Inputs are the synthetic Fountain scripts of testdata/input and Script,
// by name.
func Inputs(t *testing.T) map[string]string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	files, err := filepath.Glob(filepath.Join(filepath.Dir(here), "..", "..", "testdata", "input", "*.fountain"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no test scripts: %v", err)
	}
	out := map[string]string{"script": Script}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = string(data)
	}
	return out
}

// Rows are a script's rows as "type: contents", those keep leaves out
// (nil: all but the blank ones) left out.
func Rows(s lex.Screenplay, keep func(lex.Line) bool) []string {
	var out []string
	for _, l := range s {
		if keep == nil && l.Type == lex.TypeEmpty || keep != nil && !keep(l) {
			continue
		}
		row := string(l.Type) + ": " + l.Contents
		if l.Revision != "" {
			row += " {" + l.Revision + "}"
		}
		out = append(out, row)
	}
	return out
}

// Same fails the test where two scripts' rows part.
func Same(t *testing.T, what string, want, got []string) {
	t.Helper()
	for i := 0; i < len(want) || i < len(got); i++ {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			t.Errorf("%s: first difference at row %d:\n  want %q\n  got  %q", what, i, w, g)
			return
		}
	}
}

// Dump is a script as Lexington's lex format, for a failure's message.
func Dump(s lex.Screenplay) string {
	var b bytes.Buffer
	_ = (&lex.LexWriter{}).Write(&b, s)
	return b.String()
}
