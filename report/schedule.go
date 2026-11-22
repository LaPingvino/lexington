package report

import (
	"fmt"
	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The shooting schedule's reports: the strips (a scene each, coloured by
// INT/EXT and day or night), the one-liner (the strips day by day, with
// each day's total in eighths), the Day Out of Days (each cast member's
// days, coded SW, W, WF, SWF and H) and the shoot days as iCalendar
// (RFC 5545) events. The schedule itself (which scenes on which day, in
// which order) is the program's: here it is the days, each the
// Scene.Order of its scenes in shooting order.
//
// The strip colours are the convention Wikipedia's "Production board"
// tabulates as the generic one and StudioBinder's (and Scenechronize's):
// white for INT. DAY, yellow for EXT. DAY, blue for INT. NIGHT, green for
// EXT. NIGHT (https://en.wikipedia.org/wiki/Production_board;
// https://www.studiobinder.com/blog/shooting-schedule-stripboard/). Movie
// Magic Scheduling swaps the night ones (green inside, blue outside), as
// the same table has it; the generic convention is the one here.
//
// The Day Out of Days codes are Movie Magic Scheduling's
// (https://mms-docs.ep.com/DayOutofDays/DOODRules.html): SW the first day
// worked ("Start … is almost always shown as SW"), W a day worked, WF the
// last ("Finish … the last day the Element is working"), SWF the only one,
// and H a day between the first and the last not worked ("all non-working
// days for that Element between their first and last working day are
// marked as H", with Allow Hold Days on, as here). Drops, pick-ups,
// travel, fittings, rehearsals and holidays are not made.

// Kinds of the schedule's tables.
const (
	KindOneLiner = "oneliner"
	KindDOOD     = "dood"
)

// The DOOD's codes.
const (
	CodeStartWork       = "SW"
	CodeWork            = "W"
	CodeWorkFinish      = "WF"
	CodeStartWorkFinish = "SWF"
	CodeHold            = "H"
)

// Day is a shoot day.
type Day struct {
	// ID names the day for good (an .ics event's UID): the program's.
	ID string
	// Date is the day's date (the zero time for none yet); Call its call
	// time, "07:00" (24 hours; "" for none).
	Date time.Time
	Call string
	// Notes are the day's location notes: where the unit goes.
	Notes string
	// Scenes are its scenes' Scene.Order, in shooting order.
	Scenes []int
}

// Schedule is the shoot days, in order.
type Schedule struct {
	Days []Day
}

// StripOptions are how strips read a heading's place and time.
type StripOptions struct {
	// Lang is the script's language (rules' Matchups: INT., BIN., EN.;
	// "" English).
	Lang string
}

// Strip is a scene as its strip on the board shows it.
type Strip struct {
	Scene
	// Exterior: the scene is outside (EXT., INT./EXT., EST.); Night: by
	// night (NIGHT, EVENING, DUSK …). DayNightFrom tells where Night comes
	// from: "" its own time, "before" the scene before's (CONTINUOUS,
	// LATER, no time).
	Exterior, Night bool
	DayNightFrom    string
	// Cast are the scene's cast as "1 ANNA", by number: who speaks and who
	// is tagged as a cast member (as the breakdown sheets').
	Cast []string
	// CastNumbers are their numbers, in order.
	CastNumbers []int
}

// IE is the strip's INT or EXT.
func (s Strip) IE() string {
	if s.Exterior {
		return "EXT"
	}
	return "INT"
}

// DN is the strip's D or N.
func (s Strip) DN() string {
	if s.Night {
		return "N"
	}
	return "D"
}

// Colour is the strip's colour: its name (white, yellow, blue, green) and
// a tint of it to print, #RRGGBB.
func (s Strip) Colour() (name, hex string) { return StripColour(s.Exterior, s.Night) }

// StripColour is a strip's colour by INT/EXT and day or night, the
// generic convention (white, yellow, blue, green).
func StripColour(exterior, night bool) (name, hex string) {
	switch {
	case !exterior && !night:
		return "white", "#FFFFFF"
	case exterior && !night:
		return "yellow", "#FFF176"
	case !exterior && night:
		return "blue", "#90CAF9"
	}
	return "green", "#A5D6A7"
}

// nightWords and dayWords are the times a heading names, in the
// languages Lexington knows (upper case). Dusk and evening count as
// night, dawn and morning as day: the four colours have no place for
// the hours between (Movie Magic's pink and orange are its own).
var (
	nightWords = []string{"NIGHT", "EVENING", "DUSK", "SUNSET", "MIDNIGHT", "TWILIGHT",
		"NACHT", "AVOND", "SCHEMERING", "ABEND", "NUIT", "SOIR", "SOIRÉE", "CRÉPUSCULE", "NOTTE", "SERA",
		"NOCHE", "TARDE-NOCHE", "NOKTO", "VESPERO", "KREPUSKO", "НОЧЬ", "ВЕЧЕР", "ليل", "ليلا", "مساء", "לילה", "ערב",
		"夜", "晚", "PIMEJA"}
	dayWords = []string{"DAY", "MORNING", "AFTERNOON", "DAWN", "SUNRISE", "NOON", "MIDDAY",
		"DAG", "OCHTEND", "MIDDAG", "TAG", "MORGEN", "MITTAG", "JOUR", "MATIN", "MIDI", "APRÈS-MIDI", "GIORNO",
		"MATTINA", "POMERIGGIO", "DÍA", "MAÑANA", "TAGO", "MATENO", "TAGMEZO", "ДЕНЬ", "УТРО", "نهار", "صباح", "יום",
		"בוקר", "日", "白天", "早", "SUNO"}
)

// dayOrNight reads a heading's time: night, day, or neither (known false).
func dayOrNight(t string) (night, known bool) {
	t = strings.ToUpper(t)
	words := strings.FieldsFunc(t, func(r rune) bool { return !unicode.IsLetter(r) && r != '-' })
	has := func(list []string) bool {
		for _, w := range list {
			if slices.Contains(words, w) {
				return true
			}
			if r := []rune(w)[0]; r >= 0x2E80 && strings.Contains(t, w) { // Han: no spaces between words
				return true
			}
		}
		return false
	}
	switch {
	case has(nightWords):
		return true, true
	case has(dayWords):
		return false, true
	}
	return false, false
}

// exteriorOf tells whether a heading's place word (INT, EXT, INT./EXT)
// is outside: EXT., the two together (either way: weather and light go
// with the outside), and an establishing shot. A forced heading, or a
// word not known, is inside.
func exteriorOf(lang, place string) bool {
	if lang == "" {
		lang = "en"
	}
	conf := rules.DefaultConf()
	role, ok := conf.RoleOf(lang, place)
	if !ok && lang != "en" {
		role, ok = conf.RoleOf("en", place)
	}
	return ok && role != rules.Inside
}

// sceneCast are a scene's cast by number: who speaks, then who is tagged
// as a cast member.
func (b Breakdown) sceneCast(sc Scene, numbers map[string]int) ([]string, []int) {
	cast := slices.Clone(sc.Characters)
	for _, e := range b.Elements[sc.Order] {
		if !isCast(b.category(e.Category)) {
			continue
		}
		if name := castName(e.Name); name != "" && !slices.Contains(cast, name) {
			cast = append(cast, name)
		}
	}
	slices.SortStableFunc(cast, func(a, b string) int { return numbers[a] - numbers[b] })
	var out []string
	var nums []int
	for _, name := range cast {
		out = append(out, strconv.Itoa(numbers[name])+" "+name)
		nums = append(nums, numbers[name])
	}
	return out, nums
}

// Strips are the scenes' strips, one each in script order (an OMITTED
// one too: it has no length and no cast). A scene whose time names no
// day or night (CONTINUOUS, LATER, none) takes the scene before's; the
// first such is day.
func (r Report) Strips(b Breakdown, o StripOptions) []Strip {
	numbers := r.CastNumbers(b)
	out := make([]Strip, len(r.Scenes))
	night := false
	for i, sc := range r.Scenes {
		s := Strip{Scene: sc}
		if sc.Omitted {
			out[i] = s
			continue
		}
		s.Exterior = exteriorOf(o.Lang, sc.Place)
		if n, known := dayOrNight(sc.Time); known {
			night = n
		} else {
			s.DayNightFrom = "before"
		}
		s.Night = night
		s.Cast, s.CastNumbers = b.sceneCast(sc, numbers)
		out[i] = s
	}
	return out
}

// byOrder are strips by their Scene.Order.
func byOrder(strips []Strip) map[int]Strip {
	m := make(map[int]Strip, len(strips))
	for _, s := range strips {
		m[s.Order] = s
	}
	return m
}

// DayLength is a day's scenes' length in all.
func (r Report) DayLength(d Day) lex.Eighths {
	var n lex.Eighths
	for _, sc := range r.Scenes {
		if slices.Contains(d.Scenes, sc.Order) {
			n += sc.Eighths
		}
	}
	return n
}

// dateText is a day's date as the reports write it (ISO; "" none).
func dateText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// OneLinerColumns are the one-liner CSV's columns.
var OneLinerColumns = []Column{
	{Numeric: true, Key: "day", Title: "Day"},
	{Key: "date", Title: "Date"},
	{Numeric: true, Key: "shoot_order", Title: "#", Data: true},
	{Key: "number", Title: "Scene"},
	{Key: "int_ext", Title: "I/E"},
	{Key: "location", Title: "Location"},
	{Key: "time", Title: "Time", Data: true},
	{Key: "day_night", Title: "D/N"},
	{Numeric: true, Key: "page", Title: "Page", Data: true},
	{Numeric: true, Key: "eighths", Title: "Eighths", Data: true},
	{Numeric: true, Key: "length", Title: "Pages"},
	{Key: "cast", Title: "Cast"},
	{Key: "synopsis", Title: "Synopsis"},
	{Key: "strip", Title: "Strip", Data: true},
}

// OneLinerTable is the one-liner: a row per strip on a day, the days in
// order and their strips in shooting order (the scenes on no day left
// out). The cast are their numbers.
func (r Report) OneLinerTable(b Breakdown, s Schedule, o StripOptions) Table {
	strips := byOrder(r.Strips(b, o))
	t := Table{Kind: KindOneLiner, Title: "One-liner", Columns: OneLinerColumns}
	n := 0
	for di, d := range s.Days {
		for _, ord := range d.Scenes {
			st, ok := strips[ord]
			if !ok {
				continue
			}
			n++
			var cast []string
			for _, c := range st.CastNumbers {
				cast = append(cast, strconv.Itoa(c))
			}
			colour, _ := st.Colour()
			t.Rows = append(t.Rows, []Cell{num(di + 1), text(dateText(d.Date)), num(n),
				{Text: st.ID(), Value: float64(n)}, text(st.IE()), text(st.Location), text(st.Time), text(st.DN()),
				page(st.Page), num(int(st.Eighths)), length(st.Eighths), list(cast), text(st.Synopsis), text(colour)})
		}
	}
	return t
}

// DOODRow is a cast member's line of the Day Out of Days.
type DOODRow struct {
	Number int
	Name   string
	// Codes are the member's code on each day ("" none).
	Codes []string
	// Work and Hold are the days worked and held; Start and Finish the
	// first and last day worked (1, 2, …).
	Work, Hold, Start, Finish int
}

// DOOD is the Day Out of Days: each cast member on a scheduled scene, by
// cast number, with their code for each day.
func (r Report) DOOD(b Breakdown, s Schedule) []DOODRow {
	numbers := r.CastNumbers(b)
	scenes := map[int]Scene{}
	for _, sc := range r.Scenes {
		scenes[sc.Order] = sc
	}
	works := map[string][]bool{}
	for di, d := range s.Days {
		for _, ord := range d.Scenes {
			sc, ok := scenes[ord]
			if !ok || sc.Omitted {
				continue
			}
			names, _ := b.sceneCast(sc, numbers)
			for _, n := range names {
				name := n[strings.IndexByte(n, ' ')+1:]
				if works[name] == nil {
					works[name] = make([]bool, len(s.Days))
				}
				works[name][di] = true
			}
		}
	}
	var out []DOODRow
	for name, days := range works {
		row := DOODRow{Number: numbers[name], Name: name, Codes: make([]string, len(days))}
		first, last := slices.Index(days, true), 0
		for i, ok := range days {
			if ok {
				last = i
			}
		}
		for i := first; i <= last; i++ {
			switch {
			case !days[i]:
				row.Codes[i] = CodeHold
				row.Hold++
				continue
			case first == last:
				row.Codes[i] = CodeStartWorkFinish
			case i == first:
				row.Codes[i] = CodeStartWork
			case i == last:
				row.Codes[i] = CodeWorkFinish
			default:
				row.Codes[i] = CodeWork
			}
			row.Work++
		}
		row.Start, row.Finish = first+1, last+1
		out = append(out, row)
	}
	slices.SortFunc(out, func(a, b DOODRow) int { return a.Number - b.Number })
	return out
}

// DOODTable is the Day Out of Days as a table: the cast number and name,
// a column per day (day_1, day_2, …), then the days worked, held, in all,
// and the first and last.
func (r Report) DOODTable(b Breakdown, s Schedule) Table {
	cols := []Column{{Numeric: true, Key: "cast_number", Title: "#"}, {Key: "name", Title: "Cast"}}
	for i, d := range s.Days {
		title := "Day " + strconv.Itoa(i+1)
		if !d.Date.IsZero() {
			title += " " + d.Date.Format("2 Jan")
		}
		cols = append(cols, Column{Key: "day_" + strconv.Itoa(i+1), Title: title})
	}
	cols = append(cols,
		Column{Numeric: true, Key: "work", Title: "Work"},
		Column{Numeric: true, Key: "hold", Title: "Hold"},
		Column{Numeric: true, Key: "total", Title: "Total"},
		Column{Numeric: true, Key: "start_day", Title: "Start"},
		Column{Numeric: true, Key: "finish_day", Title: "Finish"})
	t := Table{Kind: KindDOOD, Title: "Day Out of Days", Columns: cols}
	for _, row := range r.DOOD(b, s) {
		cells := []Cell{num(row.Number), text(row.Name)}
		for _, c := range row.Codes {
			cells = append(cells, text(c))
		}
		cells = append(cells, num(row.Work), num(row.Hold), num(row.Work+row.Hold), num(row.Start), num(row.Finish))
		t.Rows = append(t.Rows, cells)
	}
	return t
}

// schedulePDF starts a PDF of the schedule's reports in Courier Prime.
func schedulePDF(orientation, paper string, record *pdf.Drawing) (pdf.Drawer, string) {
	p := newPDF(orientation, paper, record)
	p.SetMargins(36, 36, 36)
	p.SetAutoPageBreak(false, 36)
	const face = "sched"
	pdf.AddFont(p, face, "", font.GetFont(font.CourierPrimeName, ""))
	pdf.AddFont(p, face, "B", font.GetFont(font.CourierPrimeName, "B"))
	return p, face
}

// dayTitle is a day's head line: "Day 2 of 3 · Tue 1 Dec 2026 · Call 07:00".
func dayTitle(i, n int, d Day) string {
	parts := []string{fmt.Sprintf("Day %d of %d", i+1, n)}
	if !d.Date.IsZero() {
		parts = append(parts, d.Date.Format("Mon 2 Jan 2006"))
	}
	if d.Call != "" {
		parts = append(parts, "Call "+d.Call)
	}
	return strings.Join(parts, " · ")
}

// EndOfDay is a day break's line: "End of Day 1 — 4 6/8 pages".
func EndOfDay(i int, length lex.Eighths) string {
	unit := "pages"
	if length > 0 && length <= 8 {
		unit = "page" // 3/8 page, 1 page
	}
	return fmt.Sprintf("End of Day %d — %s %s", i+1, length, unit)
}

// WriteOneLinerPDF writes the one-liner as a PDF: each day's head (date,
// call, notes), its strips in their colours (scene, I/E, set, D/N, pages,
// cast numbers, synopsis), and the day break with its total.
func WriteOneLinerPDF(w io.Writer, title string, r Report, b Breakdown, s Schedule, o StripOptions, paper string) error {
	return writeOneLinerPDF(w, title, r, b, s, o, paper, nil)
}

// DrawOneLinerPDF is WriteOneLinerPDF, and its pages as drawn.
func DrawOneLinerPDF(w io.Writer, title string, r Report, b Breakdown, s Schedule, o StripOptions, paper string) (pdf.Drawing, error) {
	var d pdf.Drawing
	err := writeOneLinerPDF(w, title, r, b, s, o, paper, &d)
	return d, err
}

func writeOneLinerPDF(w io.Writer, title string, r Report, b Breakdown, s Schedule, o StripOptions, paper string, record *pdf.Drawing) error {
	p, face := schedulePDF("P", paper, record)
	strips := byOrder(r.Strips(b, o))
	pw, ph := p.GetPageSize()
	left, top := 36.0, 36.0
	width := pw - 2*left
	bottom := ph - top
	y := top
	cols := []struct {
		label string
		part  float64
	}{{"SC.", 0.08}, {"I/E", 0.07}, {"SET", 0.43}, {"D/N", 0.06}, {"PAGES", 0.1}, {"CAST", 0.26}}
	newPage := func() {
		p.AddPage()
		y = top
		p.SetFont(face, "B", 14)
		p.SetXY(left, y)
		p.CellFormat(width*0.6, 16, "ONE-LINE SCHEDULE", "", 0, "L", false, 0, "")
		p.SetFont(face, "", 9)
		p.CellFormat(width*0.4, 16, title, "", 0, "R", false, 0, "")
		y += 20
		p.SetFont(face, "B", 7.5)
		x := left + 4
		for _, c := range cols {
			p.SetXY(x, y)
			p.CellFormat(width*c.part-4, 10, c.label, "", 0, "L", false, 0, "")
			x += width * c.part
		}
		y += 14
	}
	newPage()
	for di, d := range s.Days {
		need := 40.0
		if y+need > bottom {
			newPage()
		}
		p.SetFont(face, "B", 10)
		p.SetXY(left, y)
		p.CellFormat(width, 14, dayTitle(di, len(s.Days), d), "", 0, "L", false, 0, "")
		y += 14
		if d.Notes != "" {
			p.SetFont(face, "", 8.5)
			for _, l := range p.SplitText(d.Notes, width) {
				p.SetXY(left, y)
				p.CellFormat(width, 11, l, "", 0, "L", false, 0, "")
				y += 11
			}
		}
		y += 2
		for _, ord := range d.Scenes {
			st, ok := strips[ord]
			if !ok {
				continue
			}
			p.SetFont(face, "", 8)
			syn := p.SplitText(st.Synopsis, width-8)
			if len(syn) > 2 {
				syn = syn[:2]
			}
			h := 16 + float64(len(syn))*10
			if y+h > bottom {
				newPage()
			}
			_, hex := st.Colour()
			if rr, g, bl, ok := rgb(hex); ok {
				p.SetFillColor(rr, g, bl)
			}
			p.SetDrawColor(120, 120, 120)
			p.SetLineWidth(0.5)
			p.Rect(left, y, width, h, "FD")
			x := left + 4
			vals := []string{st.ID(), st.IE(), st.Location, st.DN(), st.Eighths.String(), joinInts(st.CastNumbers)}
			for k, c := range cols {
				cw := width * c.part
				if k == 0 || k == 2 {
					p.SetFont(face, "B", 9.5)
				} else {
					p.SetFont(face, "", 9.5)
				}
				p.SetXY(x, y+2)
				p.CellFormat(cw-4, 12, fit(p, vals[k], cw-4), "", 0, "L", false, 0, "")
				x += cw
			}
			p.SetFont(face, "", 8)
			for k, l := range syn {
				p.SetXY(left+4+width*cols[0].part, y+14+float64(k)*10)
				p.CellFormat(width-8, 10, l, "", 0, "L", false, 0, "")
			}
			y += h
		}
		if y+18 > bottom {
			newPage()
		}
		p.SetFillColor(0, 0, 0) // the day break: black, as the board's
		p.Rect(left, y, width, 15, "F")
		p.SetTextColor(255, 255, 255)
		p.SetFont(face, "B", 9.5)
		p.SetXY(left+4, y+1)
		p.CellFormat(width-8, 13, EndOfDay(di, r.DayLength(d)), "", 0, "L", false, 0, "")
		p.SetTextColor(0, 0, 0)
		y += 26
	}
	if len(s.Days) == 0 {
		p.SetFont(face, "", 11)
		p.SetXY(left, y)
		p.CellFormat(width, 14, "No shoot days yet.", "", 0, "L", false, 0, "")
	}
	return output(p, w, record)
}

func joinInts(ns []int) string {
	s := make([]string, len(ns))
	for i, n := range ns {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ", ")
}

// WriteDOODPDF writes the Day Out of Days as a PDF, landscape: a row per
// cast member, a column per day (as many pages across as the days need),
// the totals on each, and the codes' legend.
func WriteDOODPDF(w io.Writer, title string, r Report, b Breakdown, s Schedule, paper string) error {
	return writeDOODPDF(w, title, r, b, s, paper, nil)
}

// DrawDOODPDF is WriteDOODPDF, and its pages as drawn.
func DrawDOODPDF(w io.Writer, title string, r Report, b Breakdown, s Schedule, paper string) (pdf.Drawing, error) {
	var d pdf.Drawing
	err := writeDOODPDF(w, title, r, b, s, paper, &d)
	return d, err
}

func writeDOODPDF(w io.Writer, title string, r Report, b Breakdown, s Schedule, paper string, record *pdf.Drawing) error {
	p, face := schedulePDF("L", paper, record)
	rows := r.DOOD(b, s)
	pw, ph := p.GetPageSize()
	left, top := 36.0, 36.0
	width := pw - 2*left
	const numW, nameW, dayW, totW, rowH = 22.0, 130.0, 34.0, 34.0, 15.0
	perPage := max(1, int((width-numW-nameW-5*totW)/dayW))
	chunks := max(1, (len(s.Days)+perPage-1)/perPage)
	const headH = 26.0
	rowsPer := max(1, int((ph-top-30-(top+24+headH))/rowH))
	rowChunks := max(1, (len(rows)+rowsPer-1)/rowsPer)
	for page := 0; page < chunks*rowChunks; page++ {
		c, rc := page/rowChunks, page%rowChunks // the days across, then the cast down
		lo, hi := c*perPage, min((c+1)*perPage, len(s.Days))
		rlo, rhi := rc*rowsPer, min((rc+1)*rowsPer, len(rows))
		p.AddPage()
		y := top
		p.SetFont(face, "B", 14)
		p.SetXY(left, y)
		p.CellFormat(width*0.6, 16, "DAY OUT OF DAYS", "", 0, "L", false, 0, "")
		p.SetFont(face, "", 9)
		head := title
		if chunks > 1 {
			head += fmt.Sprintf(" · days %d–%d of %d", lo+1, hi, len(s.Days))
		}
		if rowChunks > 1 {
			head += fmt.Sprintf(" · cast %d–%d of %d", rlo+1, rhi, len(rows))
		}
		p.CellFormat(width*0.4, 16, head, "", 0, "R", false, 0, "")
		y += 24
		// the head row: the days' numbers and dates
		p.SetLineWidth(0.6)
		p.SetFillColor(225, 225, 225)
		x := left
		cell := func(cw, h float64, s, align string, fill bool) {
			style := "D"
			if fill {
				style = "FD"
			}
			p.Rect(x, y, cw, h, style)
			p.SetXY(x+2, y+(h-11)/2)
			p.CellFormat(cw-4, 11, fit(p, s, cw-4), "", 0, align, false, 0, "")
			x += cw
		}
		p.SetFont(face, "B", 8)
		hh := headH
		cell(numW, hh, "#", "C", true)
		cell(nameW, hh, "CAST", "L", true)
		for i := lo; i < hi; i++ {
			p.Rect(x, y, dayW, hh, "FD")
			p.SetXY(x, y+3)
			p.CellFormat(dayW, 10, strconv.Itoa(i+1), "", 0, "C", false, 0, "")
			if d := s.Days[i].Date; !d.IsZero() {
				p.SetFont(face, "", 7)
				p.SetXY(x, y+13)
				p.CellFormat(dayW, 9, d.Format("2 Jan"), "", 0, "C", false, 0, "")
				p.SetFont(face, "B", 8)
			}
			x += dayW
		}
		for _, t := range []string{"WORK", "HOLD", "TOTAL", "START", "FINISH"} {
			cell(totW, hh, t, "C", true)
		}
		y += hh
		for _, row := range rows[rlo:rhi] {
			x = left
			p.SetFont(face, "", 8.5)
			cell(numW, rowH, strconv.Itoa(row.Number), "C", false)
			cell(nameW, rowH, row.Name, "L", false)
			p.SetFont(face, "B", 8.5)
			for i := lo; i < hi; i++ {
				code := row.Codes[i]
				fill := code != "" && code != CodeHold
				if fill {
					p.SetFillColor(255, 245, 157)
				} else {
					p.SetFillColor(255, 255, 255)
				}
				cell(dayW, rowH, code, "C", fill)
			}
			p.SetFont(face, "", 8.5)
			for _, n := range []int{row.Work, row.Hold, row.Work + row.Hold, row.Start, row.Finish} {
				cell(totW, rowH, strconv.Itoa(n), "C", false)
			}
			y += rowH
		}
		if len(rows) == 0 {
			p.SetFont(face, "", 10)
			p.SetXY(left, y+6)
			p.CellFormat(width, 12, "No cast on a scheduled scene yet.", "", 0, "L", false, 0, "")
		}
		p.SetFont(face, "", 7.5)
		p.SetXY(left, ph-top-12)
		p.CellFormat(width, 10, "SW start work · W work · WF work finish · SWF start, work and finish · H hold "+
			"(codes as Movie Magic Scheduling's)", "", 0, "L", false, 0, "")
	}
	return output(p, w, record)
}
