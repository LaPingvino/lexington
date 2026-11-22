package fountain

import (
	"github.com/LaPingvino/lexington/rules"

	"os"
	"path/filepath"
	"strings"
	"testing"
)

// everyBlock is a script with every kind of block Strip knows: a title
// page, a section and a synopsis, numbered headings, notes (one a source
// link, one over two lines, one inside a boneyard), a boneyard, an
// omitted scene with its kept text, and Accolade's end blocks with the
// language line after them.
const everyBlock = `Title: The Letter
Author: Anna Writer

# Act One

= Anna finds the letter.

INT. HOUSE - DAY #1#

Anna waits. [[from: #:~:text=She waited]]

[[keep the pace slow]]

ANNA
I know. [[a note
over two lines]]

/* an old line
kept here [[with a note]] */

.OMITTED #2#

/* Omitted 2
EXT. GARDEN - DAY #2#

Rain. [[from: #:~:text=It rained]]
*/

EXT. STREET - NIGHT #3#

She runs.

/* Characters
ANNA: the lead
*/

/* Source
She waited. She ran.
*/

/* Adaptation
cut:
  "She ran."
*/

/* Production
lock: 2026-11-20
*/

/* Notes
call the producer
*/

/* Language: eo */
`

// TestStripClean drops Accolade's data only, always.
func TestStripClean(t *testing.T) {
	got := Strip(everyBlock, en, StripOptions{})
	want := `Title: The Letter
Author: Anna Writer

# Act One

= Anna finds the letter.

INT. HOUSE - DAY #1#

Anna waits.

[[keep the pace slow]]

ANNA
I know. [[a note
over two lines]]

/* an old line
kept here [[with a note]] */

.OMITTED #2#

/* Omitted 2
EXT. GARDEN - DAY #2#

Rain.
*/

EXT. STREET - NIGHT #3#

She runs.
`
	if got != want {
		t.Errorf("clean:\n%s\nwant:\n%s", got, want)
	}
}

// TestStripEachOption takes one option at a time.
func TestStripEachOption(t *testing.T) {
	clean := Strip(everyBlock, en, StripOptions{})
	for _, c := range []struct {
		key        string
		gone, kept []string
	}{
		{"notes", []string{"[[keep", "[[a note", "over two lines"}, []string{"[[with a note]]", "ANNA\nI know.\n"}},
		{"boneyard", []string{"/* an old line", "/* Omitted 2", "Rain."}, []string{".OMITTED #2#", "[[keep"}},
		{"numbers", []string{"#1#", "#3#", ".OMITTED #2#"}, []string{"INT. HOUSE - DAY\n", ".OMITTED\n", "EXT. GARDEN - DAY #2#"}},
		{"omitted", []string{"OMITTED", "/* Omitted 2", "Rain."}, []string{"/* an old line", "#3#"}},
		{"titlepage", []string{"Title:", "Author:"}, []string{"# Act One"}},
		{"outline", []string{"# Act One", "= Anna"}, []string{"Title: The Letter", "INT. HOUSE - DAY #1#"}},
	} {
		o, err := ParseStrip(c.key)
		if err != nil {
			t.Fatal(err)
		}
		got := Strip(everyBlock, en, o)
		for _, s := range c.gone {
			if strings.Contains(got, s) {
				t.Errorf("%s: %q still there:\n%s", c.key, s, got)
			}
		}
		for _, s := range c.kept {
			if !strings.Contains(got, s) {
				t.Errorf("%s: %q gone:\n%s", c.key, s, got)
			}
		}
		if strings.Contains(got, "\n\n\n") || strings.HasPrefix(got, "\n") || strings.HasSuffix(got, "\n\n") {
			t.Errorf("%s: blank lines left over:\n%q", c.key, got)
		}
		if got == clean {
			t.Errorf("%s: nothing more taken out than clean", c.key)
		}
		checkReadsAsBefore(t, c.key, clean, got)
	}
}

// TestStripPresets are the three sets, as the command line and Accolade
// name them.
func TestStripPresets(t *testing.T) {
	want := map[string]string{
		"clean": Strip(everyBlock, en, StripOptions{}),
		"reading": `Title: The Letter
Author: Anna Writer

INT. HOUSE - DAY #1#

Anna waits.

ANNA
I know.

/* an old line
kept here [[with a note]] */

.OMITTED #2#

/* Omitted 2
EXT. GARDEN - DAY #2#

Rain.
*/

EXT. STREET - NIGHT #3#

She runs.
`,
		"writer": `Title: The Letter
Author: Anna Writer

# Act One

= Anna finds the letter.

INT. HOUSE - DAY

Anna waits.

[[keep the pace slow]]

ANNA
I know. [[a note
over two lines]]

/* an old line
kept here [[with a note]] */

EXT. STREET - NIGHT

She runs.
`,
	}
	if len(StripPresets) != len(want) {
		t.Fatalf("%d presets, want %d", len(StripPresets), len(want))
	}
	for _, p := range StripPresets {
		o, err := ParseStrip(p.Key)
		if err != nil || o != p.Options {
			t.Fatalf("%s: %v %v", p.Key, o, err)
		}
		if got := Strip(everyBlock, en, o); got != want[p.Key] {
			t.Errorf("%s:\n%s\nwant:\n%s", p.Key, got, want[p.Key])
		}
	}
	// a comma list adds up
	o, err := ParseStrip("writer, notes,TITLEPAGE")
	if err != nil || o != (StripOptions{SceneNumbers: true, Omitted: true, Notes: true, TitlePage: true}) {
		t.Errorf("writer,notes,titlepage: %+v %v", o, err)
	}
	if o.String() != "notes,numbers,omitted,titlepage" {
		t.Errorf("String: %q", o.String())
	}
	if _, err := ParseStrip("notes,nothing"); err == nil {
		t.Error("an unknown key is no error")
	}
}

// TestStripLinksEverywhere: a source link goes in every preset, inside a
// kept boneyard (an omitted scene's) too.
func TestStripLinksEverywhere(t *testing.T) {
	for _, o := range presetOptions() {
		if got := Strip(everyBlock, en, o); strings.Contains(got, "from:") {
			t.Errorf("%s keeps a link:\n%s", o, got)
		}
	}
}

// TestStripBoneyardAsParsed: a boneyard is what the parser reads as one,
// from a line starting with /* (a /* within a line is text), to the end
// of the text when it is never closed.
func TestStripBoneyardAsParsed(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		o              StripOptions
	}{
		{"a /* within a line", "He/*she*/ waits.\n\nBOB\nHi.\n", "He/*she*/ waits.\n\nBOB\nHi.\n", StripOptions{Boneyard: true}},
		{"text after the close", "Anna.\n\n/* old */ gone too\n\nBob.\n", "Anna.\n\nBob.\n", StripOptions{Boneyard: true}},
		{"never closed: kept in clean", "Anna. [[from: x]]\n\n/* open\nBob. [[from: y]] [[n]]\n", "Anna.\n\n/* open\nBob. [[n]]\n", StripOptions{}},
		{"never closed: all of it goes", "Anna.\n\n/* open\nBob.\n", "Anna.\n", StripOptions{Boneyard: true}},
		{"notes in a kept boneyard stay", "Anna. [[n]]\n\n/* old [[m]] */\n", "Anna.\n\n/* old [[m]] */\n", StripOptions{Notes: true}},
	} {
		if got := Strip(c.in, en, c.o); got != c.want {
			t.Errorf("%s:\n%q\nwant\n%q", c.name, got, c.want)
		}
	}
}

// TestStripAll takes everything: the script itself is left, and strip it
// again changes nothing.
func TestStripAll(t *testing.T) {
	var all StripOptions
	for _, s := range StripKeys {
		*s.Of(&all) = true
	}
	got := Strip(everyBlock, en, all)
	want := "INT. HOUSE - DAY\n\nAnna waits.\n\nANNA\nI know.\n\nEXT. STREET - NIGHT\n\nShe runs.\n"
	if got != want {
		t.Errorf("all:\n%q\nwant:\n%q", got, want)
	}
	for _, o := range append([]StripOptions{all}, presetOptions()...) {
		once := Strip(everyBlock, en, o)
		if twice := Strip(once, en, o); twice != once {
			t.Errorf("%s twice:\n%q\nonce:\n%q", o, twice, once)
		}
	}
}

func presetOptions() []StripOptions {
	var out []StripOptions
	for _, p := range StripPresets {
		out = append(out, p.Options)
	}
	return out
}

// TestStripUnchanged: a script with nothing to strip comes back byte for
// byte, its line ends and its last new line (or none) as they were.
func TestStripUnchanged(t *testing.T) {
	files, _ := filepath.Glob("../examples/*.fountain")
	more, _ := filepath.Glob("../testdata/input/*.fountain")
	files = append(files, more...)
	if len(files) == 0 {
		t.Fatal("no scripts")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		for name, v := range map[string]string{
			"as is":        text,
			"crlf":         strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n"),
			"no last line": strings.TrimRight(text, "\r\n"),
			"spaces":       text + "\n\n  \n",
		} {
			if got := Strip(v, en, StripOptions{}); got != v {
				t.Errorf("%s, %s: changed", filepath.Base(f), name)
			}
		}
	}
	// and an option with nothing to take
	for _, s := range []string{"INT. HOUSE - DAY\n\nAnna waits.\r\n", "Anna waits.", ""} {
		if got := Strip(s, en, StripOptions{TitlePage: true, Omitted: true, Outline: true}); got != s {
			t.Errorf("%q: %q", s, got)
		}
	}
}

// TestStripKeepsReading: what is left reads as it did; where a line
// would read otherwise, it gets its force mark.
func TestStripKeepsReading(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		o              StripOptions
	}{
		// BOB after a boneyard line is action (a cue needs a blank line before it), and stays so
		{"action after a boneyard line", "Anna waits.\n\n/* gone */\nBOB\nHi.\n", "Anna waits.\n\n!BOB\nHi.\n", StripOptions{Boneyard: true}},
		{"action after the title page", "Title: X\n\n\nAnna: she waits.\n", "!Anna: she waits.\n", StripOptions{TitlePage: true}},
		{"note line in a speech", "BOB\nHi.\n[[beat]]\nThere.\n", "BOB\nHi.\nThere.\n", StripOptions{Notes: true}},
		{"note before a cue", "Anna waits.\n\n[[x]]\n\nBOB\nHi.\n", "Anna waits.\n\nBOB\nHi.\n", StripOptions{Notes: true}},
		{"link inside a line", "She runs [[from: \"She ran.\"]] away.\n", "She runs away.\n", StripOptions{}},
		{"crlf", "Anna.\r\n\r\n[[x]]\r\n\r\nBob.\r\n", "Anna.\r\n\r\nBob.\r\n", StripOptions{Notes: true}},
		// a heading needs a blank line before it: this one was action
		{"section before a heading", "Anna.\n\n# One\nINT. A - DAY\n\nB.\n", "Anna.\n\n!INT. A - DAY\n\nB.\n", StripOptions{Outline: true}},
	} {
		got := Strip(c.in, en, c.o)
		if got != c.want {
			t.Errorf("%s:\n%q\nwant\n%q", c.name, got, c.want)
		}
	}
}

// checkReadsAsBefore: every line of got reads as the line it came from
// in was read.
func checkReadsAsBefore(t *testing.T, name, in, got string) {
	t.Helper()
	before := map[string]string{}
	el := elementsOf(strings.TrimSuffix(in, "\n"), rules.Words(en), false)
	for i, l := range strings.Split(strings.TrimSuffix(in, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			before[strings.TrimSpace(l)] = el[i+1]
		}
	}
	now := elementsOf(strings.TrimSuffix(got, "\n"), rules.Words(en), false)
	for i, l := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if w, ok := before[strings.TrimSpace(l)]; ok && now[i+1] != w {
			t.Errorf("%s: %q reads as %s, was %s", name, l, now[i+1], w)
		}
	}
}

// TestEndBlocks finds Accolade's blocks from the notes back, in any
// order, and only at the very end.
func TestEndBlocks(t *testing.T) {
	text := "INT. A\n\n/* Production\np\n*/\n\n/* Source\ns * / t\n*/\n\n/* Notes\nn\n*/\n"
	blocks, body := EndBlocks(text)
	if body != len("INT. A") {
		t.Errorf("body %d", body)
	}
	var names []string
	for _, b := range blocks {
		names = append(names, b.Name)
		if text[b.Start:b.Start+2] != "\n\n" {
			t.Errorf("%s starts at %q", b.Name, text[b.Start:])
		}
	}
	if strings.Join(names, " ") != "Notes Source Production" {
		t.Errorf("found %v", names)
	}
	if got := EndBlockMarks(EndSource).Unescape(blocks[1].Inside); got != "s */ t" {
		t.Errorf("source inside %q", got)
	}
	// a block not at the end is the script's
	if _, body := EndBlocks("/* Notes\nn\n*/\n\nINT. A\n"); body != len("/* Notes\nn\n*/\n\nINT. A\n") {
		t.Errorf("a block before the script found: %d", body)
	}
	if at, lang := LanguageLine("A.\n\n/* Language: eo */\n"); at != 3 || lang != "eo" {
		t.Errorf("language line %d %q", at, lang)
	}
	if !IsSourceLink(" From: x") || IsSourceLink("fro") || IsSourceLink("a from: x") {
		t.Error("IsSourceLink")
	}
}
