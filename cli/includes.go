package cli

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/lexington/bundle"
	"github.com/LaPingvino/lexington/fdx"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Highland's {{INCLUDE}}s and the page's header and footer on the
// command line (lex/highland.go, lex/includes.go).

// IncludeLoader reads the files {{INCLUDE}}s name from the disk, each
// relative to the folder of the file that includes it: a bundle
// (.highland, .textbundle…) its text, an .fdx as Final Draft's, anything
// else as Fountain in o's language. Their ids are their absolute paths.
func IncludeLoader(o Options) lex.IncludeLoader {
	lang := includeConf(o).Language(cmpOr(o.SceneIn, "en"))
	return func(path, from string) (string, lex.Screenplay, error) {
		full := path
		if !filepath.IsAbs(full) {
			full = filepath.Join(filepath.Dir(from), filepath.FromSlash(path))
		}
		if abs, err := filepath.Abs(full); err == nil {
			full = abs
		}
		var text []byte
		if bundle.Is(full) || isDir(full) {
			b, err := bundle.Read(full)
			if err != nil {
				if _, serr := os.Stat(full); serr != nil {
					return full, nil, serr
				}
				return full, nil, err
			}
			if text, err = b.ReadText(); err != nil {
				return full, nil, err
			}
		} else {
			var err error
			if text, err = os.ReadFile(full); err != nil {
				return full, nil, err
			}
		}
		if strings.EqualFold(filepath.Ext(full), ".fdx") {
			s, err := fdx.ParseWithError(bytes.NewReader(text))
			if err != nil {
				return full, nil, fmt.Errorf("not Final Draft: %w", err)
			}
			return full, s, nil
		}
		return full, fountain.ParseLinesIn(lang, bytes.NewReader(text)), nil
	}
}

// includeConf is o's settings, for the language included files are read
// in: Lexington's own when o names no file that is there.
func includeConf(o Options) rules.TOMLConf {
	if o.Conf == nil {
		if _, err := os.Stat(o.ConfigFile); o.ConfigFile == "" || err != nil {
			return rules.DefaultConf()
		}
	}
	return confOf(o)
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// includeFrom is the file o's script's includes are relative to: its
// IncludeFrom, or its Input; with neither, a file in the working folder.
func includeFrom(o Options) string {
	from := cmpOr(o.IncludeFrom, o.Input)
	if from == "" || from == "-" {
		from = "-"
	}
	if abs, err := filepath.Abs(from); err == nil {
		from = abs
	}
	return from
}

// ResolveIncludes is the script with its included files put in
// (lex.ResolveIncludes), read from the disk as IncludeLoader reads them,
// relative to o's IncludeFrom (or Input).
func ResolveIncludes(s lex.Screenplay, o Options) (lex.Screenplay, []lex.IncludeProblem) {
	if !lex.HasIncludes(s) {
		return s, nil
	}
	return lex.ResolveIncludes(s, includeFrom(o), IncludeLoader(o))
}

// printing tells whether o's output prints the script (its includes put
// in), rather than writing it as it is (Fountain, lex).
func printing(o Options) bool {
	return o.To != internal.FormatFountain && o.To != internal.FormatLex
}

// forPrint is the script as o prints it: its includes put in (each one
// that fails logged), and o's header and footer when it has none of its
// own.
func forPrint(o Options, s lex.Screenplay) lex.Screenplay {
	if !printing(o) {
		return s
	}
	s, problems := ResolveIncludes(s, o)
	for _, p := range problems {
		log.Print(p.Message)
	}
	return withPageTexts(s, o.Header, o.Footer)
}

// withPageTexts is s with a {{HEADER: header}} and a {{FOOTER: footer}}
// at the start of its body, for each it has none of (""s: none).
func withPageTexts(s lex.Screenplay, header, footer string) lex.Screenplay {
	header, footer = strings.TrimSpace(header), strings.TrimSpace(footer)
	if header == "" && footer == "" {
		return s
	}
	for _, l := range s {
		if l.Type == lex.TypeBoneyard {
			continue
		}
		for _, d := range lex.FindDirectives(l.Contents) {
			switch d.Kind {
			case lex.DirectiveHeader:
				header = ""
			case lex.DirectiveFooter:
				footer = ""
			}
		}
	}
	var add lex.Screenplay
	if header != "" {
		add = append(add, lex.Line{Type: lex.TypeAction, Contents: "{{HEADER: " + header + "}}"})
	}
	if footer != "" {
		add = append(add, lex.Line{Type: lex.TypeAction, Contents: "{{FOOTER: " + footer + "}}"})
	}
	if add == nil {
		return s
	}
	at := 0
	if len(s) > 0 && s[0].Type == lex.TypeTitlePage {
		for i, l := range s {
			if l.Type == lex.TypeNewPage {
				at = i + 1
				break
			}
		}
	}
	out := make(lex.Screenplay, 0, len(s)+len(add))
	out = append(out, s[:at]...)
	out = append(out, add...)
	return append(out, s[at:]...)
}

// checkIncludes are the warnings `lexington check` gives for a script's
// includes (path its file, text the script): each that cannot be put in.
func checkIncludes(path string, text []byte, o Options) []string {
	s := fountain.ParseLinesIn(includeConf(o).Language(cmpOr(o.SceneIn, "en")), bytes.NewReader(text))
	o.IncludeFrom = path
	_, problems := ResolveIncludes(s, o)
	var out []string
	for _, p := range problems {
		w := p.Message
		if p.Source > 0 {
			w = fmt.Sprintf("line %d: %s", p.Source, w)
		}
		out = append(out, w)
	}
	return out
}

// usesLastPage tells whether the script's header or footer has the last
// page's number (%pp).
func usesLastPage(s lex.Screenplay) bool {
	h, f := lex.PageTexts(lex.Printable(s, nil))
	return h != nil && h.Uses(lex.PartPages) || f != nil && f.Uses(lex.PartPages)
}
