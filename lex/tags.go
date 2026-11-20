package lex

import (
	"strings"
	"unicode"
)

// Breakdown tags: words of the script tagged as an element (a prop, a
// vehicle, a cast member) of a category, as a production's breakdown
// lists them per scene.
//
// The categories are Final Draft's, which Movie Magic Scheduling, Fade
// In (OSF), Beat and StudioBinder share: their Ids, Names and Numbers as
// Final Draft 13 writes them in a new script (testdata/fdx/fdx-mcp-server/
// NewFile.fdx, Version 6). Final Draft 13 gives them all one colour but
// Cast's (#94AA11150000), so the colours are the ones Final Draft wrote
// in a Version 5 file (testdata/fdx/screenplay-tools/TestFDX-FD.fdx) and
// Fade In writes in OSF 4.0 (the osf-sdk's v4/OSF-4.0.xml,
// <tag_categories>): the same values in both. Categories Final Draft 13
// added after that file (Art Department, Script Day, Unit, Sequence,
// Location, Comments, Synopsis) have colours of Lexington's own.
//
// The older names and Ids are read as the same categories (Aliases):
// Version 5's "Cast" and "Animal Handler", its Miscellaneous's Id, and
// Beat's own category keys (BeatTagging.m).

// TagCategory is a breakdown category.
type TagCategory struct {
	ID     string // Final Draft's (a UUID; one of one's own for a new category)
	Name   string
	Number int    // Final Draft's Number
	Color  string // #RRGGBB
	// Aliases are other names (and Ids) of the category, read as it.
	Aliases []string
}

// TagCategories are the standard categories, in the order a breakdown
// sheet lists them.
var TagCategories = []TagCategory{
	{"01fc9642-84ff-4366-b37c-a3068dee57e8", "Cast Members", 2, "#0000FF", []string{"Cast", "cast", "Character"}},
	{"028a4e2b-b507-4d09-88ab-90e3edae9071", "Background Actors", 3, "#00BFFF", []string{"Extras", "extras", "Background"}},
	{"0377dbe6-77a3-41af-bda8-86eb2468fdbf", "Stunts", 4, "#FFA500", []string{"Stunt", "stunt"}},
	{"04721a56-f54b-49c8-80ad-d53887d6b851", "Vehicles", 5, "#8B0000", []string{"Vehicle", "vehicle"}},
	{"05c556eb-6bc1-4a3a-b09f-f8b5ba1b6afa", "Props", 6, "#FF0000", []string{"Prop", "prop"}},
	{"47b02ff1-5161-4137-b736-f36eebba7643", "Camera", 7, "#CCFF00", []string{"camera"}},
	{"069e18b8-2109-4f3d-94e7-d802027a60a8", "Special Effects", 8, "#1E90FF", []string{"Special Effect", "SFX", "sfx", "Practical FX"}},
	{"0726fa85-1e65-4ab8-87de-bf21d09b01f0", "Wardrobe", 9, "#800080", []string{"Costume", "costume", "Costumes"}},
	{"08ae1eef-32ce-415f-9a9b-0982d2453ec4", "Makeup/Hair", 10, "#9370DB", []string{"Makeup", "makeup", "Makeup & hair", "Hair"}},
	{"09cb0d1c-ce01-4f22-bb64-b5f2e6c491c6", "Animals", 11, "#CD853F", []string{"Animal", "animal"}},
	{"0ae40617-cc7c-48e6-ae2b-5aaecc09986f", "Animal Wrangler", 12, "#8B4513", []string{"Animal Handler"}},
	{"0b0b44c9-aa4b-4c40-88b1-d94472ad7a26", "Music", 13, "#20B2AA", []string{"music"}},
	{"0ce7d308-096d-4603-8fe8-349f72cd89ff", "Sound", 15, "#008080", []string{"sound"}},
	{"c86eae40-3b01-41c3-a7de-6859e6ec971d", "Art Department", 16, "#A0522D", []string{"Art"}},
	{"0debb71b-5743-4c53-80cc-e17e841ce645", "Set Dressing", 17, "#BA55D3", []string{"Scenography", "setDesign"}},
	{"0e7a8fc5-5441-4bad-a9bf-5ddd3fe51c69", "Greenery", 18, "#008000", nil},
	{"0ff5cda4-4d43-4cfe-940f-91380c46fdad", "Special Equipment", 19, "#BDB76B", nil},
	{"109d0eaa-0334-4823-ac0c-b44d3f209dc4", "Security", 20, "#DAA520", nil},
	{"1179a4b1-70ee-4011-b4a2-809a0af09e92", "Additional Labor", 21, "#B8860B", []string{"Additional Labour", "Additional Work"}},
	{"12ab0932-e3b9-4b4a-bcd0-3da1b4e61d5e", "Visual Effects", 23, "#32CD32", []string{"VFX", "vfx", "Optical FX"}},
	{"135cc9d1-c4d5-4d00-83d9-571f584ea9cd", "Mechanical Effects", 24, "#B22222", []string{"Mechanical FX"}},
	{"ce04f547-f7ee-40c9-ab66-d95a0c98034e", "Miscellaneous", 25, "#708090",
		[]string{"148267ae-50ae-4746-8202-659fb7447b55", "Misc", "Misc.", "Other", "other"}},
	{"15b6f4fd-4e74-4ad8-9971-b239d88c2997", "Notes", 26, "#FFD700", nil},
	{"63c140da-ef2b-491a-b416-b46f461abb89", "Script Day", 27, "#4682B4", nil},
	{"849f1ebf-5507-4f33-bff6-3a5b4d73be14", "Unit", 28, "#696969", nil},
	{"70877d87-30ef-45b6-be46-c6fa94b83a71", "Sequence", 29, "#6A5ACD", nil},
	{"c5e89e4d-f83e-4c28-950c-92a63f1b5f26", "Location", 30, "#2E8B57", nil},
	{"216f33fd-fc42-4269-be01-b05b18f815a0", "Comments", 31, "#A9A9A9", nil},
	{"8e5e75c2-713b-47df-a75f-f12648b98ded", "Synopsis", 1, "#000000", nil},
}

// CastCategory is the name of the category of the cast.
const CastCategory = "Cast Members"

// TagCategoryOf is the standard category a name or an Id stands for
// (its Name, an Alias, any case; spaces, dots and & aside).
func TagCategoryOf(s string) (TagCategory, bool) {
	k := categoryKey(s)
	if k == "" {
		return TagCategory{}, false
	}
	for _, c := range TagCategories {
		if categoryKey(c.ID) == k || categoryKey(c.Name) == k {
			return c, true
		}
	}
	for _, c := range TagCategories {
		for _, a := range c.Aliases {
			if categoryKey(a) == k {
				return c, true
			}
		}
	}
	return TagCategory{}, false
}

func categoryKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TagElement is a thing tagged: one per name and category, tagged in as
// many places as it appears.
type TagElement struct {
	ID       string // a UUID, lower case (Final Draft's TagDefinition Id)
	Category string // its category's ID
	Name     string // the label: "revolver"
}

// TagSet are a script's breakdown categories and elements: what an FDX's
// <TagData> holds besides the tags in the text.
type TagSet struct {
	// Categories are those of the elements (and any of one's own): the
	// standard ones (TagCategories) are always written.
	Categories []TagCategory
	Elements   []TagElement
}

// Category is the category with the Id, of the set's or a standard one.
func (s *TagSet) Category(id string) (TagCategory, bool) {
	if s != nil {
		for _, c := range s.Categories {
			if c.ID == id {
				return c, true
			}
		}
	}
	return TagCategoryOf(id)
}

// LineTag is words of a line tagged as elements: Text, the At-th time
// (from 0) it stands in the line's TagPlain text.
type LineTag struct {
	Text     string
	At       int
	Elements []string // the elements' IDs
}

// TagPlain is a line's text as the tags count it: without its notes,
// emphasis markers and (for a scene heading) its number; what an FDX
// paragraph's runs hold.
func TagPlain(contents string, scene bool) string {
	text := StripNotes(contents)
	if scene {
		text, _ = SceneNumber(text)
	}
	var b strings.Builder
	for _, s := range Emphasis(text) {
		b.WriteString(s.Text)
	}
	return b.String()
}

// TagIndex is where the At-th Text is in plain, a byte offset (-1: not
// there). Matched as written, else in any case.
func TagIndex(plain, text string, at int) int {
	if text == "" {
		return -1
	}
	for _, fold := range []bool{false, true} {
		p, t := plain, text
		if fold {
			p, t = strings.ToLower(plain), strings.ToLower(text)
			if len(p) != len(plain) { // a case change of another length
				return -1
			}
		}
		off := 0
		for n := 0; ; n++ {
			i := strings.Index(p[off:], t)
			if i < 0 {
				break
			}
			if n == at {
				return off + i
			}
			off += i + len(t)
		}
	}
	return -1
}
