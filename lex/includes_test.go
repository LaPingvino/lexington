package lex

import (
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"testing"
)

// The cases of highwater-too's includes.test.ts (MIT).

func memoryLoader(files map[string]Screenplay) IncludeLoader {
	return func(path, from string) (string, Screenplay, error) {
		s, ok := files[path]
		if !ok {
			return "", nil, fmt.Errorf("open %s: %w", path, fs.ErrNotExist)
		}
		return path, s, nil
	}
}

func act(s string) Line     { return Line{Type: TypeAction, Contents: s} }
func include(p string) Line { return act("{{INCLUDE: " + p + "}}") }

func texts(s Screenplay) []string {
	var out []string
	for _, l := range s {
		out = append(out, l.Contents)
	}
	return out
}

// Nested, the included title page left out, the included lines' Source
// the include's.
func TestResolveIncludesNested(t *testing.T) {
	main := Screenplay{act("One."), {Type: TypeEmpty}, {Type: TypeAction, Contents: "{{INCLUDE: a}}", Source: 3}, {Type: TypeEmpty}, act("Four.")}
	a := Screenplay{{Type: TypeTitlePage}, {Type: TypeTitle, Contents: "Not printed"}, {Type: TypeNewPage}, {Type: TypeEmpty}, act("Two."), {Type: TypeEmpty}, include("b"), {Type: TypeEmpty}}
	b := Screenplay{act("Three.")}
	got, problems := ResolveIncludes(main, "main", memoryLoader(map[string]Screenplay{"a": a, "b": b}))
	if problems != nil {
		t.Fatal(problems)
	}
	want := []string{"One.", "", "{{INCLUDE: a}}", "Two.", "", "{{INCLUDE: b}}", "Three.", "", "Four."}
	if !reflect.DeepEqual(texts(got), want) {
		t.Errorf("%q", texts(got))
	}
	if got[6].Source != 3 || got[3].Source != 3 {
		t.Errorf("sources %d %d", got[3].Source, got[6].Source)
	}
	var printed []string
	for _, l := range Printable(got, nil) {
		printed = append(printed, l.Contents)
	}
	if want := []string{"One.", "", "Two.", "", "Three.", "", "Four."}; !reflect.DeepEqual(printed, want) {
		t.Errorf("printed %q", printed)
	}
	if main[2].Contents != "{{INCLUDE: a}}" || len(main) != 5 {
		t.Error("the screenplay changed")
	}
}

func TestResolveIncludesProblems(t *testing.T) {
	main := Screenplay{include("a"), {Type: TypeEmpty}, include("missing")}
	files := map[string]Screenplay{"a": {act("A."), {Type: TypeEmpty}, include("b")}, "b": {include("a")}}
	got, problems := ResolveIncludes(main, "main", memoryLoader(files))
	var msgs []string
	for _, p := range problems {
		msgs = append(msgs, p.From+": "+p.Message)
	}
	if want := []string{"b: Include cycle: main → a → b → a", `main: Included file not found: "missing"`}; !reflect.DeepEqual(msgs, want) {
		t.Errorf("%q", msgs)
	}
	want := []string{"{{INCLUDE: a}}", "A.", "", "{{INCLUDE: b}}", "{{INCLUDE: a}}", "Include cycle: main → a → b → a", "",
		"{{INCLUDE: missing}}", `Included file not found: "missing"`}
	if !reflect.DeepEqual(texts(got), want) {
		t.Errorf("%q", texts(got))
	}
	_, problems = ResolveIncludes(Screenplay{include("x")}, "m", func(string, string) (string, Screenplay, error) {
		return "", nil, errors.New("permission denied")
	})
	if len(problems) != 1 || problems[0].Message != `Could not read included file "x": permission denied` {
		t.Errorf("%v", problems)
	}
	// a file that includes itself, deeper than the depth: the cycle is found first
	_, problems = ResolveIncludes(Screenplay{include("self")}, "self", memoryLoader(map[string]Screenplay{"self": {include("self")}}))
	if len(problems) != 1 || problems[0].Message != "Include cycle: self → self" {
		t.Errorf("%v", problems)
	}
}
