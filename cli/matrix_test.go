package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

var updateFormats = flag.Bool("update-formats", false, "rewrite web/formats.json and the README's table of formats")

// The format matrix: every format Lexington reads and writes, its round
// trip from Fountain over the rich test scripts, and per feature whether
// it survives, degrades or is lost. It is web/formats.json, which the
// web version's Formats page shows, and the README's table; both are
// made here, and the test fails when the committed ones are stale
// (go test ./cli -run TestFormatMatrix -update-formats rewrites them).

// Status is what a round trip does to a feature.
type Status string

// The statuses.
const (
	Survives Status = "survives"
	Degrades Status = "degrades"
	Lost     Status = "lost"
)

// matrixFeature is a feature and how to find it in a script.
type matrixFeature struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// of are what of the feature a script has, in order
	of func(lex.Screenplay) []string
}

// Cell is a feature's status in a format, with a note.
type Cell struct {
	Status Status `json:"status"`
	Note   string `json:"note,omitempty"`
	// Measured is what the round trip measured, for a declared cell
	// (declared): so that a change in it shows here too.
	Measured string `json:"measured,omitempty"`
}

// MatrixFormat is a format: whether it is read and written, what it
// needs, and its features' cells (round-trip formats only).
type MatrixFormat struct {
	Key      string          `json:"key"`
	Name     string          `json:"name"`
	Read     bool            `json:"read"`
	Write    bool            `json:"write"`
	Needs    string          `json:"needs,omitempty"`
	Features map[string]Cell `json:"features,omitempty"`
}

// Matrix is web/formats.json.
type Matrix struct {
	About    string          `json:"about"`
	Features []matrixFeature `json:"features"`
	Formats  []MatrixFormat  `json:"formats"`
}

// outside are the rows outside the title page.
func outside(s lex.Screenplay, keep func(lex.Line) []string) []string {
	var out []string
	inTitle := false
	for _, l := range s {
		switch l.Type {
		case lex.TypeTitlePage:
			inTitle = true
			continue
		case lex.TypeNewPage:
			if inTitle {
				inTitle = false
				continue
			}
		}
		if !inTitle {
			out = append(out, keep(l)...)
		}
	}
	return out
}

// plain is a row's text without its notes, its spaces one.
func plain(c lex.Content) string {
	return strings.Join(strings.Fields(lex.StripNotes(string(c))), " ")
}

// ofType are the rows of a type, as plain text, that keep takes.
func ofType(t lex.ElementType, keep func(string) bool) func(lex.Screenplay) []string {
	return func(s lex.Screenplay) []string {
		return outside(s, func(l lex.Line) []string {
			if p := plain(l.Contents); l.Type == t && p != "" && (keep == nil || keep(p)) {
				return []string{p}
			}
			return nil
		})
	}
}

// actBreak tells a first-level section (# Act One) from a deeper one.
func actBreak(p string) bool { return strings.HasPrefix(p, "# ") }

// The features' keys, and the formats' that need it here.
const (
	featEmphasis, featDual, featNotes, featCentred = "emphasis", "dual", "notes", "centred"
	featTitle, featTrans, featOutline, featActs    = "titlepage", "transitions", "outline", "acts"
	keyTrelby, keyCeltx                            = "trelby", "celtx"
)

var matrixFeatures = []matrixFeature{
	{Key: featEmphasis, Name: "Emphasis", of: func(s lex.Screenplay) []string {
		return outside(s, func(l lex.Line) []string {
			var out []string
			for _, sp := range lex.Emphasis(lex.StripNotes(string(l.Contents))) {
				if t := strings.TrimSpace(sp.Text); t != "" && (sp.Bold || sp.Italic || sp.Underline) {
					out = append(out, fmt.Sprintf("%v%v%v:%s", sp.Bold, sp.Italic, sp.Underline, t))
				}
			}
			return out
		})
	}},
	{Key: featDual, Name: "Dual dialogue", of: func(s lex.Screenplay) []string {
		var out []string
		in := false
		for _, l := range s {
			switch l.Type {
			case lex.TypeDualOpen:
				in = true
				out = append(out, "(")
			case lex.TypeDualNext:
				out = append(out, "|")
			case lex.TypeDualClose:
				in = false
				out = append(out, ")")
			case lex.TypeSpeaker:
				if in {
					out = append(out, strings.ToUpper(plain(l.Contents)))
				}
			}
		}
		return out
	}},
	{Key: featNotes, Name: "Notes", of: func(s lex.Screenplay) []string {
		return outside(s, func(l lex.Line) []string {
			var out []string
			for _, n := range lex.Notes(string(l.Contents)) {
				if n = strings.Join(strings.Fields(n), " "); n != "" {
					out = append(out, n)
				}
			}
			return out
		})
	}},
	{Key: featCentred, Name: "Centred text", of: ofType(lex.TypeCenter, nil)},
	{Key: featTitle, Name: "Title page", of: func(s lex.Screenplay) []string {
		var out []string
		inTitle := false
		for _, l := range s {
			switch {
			case l.Type == lex.TypeTitlePage:
				inTitle = true
			case l.Type == lex.TypeNewPage:
				inTitle = false
			case inTitle && l.Type != lex.TypeEmpty:
				v := strings.Join(strings.Fields(lex.Unstyled(string(l.Contents))), " ")
				out = append(out, strings.ToLower(string(l.Type))+": "+v)
			}
		}
		slices.Sort(out) // the fields' order is the format's
		return out
	}},
	{Key: featTrans, Name: "Transitions", of: ofType(lex.TypeTrans, nil)},
	{Key: featOutline, Name: "Sections and synopses", of: func(s lex.Screenplay) []string {
		return slices.Concat(ofType(lex.TypeSection, func(p string) bool { return !actBreak(p) })(s),
			ofType(lex.TypeSynopsis, nil)(s))
	}},
	{Key: featActs, Name: "Act breaks", of: ofType(lex.TypeSection, actBreak)},
}

// declared are what a round trip cannot see, or says better: a feature
// that comes back whole to Lexington but is kept in the other program in
// a way of its own (Fountain's markers in the text), and where a lost or
// changed one goes. The test fails if one declared kept is lost.
var declared = map[string]map[string]Cell{
	keyTrelby: {
		featEmphasis: {Status: Degrades,
			Note: "as Fountain's markers in the text (`**bold**`): Trelby styles only whole element types"},
		featDual: {Status: Degrades, Note: "a caret after the second speaker (`BOB ^`), the speeches one after the other"},
		featCentred: {Status: Degrades, Note: "action indented to the centre, as Trelby's own Fountain import does; " +
			"a line nearly as wide as the page keeps `> <`"},
		featOutline: {Status: Degrades, Note: "a deeper section and a synopsis are notes (`## …`, `= …`)"},
		featActs:    {Status: Degrades, Note: "Trelby's act break, in capitals"},
	},
	keyCeltx: {
		featDual:    {Status: Degrades, Note: "a caret after the second speaker (`BOB ^`), as through Trelby"},
		featCentred: {Status: Degrades, Note: "as Fountain's markers (`> THE END <`)"},
		featOutline: {Status: Degrades, Note: "sections and synopses not on a heading as notes (`# …`, `= …`)"},
	},
}

func init() {
	// OSF has no outline: Fade In's and the OSF's are alike
	osf := map[string]Cell{
		featNotes: {Status: Survives, Note: "the outline comes back in notes as well"},
		featOutline: {Status: Degrades,
			Note: "a section, and a synopsis not on a heading, go in the next paragraph's note: OSF has no outline"},
		featActs: {Status: Lost, Note: "in the next paragraph's note"},
	}
	declared["fadein"], declared["osf"] = osf, osf
}

// matrixScripts are the rich test scripts by name, and which a format
// reads: the PDF only the synthetic one (it is slow, under -race).
func matrixScripts(t *testing.T) (map[string]string, func(format, name string) bool) {
	all := formattest.Inputs(t)
	return all, func(format, name string) bool { return format != internal.FormatPDF || name == "script" }
}

// scriptName is how a note names a test script.
func scriptName(name string) string {
	if name == "script" {
		return "the synthetic script"
	}
	return "testdata/input/" + name
}

// roundTrip writes src as format and reads it back.
func roundTrip(t *testing.T, dir, src, format string) (lex.Screenplay, error) {
	f, _ := FormatFor(format)
	out := filepath.Join(dir, strings.TrimSuffix(filepath.Base(src), ".fountain")+"."+f.Extension())
	if code := Main(context.Background(), []string{flagIn, src, flagOut, out}); code != 0 {
		return nil, fmt.Errorf("writing %s: exit %d", format, code)
	}
	return ReadScreenplay(context.Background(), out, Options{})
}

// compare is a feature's status in one script: want what it had, got
// what came back.
func compare(want, got []string) (Status, string) {
	switch {
	case slices.Equal(want, got):
		return Survives, ""
	case len(got) == 0:
		return Lost, ""
	case slices.EqualFunc(want, got, strings.EqualFold):
		return Degrades, "in capitals"
	}
	rest := slices.Clone(got)
	kept := 0
	for _, w := range want {
		if i := slices.Index(rest, w); i >= 0 {
			kept++
			rest = slices.Delete(rest, i, i+1)
		}
	}
	switch {
	case kept == len(want) && len(rest) == 0:
		return Degrades, "all come back, in another order"
	case kept == len(want):
		return Degrades, "all come back, with more beside them"
	}
	return Degrades, fmt.Sprintf("%d of %d come back as they were", kept, len(want))
}

func buildMatrix(t *testing.T) Matrix {
	scripts, reads := matrixScripts(t)
	names := slices.Sorted(func(yield func(string) bool) {
		for n := range scripts {
			if !yield(n) {
				return
			}
		}
	})
	dir := t.TempDir()
	m := Matrix{
		About: "What each format keeps of a screenplay, made by cli/matrix_test.go (TestFormatMatrix): " +
			"each format Lexington both reads and writes, written from Fountain and read back, over its rich test scripts.",
		Features: matrixFeatures,
	}
	readable := map[string]bool{}
	for _, in := range Inputs {
		readable[in.Key] = true
	}
	for _, f := range Formats {
		mf := MatrixFormat{Key: f.Key, Name: f.Name, Read: readable[f.Key], Write: true, Needs: f.Needs}
		if !mf.Read || f.Needs != "" {
			m.Formats = append(m.Formats, mf)
			continue
		}
		mf.Features = map[string]Cell{}
		worst := map[string]Cell{}
		seen := map[string]int{}
		lost := map[string]int{}
		for _, name := range names {
			if !reads(f.Key, name) {
				continue
			}
			src := filepath.Join(dir, strings.TrimSuffix(name, ".fountain")+".fountain")
			if err := os.WriteFile(src, []byte(scripts[name]), 0o644); err != nil {
				t.Fatal(err)
			}
			want, err := ReadScreenplay(context.Background(), src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			sub := filepath.Join(dir, f.Key)
			if err = os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			back, err := roundTrip(t, sub, src, f.Key)
			if err != nil {
				t.Fatalf("%s, %s: %v", f.Key, name, err)
			}
			for _, ft := range matrixFeatures {
				w := ft.of(want)
				if len(w) == 0 {
					continue
				}
				seen[ft.Key]++
				st, note := compare(w, ft.of(back))
				switch st {
				case Lost:
					lost[ft.Key]++
				case Degrades:
					if worst[ft.Key].Status != Degrades {
						worst[ft.Key] = Cell{Status: Degrades, Note: note + " (in " + scriptName(name) + ")"}
					}
				}
			}
		}
		for _, ft := range matrixFeatures {
			c := Cell{Status: Survives}
			switch {
			case seen[ft.Key] == 0:
				continue
			case lost[ft.Key] == seen[ft.Key]:
				c = Cell{Status: Lost}
			case worst[ft.Key].Status == Degrades:
				c = worst[ft.Key]
			case lost[ft.Key] > 0:
				c = Cell{Status: Degrades, Note: fmt.Sprintf("lost in %d of %d scripts", lost[ft.Key], seen[ft.Key])}
			}
			if d, ok := declared[f.Key][ft.Key]; ok {
				if c.Status == Lost && d.Status != Lost {
					t.Errorf("%s %s is declared %s (%s), but lost", f.Key, ft.Key, d.Status, d.Note)
				}
				measured := string(c.Status)
				if c.Note != "" {
					measured += ": " + c.Note
				}
				c = d
				c.Measured = measured
			}
			mf.Features[ft.Key] = c
		}
		m.Formats = append(m.Formats, mf)
	}
	return m
}

// matrixMarks are the cells as the tables show them.
var matrixMarks = map[Status]string{Survives: "✓", Degrades: "~", Lost: "✗"}

// readmeTable is the matrix as the README's table, with its notes.
func readmeTable(m Matrix) string {
	var b strings.Builder
	b.WriteString("| Format | Read | Write |")
	for _, f := range m.Features {
		b.WriteString(" " + f.Name + " |")
	}
	b.WriteString("\n|---|---|---|" + strings.Repeat("---|", len(m.Features)) + "\n")
	var notes []string
	var only []string
	for _, f := range m.Formats {
		if f.Features == nil {
			name := f.Name
			if f.Needs != "" {
				name += " (" + f.Needs + ")"
			}
			only = append(only, name)
			continue
		}
		b.WriteString("| " + f.Name + " | ✓ | ✓ |")
		for _, ft := range m.Features {
			c, ok := f.Features[ft.Key]
			switch {
			case !ok:
				b.WriteString(" |")
			case c.Note != "":
				i := slices.Index(notes, c.Note)
				if i < 0 {
					notes = append(notes, c.Note)
					i = len(notes) - 1
				}
				fmt.Fprintf(&b, " %s<sup>%d</sup> |", matrixMarks[c.Status], i+1)
			default:
				b.WriteString(" " + matrixMarks[c.Status] + " |")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n✓ survives the round trip, ~ degrades, ✗ is lost; " +
		"an empty cell: the format is not tried with it.\n\n")
	for i, n := range notes {
		fmt.Fprintf(&b, "%d. %s.\n", i+1, strings.TrimSuffix(n, "."))
	}
	b.WriteString("\nWritten only: " + strings.Join(only, ", ") + ".\n")
	return b.String()
}

const readmeStart, readmeEnd = "<!-- formats:start -->\n", "<!-- formats:end -->"

func TestFormatMatrix(t *testing.T) {
	m := buildMatrix(t)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	jsonPath := filepath.Join("..", "web", "formats.json")
	readmePath := filepath.Join("..", "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	before, rest, ok1 := strings.Cut(string(readme), readmeStart)
	_, after, ok2 := strings.Cut(rest, readmeEnd)
	if !ok1 || !ok2 {
		t.Fatalf("the README has no %q … %q", readmeStart, readmeEnd)
	}
	newReadme := before + readmeStart + readmeTable(m) + readmeEnd + after
	if *updateFormats {
		if err = os.WriteFile(jsonPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(readmePath, []byte(newReadme), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(jsonPath)
	if err != nil || !bytes.Equal(committed, data) {
		t.Errorf("web/formats.json is stale (go test ./cli -run TestFormatMatrix -update-formats):\n%s", data)
	}
	if newReadme != string(readme) {
		t.Errorf("the README's table of formats is stale (go test ./cli -run TestFormatMatrix -update-formats)")
	}
}
