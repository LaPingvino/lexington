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
	return c.normalized(), nil
}

// Encode is the configuration as TOML text, its empty parts left out.
func (c TOMLConf) Encode() ([]byte, error) {
	var b bytes.Buffer
	if err := toml.NewEncoder(&b).Encode(c); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// With is c with layer over it: each language, each element set, and
// the Standard that layer has take the place of c's (a language whole:
// the fields its table leaves out are the default's, Language); c
// itself is not changed.
func (c TOMLConf) With(layer TOMLConf) TOMLConf {
	c, layer = c.normalized(), layer.normalized()
	out := TOMLConf{
		Elements:  maps.Clone(c.Elements),
		Languages: maps.Clone(c.Languages),
		Standard:  c.Standard,
	}
	if len(layer.Languages) > 0 && out.Languages == nil {
		out.Languages = map[string]Language{}
	}
	for k, v := range layer.Languages {
		out.Languages[k] = v.clone()
	}
	if len(layer.Elements) > 0 && out.Elements == nil {
		out.Elements = map[string]Set{}
	}
	for k, v := range layer.Elements {
		out.Elements[k] = v
	}
	if layer.Standard != "" {
		out.Standard = layer.Standard
	}
	out.TitlePages = maps.Clone(c.TitlePages)
	if len(layer.TitlePages) > 0 && out.TitlePages == nil {
		out.TitlePages = map[string]TitlePage{}
	}
	for k, v := range layer.TitlePages { // a template whole, as a set
		out.TitlePages[k] = v.clone()
	}
	if len(layer.Extensions) > 0 {
		out.Extensions = maps.Clone(c.Extensions)
		if out.Extensions == nil {
			out.Extensions = map[string]string{}
		}
		maps.Copy(out.Extensions, layer.Extensions)
	} else {
		out.Extensions = maps.Clone(c.Extensions)
	}
	return out
}

// Part is what of c a script in the languages langs, laid out with the
// element sets sets, uses: those languages, each in full as c reads it
// (Language), those sets (the ones c has), and the Standard (the
// standard language's words are needed to write it as standard
// Fountain: give it in langs), and the title page templates titles of
// c's own (a preset travels by its name alone).
func (c TOMLConf) Part(langs, sets []string, titles ...string) TOMLConf {
	c = c.normalized()
	out := TOMLConf{Standard: c.StandardLanguage()}
	for _, l := range langs {
		if c.Knows(l) && l != Hekaya {
			if out.Languages == nil {
				out.Languages = map[string]Language{}
			}
			out.Languages[l] = c.Language(l) // the language in full
		}
	}
	for e, l := range c.Extensions { // one's own extensions for these languages
		if slices.Contains(langs, l) {
			if out.Extensions == nil {
				out.Extensions = map[string]string{}
			}
			out.Extensions[e] = l
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
	for _, t := range titles {
		if tp, ok := c.TitlePages[t]; ok {
			if out.TitlePages == nil {
				out.TitlePages = map[string]TitlePage{}
			}
			out.TitlePages[t] = tp.clone()
		}
	}
	return out
}

// The kinds of Item.
const (
	ItemLanguage  = "language"
	ItemElements  = "elements"
	ItemExtension = "extension"
	ItemTitlePage = "titlepage"
	ItemStandard  = "standard"
)

// Item is one named thing in a configuration: a language (Language), an
// element set, an extension, a title page template, or the Standard.
type Item struct {
	Kind string // one of Item...: ItemLanguage, ItemElements and so on
	Key  string // the language, set, extension or template ("" for the standard)
}

// Items are what c holds, languages first, each in key order.
func (c TOMLConf) Items() []Item {
	c = c.normalized()
	var out []Item
	for _, l := range c.LanguageNames() {
		out = append(out, Item{ItemLanguage, l})
	}
	for _, s := range slices.Sorted(maps.Keys(c.Elements)) {
		out = append(out, Item{ItemElements, s})
	}
	for _, e := range slices.Sorted(maps.Keys(c.Extensions)) {
		out = append(out, Item{ItemExtension, e})
	}
	for _, t := range slices.Sorted(maps.Keys(c.TitlePages)) {
		out = append(out, Item{ItemTitlePage, t})
	}
	if c.Standard != "" {
		out = append(out, Item{ItemStandard, ""})
	}
	return out
}

// Of is the item as c has it, as a configuration of only that item, and
// whether c has it.
func (c TOMLConf) Of(it Item) (TOMLConf, bool) {
	switch it.Kind {
	case ItemLanguage:
		p := c.Part([]string{it.Key}, nil)
		p.Standard = ""
		return p, p.Languages != nil
	case ItemElements:
		p := c.Part(nil, []string{it.Key})
		p.Standard = ""
		return p, p.Elements != nil
	case ItemTitlePage:
		p := c.Part(nil, nil, it.Key)
		p.Standard = ""
		return p, p.TitlePages != nil
	case ItemStandard:
		return TOMLConf{Standard: c.Standard}, c.Standard != ""
	case ItemExtension:
		l, ok := c.Extensions[it.Key]
		return TOMLConf{Extensions: map[string]string{it.Key: l}}, ok
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
// language's words, matchup and direction, an element set, or a title
// page template, as to.
func Renamed(item TOMLConf, it Item, to string) (TOMLConf, error) {
	to = strings.TrimSpace(to)
	if to == "" || strings.ContainsAny(to, " .\"'[]=#") {
		return TOMLConf{}, fmt.Errorf("%q cannot be a name in the configuration", to)
	}
	out := TOMLConf{}
	switch it.Kind {
	case ItemLanguage:
		item = item.normalized()
		if l, ok := item.Languages[it.Key]; ok {
			out.Languages = map[string]Language{to: l.clone()}
		}
	case ItemElements:
		out.Elements = map[string]Set{to: item.Elements[it.Key]}
	case ItemTitlePage:
		if t, ok := item.TitlePages[it.Key]; ok {
			out.TitlePages = map[string]TitlePage{to: t.clone()}
		}
	default:
		return TOMLConf{}, fmt.Errorf("the %s has no name to change", it.Kind)
	}
	return out, nil
}
