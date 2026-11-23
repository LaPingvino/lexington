package lex

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

// {{INCLUDE: file}}, as Highland 2 does it (highwater-too's includes.ts,
// MIT; highland.go): on a line of its own, another file's text is printed
// where the line is, its title page left out, read as the including
// script is. Paths are the including file's: relative to its folder.
// Included files include others in turn; a file that includes itself,
// over any number of others, is a cycle, reported and not followed, as is
// nesting deeper than MaxIncludeDepth. A file that is not there prints a
// line saying so. Only printing does this: the script, and the Fountain
// written of it, keep the line as it is.

// MaxIncludeDepth is how deep includes go.
const MaxIncludeDepth = 16

// IncludeLoader reads the file an {{INCLUDE: path}} names, path as
// written, from the file (its id) that includes it: its id (as the
// absolute path: the same file, the same id) and its screenplay. An
// error that is fs.ErrNotExist is a file that is not there.
type IncludeLoader func(path, from string) (id string, s Screenplay, err error)

// IncludeProblem is an include that could not be done.
type IncludeProblem struct {
	From    string // the including file's id
	Path    string // as written
	Source  int    // the include's line in the root script (Line.Source), 0 not known
	Message string
}

func (p IncludeProblem) Error() string { return p.Message }

// HasIncludes tells whether s has an {{INCLUDE}} line.
func HasIncludes(s Screenplay) bool {
	for _, l := range s {
		if includePath(l) != "" {
			return true
		}
	}
	return false
}

// includePath is the file a line of nothing but directives includes,
// "" for none.
func includePath(l Line) string {
	if !IsDirectiveLine(l) {
		return ""
	}
	for _, d := range FindDirectives(l.Contents) {
		if d.Kind == DirectiveInclude && d.Path != "" {
			return d.Path
		}
	}
	return ""
}

// ResolveIncludes is s (read from the file from, its id) with its
// included files' lines after each {{INCLUDE}} line (which stays, for
// the writers to leave out). Their lines' Source is the include line's,
// so that a printed page leads back to the script being written. Where a
// file cannot be included, an action line says why; the problems are
// returned as well. s itself is not changed.
func ResolveIncludes(s Screenplay, from string, load IncludeLoader) (Screenplay, []IncludeProblem) {
	var problems []IncludeProblem
	var resolve func(s Screenplay, from string, stack []string, source int) Screenplay
	resolve = func(s Screenplay, from string, stack []string, source int) Screenplay {
		if source == 0 && !HasIncludes(s) {
			return s
		}
		out := make(Screenplay, 0, len(s))
		for _, l := range s {
			if source > 0 {
				l.Source = source
			}
			out = append(out, l)
			path := includePath(l)
			if path == "" {
				continue
			}
			src := l.Source
			fail := func(msg string) {
				problems = append(problems, IncludeProblem{From: from, Path: path, Source: src, Message: msg})
				out = append(out, Line{Type: TypeAction, Contents: msg, Source: src})
			}
			if len(stack) >= MaxIncludeDepth {
				fail(fmt.Sprintf("Includes are nested more than %d deep at %q", MaxIncludeDepth, path))
				continue
			}
			id, inc, err := load(path, from)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				fail(fmt.Sprintf("Included file not found: %q", path))
				continue
			case err != nil:
				fail(fmt.Sprintf("Could not read included file %q: %v", path, err))
				continue
			case slices.Contains(stack, id):
				var names []string
				for _, f := range append(slices.Clone(stack), id) {
					names = append(names, baseName(f))
				}
				fail("Include cycle: " + strings.Join(names, " → "))
				continue
			}
			out = append(out, resolve(includedBody(inc), id, append(slices.Clone(stack), id), src)...)
		}
		return out
	}
	return resolve(s, from, []string{from}, 0), problems
}

// includedBody is an included screenplay as it is printed in another:
// without its title page, nor blank lines at its ends.
func includedBody(s Screenplay) Screenplay {
	if len(s) > 0 && s[0].Type == TypeTitlePage {
		i := slices.IndexFunc(s, func(l Line) bool { return l.Type == TypeNewPage })
		if i < 0 {
			return nil
		}
		s = s[i+1:]
	}
	for len(s) > 0 && s[0].Type == TypeEmpty {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1].Type == TypeEmpty {
		s = s[:len(s)-1]
	}
	return s
}

// baseName is a file's name without its folders (ids are paths, with /
// or \).
func baseName(id string) string {
	return id[strings.LastIndexAny(id, `/\`)+1:]
}
