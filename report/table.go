package report

import (
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// The reports, by the names Table takes.
const (
	KindScenes     = "scenes"
	KindLocations  = "locations"
	KindCharacters = "characters"
	KindCast       = "cast"
)

// Kinds are the reports, in the order they are offered.
var Kinds = []string{KindScenes, KindLocations, KindCharacters, KindCast}

// Column is a column of a report.
type Column struct {
	// Key is its name in the CSV's header row: lower case ASCII and
	// underscores, stable (see the Columns below).
	Key string
	// Title is its heading on screen and on paper.
	Title string
	// Data: a column for the CSV only (the eighths as a number, which
	// the length shows on screen).
	Data bool
	// Numeric: a column of numbers (or lengths), aligned right.
	Numeric bool
}

// Cell is a report's value.
type Cell struct {
	// Text is the value as written; List the values of a cell that holds
	// several (joined with ListSeparator in the CSV, with ", " on
	// screen).
	Text string
	List []string
	// Value orders the column: a number (a page, eighths, a scene's
	// place in the script for its number), or 0 and its Text.
	Value float64
	// Label is shown on screen and on paper instead of Text, when set
	// (OMITTED in an omitted scene's row).
	Label string
}

// ListSeparator joins a cell's values in the CSV.
const ListSeparator = "; "

// String is the cell as shown.
func (c Cell) String() string {
	if c.Label != "" {
		return c.Label
	}
	if c.List != nil {
		return strings.Join(c.List, ", ")
	}
	return c.Text
}

// csv is the cell as the CSV has it.
func (c Cell) csv() string {
	if c.List != nil {
		return strings.Join(c.List, ListSeparator)
	}
	return c.Text
}

// Table is a report as rows of cells.
type Table struct {
	Kind    string
	Title   string
	Columns []Column
	Rows    [][]Cell
}

// The CSV layouts: these columns, in this order, under a header row of
// their keys. Lists in a cell are joined with "; ". Lengths are given
// twice: eighths (a whole number: 11) and length (as written: 1 3/8).
var (
	SceneColumns = []Column{
		{Numeric: true, Key: "order", Title: "#", Data: true},
		{Key: "number", Title: "Scene"},
		{Key: "heading", Title: "Heading", Data: true},
		{Key: "int_ext", Title: "I/E"},
		{Key: "location", Title: "Location"},
		{Key: "time", Title: "Time"},
		{Numeric: true, Key: "page", Title: "Page"},
		{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
		{Numeric: true, Key: "length", Title: "Length"},
		{Key: "characters", Title: "Characters"},
		{Key: "omitted", Title: "Omitted", Data: true},
	}
	LocationColumns = []Column{
		{Key: "location", Title: "Location"},
		{Key: "int_ext", Title: "I/E"},
		{Key: "times", Title: "Times"},
		{Numeric: true, Key: "scene_count", Title: "Count"},
		{Key: "scenes", Title: "Scenes"},
		{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
		{Numeric: true, Key: "length", Title: "Length"},
	}
	CharacterColumns = []Column{
		{Key: "name", Title: "Character"},
		{Numeric: true, Key: "cast_number", Title: "Cast #"},
		{Numeric: true, Key: "scene_count", Title: "Count"},
		{Key: "scenes", Title: "Scenes"},
		{Key: "first_scene", Title: "First"},
		{Numeric: true, Key: "first_page", Title: "Page", Data: true},
		{Key: "last_scene", Title: "Last"},
		{Numeric: true, Key: "last_page", Title: "Last page", Data: true},
		{Numeric: true, Key: "speeches", Title: "Speeches"},
		{Numeric: true, Key: "words", Title: "Words"},
		{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
		{Numeric: true, Key: "length", Title: "Length"},
	}
	CastColumns = []Column{
		{Numeric: true, Key: "cast_number", Title: "#"},
		{Key: "name", Title: "Character"},
		{Numeric: true, Key: "scene_count", Title: "Scenes"},
		{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
		{Numeric: true, Key: "length", Title: "Length"},
		{Numeric: true, Key: "speeches", Title: "Speeches"},
		{Numeric: true, Key: "words", Title: "Words"},
		{Key: "first_scene", Title: "First"},
		{Numeric: true, Key: "first_page", Title: "Page"},
	}
)

func text(s string) Cell        { return Cell{Text: s} }
func num(n int) Cell            { return Cell{Text: itoa(n), Value: float64(n)} }
func list(l []string) Cell      { return Cell{List: append([]string{}, l...), Value: float64(len(l))} }
func page(n int) Cell           { c := num(n); c.Text = pageText(n); return c }
func length(e lex.Eighths) Cell { return Cell{Text: e.String(), Value: float64(e)} }

func itoa(n int) string { return strconv.Itoa(n) }

func pageText(n int) string {
	if n <= 0 {
		return ""
	}
	return itoa(n)
}

// Table is a report as a table: kind is one of Kinds.
func (r Report) Table(kind string) (Table, error) {
	order := map[string]int{} // a scene ID's place, to order by number
	for _, s := range r.Scenes {
		order[s.ID()] = s.Order
	}
	sceneID := func(id string) Cell { return Cell{Text: id, Value: float64(order[id])} }
	t := Table{Kind: kind}
	switch kind {
	case KindScenes:
		t.Title, t.Columns = "Scenes", SceneColumns
		for _, s := range r.Scenes {
			omitted, l := "", length(s.Eighths)
			if s.Omitted {
				omitted, l = "yes", Cell{}
			}
			t.Rows = append(t.Rows, []Cell{num(s.Order), sceneID(s.ID()), text(s.Heading), text(s.Place),
				text(s.Location), text(s.Time), page(s.Page), num(int(s.Eighths)), l, list(s.Characters), text(omitted)})
			if s.Omitted { // nothing but its number and its OMITTED
				t.Rows[len(t.Rows)-1][4] = Cell{Label: lex.Omitted}
			}
		}
	case KindLocations:
		t.Title, t.Columns = "Locations", LocationColumns
		for _, l := range r.Locations {
			t.Rows = append(t.Rows, []Cell{text(l.Name), list(l.Places), list(l.Times), num(len(l.Scenes)),
				list(l.Scenes), num(int(l.Eighths)), length(l.Eighths)})
		}
	case KindCharacters:
		t.Title, t.Columns = "Characters", CharacterColumns
		cast := map[string]int{}
		for _, c := range r.Cast() {
			cast[c.Name] = c.Number
		}
		for _, c := range r.Characters {
			t.Rows = append(t.Rows, []Cell{text(c.Name), num(cast[c.Name]), num(len(c.Scenes)), list(c.Scenes),
				sceneID(c.First), page(c.FirstPage), sceneID(c.Last), page(c.LastPage), num(c.Speeches),
				num(c.Words), num(int(c.Eighths)), length(c.Eighths)})
		}
	case KindCast:
		t.Title, t.Columns = "Cast", CastColumns
		for _, c := range r.Cast() {
			t.Rows = append(t.Rows, []Cell{num(c.Number), text(c.Name), num(len(c.Scenes)), num(int(c.Eighths)),
				length(c.Eighths), num(c.Speeches), num(c.Words), sceneID(c.First), page(c.FirstPage)})
		}
	default:
		return Table{}, fmt.Errorf("no report %q: choose from %s", kind, strings.Join(Kinds, ", "))
	}
	return t, nil
}

// WriteCSV writes the table as CSV (RFC 4180: UTF-8, commas, quotes where
// needed, CRLF line ends): a header row of the columns' keys, then a row
// for each row, all columns.
func (t Table) WriteCSV(w io.Writer) error {
	c := csv.NewWriter(w)
	c.UseCRLF = true
	header := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		header[i] = col.Key
	}
	if err := c.Write(header); err != nil {
		return err
	}
	for _, row := range t.Rows {
		rec := make([]string, len(row))
		for i, cell := range row {
			rec[i] = cell.csv()
		}
		if err := c.Write(rec); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

// Shown are the columns shown on screen and on paper (not the CSV's data
// columns), by their place in the row.
func (t Table) Shown() []int {
	var out []int
	for i, c := range t.Columns {
		if !c.Data {
			out = append(out, i)
		}
	}
	return out
}

// WriteHTML writes tables as a web page to print (a PDF from the print
// dialog), under a title and a line below it (a summary; may be "").
func WriteHTML(w io.Writer, title, summary string, tables ...Table) error {
	esc := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html><html><head><meta charset="utf-8"><title>%s</title><style>`+
		`body{font:10.5pt/1.35 "Courier Prime","Courier New",Courier,monospace;margin:1.5em;color:#000;background:#fff}`+
		`h1{font-size:14pt;margin:0 0 .2em}h2{font-size:12pt;margin:1.4em 0 .4em}p{margin:0 0 .8em}`+
		`table{border-collapse:collapse;width:100%%}th,td{text-align:left;vertical-align:top;padding:.2em .5em;border-bottom:1px solid #bbb}`+
		`th{border-bottom:2px solid #000}td.n,th.n{text-align:right;white-space:nowrap}thead{display:table-header-group}`+
		`tr{page-break-inside:avoid}@page{margin:.6in}</style></head><body>`+"\n<h1>%s</h1>\n", esc(title), esc(title))
	if summary != "" {
		fmt.Fprintf(&b, "<p>%s</p>\n", esc(summary))
	}
	for _, t := range tables {
		shown := t.Shown()
		class := func(i int) string {
			if t.Columns[i].Numeric {
				return ` class="n"`
			}
			return ""
		}
		if len(tables) > 1 {
			fmt.Fprintf(&b, "<h2>%s</h2>\n", esc(t.Title))
		}
		b.WriteString("<table><thead><tr>")
		for _, i := range shown {
			fmt.Fprintf(&b, "<th%s>%s</th>", class(i), esc(t.Columns[i].Title))
		}
		b.WriteString("</tr></thead><tbody>\n")
		for _, row := range t.Rows {
			b.WriteString("<tr>")
			for _, i := range shown {
				fmt.Fprintf(&b, "<td%s>%s</td>", class(i), esc(row[i].String()))
			}
			b.WriteString("</tr>\n")
		}
		b.WriteString("</tbody></table>\n")
	}
	b.WriteString("</body></html>\n")
	_, err := io.WriteString(w, b.String())
	return err
}
