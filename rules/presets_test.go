package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPresetsAreValid(t *testing.T) {
	ps := Presets()
	if len(ps) < 5 || ps[0].Key != ScreenplayPreset {
		t.Fatalf("presets: %d, first %q", len(ps), ps[0].Key)
	}
	for _, p := range ps {
		if p.Name == "" || p.Description == "" {
			t.Errorf("%s: no name or description", p.Key)
		}
		if err := p.Elements.Validate(); err != nil {
			t.Errorf("%s: %v", p.Key, err)
		}
		// what a preset does not set is the screenplay's, not hidden
		for k := range Default {
			if _, ok := p.Elements[k]; !ok {
				t.Errorf("%s: no %s", p.Key, k)
			}
		}
	}
}

// The inverse preset is the configuration published in 2022
// (gist 3412b228c1f5aac8eb405445b9393c87).
func TestInversePreset(t *testing.T) {
	p, ok := GetPreset("inverse")
	if !ok {
		t.Fatal("no inverse preset")
	}
	sp := p.Elements.Get("speaker")
	if sp.Left != 1.5 || sp.Style != "bi" || sp.Postfix != ":" {
		t.Errorf("speaker %+v", sp)
	}
	if d := p.Elements.Get("dialog"); d.Left != 1.75 {
		t.Errorf("dialog %+v", d)
	}
	if s := p.Elements.Get("section"); s.Hide || s.Prefix != "[ " || s.Align != "C" {
		t.Errorf("section %+v", s)
	}
	// the screenplay's dual dialogue, which the gist did not set
	if d := p.Elements.Get("dualdialog"); d.Hide || d != Default.Get("dualdialog") {
		t.Errorf("dualdialog %+v", d)
	}
}

// The presets can be used with -e whatever the configuration file, which
// may still override one by its name.
func TestConfigurationHasThePresets(t *testing.T) {
	if _, ok := GetConf(filepath.Join(t.TempDir(), "none.toml")).Elements["stageplay"]; !ok {
		t.Error("no stageplay without a configuration file")
	}
	file := filepath.Join(t.TempDir(), "lexington.toml")
	conf := "[Elements.radio.speaker]\nLeft = 2.0\n[Elements.mine.action]\nLeft = 1.5\n"
	if err := os.WriteFile(file, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	c := GetConf(file)
	if _, ok := c.Elements["stageplay"]; !ok {
		t.Error("no stageplay with a configuration file")
	}
	if c.Elements["radio"].Get("speaker").Left != 2.0 || c.Elements["radio"].Get("speaker").Postfix != "" {
		t.Errorf("the file's radio did not replace the preset: %+v", c.Elements["radio"].Get("speaker"))
	}
	if _, ok := c.Elements["mine"]; !ok {
		t.Error("the file's own set is gone")
	}
}
