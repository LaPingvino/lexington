package rules

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// A configuration can travel with a script: the part of it the script
// uses (Part), written out (Encode), read back (Decode) and laid over the
// configuration of the machine that opens it (With). These know every
// field, so a field added to TOMLConf travels by being added here.

// Decode reads a configuration from TOML text.
func Decode(data []byte) (TOMLConf, error) {
	var c TOMLConf
	m, err := toml.NewDecoder(bytes.NewReader(data)).Decode(&c)
	if err != nil {
		return TOMLConf{}, err
	}
	c.metadata = m
	return c, nil
}

// Encode is the configuration as TOML text, its empty parts left out.
func (c TOMLConf) Encode() ([]byte, error) {
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// With is c with layer over it: each language's scene words, matchup and
// direction,
// each element set, and the Standard that layer has take the place of
// c's; c itself is not changed.
func (c TOMLConf) With(layer TOMLConf) TOMLConf {
	out := TOMLConf{
		Elements:   maps.Clone(c.Elements),
		Scenes:     maps.Clone(c.Scenes),
		Matchups:   maps.Clone(c.Matchups),
		Standard:   c.Standard,
		Directions: maps.Clone(c.Directions),
	}
	put := func(dst *map[string][]string, src map[string][]string) {
		if len(src) > 0 && *dst == nil {
			*dst = map[string][]string{}
		}
		for k, v := range src {
			(*dst)[k] = slices.Clone(v)
		}
	}
	put(&out.Scenes, layer.Scenes)
	put(&out.Matchups, layer.Matchups)
	if len(layer.Elements) > 0 && out.Elements == nil {
		out.Elements = map[string]Set{}
	}
	for k, v := range layer.Elements {
		out.Elements[k] = v
	}
	if layer.Standard != "" {
		out.Standard = layer.Standard
	}
	if len(layer.Directions) > 0 && out.Directions == nil {
		out.Directions = map[string]string{}
	}
	maps.Copy(out.Directions, layer.Directions)
	return out
}

// Part is what of c a script in the languages langs, laid out with the
// element sets sets, uses: those languages' scene words, matchups and
// directions (each as c reads it, "ltr" too), those sets (the ones c has), and the Standard (the standard language's
// words are needed to write it as standard Fountain: give it in langs).
func (c TOMLConf) Part(langs, sets []string) TOMLConf {
	out := TOMLConf{Standard: c.StandardLanguage()}
	for _, l := range langs {
		if w, ok := c.Scenes[l]; ok {
			if out.Scenes == nil {
				out.Scenes = map[string][]string{}
			}
			out.Scenes[l] = slices.Clone(w)
		}
		if m, ok := c.Matchups[l]; ok {
			if out.Matchups == nil {
				out.Matchups = map[string][]string{}
			}
			out.Matchups[l] = slices.Clone(m)
		}
		if _, ok := c.Directions[l]; ok || c.Scenes[l] != nil || c.Matchups[l] != nil {
			if out.Directions == nil {
				out.Directions = map[string]string{}
			}
			out.Directions[l] = c.Direction(l) // "ltr" too: the language in full
		}
	}
	for _, s := range sets {
		if set, ok := c.Elements[s]; ok {
			if out.Elements == nil {
				out.Elements = map[string]Set{}
			}
			out.Elements[s] = set
		}
	}
	return out
}

// Item is one named thing in a configuration: a language (its scene
// words, matchup and direction) or an element set, or the Standard.
type Item struct {
	Kind string // "language", "elements" or "standard"
	Key  string // the language or set ("" for the standard)
}

// Items are what c holds, languages first, each in key order.
func (c TOMLConf) Items() []Item {
	var out []Item
	langs := slices.Sorted(maps.Keys(c.Scenes))
	for _, l := range slices.Concat(slices.Collect(maps.Keys(c.Matchups)), slices.Collect(maps.Keys(c.Directions))) {
		if !slices.Contains(langs, l) {
			langs = append(langs, l)
		}
	}
	slices.Sort(langs)
	for _, l := range langs {
		out = append(out, Item{"language", l})
	}
	for _, s := range slices.Sorted(maps.Keys(c.Elements)) {
		out = append(out, Item{"elements", s})
	}
	if c.Standard != "" {
		out = append(out, Item{"standard", ""})
	}
	return out
}

// Of is the item as c has it, as a configuration of only that item, and
// whether c has it.
func (c TOMLConf) Of(it Item) (TOMLConf, bool) {
	switch it.Kind {
	case "language":
		p := c.Part([]string{it.Key}, nil)
		p.Standard = ""
		return p, p.Scenes != nil || p.Matchups != nil || p.Directions != nil
	case "elements":
		p := c.Part(nil, []string{it.Key})
		p.Standard = ""
		return p, p.Elements != nil
	case "standard":
		return TOMLConf{Standard: c.Standard}, c.Standard != ""
	}
	return TOMLConf{}, false
}

// Same tells whether a and b hold the same item alike.
func Same(a, b TOMLConf) bool {
	ea, err1 := a.Encode()
	eb, err2 := b.Encode()
	return err1 == nil && err2 == nil && bytes.Equal(ea, eb)
}

// Renamed is a configuration of one item (Of) under another key: a
// language's words, matchup and direction, or an element set, as to.
func Renamed(item TOMLConf, it Item, to string) (TOMLConf, error) {
	to = strings.TrimSpace(to)
	if to == "" || strings.ContainsAny(to, " .\"'[]=#") {
		return TOMLConf{}, fmt.Errorf("%q cannot be a name in the configuration", to)
	}
	out := TOMLConf{}
	switch it.Kind {
	case "language":
		if w, ok := item.Scenes[it.Key]; ok {
			out.Scenes = map[string][]string{to: w}
		}
		if m, ok := item.Matchups[it.Key]; ok {
			out.Matchups = map[string][]string{to: m}
		}
		if d, ok := item.Directions[it.Key]; ok {
			out.Directions = map[string]string{to: d}
		}
	case "elements":
		out.Elements = map[string]Set{to: item.Elements[it.Key]}
	default:
		return TOMLConf{}, fmt.Errorf("the %s has no name to change", it.Kind)
	}
	return out, nil
}
