package rules

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Presets are ready-made element sets for kinds of scripts other than
// the screenplay (see docs/presets.md). Each is a TOML file in presets/
// with a name, a description and the elements that differ from Default;
// the others are Default's.
//
//go:embed presets/*.toml
var presetFiles embed.FS

// Preset is a ready-made element set.
type Preset struct {
	Key         string // as given to -e, e.g. "stageplay"
	Name        string
	Description string
	Elements    Set
}

// ScreenplayPreset is the key of the default, the screenplay.
const ScreenplayPreset = "default"

var presets = loadPresets()

func loadPresets() []Preset {
	list := []Preset{{
		Key:  ScreenplayPreset,
		Name: "Screenplay",
		Description: "The standard screenplay format of film and single-camera " +
			"television (US Letter, Courier 12).",
		Elements: Default,
	}}
	files, err := presetFiles.ReadDir("presets")
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		var p struct {
			Name, Description string
			Elements          Set
		}
		if _, err := toml.DecodeFS(presetFiles, path.Join("presets", f.Name()), &p); err != nil {
			panic(fmt.Sprintf("preset %s: %v", f.Name(), err))
		}
		set := Set{}
		for k, v := range Default {
			set[k] = v
		}
		for k, v := range p.Elements {
			set[k] = v
		}
		list = append(list, Preset{
			Key:         strings.TrimSuffix(f.Name(), ".toml"),
			Name:        p.Name,
			Description: strings.Join(strings.Fields(p.Description), " "),
			Elements:    set,
		})
	}
	sort.SliceStable(list[1:], func(i, j int) bool { return list[1+i].Name < list[1+j].Name })
	return list
}

// Presets lists the presets, the screenplay first.
func Presets() []Preset {
	return append([]Preset(nil), presets...)
}

// GetPreset is the preset with the given key.
func GetPreset(key string) (Preset, bool) {
	for _, p := range presets {
		if p.Key == key {
			return p, true
		}
	}
	return Preset{}, false
}

// withPresets adds the presets to a configuration's element sets, where
// it does not have a set of that name itself.
func (c TOMLConf) withPresets() TOMLConf {
	if c.Elements == nil {
		c.Elements = map[string]Set{}
	}
	for _, p := range presets {
		if _, ok := c.Elements[p.Key]; !ok {
			c.Elements[p.Key] = p.Elements
		}
	}
	return c
}
