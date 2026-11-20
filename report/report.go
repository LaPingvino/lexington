// Package report makes a screenplay's production reports, which store
// nothing and are made from the script alone: the scenes (number,
// heading split into INT/EXT, location and time, page, length in eighths
// of a page, who speaks), the locations, the characters, and the cast
// list numbered by size. Each is a Table, written as CSV (a documented,
// stable layout: see Columns) or as an HTML page to print.
//
// Lengths are measured as the PDF writer lays the script out (its rules,
// paper and pagination: pdf.PDFWriter.Measure), in eighths of a page
// (lex.Eighths).
package report

import (
	"cmp"
	"slices"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

// Options are how a report reads the script.
type Options struct {
	// Scenes are the scene headings' starts in the script's language
	// (INT, EXT, ...: a scene configuration of rules.TOMLConf); nil for
	// English.
	Scenes []string
	// Aliases are other names of characters: a cue's name (any case) to
	// the character's own (ANNIE → ANNA). The character is reported under
	// its own name only.
	Aliases map[string]string
	// PDF is the writer whose layout measures the pages (its rules and
	// paper); nil for Letter and rules.Default.
	PDF *pdf.PDFWriter
}

// Scene is a scene of the script.
type Scene struct {
	Order    int    // its place in the script: 1, 2, ...
	Number   string // its scene number (#12A#), "" if it has none
	Heading  string // as printed: without its number and notes
	Place    string // INT, EXT, INT./EXT, ... as written, without a dot ("" for none)
	Location string // the heading after the place, before the time
	Time     string // after the heading's last dash: DAY, NIGHT, ...
	Page     int    // the page its heading is printed on
	Eighths  lex.Eighths
	// Characters are those who speak in it, in the order they first do.
	Characters []string
	Omitted    bool // an OMITTED scene: it keeps its number, it has no length
	// Synopsis is the scene's first synopsis (Fountain's "= …"), as a
	// breakdown sheet's.
	Synopsis string
}

// ID is how the reports name the scene: its number, or its place in the
// script when it has none.
func (s Scene) ID() string {
	if s.Number != "" {
		return s.Number
	}
	return itoa(s.Order)
}

// Location is a place the script is set in, the scenes there together.
type Location struct {
	Name    string
	Places  []string // INT, EXT, ... as its scenes have them, in order of first use
	Times   []string // DAY, NIGHT, ... likewise
	Scenes  []string // the scenes' IDs, in script order
	Eighths lex.Eighths
}

// Character is someone who speaks in the script.
type Character struct {
	Name                string
	Scenes              []string // the IDs of the scenes they speak in
	First, Last         string   // the first and last of them
	FirstPage, LastPage int
	Speeches            int // their cues
	Words               int // the words of their dialogue
	Eighths             lex.Eighths
}

// CastMember is a character with their cast number.
type CastMember struct {
	Number int
	Character
}

// Report is a script's reports.
type Report struct {
	Scenes     []Scene     // in script order
	Locations  []Location  // in order of first appearance
	Characters []Character // in order of first appearance
}

// Make reads the reports from a screenplay, as parsed (its title page,
// boneyard and notes are no scene's).
func Make(s lex.Screenplay, o Options) Report {
	starts := o.Scenes
	if starts == nil {
		starts = rules.DefaultConf().Scenes["en"]
	}
	w := o.PDF
	if w == nil {
		w = &pdf.PDFWriter{}
	}
	lengths := w.Measure(s)
	var r Report
	chars := map[string]*Character{}
	var order []string
	var scene *Scene
	var speaker *Character
	inTitle := false
	for _, l := range s {
		switch l.Type {
		case lex.TypeTitlePage:
			inTitle = true
		case lex.TypeNewPage:
			inTitle = false
		}
		if inTitle {
			continue
		}
		switch l.Type {
		case lex.TypeScene:
			r.Scenes = append(r.Scenes, heading(l.Contents, starts))
			scene = &r.Scenes[len(r.Scenes)-1]
			scene.Order = len(r.Scenes)
			speaker = nil
		case lex.TypeSpeaker:
			name := o.alias(CueName(l.Contents))
			if name == "" {
				speaker = nil
				continue
			}
			c := chars[name]
			if c == nil {
				c = &Character{Name: name}
				chars[name] = c
				order = append(order, name)
			}
			c.Speeches++
			speaker = c
			if scene != nil && !slices.Contains(scene.Characters, name) {
				scene.Characters = append(scene.Characters, name)
			}
		case lex.TypeDialog:
			if speaker != nil {
				speaker.Words += countWords(lex.Unstyled(lex.StripNotes(l.Contents)))
			}
		case lex.TypeParen, lex.TypeEmpty, lex.TypeDualOpen, lex.TypeDualNext, lex.TypeDualClose, lex.TypeNote:
		case lex.TypeSynopsis:
			if scene != nil && scene.Synopsis == "" {
				scene.Synopsis = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lex.StripNotes(l.Contents)), "="))
			}
			speaker = nil
		default:
			speaker = nil
		}
	}
	if len(lengths) == len(r.Scenes) {
		for i := range r.Scenes {
			r.Scenes[i].Page = lengths[i].Page
			if !r.Scenes[i].Omitted {
				r.Scenes[i].Eighths = lengths[i].Eighths
			}
		}
	}
	// the characters' scenes, and the locations
	places := map[string]*Location{}
	var placeOrder []string
	for _, sc := range r.Scenes {
		for _, name := range sc.Characters {
			c := chars[name]
			if c.First == "" {
				c.First, c.FirstPage = sc.ID(), sc.Page
			}
			c.Last, c.LastPage = sc.ID(), sc.Page
			c.Scenes = append(c.Scenes, sc.ID())
			c.Eighths += sc.Eighths
		}
		if sc.Omitted {
			continue
		}
		key := locationKey(sc.Location)
		loc := places[key]
		if loc == nil {
			loc = &Location{Name: sc.Location}
			places[key] = loc
			placeOrder = append(placeOrder, key)
		}
		loc.Places = addOnce(loc.Places, sc.Place)
		loc.Times = addOnce(loc.Times, sc.Time)
		loc.Scenes = append(loc.Scenes, sc.ID())
		loc.Eighths += sc.Eighths
	}
	for _, k := range placeOrder {
		r.Locations = append(r.Locations, *places[k])
	}
	for _, name := range order {
		r.Characters = append(r.Characters, *chars[name])
	}
	return r
}

// Cast is the cast list: the characters by size, numbered 1, 2, 3 ... as
// a production numbers its cast. Size is the scenes they speak in, then
// their length in eighths, their speeches, their words; a tie goes to the
// one who speaks first, then by name.
func (r Report) Cast() []CastMember {
	first := map[string]int{}
	for i, c := range r.Characters {
		first[c.Name] = i
	}
	chars := slices.Clone(r.Characters)
	slices.SortStableFunc(chars, func(a, b Character) int {
		return cmp.Or(
			cmp.Compare(len(b.Scenes), len(a.Scenes)),
			cmp.Compare(b.Eighths, a.Eighths),
			cmp.Compare(b.Speeches, a.Speeches),
			cmp.Compare(b.Words, a.Words),
			cmp.Compare(first[a.Name], first[b.Name]),
			strings.Compare(a.Name, b.Name),
		)
	})
	out := make([]CastMember, len(chars))
	for i, c := range chars {
		out[i] = CastMember{Number: i + 1, Character: c}
	}
	return out
}

// Length is the scenes' length in all, OMITTED ones having none.
func (r Report) Length() lex.Eighths {
	var n lex.Eighths
	for _, s := range r.Scenes {
		n += s.Eighths
	}
	return n
}

func (o Options) alias(name string) string {
	if name == "" || o.Aliases == nil {
		return name
	}
	if a, ok := o.Aliases[name]; ok {
		return CueName(a)
	}
	key := locationKey(name)
	for k, a := range o.Aliases {
		if locationKey(k) == key {
			return CueName(a)
		}
	}
	return name
}

// CueName is the character a cue names: in capitals, without its notes,
// its extension ((V.O.), (CONT'D)), and Fountain's @ and ^.
func CueName(cue string) string {
	n := strings.TrimSpace(lex.StripNotes(cue))
	n = strings.TrimSpace(strings.TrimSuffix(n, "^"))
	n = strings.TrimPrefix(n, "@")
	if i := strings.Index(n, "("); i >= 0 {
		n = n[:i]
	}
	return strings.ToUpper(strings.Join(strings.Fields(n), " "))
}

// dashes part a heading's location from its time.
var dashes = []string{" - ", " – ", " — ", " -- "}

// heading reads a scene heading: its number, place, location and time.
func heading(contents string, starts []string) Scene {
	var s Scene
	if n, ok := lex.OmittedNumber(contents); ok {
		return Scene{Number: n, Heading: lex.Omitted, Omitted: true}
	}
	text, number := lex.SceneNumber(contents)
	s.Number = number
	text = strings.Join(strings.Fields(strings.TrimPrefix(strings.TrimSpace(text), ".")), " ")
	s.Heading = strings.ToUpper(text)
	rest := s.Heading
	if tag := headingTag(s.Heading, starts); tag != "" {
		s.Place = tag
		rest = strings.TrimLeft(rest[len(tag):], ". -")
	}
	cut, at := -1, ""
	for _, d := range dashes {
		if i := strings.LastIndex(rest, d); i > cut {
			cut, at = i, d
		}
	}
	if cut >= 0 {
		s.Location, s.Time = strings.TrimSpace(rest[:cut]), strings.TrimSpace(rest[cut+len(at):])
	} else {
		s.Location = strings.TrimSpace(rest)
	}
	return s
}

// headingTag is the start of a heading (in capitals) as written, without
// its dot: the longest of the starts it begins with, followed by a dot, a
// space, a dash or nothing.
func headingTag(h string, starts []string) string {
	best := ""
	for _, st := range starts {
		t := strings.TrimRight(strings.ToUpper(st), ".")
		if t == "" || len(t) <= len(best) || !strings.HasPrefix(h, t) {
			continue
		}
		if len(h) == len(t) || strings.ContainsRune(". -", rune(h[len(t)])) {
			best = t
		}
	}
	return best
}

// locationKey is a name as compared: in capitals, its spaces one.
func locationKey(name string) string {
	return strings.ToUpper(strings.Join(strings.Fields(name), " "))
}

func addOnce(list []string, s string) []string {
	if s == "" || slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// countWords counts the words in text, punctuation alone not one.
func countWords(text string) int {
	n := 0
	for _, f := range strings.Fields(text) {
		if strings.IndexFunc(f, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			n++
		}
	}
	return n
}
