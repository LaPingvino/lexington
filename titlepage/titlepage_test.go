package titlepage

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

func parse(t *testing.T, src string) lex.Screenplay {
	t.Helper()
	return fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(src))
}

// placed is the layout as "place: Key=line|line; Key=…" lines.
func placed(blocks []Block) string {
	var out []string
	for _, b := range blocks {
		var fs []string
		for _, f := range b.Fields {
			v := strings.Join(f.Lines, "|")
			if f.Picture != "" {
				v = "picture " + f.Picture
			}
			fs = append(fs, f.Key+"="+v)
		}
		out = append(out, b.Place+": "+strings.Join(fs, "; "))
	}
	return strings.Join(out, "\n")
}

const mixed = "Contact:\n    Next Level\n    Solvang\nTitle: MIXED\nDraft date: 1/20\nAuthor: Someone\n" +
	"Credit: Written by\nLogline: A tale.\n\nEXT. PLACE - DAY\n\nAction.\n"

// The order the keys are written in does not place them: Fountain's
// centred keys in Fountain's order, the rest at the foot as written.
func TestStandardPlacesByKey(t *testing.T) {
	got := placed(Standard().Layout(parse(t, mixed)))
	want := "centre: Title=MIXED; Credit=Written by; Author=Someone\n" +
		"foot-left: Contact=Next Level|Solvang; Draft date=1/20; Logline=A tale."
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A language's TitleCentred takes the place of Fountain's.
func TestLanguageCentred(t *testing.T) {
	got := placed(Template{Centred: []string{"Logline", "Title"}}.Layout(parse(t, mixed)))
	want := "centre: Logline=A tale.; Title=MIXED\n" +
		"foot-left: Contact=Next Level|Solvang; Draft date=1/20; Author=Someone; Credit=Written by"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	c := rules.DefaultConf()
	c.Languages["en"] = rules.Language{TitleCentred: []string{"Title"}}
	if tpl, ok := For(c, "", "en"); !ok || len(tpl.Centred) != 1 {
		t.Errorf("For: %+v %v", tpl, ok)
	}
}

// A template's keys in its places and order; "*" for the rest; hidden
// ones left out; a free place a block of its own.
func TestTemplatePlaces(t *testing.T) {
	tpl := Template{Page: rules.TitlePage{Keys: []rules.TitleKey{
		{Key: "Logline", Place: "top"},
		{Key: "author", Place: "centre"},
		{Key: "Title", Place: "centre", Size: 24},
		{Key: "Draft date", Place: "foot-right"},
		{Key: "Credit", Hide: true},
		{Key: "*", Place: "free", X: 1, Y: -1},
	}}}
	got := placed(tpl.Layout(parse(t, mixed)))
	want := "top: Logline=A tale.\ncentre: Author=Someone; Title=MIXED\nfoot-right: Draft date=1/20\nfree: Contact=Next Level|Solvang"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// a named key without a place: the language's
	tpl = Template{Page: rules.TitlePage{Keys: []rules.TitleKey{{Key: "Author", Size: 14}, {Key: "Contact"}}}}
	got = placed(tpl.Layout(parse(t, mixed)))
	want = "centre: Author=Someone; Title=MIXED; Credit=Written by\nfoot-left: Contact=Next Level|Solvang; Draft date=1/20; Logline=A tale."
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A Markdown picture is a picture under any key; one the template does
// not name goes above the title. A key marked Picture takes a bare name.
func TestPictures(t *testing.T) {
	s := parse(t, "Title: T\nLogo: ![The logo](assets/logo.png)\nArt: art.jpg\n\nINT. A - DAY\n")
	got := placed(Standard().Layout(s))
	want := "centre: Logo=picture assets/logo.png; Title=T\nfoot-left: Art=art.jpg"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	tpl := Template{Page: rules.TitlePage{Keys: []rules.TitleKey{{Key: "Art", Place: "top", Picture: true}, {Key: "Logo", Place: "foot-right"}}}}
	got = placed(tpl.Layout(s))
	want = "top: Art=picture art.jpg\ncentre: Title=T\nfoot-right: Logo=picture assets/logo.png"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if f, a, ok := Picture("![alt text]( <my file.png> )", false); !ok || f != "my file.png" || a != "alt text" {
		t.Errorf("Picture: %q %q %v", f, a, ok)
	}
}

// Stream: the centred fields first, a metasection row, the foot's; the
// rest of the script as it was.
func TestStream(t *testing.T) {
	var got []string
	for _, l := range Standard().Stream(parse(t, mixed)) {
		if l.Type != lex.TypeEmpty {
			got = append(got, string(l.Type)+"="+l.Contents)
		}
	}
	want := "titlepage=|Title=MIXED|Credit=Written by|Author=Someone|metasection=|Contact=Next Level|Contact=Solvang|" +
		"Draft date=1/20|Logline=A tale.|newpage=|scene=EXT. PLACE - DAY|action=Action."
	if g := strings.Join(got, "|"); g != want {
		t.Errorf("got:\n%s\nwant:\n%s", g, want)
	}
}

// The parse keeps the text's order: the rows are the lines as written.
func TestParseKeepsOrder(t *testing.T) {
	var got []string
	for _, l := range parse(t, mixed) {
		if l.Type != lex.TypeEmpty {
			got = append(got, string(l.Type))
		}
	}
	want := "titlepage Contact Contact Title Draft date Author Credit Logline newpage scene action"
	if g := strings.Join(got, " "); g != want {
		t.Errorf("got %s\nwant %s", g, want)
	}
}

// Templates come from the configuration, layered over the presets.
func TestTemplatesFromConf(t *testing.T) {
	var c rules.TOMLConf
	if _, err := toml.Decode(`
[TitlePages.mine]
Name = "Mine"
Centre = 3
[[TitlePages.mine.Keys]]
Key = "Title"
Size = 20
[[TitlePages.mine.Keys]]
Key = "Draft date"
Place = "foot-right"
[TitlePages.bold]
Description = "my bold"
`, &c); err != nil {
		t.Fatal(err)
	}
	full := rules.DefaultConf().With(c)
	mine, ok := full.TitlePage("mine")
	if !ok || mine.Centre != 3 || len(mine.Keys) != 2 || mine.Keys[1].Key != "Draft date" {
		t.Errorf("mine: %+v %v", mine, ok)
	}
	if b, _ := full.TitlePage("bold"); b.Description != "my bold" || len(b.Keys) != 0 {
		t.Errorf("one's own bold takes the preset's place: %+v", b)
	}
	if p, ok := full.TitlePage("Poster"); !ok || len(p.Keys) == 0 {
		t.Error("the poster preset is known")
	}
	if _, ok := full.TitlePage(""); !ok {
		t.Error("the standard template is known by no name")
	}
	if _, ok := full.TitlePage("nothing"); ok {
		t.Error("an unknown template is found")
	}
	if got := strings.Join(full.TitlePageNames(), " "); got != "standard bold poster mine" {
		t.Errorf("names: %s", got)
	}
	if p := full.Part(nil, nil, "mine", "poster"); len(p.TitlePages) != 1 {
		t.Errorf("Part carries one's own templates only: %v", p.TitlePages)
	}
	enc, err := rules.DefaultConf().Encode()
	if err != nil || strings.Contains(string(enc), "TitlePages") {
		t.Errorf("the presets are not written into a configuration: %v", err)
	}
}

// A field's format: the set's title format in the centre, its meta
// format at the foot, with the entry's type over it.
func TestFormat(t *testing.T) {
	f := Format(rules.Default, rules.TitleKey{Place: rules.PlaceCentre, Size: 24, Style: "b"})
	if f.Align != "C" || f.Size != 24 || f.Style != "B" {
		t.Errorf("centre: %+v", f)
	}
	f = Format(rules.Default, rules.TitleKey{Place: rules.PlaceFootRight})
	if f.Align != "R" || f.Size != 12 {
		t.Errorf("foot-right: %+v", f)
	}
}

// A key spelled as a body element is read as a key (with a capital), and
// the title page goes on after it; an empty TitleCentred is Fountain's.
func TestKeyLikeABodyElement(t *testing.T) {
	s := parse(t, "picture: ![Art](art.png)\naction: none\nTitle: X\nAuthor: A\n\nINT. A - DAY\n")
	got := placed(Template{Centred: []string{}}.Layout(s))
	want := "centre: Picture=picture art.png; Title=X; Author=A\nfoot-left: Action=none"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if c := (rules.Language{TitleCentred: []string{}}).Centred(); len(c) != 4 {
		t.Errorf("empty TitleCentred: %v", c)
	}
}
