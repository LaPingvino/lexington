package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// carriedZip is an .accolade (or, by name, any bundle) carrying a format
// of one's own, Based on the radio preset, and a language's words.
func carriedZip(t *testing.T, format string) []byte {
	return zipOf(t,
		"Barn.textbundle/info.json", accoladeInfo(`"format":"`+format+`"`),
		"Barn.textbundle/text.fountain", script,
		"Barn.textbundle/assets/lexington/lexington.toml", "[Languages.xx]\nScenes = [\"SZENA\"]\n",
		"Barn.textbundle/assets/lexington/formats/wide.toml",
		"Name = \"Wide\"\nBase = \"radio\"\n[Elements.action]\nLeft = 4.25\n",
		"Barn.textbundle/assets/lexington/formats/../evil.toml", "Name = \"Evil\"\n",
	)
}

// accoladeInfo is an .accolade's info.json with Accolade's key holding
// fields (JSON, without braces).
func accoladeInfo(fields string) string {
	if fields != "" {
		fields = "," + fields
	}
	return `{"version":2,"type":"com.quoteunquoteapps.fountain","eu.kiefte.Accolade":{"version":1` + fields + `}}`
}

func writeFile(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readInput(t *testing.T, o *Options) {
	t.Helper()
	if _, err := readBundleInput(o); err != nil {
		t.Fatal(err)
	}
}

// An .accolade's carried settings are laid over the machine's: its
// format is -e's default, its format files and languages are there; a
// flag wins, and other bundles carry nothing.
func TestCarriedSettings(t *testing.T) {
	dir := t.TempDir()
	acc := filepath.Join(dir, "barn.accolade")
	writeFile(t, acc, carriedZip(t, "wide"))

	o := Options{Input: acc, ConfigFile: filepath.Join(dir, "none.toml")}
	readInput(t, &o)
	if o.Elements != "wide" {
		t.Fatalf("-e is %q, want the script's format", o.Elements)
	}
	conf := confOf(o)
	wide := conf.Elements["wide"]
	if wide["action"].Left != 4.25 || wide["speaker"].Postfix != ":" {
		t.Errorf("the carried format (on radio): action %+v, speaker %+v", wide["action"], wide["speaker"])
	}
	if _, ok := conf.Elements["evil"]; ok {
		t.Error("a format file outside formats/ was read")
	}
	if !slices.Contains(conf.Scenes("xx"), "SZENA") {
		t.Errorf("the carried language: %q", conf.Scenes("xx"))
	}
	if _, ok := conf.Elements["stageplay"]; !ok {
		t.Error("the machine's presets are gone")
	}

	flag := Options{Input: acc, ConfigFile: o.ConfigFile, Elements: "stageplay"}
	readInput(t, &flag)
	if flag.Elements != "stageplay" {
		t.Errorf("-e stageplay became %q", flag.Elements)
	}
	if _, ok := confOf(flag).Elements["wide"]; !ok {
		t.Error("with -e, the carried formats are still there")
	}

	hl := filepath.Join(dir, "barn.highland")
	writeFile(t, hl, carriedZip(t, "wide"))
	other := Options{Input: hl, ConfigFile: o.ConfigFile}
	readInput(t, &other)
	if other.carried != nil || other.Elements != "" {
		t.Errorf("a .highland carried %+v (-e %q)", other.carried, other.Elements)
	}

	// printed in it: the HTML differs from the screenplay's
	html := func(in string) string {
		out := filepath.Join(dir, filepath.Base(in)+".html")
		run(t, Options{Input: in, Output: out})
		return readFile(t, out)
	}
	if html(acc) == html(hl) {
		t.Error("the .accolade printed as the screenplay")
	}

	// a format of Accolade's that the bundle does not carry: the screenplay
	series := filepath.Join(dir, "series.accolade")
	writeFile(t, series, carriedZip(t, "series"))
	so := Options{Input: series, Output: filepath.Join(dir, "series.html"), ConfigFile: o.ConfigFile}
	if err := Convert(context.Background(), so); err != nil {
		t.Errorf("Accolade's own format: %v", err)
	}
}

// What Accolade writes in its script (end blocks, links, colours, locked
// and omitted scenes, boneyard) comes out of `-i x.accolade -o
// x.fountain` as it is in text.fountain.
func TestAccoladeBlocksKept(t *testing.T) {
	text := `Title: Barn
Author: Joop

INT. KITCHEN - NIGHT [[red]] #12A#

Anna opens the door. [[from: #:~:text=She opened the door,into the rain.]]

ANNA
It's *late*. [[Check the date.]]

EXT. YARD - DAY [[#ff8800]] [[London]] #13#

/* a cut line */

.OMITTED #14#

/* Omitted 14
INT. HALL - DAY

Gone.
*/

/* Production
scene numbers: locked, 10A
revision: Blue, 2026-11-20
omitted runs: print as one
*/

/* Adaptation
cut:
#:~:text=It was late.
*/

/* Characters
ANNA: the lead
  age: 34
ben
characters: ANNA = Anna, Annie
not characters: Monday, Lord
places: the house = INT. KOVAČ HOUSE
*/

/* Fix-Up
Fix Lines Like This
*/

/* Source
Once upon a time.
*/

/* Notes
Check the date.
*/
`
	dir := t.TempDir()
	acc := filepath.Join(dir, "barn.accolade")
	writeFile(t, acc, zipOf(t,
		"Barn.textbundle/info.json", accoladeInfo(""),
		"Barn.textbundle/text.fountain", text))
	out := filepath.Join(dir, "barn.fountain")
	run(t, Options{Input: acc, Output: out})
	if got := readFile(t, out); got != text {
		t.Errorf("got\n%s", got)
	}
}

// An .accolade's title page template: the one info.json names is
// -titlepage's default, one of the script's own comes in its
// lexington.toml, a flag wins, and one it names but does not carry
// prints as Fountain's.
func TestCarriedTitlePage(t *testing.T) {
	dir := t.TempDir()
	const titled = "Title: The Barn\nCredit: written by\nAuthor: Joop\nLogline: A storm.\n\nINT. BARN - DAY\n\nRain.\n"
	bundleWith := func(name, tpl, conf string) string {
		p := filepath.Join(dir, name)
		pairs := []string{
			"Barn.textbundle/info.json", accoladeInfo(`"titlePage":"` + tpl + `"`),
			"Barn.textbundle/text.fountain", titled,
		}
		if conf != "" {
			pairs = append(pairs, "Barn.textbundle/assets/lexington/lexington.toml", conf)
		}
		writeFile(t, p, zipOf(t, pairs...))
		return p
	}
	own := bundleWith("own.accolade", "loud",
		"[TitlePages.loud]\nName = \"Loud\"\n[[TitlePages.loud.Keys]]\nKey = \"Logline\"\nPlace = \"top\"\n"+
			"[[TitlePages.loud.Keys]]\nKey = \"Title\"\nPlace = \"centre\"\n")
	o := Options{Input: own, ConfigFile: filepath.Join(dir, "none.toml")}
	readInput(t, &o)
	if o.TitlePage != "loud" {
		t.Fatalf("-titlepage is %q, want the script's", o.TitlePage)
	}
	flag := Options{Input: own, ConfigFile: o.ConfigFile, TitlePage: "bold"}
	readInput(t, &flag)
	if flag.TitlePage != "bold" {
		t.Errorf("-titlepage bold became %q", flag.TitlePage)
	}

	html := func(in string, extra Options) string {
		out := filepath.Join(dir, filepath.Base(in)+extra.TitlePage+".html")
		extra.Input, extra.Output = in, out
		run(t, extra)
		return readFile(t, out)
	}
	loud, plain := html(own, Options{}), html(own, Options{TitlePage: "standard"})
	if loud == plain {
		t.Errorf("the carried template made no difference:\n%s", loud)
	}
	if i, j := strings.Index(loud, "A storm."), strings.Index(loud, "The Barn"); i < 0 || j < 0 || i > j {
		t.Errorf("the carried template's order (Logline on top):\n%s", loud)
	}

	// named, not carried: Fountain's, with a word on the log
	lost := bundleWith("lost.accolade", "gone", "")
	var got string
	logged := captureLog(t, func() { got = html(lost, Options{}) })
	if got != html(lost, Options{TitlePage: "standard"}) || !strings.Contains(logged, `"gone"`) {
		t.Errorf("a template not carried: log %q\n%s", logged, got)
	}
	// a preset by its name alone
	if html(bundleWith("bold.accolade", "bold", ""), Options{}) == plain {
		t.Error("the preset named in info.json was not used")
	}
}
