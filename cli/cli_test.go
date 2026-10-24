package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

const script = "Title: The Barn\n\nINT. BARN - DAY\n\nRain falls.\n\nANNA\nIt's coming.\n"

func TestConvert(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Formats {
		if f.Needs != "" {
			continue
		}
		out := filepath.Join(dir, "barn."+f.Key)
		o := Options{Input: in, Output: out, ConfigFile: filepath.Join(dir, "none.toml")}
		if err := Convert(context.Background(), o); err != nil {
			t.Errorf("%s: %v", f.Key, err)
			continue
		}
		if data, err := os.ReadFile(out); err != nil || len(data) == 0 {
			t.Errorf("%s: %d bytes, %v", f.Key, len(data), err)
		}
	}
}

func TestWriteParsedScreenplay(t *testing.T) {
	out := filepath.Join(t.TempDir(), "barn.html")
	s := fountain.Parse(nil, strings.NewReader(script))
	if err := Write(context.Background(), s, Options{To: "html", Output: out, ConfigFile: "none.toml"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "It&#39;s coming.") && !strings.Contains(string(data), "It's coming.") {
		t.Errorf("html lacks the dialogue:\n%s", data)
	}
}

func TestMainExitCodes(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	for args, want := range map[string]int{
		"-i " + in + " -o " + filepath.Join(dir, "x.html"):              0,
		"-i " + in + " -to nonsense -o " + filepath.Join(dir, "x.none"): 1,
		"-i " + filepath.Join(dir, "missing.fountain"):                  1,
		"-presets":      0,
		"-no-such-flag": 2,
	} {
		full := append(strings.Fields(args), "-config", filepath.Join(dir, "none.toml"))
		if got := Main(context.Background(), full); got != want {
			t.Errorf("%s: exit %d, want %d", args, got, want)
		}
	}
}

// The formats made with other programs, where those are installed (CI's
// integration job installs them).
func TestExternalFormats(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Formats {
		if f.Needs == "" {
			continue
		}
		if !f.Available() {
			t.Logf("%s: %s is not installed", f.Key, f.Needs)
			continue
		}
		out := filepath.Join(dir, "barn-"+f.Key+"."+f.Extension())
		o := Options{Input: in, Output: out, To: f.Key, ConfigFile: filepath.Join(dir, "none.toml")}
		if err := Convert(context.Background(), o); err != nil {
			t.Errorf("%s: %v", f.Key, err)
			continue
		}
		if data, err := os.ReadFile(out); err != nil || len(data) == 0 {
			t.Errorf("%s: %d bytes, %v", f.Key, len(data), err)
		}
	}
}

// A folder of format files: one based on a preset, with Accolade's part,
// which Lexington skips.
func TestFormatDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "strip.toml"), []byte("Name = \"Strip\"\nBase = \"bd\"\nPage = \"a4\"\n\n[Elements.action]\nLeft = 2.0\n\n[[Accolade.Buttons]]\nLabel = \"Plaat\"\nInsert = \".PLAAT {n}\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list, err := rules.PresetsIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	bd, _ := rules.GetPreset("bd")
	var strip rules.Preset
	for _, p := range list {
		if p.Key == "strip" {
			strip = p
		}
	}
	if strip.Name != "Strip" || strip.Elements["action"].Left != 2.0 || strip.Elements["speaker"] != bd.Elements["speaker"] {
		t.Errorf("strip: %+v", strip)
	}
	in := filepath.Join(dir, "s.fountain")
	_ = os.WriteFile(in, []byte("INT. HOUSE - DAY\n\nRain.\n"), 0o644)
	o := Options{Input: in, Output: filepath.Join(dir, "s.pdf"), To: "pdf", Elements: "strip", FormatDir: dir, ConfigFile: "/nonexistent"}
	if err := Convert(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	if pageFor(o) != "a4" {
		t.Errorf("page %q", pageFor(o))
	}
}
