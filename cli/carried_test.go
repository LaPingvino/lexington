package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// carriedZip is an .accolade (or, by name, any bundle) carrying a format
// of one's own, Based on the radio preset, and a language's words.
func carriedZip(t *testing.T, format string) []byte {
	return zipOf(t,
		"Barn.textbundle/info.json", `{"version":2,"type":"com.quoteunquoteapps.fountain","eu.kiefte.Accolade":{"version":1,"format":"`+format+`"}}`,
		"Barn.textbundle/text.fountain", script,
		"Barn.textbundle/assets/lexington/lexington.toml", "[Languages.xx]\nScenes = [\"SZENA\"]\n",
		"Barn.textbundle/assets/lexington/formats/wide.toml", "Name = \"Wide\"\nBase = \"radio\"\n[Elements.action]\nLeft = 4.25\n",
		"Barn.textbundle/assets/lexington/formats/../evil.toml", "Name = \"Evil\"\n",
	)
}

// An .accolade's carried settings are laid over the machine's: its
// format is -e's default, its format files and languages are there; a
// flag wins, and other bundles carry nothing.
func TestCarriedSettings(t *testing.T) {
	dir := t.TempDir()
	acc := filepath.Join(dir, "barn.accolade")
	os.WriteFile(acc, carriedZip(t, "wide"), 0o644)

	o := Options{Input: acc, ConfigFile: filepath.Join(dir, "none.toml")}
	if _, err := readBundleInput(&o); err != nil {
		t.Fatal(err)
	}
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
	readBundleInput(&flag)
	if flag.Elements != "stageplay" {
		t.Errorf("-e stageplay became %q", flag.Elements)
	}
	if _, ok := confOf(flag).Elements["wide"]; !ok {
		t.Error("with -e, the carried formats are still there")
	}

	hl := filepath.Join(dir, "barn.highland")
	os.WriteFile(hl, carriedZip(t, "wide"), 0o644)
	other := Options{Input: hl, ConfigFile: o.ConfigFile}
	readBundleInput(&other)
	if other.carried != nil || other.Elements != "" {
		t.Errorf("a .highland carried %+v (-e %q)", other.carried, other.Elements)
	}

	// printed in it: the HTML differs from the screenplay's
	html := func(in string) string {
		out := filepath.Join(dir, filepath.Base(in)+".html")
		run(t, Options{Input: in, Output: out})
		data, _ := os.ReadFile(out)
		return string(data)
	}
	if html(acc) == html(hl) {
		t.Error("the .accolade printed as the screenplay")
	}

	// a format of Accolade's that the bundle does not carry: the screenplay
	series := filepath.Join(dir, "series.accolade")
	os.WriteFile(series, carriedZip(t, "series"), 0o644)
	if err := Convert(context.Background(), Options{Input: series, Output: filepath.Join(dir, "series.html"), ConfigFile: o.ConfigFile}); err != nil {
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
	os.WriteFile(acc, zipOf(t,
		"Barn.textbundle/info.json", `{"version":2,"type":"com.quoteunquoteapps.fountain","eu.kiefte.Accolade":{"version":1}}`,
		"Barn.textbundle/text.fountain", text), 0o644)
	out := filepath.Join(dir, "barn.fountain")
	run(t, Options{Input: acc, Output: out})
	if got, _ := os.ReadFile(out); string(got) != text {
		t.Errorf("got\n%s", got)
	}
}
