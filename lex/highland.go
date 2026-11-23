package lex

// Highland's directives, done (not only left out of print, directives.go).
// What each does is Highland 2's, as eugenefischer/highwater-too (MIT,
// Copyright (c) 2026 eugenefischer) reads it and has checked against the
// PDFs Highland 2 prints: packages/parser/src/directives.ts and
// includes.ts, and its tests (ported in highland_test.go and
// includes_test.go). The code is Lexington's own.
//
//   - {{HEADER: …}} and {{FOOTER: …}}: the page's header and footer. One
//     segment goes on the default side (a header's right, a footer's
//     centre); "A | B" is left and right; "A | B | C" left, centre and
//     right; an empty segment prints nothing. %p is the page's number,
//     %pp the last page's, %# the section the page is in (its first text);
//     %none turns it off. A page has the header in effect at its first
//     text, the footer in effect at its end. A page with a header has no
//     "2." of its own.
//   - {{PAGENUMBER: n}}: the page it is on is page n, and on from there.
//   - {{INCLUDE: file}}: another file's text, put in where the line is
//     (ResolveIncludes).
//   - {{SERIES}}: one counter, 1, 2, 3…; {{SERIES: 5}} sets it to 5; any
//     other value counts on as if there were none.
//   - {{TIMESTAMP}}: the day it is printed, "Sep 28, 2026".
//   - {{TOC}}: a table of contents of the sections; {{TOC: # ###}} of
//     those levels only.
//   - {{%m: label}}: a marker, for the writer; never printed.

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DirectiveKind is what a directive does.
type DirectiveKind int

const (
	DirectiveOther DirectiveKind = iota // one Lexington does not know: left as it is
	DirectiveHeader
	DirectiveFooter
	DirectivePageNumber
	DirectiveInclude
	DirectiveSeries
	DirectiveTimestamp
	DirectiveTOC
	DirectiveMarker
	DirectiveRevision // {{REVISION: #color}} and {{/REVISION}}: a revision's tags
)

// PagePartKind is what a piece of a header or footer is.
type PagePartKind int

const (
	PartText    PagePartKind = iota
	PartPage                 // %p
	PartPages                // %pp: the last page's number
	PartSection              // %#: the section the page is in
)

// PagePart is a piece of a header's or footer's segment.
type PagePart struct {
	Kind PagePartKind
	Text string // PartText's
}

// Directive is one of Highland's {{…}}, read.
type Directive struct {
	Kind DirectiveKind
	// Text is the directive as written, the braces with it; Name its
	// name as written, Value what is after the colon (trimmed), HasValue
	// whether there is a colon.
	Text, Name, Value string
	HasValue          bool
	// A header's or footer's: Off (%none), and its left, centre and right
	// segments (nil: nothing there).
	Off                 bool
	Left, Center, Right []PagePart
	// Number is PAGENUMBER's page, or the number SERIES sets the counter
	// to; HasNumber whether there is one.
	Number    int
	HasNumber bool
	// Path is INCLUDE's file, as written; Label a marker's (or a SERIES'
	// value that is no number); Levels the TOC's levels (nil: all).
	Path, Label string
	Levels      []int
}

var (
	directiveShape = regexp.MustCompile(`^\{\{\s*([^:{}]*?)\s*(?::([^{}]*))?\}\}$`)
	anyDirective   = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	pageToken      = regexp.MustCompile(`%pp|%p|%#`)
	allDigits      = regexp.MustCompile(`^\d+$`)
	tocLevel       = regexp.MustCompile(`#+`)
	noneToken      = regexp.MustCompile(`(?i)%none\b`)
)

// ParseDirective reads one directive, braces and all ("{{HEADER: %p}}").
// Names are read in any case.
func ParseDirective(text string) Directive {
	d := Directive{Text: text}
	if m := directiveShape.FindStringSubmatchIndex(text); m != nil {
		d.Name = text[m[2]:m[3]]
		if m[4] >= 0 {
			d.Value, d.HasValue = strings.TrimSpace(text[m[4]:m[5]]), true
		}
	} else {
		d.Name = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "{{"), "}}"))
	}
	name := strings.ToUpper(d.Name)
	switch {
	case name == "HEADER" || name == "FOOTER":
		d.Kind = DirectiveHeader
		if name == "FOOTER" {
			d.Kind = DirectiveFooter
		}
		d.pageText()
	case name == "PAGENUMBER":
		d.Kind = DirectivePageNumber
		if allDigits.MatchString(d.Value) {
			d.Number, _ = strconv.Atoi(d.Value)
			d.HasNumber = true
		}
	case name == "INCLUDE":
		d.Kind, d.Path = DirectiveInclude, d.Value
	case name == "SERIES":
		d.Kind = DirectiveSeries
		if allDigits.MatchString(d.Value) {
			d.Number, _ = strconv.Atoi(d.Value)
			d.HasNumber = true
		} else {
			d.Label = d.Value
		}
	case name == "TIMESTAMP":
		d.Kind = DirectiveTimestamp
	case name == "TOC":
		d.Kind = DirectiveTOC
		for _, h := range tocLevel.FindAllString(d.Value, -1) {
			if !slices.Contains(d.Levels, len(h)) {
				d.Levels = append(d.Levels, len(h))
			}
		}
		slices.Sort(d.Levels)
	case name == "%M":
		d.Kind, d.Label = DirectiveMarker, d.Value
	case name == "REVISION" || strings.HasPrefix(name, "/") && strings.TrimSpace(name[1:]) == "REVISION":
		d.Kind = DirectiveRevision
	}
	return d
}

// pageText reads a header's or footer's value into its segments, as
// Highland's cheat sheet has them.
func (d *Directive) pageText() {
	if noneToken.MatchString(d.Value) {
		d.Off = true
		return
	}
	segs := strings.Split(d.Value, "|")
	for i := range segs {
		segs[i] = strings.TrimSpace(segs[i])
	}
	switch len(segs) {
	case 1:
		if d.Kind == DirectiveHeader {
			d.Right = pageParts(segs[0])
		} else {
			d.Center = pageParts(segs[0])
		}
	case 2:
		d.Left, d.Right = pageParts(segs[0]), pageParts(segs[1])
	default:
		d.Left = pageParts(segs[0])
		d.Center = pageParts(strings.Join(segs[1:len(segs)-1], " | "))
		d.Right = pageParts(segs[len(segs)-1])
	}
}

func pageParts(seg string) []PagePart {
	if seg == "" {
		return nil
	}
	var out []PagePart
	last := 0
	for _, m := range pageToken.FindAllStringIndex(seg, -1) {
		if m[0] > last {
			out = append(out, PagePart{Text: seg[last:m[0]]})
		}
		switch seg[m[0]:m[1]] {
		case "%pp":
			out = append(out, PagePart{Kind: PartPages})
		case "%p":
			out = append(out, PagePart{Kind: PartPage})
		default:
			out = append(out, PagePart{Kind: PartSection})
		}
		last = m[1]
	}
	if last < len(seg) {
		out = append(out, PagePart{Text: seg[last:]})
	}
	return out
}

// FillPage is a header's or footer's segment as printed on a page: page
// its number, pages the last page's, section the section it is in.
func FillPage(parts []PagePart, page, pages, section string) string {
	var b strings.Builder
	for _, p := range parts {
		switch p.Kind {
		case PartText:
			b.WriteString(p.Text)
		case PartPage:
			b.WriteString(page)
		case PartPages:
			b.WriteString(pages)
		case PartSection:
			b.WriteString(section)
		}
	}
	return b.String()
}

// Uses tells whether a header or footer has a piece of the kind.
func (d Directive) Uses(k PagePartKind) bool {
	for _, seg := range [][]PagePart{d.Left, d.Center, d.Right} {
		for _, p := range seg {
			if p.Kind == k {
				return true
			}
		}
	}
	return false
}

// FindDirectives are the directives in s, in their order.
func FindDirectives(s string) []Directive {
	if !strings.Contains(s, "{{") {
		return nil
	}
	var out []Directive
	for _, m := range anyDirective.FindAllString(s, -1) {
		out = append(out, ParseDirective(m))
	}
	return out
}

// Series is {{SERIES}}'s counter: one for a whole printed script (its
// included files with it). A number sets it; anything else adds one.
// Highland 2 printed SERIES, SERIES: test, SERIES, SERIES: 5, SERIES as 1,
// 2, 3, 5, 6.
type Series struct{ n int }

// Next is the number the series directive d prints.
func (c *Series) Next(d Directive) int {
	if d.HasNumber {
		c.n = d.Number
	} else {
		c.n++
	}
	return c.n
}

var months = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// FormatTimestamp is {{TIMESTAMP}} as Highland prints it: "Sep 28, 2026".
func FormatTimestamp(t time.Time) string {
	return months[t.Month()-1] + " " + strconv.Itoa(t.Day()) + ", " + strconv.Itoa(t.Year())
}

// Today is the day {{TIMESTAMP}} prints (the tests set it).
var Today = time.Now

// valueDirective finds SERIES and TIMESTAMP in any case.
var valueDirective = regexp.MustCompile(`(?i)\{\{\s*(?:series|timestamp)\s*(?::[^{}]*)?\}\}`)

// ExpandValues is the screenplay with {{SERIES}} and {{TIMESTAMP}} as they
// print: the series' numbers counted in the order of the text (title page
// first), the day Today. The boneyard is left as it is. The screenplay
// itself is not changed.
func ExpandValues(s Screenplay) Screenplay {
	var c Series
	var out Screenplay
	for i, l := range s {
		if l.Type == TypeBoneyard || !strings.Contains(l.Contents, "{{") || !valueDirective.MatchString(l.Contents) {
			continue
		}
		if out == nil {
			out = slices.Clone(s)
		}
		out[i].Contents = expandValues(l.Contents, &c)
	}
	if out == nil {
		return s
	}
	return out
}

func expandValues(s string, c *Series) string {
	return valueDirective.ReplaceAllStringFunc(s, func(m string) string {
		d := ParseDirective(m)
		if d.Kind == DirectiveSeries {
			return strconv.Itoa(c.Next(d))
		}
		return FormatTimestamp(Today())
	})
}

// isPageDirective tells whether a directive is one the pages do
// (Line.Directives carries them to the writers).
func isPageDirective(d Directive) bool {
	return d.Kind == DirectiveHeader || d.Kind == DirectiveFooter || d.Kind == DirectivePageNumber
}

// pageDirectives are the page directives in s, as written, one a line.
func pageDirectives(s string) string {
	var out []string
	for _, d := range FindDirectives(s) {
		if isPageDirective(d) {
			out = append(out, d.Text)
		}
	}
	return strings.Join(out, "\n")
}

// LineDirectives are the page directives a printable line carries
// (Line.Directives), read.
func LineDirectives(l Line) []Directive { return FindDirectives(l.Directives) }

// PageTexts are the first header and footer of a printable screenplay
// (nil for none): for the formats with one header and footer for the
// whole document (HTML, FDX).
func PageTexts(s Screenplay) (header, footer *Directive) {
	for _, l := range s {
		for _, d := range LineDirectives(l) {
			switch {
			case d.Kind == DirectiveHeader && header == nil:
				header = &d
			case d.Kind == DirectiveFooter && footer == nil:
				footer = &d
			}
		}
	}
	return header, footer
}

// HasPageDirectives tells whether a printable screenplay has page
// directives or a table of contents.
func HasPageDirectives(s Screenplay) bool {
	for _, l := range s {
		if l.Directives != "" || l.TOC != 0 {
			return true
		}
	}
	return false
}

// tocLine tells whether a line is a {{TOC}} alone (notes and other
// unprinted directives aside), and its directive.
func tocLine(l Line) (Directive, bool) {
	if !IsDirectiveLine(l) {
		return Directive{}, false
	}
	for _, d := range FindDirectives(l.Contents) {
		if d.Kind == DirectiveTOC {
			return d, true
		}
	}
	return Directive{}, false
}

// tableOfContents is a {{TOC}}'s entries: a line for each section of s of
// its levels (all, without), its text with two spaces for each level
// under the highest in it, in TOC the section's number among all the sections of
// s, counted from 1 (a writer that knows the pages puts its page beside
// it).
func tableOfContents(s Screenplay, d Directive) Screenplay {
	var out Screenplay
	n := 0
	for _, l := range s {
		if l.Type != TypeSection {
			continue
		}
		n++
		lv := sectionLevel(l)
		if d.Levels != nil && !slices.Contains(d.Levels, lv) {
			continue
		}
		text := SectionText(StripDirectives(l.Contents))
		if text == "" {
			continue
		}
		out = append(out, Line{Type: TypeAction, Contents: text, TOC: n, Block: lv})
	}
	top := 0 // the highest level in it: not indented
	for _, l := range out {
		if top == 0 || l.Block < top {
			top = l.Block
		}
	}
	for i := range out {
		out[i].Contents = strings.Repeat("  ", out[i].Block-top) + out[i].Contents
		out[i].Block = 0
	}
	return out
}

// TOCLines are the entries of the table of contents a line is, if it is
// a {{TOC}} alone (nil if not), from the sections of s (with their
// values, ExpandValues): for the writers that go through the script
// themselves.
func TOCLines(s Screenplay, l Line) Screenplay {
	d, ok := tocLine(l)
	if !ok {
		return nil
	}
	toc := tableOfContents(s, d)
	if toc == nil {
		toc = Screenplay{}
	}
	return toc
}
