package fdx

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

const taggedScript = `INT. KITCHEN - NIGHT #12#

Anna puts the *old* revolver on the table.[[check the prop]] A revolver again.

ANNA
Where is the taxi?

EXT. STREET - NIGHT

The taxi waits. A dog barks at the taxi.
`

func taggedSet() *lex.TagSet {
	return &lex.TagSet{
		Categories: []lex.TagCategory{{ID: "aaaaaaaa-0000-4000-8000-000000000001", Name: "Puppets", Color: "#123456"}},
		Elements: []lex.TagElement{
			{ID: "e1", Category: "05c556eb-6bc1-4a3a-b09f-f8b5ba1b6afa", Name: "revolver"},
			{ID: "e2", Category: "04721a56-f54b-49c8-80ad-d53887d6b851", Name: "taxi"},
			{ID: "e3", Category: "09cb0d1c-ce01-4f22-bb64-b5f2e6c491c6", Name: "dog"},
			{ID: "e4", Category: "c5e89e4d-f83e-4c28-950c-92a63f1b5f26", Name: "KITCHEN"},
			{ID: "e5", Category: "aaaaaaaa-0000-4000-8000-000000000001", Name: "dog puppet"},
		},
	}
}

func TestTagsWrittenAndRead(t *testing.T) {
	s := fountain.ParseLines(rules.DefaultConf().Scenes["en"], strings.NewReader(taggedScript))
	tags := map[int][]lex.LineTag{
		1: {{Text: "KITCHEN", Elements: []string{"e4"}}},                                                       // a numbered heading
		3: {{Text: "revolver", Elements: []string{"e1"}}, {Text: "revolver", At: 1, Elements: []string{"e1"}}}, // after emphasis and a note
		6: {{Text: "taxi", Elements: []string{"e2"}}},
		10: {{Text: "taxi", At: 1, Elements: []string{"e2"}}, {Text: "dog", Elements: []string{"e3", "e5"}},
			{Text: "nowhere", Elements: []string{"e1"}}},
	}
	var buf bytes.Buffer
	scenes := []SceneProperty{{Page: 1, Length: 3}, {Page: 2, Length: 11}}
	if err := (&FDXWriter{Tags: taggedSet(), LineTags: tags, Scenes: scenes}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<TagCategory Color="#FFFF00000000" Id="05c556eb-6bc1-4a3a-b09f-f8b5ba1b6afa" Name="Props" Number="6" Style="Bold">`,
		`<TagCategory Color="#121234345656" Id="aaaaaaaa-0000-4000-8000-000000000001" Name="Puppets" Number="32" Style="Bold">`,
		`<TagDefinition CatId="05c556eb-6bc1-4a3a-b09f-f8b5ba1b6afa" Id="e1" Label="revolver" Number="1">`,
		`<Text TagNumber="1">KITCHEN</Text>`,
		`<Text TagNumber="2">revolver</Text>`,
		`<Text TagNumber="3">revolver</Text>`,
		`<Text Style="Italic">old</Text>`,
		`<SceneProperties Length="3/8" Page="1" Title="">`,
		`<SceneProperties Length="1 3/8" Page="2" Title="">`,
		`<DefId>e3</DefId>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in\n%s", want, out)
		}
	}
	if strings.Count(out, `<Tag Number=`) != 6 { // the words not there: no tag
		t.Errorf("%d tags", strings.Count(out, `<Tag Number=`))
	}
	if !regexp.MustCompile(`<Tag Number="6">\s*<DefId>e3</DefId>\s*<DefId>e5</DefId>`).MatchString(out) {
		t.Error("the dog's place is not both elements'")
	}

	back, set, byRow, err := ParseTags(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if set == nil || len(set.Elements) != 5 {
		t.Fatalf("set %+v", set)
	}
	if c, ok := set.Category("aaaaaaaa-0000-4000-8000-000000000001"); !ok || c.Name != "Puppets" || c.Color != "#123456" {
		t.Errorf("own category %+v", c)
	}
	got := map[string][]string{}
	for row, ts := range byRow {
		for _, tag := range ts {
			got[back[row].Contents] = append(got[back[row].Contents], tag.Text+"@"+string(rune('0'+tag.At))+"="+strings.Join(tag.Elements, "+"))
		}
	}
	want := map[string][]string{
		"INT. KITCHEN - NIGHT #12#": {"KITCHEN@0=e4"},
		"Anna puts the *old* revolver on the table. A revolver again. [[check the prop]]": {"revolver@0=e1", "revolver@1=e1"},
		"Where is the taxi?":                       {"taxi@0=e2"},
		"The taxi waits. A dog barks at the taxi.": {"dog@0=e3+e5", "taxi@1=e2"},
	}
	for k, v := range want {
		if strings.Join(got[k], ",") != strings.Join(v, ",") {
			t.Errorf("%q: %v, want %v (all: %v)", k, got[k], v, got)
		}
	}
}

// An OMITTED scene has its page but no length.
func TestOmittedHasNoLength(t *testing.T) {
	s := fountain.ParseLines(rules.DefaultConf().Scenes["en"], strings.NewReader("INT. A - DAY #1#\n\nRain.\n\n.OMITTED #2#\n\nEXT. B - DAY #3#\n\nSun.\n"))
	var buf bytes.Buffer
	scenes := []SceneProperty{{Page: 1, Length: 2}, {Page: 1, Length: 1}, {Page: 1, Length: 2}}
	if err := (&FDXWriter{Scenes: scenes}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(buf.String(), `Length="2/8"`); n != 2 || strings.Contains(buf.String(), `Length="1/8"`) {
		t.Errorf("%d lengths; the omitted one's: %v", n, strings.Contains(buf.String(), `Length="1/8"`))
	}
}

// Without tags or lengths, nothing of either is written (the other
// goldens stay as they are).
func TestNoTagData(t *testing.T) {
	s := fountain.ParseLines(rules.DefaultConf().Scenes["en"], strings.NewReader(taggedScript))
	var buf bytes.Buffer
	if err := (&FDXWriter{}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); strings.Contains(out, "TagData") || strings.Contains(out, "Length=") || strings.Contains(out, "TagNumber") {
		t.Errorf("tags or lengths written:\n%s", out)
	}
}

// Final Draft's own TagData: Version 5's names and its Miscellaneous's Id
// are the standard categories.
func TestFinalDraftCategories(t *testing.T) {
	for _, f := range []string{"../testdata/fdx/screenplay-tools/TestFDX-FD.fdx", "../testdata/fdx/fdx-mcp-server/NewFile.fdx"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		_, set, _, err := ParseTags(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if set == nil || len(set.Categories) != 0 {
			t.Errorf("%s: categories of their own %+v", f, set)
		}
	}
	for name, want := range map[string]string{"Cast": "Cast Members", "Animal Handler": "Animal Wrangler",
		"148267ae-50ae-4746-8202-659fb7447b55": "Miscellaneous", "prop": "Props", "costume": "Wardrobe",
		"MAKEUP/HAIR": "Makeup/Hair", "setDesign": "Set Dressing"} {
		if c, ok := lex.TagCategoryOf(name); !ok || c.Name != want {
			t.Errorf("%s: %+v", name, c)
		}
	}
}
