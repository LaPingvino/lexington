package lex

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// The cases of highwater-too's directives.test.ts (MIT), which match what
// Highland 2 prints.

var (
	pP  = PagePart{Kind: PartPage}
	pPP = PagePart{Kind: PartPages}
	pS  = PagePart{Kind: PartSection}
)

func pT(s string) PagePart { return PagePart{Text: s} }

func slots(d Directive) [3][]PagePart { return [3][]PagePart{d.Left, d.Center, d.Right} }

func TestHeaderFooterSegments(t *testing.T) {
	for in, want := range map[string][3][]PagePart{
		// one segment: headers right, footers centred
		"{{HEADER: %p}}":       {nil, nil, {pP}},
		"{{FOOTER: hi there}}": {nil, {pT("hi there")}, nil},
		// pipes: left, (centre,) right
		"{{HEADER: %# | %p}}":               {{pS}, nil, {pP}},
		"{{FOOTER: Left | Centre | Right}}": {{pT("Left")}, {pT("Centre")}, {pT("Right")}},
		"{{HEADER: Title | }}":              {{pT("Title")}, nil, nil},
		"{{HEADER: | Title |}}":             {nil, {pT("Title")}, nil},
		// tokens in text, %pp before %p
		"{{HEADER:|Claude / Manuscript Edge Cases / %p of %pp}}": {nil, nil, {pT("Claude / Manuscript Edge Cases / "), pP, pT(" of "), pPP}},
	} {
		d := ParseDirective(in)
		if d.Kind != DirectiveHeader && d.Kind != DirectiveFooter || d.Off {
			t.Errorf("%s: kind %v off %v", in, d.Kind, d.Off)
		}
		if got := slots(d); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", in, got, want)
		}
	}
	if d := ParseDirective("{{FOOTER: %none}}"); d.Kind != DirectiveFooter || !d.Off || !reflect.DeepEqual(slots(d), [3][]PagePart{}) {
		t.Errorf("%%none: %+v", d)
	}
	d := ParseDirective("{{HEADER: %# / %p of %pp}}")
	if got := FillPage(d.Right, "3", "9", "ACT ONE"); got != "ACT ONE / 3 of 9" {
		t.Errorf("filled: %q", got)
	}
}

func TestOtherDirectives(t *testing.T) {
	if d := ParseDirective("{{PAGENUMBER: 10}}"); d.Kind != DirectivePageNumber || !d.HasNumber || d.Number != 10 {
		t.Errorf("PAGENUMBER: %+v", d)
	}
	if d := ParseDirective("{{PAGENUMBER: ten}}"); d.Kind != DirectivePageNumber || d.HasNumber {
		t.Errorf("PAGENUMBER ten: %+v", d)
	}
	if d := ParseDirective("{{INCLUDE: Scenes/Act 1.fountain}}"); d.Kind != DirectiveInclude || d.Path != "Scenes/Act 1.fountain" {
		t.Errorf("INCLUDE: %+v", d)
	}
	if d := ParseDirective("{{TIMESTAMP}}"); d.Kind != DirectiveTimestamp {
		t.Errorf("TIMESTAMP: %+v", d)
	}
	if d := ParseDirective("{{TOC}}"); d.Kind != DirectiveTOC || d.Levels != nil {
		t.Errorf("TOC: %+v", d)
	}
	if d := ParseDirective("{{TOC: # ###}}"); !reflect.DeepEqual(d.Levels, []int{1, 3}) {
		t.Errorf("TOC levels: %+v", d)
	}
	if d := ParseDirective("{{%m:check this}}"); d.Kind != DirectiveMarker || d.Label != "check this" {
		t.Errorf("marker: %+v", d)
	}
	// any case; unknown ones kept
	if d := ParseDirective("{{header: %p}}"); d.Kind != DirectiveHeader {
		t.Errorf("header: %+v", d)
	}
	if d := ParseDirective("{{WIDGET: x}}"); d.Kind != DirectiveOther || d.Name != "WIDGET" || d.Value != "x" {
		t.Errorf("WIDGET: %+v", d)
	}
	if d := ParseDirective("{{WIDGET}}"); d.Kind != DirectiveOther || d.Name != "WIDGET" || d.HasValue {
		t.Errorf("WIDGET alone: %+v", d)
	}
	if got := FormatTimestamp(time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)); got != "Sep 28, 2026" {
		t.Errorf("timestamp %q", got)
	}
}

func TestSeries(t *testing.T) {
	var c Series
	var got []int
	for _, s := range []string{"{{SERIES}}", "{{SERIES: test}}", "{{SERIES}}", "{{SERIES: 5}}", "{{SERIES}}"} {
		got = append(got, c.Next(ParseDirective(s)))
	}
	if !reflect.DeepEqual(got, []int{1, 2, 3, 5, 6}) {
		t.Errorf("series %v", got)
	}
	if d := ParseDirective("{{SERIES: test}}"); d.HasNumber || d.Label != "test" {
		t.Errorf("%+v", d)
	}
}

func fixToday(t *testing.T) {
	old := Today
	Today = func() time.Time { return time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local) }
	t.Cleanup(func() { Today = old })
}

// SERIES counts over the whole script (in dialogue too), TIMESTAMP is the
// day; the screenplay itself keeps them.
func TestExpandValues(t *testing.T) {
	fixToday(t)
	s := Screenplay{
		{Type: TypeSection, Contents: "# Chapter {{SERIES}}"},
		{Type: TypeSpeaker, Contents: "BOB"}, {Type: TypeDialog, Contents: "It's {{TIMESTAMP}}, take {{series: 5}}."},
		{Type: TypeBoneyard, Contents: "{{SERIES}}"},
		{Type: TypeSection, Contents: "# Chapter {{SERIES}}"},
	}
	got := ExpandValues(s)
	for i, want := range []string{"# Chapter 1", "BOB", "It's Sep 28, 2026, take 5.", "{{SERIES}}", "# Chapter 6"} {
		if got[i].Contents != want {
			t.Errorf("%d: %q, want %q", i, got[i].Contents, want)
		}
	}
	if s[0].Contents != "# Chapter {{SERIES}}" {
		t.Error("the screenplay changed")
	}
}

// Page directives go with the next line printed; a TOC's line is its
// entries.
func TestPrintablePageDirectives(t *testing.T) {
	s := Screenplay{
		{Type: TypeAction, Contents: "{{HEADER: %# | %p}}"},
		{Type: TypeAction, Contents: "{{FOOTER: Left | Centre | Right}}"}, {Type: TypeEmpty},
		{Type: TypeAction, Contents: "{{TOC}}"}, {Type: TypeEmpty},
		{Type: TypeSection, Contents: "# Act {{SERIES}}"},
		{Type: TypeScene, Contents: "INT. HOUSE - DAY"}, {Type: TypeEmpty},
		{Type: TypeAction, Contents: "{{PAGENUMBER: 10}}"}, {Type: TypeEmpty},
		{Type: TypeSection, Contents: "## Part {{SERIES}} [[blue]]"},
		{Type: TypeAction, Contents: "Text."},
	}
	p := Printable(s, nil)
	var got []string
	for _, l := range p {
		got = append(got, l.Type+"="+l.Contents+"|"+l.Directives+"|"+string(rune('0'+l.TOC)))
	}
	want := []string{
		"action=Act 1|{{HEADER: %# | %p}}\n{{FOOTER: Left | Centre | Right}}|1",
		"action=  Part 2||2",
		"empty=||0",
		"section=# Act 1||0",
		"scene=INT. HOUSE - DAY||0",
		"empty=||0",
		"section=## Part 2|{{PAGENUMBER: 10}}|0",
		"action=Text.||0",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if h, f := PageTexts(p); h == nil || f == nil || h.Text != "{{HEADER: %# | %p}}" {
		t.Errorf("page texts %v %v", h, f)
	}
	toc := Printable(Screenplay{{Type: TypeAction, Contents: "{{TOC: ##}}"}, {Type: TypeSection, Contents: "# A"}, {Type: TypeSection, Contents: "## B"}}, nil)
	if toc[0].Contents != "B" || toc[0].TOC != 2 {
		t.Errorf("one level: %+v", toc)
	}
}
