// Package titlepage lays a script's title page out by a template
// (rules.TitlePage): which field goes where, in what type, by its key,
// never by the order the fields are written in. The parse keeps the
// text's order (a program maps the rows back to its text); the writers
// (PDF, HTML, Word, ODT) ask Layout where each field goes and draw it so.
package titlepage

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Template is a title page template with the language it places the keys
// it does not name by: what the writers are given.
type Template struct {
	Page rules.TitlePage
	// Centred are the keys printed centred when the template does not
	// name them (rules.Language.Centred); nil is Fountain's.
	Centred []string
}

// Standard is Fountain's title page: no template, Fountain's keys
// centred.
func Standard() Template { return Template{} }

// For is the template named name in c, for a script in lang: "" is the
// standard one. ok is false when c has no template of that name.
func For(c rules.TOMLConf, name, lang string) (Template, bool) {
	p, ok := c.TitlePage(name)
	return Template{Page: p, Centred: c.Language(lang).Centred()}, ok
}

// IsStandard tells whether the template places nothing of its own: the
// title page prints as Fountain's.
func (t Template) IsStandard() bool { return len(t.Page.Keys) == 0 && t.Page.Centre == 0 }

func (t Template) centred() []string {
	if len(t.Centred) == 0 {
		return rules.FountainCentred
	}
	return t.Centred
}

// Field is one field of a title page as placed: its key, its lines (the
// value's first line and those under it, as the parse has them, with
// their emphasis), or the picture it is.
type Field struct {
	Key   string
	Lines []string
	// Rows are the field's rows in the screenplay given to Layout.
	Rows []int
	// Picture is the file its value names when it is a picture (alone
	// on its line as Markdown's ![alt](file), or under a key the
	// template marks Picture): relative to the script, slashes.
	Picture, Alt string
	// Entry is how it prints: the template's entry for it with its
	// Place filled in ("" never), else the language's place.
	Entry rules.TitleKey
	// Written is its place among the fields as written.
	Written int
	rank    int
}

// Text is the field's lines joined with line breaks.
func (f Field) Text() string { return strings.Join(f.Lines, "\n") }

// Block is a place on the page and the fields in it, in their order. A
// free place has a block of its own for each field.
type Block struct {
	Place  string
	Fields []Field
}

// placeOrder is the order Layout gives the places in.
var placeOrder = map[string]int{rules.PlaceTop: 0, rules.PlaceCentre: 1, rules.PlaceFootLeft: 2, rules.PlaceFootRight: 3, rules.PlaceFree: 4}

// markdownPicture is a value that is a picture: ![alt](file).
var markdownPicture = regexp.MustCompile(`^!\[([^\]]*)\]\(\s*<?([^)>]+?)>?\s*\)$`)

// Picture is the file a field's value names as a picture, and its alt
// text: a Markdown picture, or (marked) the value as it is.
func Picture(value string, marked bool) (file, alt string, ok bool) {
	v := strings.TrimSpace(lex.StripNotes(value))
	if m := markdownPicture.FindStringSubmatch(v); m != nil {
		return strings.TrimSpace(m[2]), m[1], true
	}
	if marked && v != "" {
		return v, "", true
	}
	return "", "", false
}

// Rows are the title page's rows in s: from its titlepage row to the
// page break after it (or the script's first element, a screenplay
// made without the break), [from, to); none (0, 0) without one.
func Rows(s lex.Screenplay) (from, to int) {
	if len(s) == 0 || s[0].Type != lex.TypeTitlePage {
		return 0, 0
	}
	to = 1
	for to < len(s) && s[to].Type != lex.TypeNewPage && !lex.IsBodyType(s[to].Type) {
		to++
	}
	return 0, to
}

// IsBody tells whether a row is of the script's body, not a title page
// field.
func IsBody(t lex.ElementType) bool { return lex.IsBodyType(t) || t == lex.TypeNewPage }

// Fields are the title page's fields in s, as written: consecutive rows
// of one key are one field (a key's lines under it).
func Fields(s lex.Screenplay) []Field {
	from, to := Rows(s)
	var out []Field
	for i := from + 1; i < to; i++ {
		r := s[i]
		switch r.Type {
		case lex.TypeEmpty, "metasection", lex.TypeNote, lex.TypeBoneyard:
			continue
		}
		if n := len(out); n > 0 && out[n-1].Key == r.Type && out[n-1].Rows[len(out[n-1].Rows)-1] == i-1 {
			out[n-1].Lines = append(out[n-1].Lines, r.Contents)
			out[n-1].Rows = append(out[n-1].Rows, i)
			continue
		}
		out = append(out, Field{Key: r.Type, Lines: []string{r.Contents}, Rows: []int{i}, Written: len(out)})
	}
	return out
}

// Place is the fields placed by the template: each with its entry and
// place, hidden ones left out, in blocks in the order top, centre, foot
// left, foot right, free. In a place the fields the template names come
// in its order, then the language's centred keys in theirs, then the
// rest as written. A picture the template does not name (only its "*")
// goes first in the centre, above the title.
func (t Template) Place(fields []Field) []Block {
	var placed []Field
	centred := t.centred()
	n := len(t.Page.Keys)
	for _, f := range fields {
		e, rank, ok := t.Page.Entry(f.Key)
		place := strings.ToLower(strings.TrimSpace(e.Place))
		if place == "center" {
			place = rules.PlaceCentre
		}
		if _, known := placeOrder[place]; !known {
			place = ""
		}
		at := slices.IndexFunc(centred, func(k string) bool { return strings.EqualFold(k, f.Key) })
		if place == "" {
			place = rules.PlaceFootLeft
			if at >= 0 {
				place = rules.PlaceCentre
			}
		}
		switch {
		case ok:
		case at >= 0:
			rank = n + at
		default:
			rank = n + len(centred)
		}
		if e.Hide {
			continue
		}
		named := ok && strings.TrimSpace(e.Key) != rules.OtherKeys
		if len(f.Lines) == 1 {
			if file, alt, pic := Picture(f.Lines[0], e.Picture); pic {
				f.Picture, f.Alt = file, alt
				if !named { // a picture the template does not place: above the title
					place, rank = rules.PlaceCentre, -1
					e = rules.TitleKey{}
				}
			}
		}
		e.Key, e.Place = f.Key, place
		f.Entry, f.rank = e, rank
		placed = append(placed, f)
	}
	slices.SortStableFunc(placed, func(a, b Field) int {
		return cmp.Or(placeOrder[a.Entry.Place]-placeOrder[b.Entry.Place], a.rank-b.rank, a.Written-b.Written)
	})
	var out []Block
	for _, f := range placed {
		if n := len(out); n > 0 && out[n-1].Place == f.Entry.Place && f.Entry.Place != rules.PlaceFree {
			out[n-1].Fields = append(out[n-1].Fields, f)
			continue
		}
		out = append(out, Block{Place: f.Entry.Place, Fields: []Field{f}})
	}
	return out
}

// Layout is the title page of s placed by the template (Fields, Place);
// none without a title page.
func (t Template) Layout(s lex.Screenplay) []Block { return t.Place(Fields(s)) }

// Format is the element format a field prints in, from the set's: its
// title format in the centre and at the top, its meta format elsewhere,
// with the template's type, style and alignment over it.
func Format(set rules.Set, e rules.TitleKey) rules.Format {
	key := rules.KeyMeta
	if e.Place == rules.PlaceCentre || e.Place == rules.PlaceTop {
		key = rules.KeyTitle
	}
	f := set.GetWithKey(key)
	if f.Hide { // a set without the key: the title page prints still
		f.Hide = false
		if key == rules.KeyTitle {
			f.Align = "C"
		}
	}
	switch e.Place {
	case rules.PlaceTop:
		f.Align = "C"
	case rules.PlaceFootRight:
		f.Align = "R"
	case rules.PlaceFree:
		f.Align = "L"
	}
	if e.Font != "" {
		f.Font = e.Font
	}
	if e.Size > 0 {
		f.Size = e.Size
	}
	if e.Style != "" {
		f.Style = strings.ToUpper(e.Style)
	}
	if a := strings.ToUpper(strings.TrimSpace(e.Align)); a == "L" || a == "C" || a == "R" {
		f.Align = a
	}
	return f
}

// Caps is a line as the entry prints it: in capitals if it says so.
func Caps(e rules.TitleKey, line string) string {
	if e.Caps {
		return strings.ToUpper(line)
	}
	return line
}

// PictureHeight is the height the entry prints a picture in, in inches
// on Letter.
func PictureHeight(e rules.TitleKey) float64 {
	if e.Height > 0 {
		return e.Height
	}
	return rules.PictureHeight
}

// CentreAt is where the centred block starts, in inches from the top of
// a Letter page.
func (t Template) CentreAt() float64 {
	if t.Page.Centre > 0 {
		return t.Page.Centre
	}
	return 4
}

// Stream is s with its title page laid out by the template for a writer
// that writes lines one after another (LaTeX, Markdown): the title page's
// rows in the order Layout places them, the top's and the centre's
// first, then a lex.TypeMetaSection row and the foot's and the free
// places' (hidden fields left out). The rest of s is as it was.
func (t Template) Stream(s lex.Screenplay) lex.Screenplay {
	from, to := Rows(s)
	if to == 0 {
		return s
	}
	out := append(lex.Screenplay{}, s[from])
	foot := false
	for _, b := range t.Layout(s) {
		if b.Place != rules.PlaceTop && b.Place != rules.PlaceCentre && !foot {
			out = append(out, lex.Line{Type: lex.TypeMetaSection})
			foot = true
		}
		for _, f := range b.Fields {
			for _, r := range f.Rows {
				out = append(out, s[r])
			}
		}
	}
	return append(out, s[to:]...)
}
