package report

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/phpdave11/gofpdf"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/lex"
)

// Breakdown sheets and element reports: a scene's sheet is what the
// report reads from the script (number, INT/EXT, location, time, page,
// length in eighths, synopsis, who speaks) and the elements tagged in it
// (Breakdown), by category; the element report lists each element with
// its scenes. The categories are lex.TagCategories and one's own.
//
// The sheet's layout is the classic one (StudioBinder's and Movie Magic's
// sheets, the "breakdown sheet" of the AD handbooks): a head of scene,
// INT/EXT, set, D/N and pages, the synopsis, then a box per category with
// its elements, the cast by their cast numbers.

// Kinds of the breakdown's tables.
const (
	KindSheets   = "breakdown"
	KindElements = "elements"
)

// Breakdown is what is tagged in the script's scenes.
type Breakdown struct {
	// Categories are one's own categories, listed after the standard
	// ones (lex.TagCategories).
	Categories []lex.TagCategory
	// Elements are the elements tagged in each scene, by its Scene.Order,
	// in the order they are tagged in it.
	Elements map[int][]Element
}

// Element is a thing tagged: its category's Name, and its own name.
type Element struct {
	Category string
	Name     string
}

// Sheet is a scene's breakdown sheet.
type Sheet struct {
	Scene
	// Cast are the scene's cast, as "1 ANNA": who speaks and who is
	// tagged as a cast member, by cast number.
	Cast []string
	// Groups are the other categories with elements in the scene, in
	// order.
	Groups []Group
}

// Group is a category's elements on a sheet.
type Group struct {
	Category lex.TagCategory
	Names    []string
}

// categories are the categories in their order: the standard ones, then
// one's own.
func (b Breakdown) categories() []lex.TagCategory {
	out := slices.Clone(lex.TagCategories)
	for _, c := range b.Categories {
		if _, ok := lex.TagCategoryOf(c.Name); !ok {
			out = append(out, c)
		}
	}
	return out
}

// category is the category of a name: a standard one's (by name or
// alias), one's own, or a new one of that name.
func (b Breakdown) category(name string) lex.TagCategory {
	if c, ok := lex.TagCategoryOf(name); ok {
		return c
	}
	for _, c := range b.Categories {
		if strings.EqualFold(c.Name, name) {
			return c
		}
	}
	return lex.TagCategory{Name: name}
}

// isCast tells whether a category is the cast's.
func isCast(c lex.TagCategory) bool { return c.Name == lex.CastCategory }

// sheetSkipped are the categories a sheet has columns of its own for: the
// cast, the location (the heading's), the synopsis.
func sheetSkipped(c lex.TagCategory) bool {
	return isCast(c) || c.Name == "Location" || c.Name == "Synopsis"
}

// CastNumbers are the cast's numbers: those who speak as Cast numbers
// them, then those only tagged as cast members, in the order they are
// first tagged, after them. Names in capitals, as cues are.
func (r Report) CastNumbers(b Breakdown) map[string]int {
	out := map[string]int{}
	n := 0
	for _, c := range r.Cast() {
		out[c.Name] = c.Number
		n = max(n, c.Number)
	}
	for _, sc := range r.Scenes {
		for _, e := range b.Elements[sc.Order] {
			if !isCast(b.category(e.Category)) {
				continue
			}
			name := castName(e.Name)
			if _, ok := out[name]; !ok && name != "" {
				n++
				out[name] = n
			}
		}
	}
	return out
}

// castName is a cast member's name as the reports have it: a cue's.
func castName(s string) string { return strings.ToUpper(CueName(strings.TrimSpace(s))) }

// Sheets are the scenes' breakdown sheets, OMITTED ones left out.
func (r Report) Sheets(b Breakdown) []Sheet {
	numbers := r.CastNumbers(b)
	cats := b.categories()
	var out []Sheet
	for _, sc := range r.Scenes {
		if sc.Omitted {
			continue
		}
		sh := Sheet{Scene: sc}
		sh.Cast, _ = b.sceneCast(sc, numbers)
		byCat := map[string][]string{}
		for _, e := range b.Elements[sc.Order] {
			c := b.category(e.Category)
			if isCast(c) {
				continue
			}
			key := strings.ToLower(c.Name)
			if !slices.ContainsFunc(byCat[key], func(n string) bool { return strings.EqualFold(n, e.Name) }) {
				byCat[key] = append(byCat[key], e.Name)
			}
		}
		for _, c := range cats {
			if names := byCat[strings.ToLower(c.Name)]; len(names) > 0 && !isCast(c) {
				sh.Groups = append(sh.Groups, Group{Category: c, Names: names})
			}
		}
		for _, e := range b.Elements[sc.Order] { // one's own, not listed in Categories
			c := b.category(e.Category)
			if c.ID == "" && !slices.ContainsFunc(sh.Groups, func(g Group) bool { return strings.EqualFold(g.Category.Name, c.Name) }) {
				sh.Groups = append(sh.Groups, Group{Category: c, Names: byCat[strings.ToLower(c.Name)]})
			}
		}
		out = append(out, sh)
	}
	return out
}

// Key is a category's CSV column key: its name in lower case, words
// joined with "_" ("Makeup/Hair": makeup_hair).
func Key(name string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if gap && b.Len() > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			gap = false
		} else {
			gap = true
		}
	}
	return b.String()
}

// SheetColumns are the breakdown CSV's columns: the scene's, the
// synopsis, the cast, then one per category (the standard ones but the
// cast, location and synopsis, in order, then one's own).
func (b Breakdown) SheetColumns() []Column {
	cols := []Column{
		{Numeric: true, Key: "order", Title: "#", Data: true},
		{Key: "number", Title: "Scene"},
		{Key: "int_ext", Title: "I/E"},
		{Key: "location", Title: "Location"},
		{Key: "time", Title: "Time"},
		{Numeric: true, Key: "page", Title: "Page"},
		{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
		{Numeric: true, Key: "length", Title: "Length"},
		{Key: "synopsis", Title: "Synopsis"},
		{Key: "cast", Title: "Cast"},
	}
	for _, c := range b.categories() {
		if !sheetSkipped(c) {
			cols = append(cols, Column{Key: Key(c.Name), Title: c.Name})
		}
	}
	return cols
}

// SheetTable is the sheets as a table, a row per scene.
func (b Breakdown) SheetTable(sheets []Sheet) Table {
	t := Table{Kind: KindSheets, Title: "Breakdown", Columns: b.SheetColumns()}
	for _, sh := range sheets {
		row := []Cell{num(sh.Order), Cell{Text: sh.ID(), Value: float64(sh.Order)}, text(sh.Place), text(sh.Location),
			text(sh.Time), page(sh.Page), num(int(sh.Eighths)), length(sh.Eighths), text(sh.Synopsis), list(sh.Cast)}
		for _, col := range t.Columns[len(row):] {
			var names []string
			for _, g := range sh.Groups {
				if Key(g.Category.Name) == col.Key {
					names = g.Names
				}
			}
			row = append(row, list(names))
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// ElementColumns are the element report's columns.
var ElementColumns = []Column{
	{Key: "category", Title: "Category"},
	{Key: "element", Title: "Element"},
	{Numeric: true, Key: "cast_number", Title: "Cast #", Data: true},
	{Numeric: true, Key: "scene_count", Title: "Count"},
	{Key: "scenes", Title: "Scenes"},
	{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
	{Numeric: true, Key: "length", Title: "Length"},
	{Key: "first_scene", Title: "First"},
	{Numeric: true, Key: "first_page", Title: "Page"},
}

// ElementTable is the element report: each element (the cast with those
// who speak) with its scenes, by category in the sheets' order, then in
// order of first appearance.
func (r Report) ElementTable(b Breakdown) Table {
	type item struct {
		cat, name     string
		scenes        []string
		eighths       lex.Eighths
		first         string
		firstPage, at int
	}
	numbers := r.CastNumbers(b)
	items := map[string]*item{}
	var order []string
	add := func(cat lex.TagCategory, name string, sc Scene) {
		if isCast(cat) {
			name = castName(name)
		}
		k := strings.ToLower(cat.Name) + "\x00" + strings.ToLower(name)
		it := items[k]
		if it == nil {
			it = &item{cat: cat.Name, name: name, first: sc.ID(), firstPage: sc.Page, at: len(order)}
			items[k] = it
			order = append(order, k)
		}
		if !slices.Contains(it.scenes, sc.ID()) {
			it.scenes = append(it.scenes, sc.ID())
			it.eighths += sc.Eighths
		}
	}
	cast := b.category(lex.CastCategory)
	for _, sc := range r.Scenes {
		if sc.Omitted {
			continue
		}
		for _, c := range sc.Characters {
			add(cast, c, sc)
		}
		for _, e := range b.Elements[sc.Order] {
			add(b.category(e.Category), e.Name, sc)
		}
	}
	rank := map[string]int{}
	for i, c := range b.categories() {
		rank[c.Name] = i
	}
	slices.SortStableFunc(order, func(x, y string) int {
		a, b := items[x], items[y]
		ra, oka := rank[a.cat]
		rb, okb := rank[b.cat]
		if !oka {
			ra = len(rank)
		}
		if !okb {
			rb = len(rank)
		}
		if ra != rb {
			return ra - rb
		}
		if a.cat == lex.CastCategory {
			return numbers[a.name] - numbers[b.name]
		}
		return a.at - b.at
	})
	sceneOrder := map[string]int{}
	for _, s := range r.Scenes {
		sceneOrder[s.ID()] = s.Order
	}
	t := Table{Kind: KindElements, Title: "Elements", Columns: ElementColumns}
	for _, k := range order {
		it := items[k]
		castNo := Cell{}
		if it.cat == lex.CastCategory {
			castNo = num(numbers[it.name])
		}
		t.Rows = append(t.Rows, []Cell{text(it.cat), text(it.name), castNo, num(len(it.scenes)), list(it.scenes),
			num(int(it.eighths)), length(it.eighths), Cell{Text: it.first, Value: float64(sceneOrder[it.first])},
			page(it.firstPage)})
	}
	return t
}

// sheetBoxes are the categories a sheet always has a box for (the
// classic sheet's), in its order; the others get one when they have
// elements.
var sheetBoxes = []string{"Background Actors", "Stunts", "Vehicles", "Props", "Special Effects", "Wardrobe",
	"Makeup/Hair", "Animals", "Music", "Sound", "Set Dressing", "Special Equipment", "Visual Effects",
	"Mechanical Effects", "Notes"}

// SheetPaper are the papers WriteSheetsPDF knows, as gofpdf names them.
var sheetPaper = map[string]string{"": "Letter", "letter": "Letter", "a4": "A4", "a5": "A5", "legal": "Legal"}

// WriteSheetsPDF writes the sheets as a PDF, a page each, on paper
// (letter, a4, a5; "" letter), under the script's title.
func WriteSheetsPDF(w io.Writer, title string, sheets []Sheet, paper string) error {
	size, ok := sheetPaper[strings.ToLower(paper)]
	if !ok {
		size = "Letter"
	}
	p := gofpdf.New("P", "pt", size, "")
	p.SetMargins(36, 36, 36)
	p.SetAutoPageBreak(false, 36)
	const face = "sheet"
	p.AddUTF8FontFromBytes(face, "", font.GetFont(font.CourierPrimeName, ""))
	p.AddUTF8FontFromBytes(face, "B", font.GetFont(font.CourierPrimeName, "B"))
	pw, ph := p.GetPageSize()
	left, top := 36.0, 36.0
	width := pw - 2*left
	for i, sh := range sheets {
		p.AddPage()
		y := top
		p.SetFont(face, "B", 15)
		p.SetXY(left, y)
		p.CellFormat(width*0.6, 18, "BREAKDOWN SHEET", "", 0, "L", false, 0, "")
		p.SetFont(face, "", 9)
		p.CellFormat(width*0.4, 18, fmt.Sprintf("Sheet %d of %d", i+1, len(sheets)), "", 0, "R", false, 0, "")
		y += 18
		if title != "" {
			p.SetXY(left, y)
			p.CellFormat(width, 12, title, "", 0, "L", false, 0, "")
			y += 14
		}
		y += 4
		// the head: scene, INT/EXT, set, D/N, pages, page
		heads := []struct {
			label, value string
			part         float64
		}{
			{"SCENE", sh.ID(), 0.12}, {"INT/EXT", sh.Place, 0.12}, {"SET / LOCATION", sh.Location, 0.40},
			{"D/N", sh.Time, 0.14}, {"PAGES", sh.Eighths.String(), 0.11}, {"PAGE", pageText(sh.Page), 0.11},
		}
		x := left
		p.SetLineWidth(0.8)
		for _, h := range heads {
			cw := width * h.part
			p.Rect(x, y, cw, 36, "D")
			p.SetFont(face, "", 7)
			p.SetXY(x+3, y+3)
			p.CellFormat(cw-6, 8, h.label, "", 0, "L", false, 0, "")
			p.SetFont(face, "B", 11)
			p.SetXY(x+3, y+14)
			p.CellFormat(cw-6, 18, fit(p, h.value, cw-6), "", 0, "L", false, 0, "")
			x += cw
		}
		y += 36
		// the synopsis
		p.SetFont(face, "", 9)
		lines := p.SplitText(sh.Synopsis, width-6)
		sh2 := max(30, 12+float64(len(lines))*11)
		p.Rect(left, y, width, sh2, "D")
		p.SetFont(face, "", 7)
		p.SetXY(left+3, y+3)
		p.CellFormat(width-6, 8, "SYNOPSIS", "", 0, "L", false, 0, "")
		p.SetFont(face, "", 9)
		for k, l := range lines {
			p.SetXY(left+3, y+13+float64(k)*11)
			p.CellFormat(width-6, 11, l, "", 0, "L", false, 0, "")
		}
		y += sh2 + 8
		// the boxes: the cast first, then the categories
		type box struct {
			name, color string
			items       []string
		}
		boxes := []box{{lex.CastCategory, colorOf(lex.CastCategory), sh.Cast}}
		groups := map[string]Group{}
		for _, g := range sh.Groups {
			groups[g.Category.Name] = g
		}
		for _, name := range sheetBoxes {
			boxes = append(boxes, box{name, colorOf(name), groups[name].Names})
		}
		for _, g := range sh.Groups {
			if !slices.Contains(sheetBoxes, g.Category.Name) {
				boxes = append(boxes, box{g.Category.Name, g.Category.Color, g.Names})
			}
		}
		const cols = 3
		cw := width / cols
		for k := 0; k < len(boxes); k += cols {
			row := boxes[k:min(k+cols, len(boxes))]
			rows := 3
			for _, b := range row {
				n := 0
				for _, it := range b.items {
					n += len(p.SplitText(it, cw-10))
				}
				rows = max(rows, n)
			}
			bh := 16 + float64(rows)*10 + 4
			if y+bh > ph-top {
				break // what does not fit: the CSV has it
			}
			for j, b := range row {
				bx := left + float64(j)*cw
				p.SetDrawColor(0, 0, 0)
				p.Rect(bx, y, cw, bh, "D")
				if r, g, bl, ok := rgb(b.color); ok {
					p.SetFillColor(r, g, bl)
					p.Rect(bx, y, 5, bh, "F")
				}
				p.SetFont(face, "B", 8)
				p.SetXY(bx+8, y+3)
				p.CellFormat(cw-10, 10, strings.ToUpper(b.name), "", 0, "L", false, 0, "")
				p.SetFont(face, "", 8.5)
				ly := y + 15
				for _, it := range b.items {
					for _, l := range p.SplitText(it, cw-10) {
						p.SetXY(bx+8, ly)
						p.CellFormat(cw-10, 10, l, "", 0, "L", false, 0, "")
						ly += 10
					}
				}
			}
			y += bh
		}
	}
	if len(sheets) == 0 {
		p.AddPage()
		p.SetFont(face, "", 11)
		p.SetXY(left, top)
		p.CellFormat(width, 14, "No scenes.", "", 0, "L", false, 0, "")
	}
	return p.Output(w)
}

// fit is s cut to fit width (with "…").
func fit(p *gofpdf.Fpdf, s string, width float64) string {
	if p.GetStringWidth(s) <= width {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 && p.GetStringWidth(string(rs)+"…") > width {
		rs = rs[:len(rs)-1]
	}
	return string(rs) + "…"
}

// colorOf is a standard category's colour.
func colorOf(name string) string {
	c, _ := lex.TagCategoryOf(name)
	return c.Color
}

// rgb reads #RRGGBB.
func rgb(c string) (r, g, b int, ok bool) {
	c = strings.TrimPrefix(c, "#")
	if len(c) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(c, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return int(v >> 16), int(v >> 8 & 0xff), int(v & 0xff), true
}
