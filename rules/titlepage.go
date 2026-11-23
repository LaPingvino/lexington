package rules

import (
	"maps"
	"slices"
	"strings"
)

// Title page templates: how a title page is laid out, by key. A script's
// title page is "Key: value" fields in the order they are written; a
// template puts each key in its place (centred under the title, at the
// top, at the foot left or right, or anywhere), in its size and style,
// whatever the order they were written in. A key the template does not
// name is placed by the script's language (Language.TitleCentred):
// centred if it is one of those, else at the foot. The writers lay the
// page out (package titlepage); the parse keeps the text's order.
//
// Templates are named entries of a configuration ([TitlePages.name]),
// layered as the element sets are: one of one's own takes the place of
// a preset of the same name, whole. The presets (TitlePagePresets) are
// known in every configuration without being written into it.

// The places a title page key can have (TitleKey.Place).
const (
	PlaceCentre    = "centre"     // the block under the title, centred
	PlaceTop       = "top"        // at the top of the page, centred
	PlaceFootLeft  = "foot-left"  // at the page's foot, on the left
	PlaceFootRight = "foot-right" // at the page's foot, on the right
	PlaceFree      = "free"       // at X, Y
)

// Places are the places, as TitleKey.Place takes them.
var Places = []string{PlaceCentre, PlaceTop, PlaceFootLeft, PlaceFootRight, PlaceFree}

// OtherKeys is the TitleKey.Key that stands for every key a template
// does not name.
const OtherKeys = "*"

// StandardTitlePage is the name of Fountain's own title page: the empty
// template, every key placed by the language.
const StandardTitlePage = "standard"

// TitlePage is a title page template.
type TitlePage struct {
	// Name is the template's name as lists show it (its key if none).
	Name string `toml:"Name,omitempty"`
	// Description says what it is for.
	Description string `toml:"Description,omitempty"`
	// Centre is where the centred block (PlaceCentre) starts, in inches
	// from the top of a Letter page (0: 4, Fountain's).
	Centre float64 `toml:"Centre,omitempty"`
	// Keys are the keys it places, each block's in this order.
	Keys []TitleKey `toml:"Keys,omitempty"`
}

// TitleKey is how a template prints one title page key.
type TitleKey struct {
	// Key is the standard key (Title, Credit, Draft date), or any key
	// of one's own (Logline, Registration); OtherKeys ("*") for every
	// key the template does not name. Case does not count.
	Key string `toml:"Key"`
	// Place is one of Places; "" the language's (TitleCentred: centre,
	// else the foot on the left).
	Place string `toml:"Place,omitempty"`
	// X and Y are a free place's position, in inches on a Letter page
	// (scaled to the paper): X from the left edge (0, centred: the
	// page's middle), Y from the top, or from the foot when negative.
	X float64 `toml:"X,omitempty"`
	Y float64 `toml:"Y,omitempty"`
	// Align is L, C or R ("": the place's: centred in the centre and at
	// the top, left at the foot's left, right at its right). In a free
	// place it is how the text stands to X.
	Align string `toml:"Align,omitempty"`
	// Font, Size (points) and Style (b, i, u joined) are its type; ""
	// and 0 are the element set's: its title format in the centre and at
	// the top, its meta format elsewhere.
	Font  string  `toml:"Font,omitempty"`
	Size  float64 `toml:"Size,omitempty"`
	Style string  `toml:"Style,omitempty"`
	// Caps prints it in capitals.
	Caps bool `toml:"Caps,omitempty"`
	// Before is the space before it, in lines (of 12 pt).
	Before float64 `toml:"Before,omitempty"`
	// Picture: its value is a picture's file (logo.png, or Markdown's
	// ![Key art](art.png)), printed Height inches high. A value written
	// as a Markdown picture is a picture under any key.
	Picture bool    `toml:"Picture,omitempty"`
	Height  float64 `toml:"Height,omitempty"`
	// Hide leaves it off the page.
	Hide bool `toml:"Hide,omitempty"`
}

// PictureHeight is a picture's height when the template gives none, in
// inches.
const PictureHeight = 2.0

// Entry is the template's entry for key, and its rank among the
// template's keys: the entry that names it, else the OtherKeys one; ok
// is false when neither is there.
func (t TitlePage) Entry(key string) (e TitleKey, rank int, ok bool) {
	key = strings.TrimSpace(key)
	other := -1
	for i, k := range t.Keys {
		if strings.EqualFold(strings.TrimSpace(k.Key), key) {
			return k, i, true
		}
		if strings.TrimSpace(k.Key) == OtherKeys && other < 0 {
			other = i
		}
	}
	if other >= 0 {
		return t.Keys[other], other, true
	}
	return TitleKey{}, 0, false
}

// Named are the keys the template names, as written (not OtherKeys).
func (t TitlePage) Named() []string {
	var out []string
	for _, k := range t.Keys {
		if k := strings.TrimSpace(k.Key); k != "" && k != OtherKeys {
			out = append(out, k)
		}
	}
	return out
}

// FountainCentred are the keys Fountain prints centred: what a language
// without TitleCentred centres.
var FountainCentred = []string{"Title", "Credit", "Author", "Source"}

// Centred are the keys the language prints centred (TitleCentred, else,
// empty or none, Fountain's).
func (l Language) Centred() []string {
	if len(l.TitleCentred) > 0 {
		return slices.Clone(l.TitleCentred)
	}
	return slices.Clone(FountainCentred)
}

// titlePagePresets are the templates every configuration knows.
var titlePagePresets = map[string]TitlePage{
	StandardTitlePage: {
		Name: "Standard",
		Description: "Fountain's: Title, Credit, Author and Source centred under each other, " +
			"the rest at the foot of the page (as the script's language has it).",
	},
	"bold": {
		Name:        "Bold",
		Description: "A big title in capitals, the credits spaced out under it; draft date at the foot on the right, the rest on the left.",
		Centre:      3.5,
		Keys: []TitleKey{
			{Key: "Title", Place: PlaceCentre, Size: 26, Style: "b", Caps: true},
			{Key: "Logline", Place: PlaceCentre, Style: "i", Before: 1.5},
			{Key: "Credit", Place: PlaceCentre, Before: 3},
			{Key: "Author", Place: PlaceCentre, Before: 1},
			{Key: "Source", Place: PlaceCentre, Before: 2, Style: "i"},
			{Key: "Draft date", Place: PlaceFootRight},
			{Key: "Draft", Place: PlaceFootRight},
			{Key: "Contact", Place: PlaceFootLeft},
			{Key: OtherKeys, Place: PlaceFootLeft},
		},
	},
	"poster": {
		Name: "Poster",
		Description: "A picture above the title (Picture: art.png, or ![Key art](art.png)), the title big under it, " +
			"then the logline and the credits; contact and draft at the foot.",
		Centre: 1.5,
		Keys: []TitleKey{
			{Key: "Picture", Place: PlaceCentre, Picture: true, Height: 3.5},
			{Key: "Title", Place: PlaceCentre, Size: 30, Style: "b", Caps: true, Before: 1},
			{Key: "Logline", Place: PlaceCentre, Style: "i", Before: 1},
			{Key: "Credit", Place: PlaceCentre, Before: 2},
			{Key: "Author", Place: PlaceCentre},
			{Key: "Source", Place: PlaceCentre, Before: 1},
			{Key: "Contact", Place: PlaceFootLeft},
			{Key: "Draft date", Place: PlaceFootRight},
			{Key: "Draft", Place: PlaceFootRight},
			{Key: OtherKeys, Place: PlaceFootLeft},
		},
	},
}

// TitlePagePresets are the title page templates every configuration
// knows, by name: standard (Fountain's), bold and poster.
func TitlePagePresets() map[string]TitlePage {
	out := map[string]TitlePage{}
	for k, v := range titlePagePresets {
		out[k] = v.clone()
	}
	return out
}

func (t TitlePage) clone() TitlePage {
	t.Keys = slices.Clone(t.Keys)
	return t
}

// TitlePage is the title page template named name: one of c's own, else
// a preset; "" is the standard one. ok is false when there is none.
func (c TOMLConf) TitlePage(name string) (TitlePage, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = StandardTitlePage
	}
	if t, ok := c.TitlePages[name]; ok {
		return t.clone(), true
	}
	if t, ok := titlePagePresets[name]; ok {
		return t.clone(), true
	}
	for k, t := range c.TitlePages { // a name in another case
		if strings.EqualFold(k, name) {
			return t.clone(), true
		}
	}
	for k, t := range titlePagePresets {
		if strings.EqualFold(k, name) {
			return t.clone(), true
		}
	}
	return TitlePage{}, false
}

// TitlePageNames are the templates c knows: the standard one, the other
// presets, then c's own, each in name order (one of c's own named as a
// preset is in the preset's place).
func (c TOMLConf) TitlePageNames() []string {
	out := []string{StandardTitlePage}
	for _, k := range slices.Sorted(maps.Keys(titlePagePresets)) {
		if k != StandardTitlePage {
			out = append(out, k)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(c.TitlePages)) {
		if !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// TitlePageName is the name a list shows for the template key.
func (c TOMLConf) TitlePageName(key string) string {
	if t, ok := c.TitlePage(key); ok && t.Name != "" {
		return t.Name
	}
	if key == "" {
		return titlePagePresets[StandardTitlePage].Name
	}
	return key
}
