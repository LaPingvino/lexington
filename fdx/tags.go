package fdx

import (
	"cmp"
	"io"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Breakdown tags in FDX (lex/tags.go), as Final Draft's Tagger keeps them:
//
//	<TagData>
//	  <TagCategories>
//	    <TagCategory Color="#FFFF00000000" Id="05c556eb-…" Name="Props" Number="6" Style="Bold"/>
//	  </TagCategories>
//	  <TagDefinitions>
//	    <TagDefinition CatId="05c556eb-…" Id="058dcea8-…" Label="revolver" Number="1"/>
//	  </TagDefinitions>
//	  <Tags>
//	    <Tag Number="1"><DefId>058dcea8-…</DefId></Tag>
//	  </Tags>
//	</TagData>
//
// and in the text <Text TagNumber="1">revolver</Text>. The categories are
// as Final Draft 13 writes them (NewFile.fdx); the definitions, tags and
// TagNumbers as Beat writes them (BeatFDXExport.m), the one writer of
// them we could read: no Final Draft file with tags was at hand, so the
// rest is *unverified*. A Tag is a place in the text; its DefIds are the
// elements tagged there (one, or several when the same words are tagged
// in more than one category). The Numbers of definitions count on through
// the file, as Beat's do.
//
// Movie Magic Scheduling's own documentation says an FDX import reads
// "the header (slugline) and the talking characters"; tags reach it only
// through a .sex file (closed), as Final Draft's knowledge base says too.
// The TagData is for Final Draft, Beat, Fade In and StudioBinder.

// FdxTagData is the <TagData> element.
type FdxTagData struct {
	Categories  FdxTagCategories  `xml:"TagCategories"`
	Definitions FdxTagDefinitions `xml:"TagDefinitions"`
	Tags        FdxTags           `xml:"Tags"`
}

// FdxTagCategories is <TagCategories>.
type FdxTagCategories struct {
	List []FdxTagCategory `xml:"TagCategory"`
}

// FdxTagCategory is a category; Color is Final Draft's 16-bit hex.
type FdxTagCategory struct {
	Color  string `xml:"Color,attr,omitempty"`
	ID     string `xml:"Id,attr"`
	Name   string `xml:"Name,attr"`
	Number string `xml:"Number,attr"`
	Style  string `xml:"Style,attr"`
}

// FdxTagDefinitions is <TagDefinitions>.
type FdxTagDefinitions struct {
	List []FdxTagDefinition `xml:"TagDefinition"`
}

// FdxTagDefinition is an element: its category, label and number.
type FdxTagDefinition struct {
	CatID  string `xml:"CatId,attr"`
	ID     string `xml:"Id,attr"`
	Label  string `xml:"Label,attr"`
	Number string `xml:"Number,attr"`
}

// FdxTags is <Tags>.
type FdxTags struct {
	List []FdxTag `xml:"Tag"`
}

// FdxTag is a place tagged: the elements tagged there.
type FdxTag struct {
	Number string   `xml:"Number,attr"`
	DefIDs []string `xml:"DefId"`
}

// fdHex16 is #RRGGBB as Final Draft's #RRRRGGGGBBBB.
func fdHex16(rgb string) string {
	rgb = strings.TrimPrefix(rgb, "#")
	if len(rgb) != 6 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('#')
	for i := 0; i < 6; i += 2 {
		b.WriteString(strings.ToUpper(rgb[i:i+2] + rgb[i:i+2]))
	}
	return b.String()
}

// rgbOf16 is Final Draft's #RRRRGGGGBBBB (or #RRGGBB) as #RRGGBB.
func rgbOf16(c string) string {
	c = strings.TrimPrefix(c, "#")
	switch len(c) {
	case 6:
		return "#" + strings.ToUpper(c)
	case 12:
		return "#" + strings.ToUpper(c[0:2]+c[4:6]+c[8:10])
	}
	return ""
}

// tagWriter numbers a set's elements and the places tagged as the
// document is built.
type tagWriter struct {
	set  *lex.TagSet
	defs map[string]bool // the elements' IDs
	used map[string]bool // those tagged somewhere
	tags []FdxTag
}

func newTagWriter(set *lex.TagSet) *tagWriter {
	if set == nil {
		return nil
	}
	t := &tagWriter{set: set, defs: map[string]bool{}, used: map[string]bool{}}
	for _, e := range set.Elements {
		t.defs[e.ID] = true
	}
	return t
}

// place gives a place tagged with elements its Tag; its Number.
func (t *tagWriter) place(elements []string) string {
	var ids []string
	for _, id := range elements {
		if t.defs[id] {
			ids = append(ids, id)
			t.used[id] = true
		}
	}
	if len(ids) == 0 {
		return ""
	}
	n := strconv.Itoa(len(t.tags) + 1)
	t.tags = append(t.tags, FdxTag{Number: n, DefIDs: ids})
	return n
}

// tagRuns puts the line's tags on a paragraph's runs: each tagged text
// found in them (TagPlain's text, which the runs hold) gets its runs of
// its own with the place's TagNumber. A tag whose words are not there is
// left out.
func (t *tagWriter) tagRuns(texts []FdxText, tags []lex.LineTag) []FdxText {
	if t == nil || len(tags) == 0 {
		return texts
	}
	var plain strings.Builder
	for _, x := range texts {
		plain.WriteString(x.Content)
	}
	type span struct {
		from, to int
		number   string
	}
	var spans []span
	for _, tag := range tags {
		i := lex.TagIndex(plain.String(), tag.Text, tag.At)
		if i < 0 {
			continue
		}
		s := span{i, i + len(tag.Text), ""}
		overlaps := false
		for _, o := range spans {
			overlaps = overlaps || (s.from < o.to && o.from < s.to)
		}
		if overlaps { // one place, one Tag: words tagged twice are the first's
			continue
		}
		if s.number = t.place(tag.Elements); s.number != "" {
			spans = append(spans, s)
		}
	}
	if len(spans) == 0 {
		return texts
	}
	// cut the runs at every span's ends
	var out []FdxText
	at := 0
	for _, x := range texts {
		start, end := at, at+len(x.Content)
		at = end
		cuts := []int{start}
		for _, s := range spans {
			for _, c := range []int{s.from, s.to} {
				if c > start && c < end {
					cuts = append(cuts, c)
				}
			}
		}
		cuts = append(cuts, end)
		sortInts(cuts)
		for k := 0; k+1 < len(cuts); k++ {
			if cuts[k] == cuts[k+1] {
				continue
			}
			piece := x
			piece.Content = x.Content[cuts[k]-start : cuts[k+1]-start]
			for _, s := range spans {
				if cuts[k] >= s.from && cuts[k+1] <= s.to {
					piece.TagNumber = s.number
				}
			}
			out = append(out, piece)
		}
		if start == end {
			out = append(out, x)
		}
	}
	return out
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// tagData is the <TagData> of the document: the standard categories and
// the set's own, every element, the places tagged.
func (t *tagWriter) tagData() *FdxTagData {
	if t == nil {
		return nil
	}
	d := &FdxTagData{}
	seen := map[string]bool{}
	addCat := func(c lex.TagCategory) {
		if seen[c.ID] {
			return
		}
		seen[c.ID] = true
		d.Categories.List = append(d.Categories.List, FdxTagCategory{Color: fdHex16(c.Color), ID: c.ID, Name: c.Name,
			Number: strconv.Itoa(c.Number), Style: "Bold"})
	}
	for _, c := range lex.TagCategories {
		addCat(c)
	}
	next := 0
	for _, c := range t.set.Categories {
		next = max(next, c.Number)
	}
	for _, c := range lex.TagCategories {
		next = max(next, c.Number)
	}
	for _, c := range t.set.Categories {
		if c.Number == 0 {
			next++
			c.Number = next
		}
		addCat(c)
	}
	for i, e := range t.set.Elements {
		cat := e.Category
		if c, ok := t.set.Category(cat); ok {
			cat = c.ID
		}
		d.Definitions.List = append(d.Definitions.List, FdxTagDefinition{CatID: cat, ID: e.ID, Label: e.Name,
			Number: strconv.Itoa(i + 1)})
	}
	d.Tags.List = t.tags
	return d
}

// SceneProperty is a scene's page and length as Final Draft writes them
// in its SceneProperties: Page="12" Length="1 3/8".
type SceneProperty struct {
	Page   int
	Length lex.Eighths
}

// readTags reads a file's TagData: the categories (the standard ones by
// their Id, else by their name; others as they are), the elements, and
// each Tag's elements by its Number.
func readTags(d *FdxTagData) (*lex.TagSet, map[string][]string) {
	if d == nil {
		return nil, nil
	}
	set := &lex.TagSet{}
	catOf := map[string]string{} // the file's category Id → ours
	for _, c := range d.Categories.List {
		if std, ok := lex.TagCategoryOf(c.ID); ok {
			catOf[c.ID] = std.ID
			continue
		}
		if std, ok := lex.TagCategoryOf(c.Name); ok {
			catOf[c.ID] = std.ID
			continue
		}
		n, _ := strconv.Atoi(c.Number)
		set.Categories = append(set.Categories, lex.TagCategory{ID: c.ID, Name: c.Name, Number: n, Color: rgbOf16(c.Color)})
		catOf[c.ID] = c.ID
	}
	for _, e := range d.Definitions.List {
		cat := cmp.Or(catOf[e.CatID], e.CatID)
		if _, ok := set.Category(cat); !ok { // a category the file does not list
			set.Categories = append(set.Categories, lex.TagCategory{ID: cat, Name: "Category " + e.CatID})
		}
		set.Elements = append(set.Elements, lex.TagElement{ID: e.ID, Category: cat, Name: e.Label})
	}
	places := map[string][]string{}
	for _, t := range d.Tags.List {
		places[t.Number] = t.DefIDs
	}
	return set, places
}

// lineTags are a paragraph's tags: each run of text with a TagNumber
// (runs next to each other with the same one together), its text and the
// time it stands in the paragraph's text before it.
func lineTags(texts []FdxText, places map[string][]string, allCaps bool) []lex.LineTag {
	var out []lex.LineTag
	var plain strings.Builder
	var cur *lex.LineTag
	curNumber := ""
	flush := func() {
		if cur != nil && strings.TrimSpace(cur.Text) != "" {
			before := plain.String()[:len(plain.String())-len(cur.Text)]
			cur.At = strings.Count(before, cur.Text)
			cur.Text = strings.TrimSpace(cur.Text)
			out = append(out, *cur)
		}
		cur, curNumber = nil, ""
	}
	for _, x := range texts {
		text := x.Content
		if allCaps || strings.Contains(x.Style, "AllCaps") {
			text = strings.ToUpper(text)
		}
		if x.TagNumber != curNumber {
			flush()
		}
		plain.WriteString(text)
		if x.TagNumber == "" || len(places[x.TagNumber]) == 0 {
			continue
		}
		if cur == nil {
			cur, curNumber = &lex.LineTag{Elements: places[x.TagNumber]}, x.TagNumber
		}
		cur.Text += text
	}
	flush()
	return out
}

// ParseTags reads an FDX's script with its breakdown tags: the set (the
// categories and elements of its <TagData>; nil if it has none) and the
// tags in the rows' text, by row (the text as TagPlain counts it).
func ParseTags(file io.Reader) (lex.Screenplay, *lex.TagSet, map[int][]lex.LineTag, error) {
	s, _, doc, rowOf, err := parseFile(file)
	if err != nil {
		return nil, nil, nil, err
	}
	set, places := readTags(doc.TagData)
	if set == nil {
		return s, nil, nil, nil
	}
	byRow := map[int][]lex.LineTag{}
	types := typeSettings(doc.ElementSettings)
	for p, first := range rowOf {
		tags := lineTags(p.Texts, places, types[p.Type].allCaps)
		if len(tags) == 0 || first < 0 {
			continue
		}
		// the paragraph's row: its own, or (a dual dialogue's, which all
		// have the dialogue's first) the first after it with the words.
		// The text the tag counts in is the row's (notes, number and
		// markers aside), which may differ from the runs' (a note
		// first): counted again there.
		for _, tag := range tags {
			for row := first; row < len(s) && row < first+64; row++ {
				plain := lex.TagPlain(s[row].Contents, s[row].Type == lex.TypeScene)
				if lex.TagIndex(plain, tag.Text, 0) < 0 {
					continue
				}
				if lex.TagIndex(plain, tag.Text, tag.At) < 0 {
					tag.At = 0
				}
				byRow[row] = append(byRow[row], tag)
				break
			}
		}
	}
	return s, set, byRow, nil
}
