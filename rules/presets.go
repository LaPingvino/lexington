package rules

import (
	"cmp"
	"embed"
	"fmt"
	"path"
	"path/filepath"
	"slices"
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
	// Page is the paper size it suggests for documents ("a5"), or "".
	Page string
}

// ScreenplayPreset is the key of the default, the screenplay.
const ScreenplayPreset = "default"

var presets = loadPresets()

// presetFile is a preset's file: Base is the preset it starts from (the
// screenplay's elements if none), Elements what differs from it.
type presetFile struct {
	Name, Description, Page, Base string
	Elements                      Set
}

func (f presetFile) preset(key string, base func(string) (Preset, bool)) Preset {
	set := Set{}
	from := Default
	if b, ok := base(f.Base); ok && f.Base != "" {
		from = b.Elements
		f.Name = cmp.Or(f.Name, b.Name)
		f.Description = cmp.Or(f.Description, b.Description)
		f.Page = cmp.Or(f.Page, b.Page)
	}
	for k, v := range from {
		set[k] = v
	}
	for k, v := range f.Elements {
		set[k] = v
	}
	return Preset{Key: key, Name: f.Name, Description: strings.Join(strings.Fields(f.Description), " "), Elements: set, Page: f.Page}
}

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
		var p presetFile
		if _, err := toml.DecodeFS(presetFiles, path.Join("presets", f.Name()), &p); err != nil {
			panic(fmt.Sprintf("preset %s: %v", f.Name(), err))
		}
		list = append(list, p.preset(strings.TrimSuffix(f.Name(), ".toml"), func(string) (Preset, bool) { return Preset{}, false }))
	}
	sort.SliceStable(list[1:], func(i, j int) bool { return list[1+i].Name < list[1+j].Name })
	return list
}

// PresetsIn are the presets with those of folders of preset files
// (name.toml, as in presets/; a later folder's over an earlier's): a file
// with a preset's name takes its place, the others are added. What a
// file leaves out (its name, description, paper) is its Base's. A file
// that cannot be read is left out, and the first such error returned.
func PresetsIn(dirs ...string) ([]Preset, error) {
	list := Presets()
	var files []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		fs, err := filepath.Glob(filepath.Join(dir, "*.toml"))
		if err != nil {
			return list, err
		}
		sort.Strings(fs)
		files = append(files, fs...)
	}
	find := func(key string) (Preset, bool) {
		for _, p := range list {
			if p.Key == key {
				return p, true
			}
		}
		return Preset{}, false
	}
	var first error
	for _, f := range files {
		var p presetFile
		if _, err := toml.DecodeFile(f, &p); err != nil {
			if first == nil {
				first = fmt.Errorf("preset %s: %w", filepath.Base(f), err)
			}
			continue
		}
		key := strings.TrimSuffix(filepath.Base(f), ".toml")
		np := p.preset(key, find)
		np.Name = cmp.Or(np.Name, key)
		if i := slices.IndexFunc(list, func(q Preset) bool { return q.Key == key }); i >= 0 {
			list[i] = np
		} else {
			list = append(list, np)
		}
	}
	return list, first
}

// PresetSource is a preset's file, to start one's own from.
func PresetSource(key string) (string, bool) {
	if key == ScreenplayPreset {
		p := presets[0]
		return fmt.Sprintf("Name = %q\nDescription = %q\n", p.Name, p.Description), true
	}
	b, err := presetFiles.ReadFile(path.Join("presets", key+".toml"))
	return string(b), err == nil
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
