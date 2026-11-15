package pdf_test

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
)

// printedPages prints a script and reads its pages back: each page's
// lines, without the page number at its top.
func printedPages(t *testing.T, s lex.Screenplay, set rules.Set, page string) [][]pdfin.PageLine {
	t.Helper()
	file := filepath.Join(t.TempDir(), "x.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: set, Page: page}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	for i, p := range pages {
		var body []pdfin.PageLine
		for k, l := range p {
			if k > 0 || !pageNumber.MatchString(l.Text) {
				body = append(body, l)
			}
		}
		pages[i] = body
	}
	return pages
}

var pageNumber = regexp.MustCompile(`^\d+\.$`)

// kind is what a printed line is, by its indent on the page (the
// default rules' margins, scaled to the paper) and its text.
func kind(l pdfin.PageLine, scale float64) string {
	x := l.X / scale
	switch {
	case l.Text == "(MORE)":
		return "more"
	case x > 4.5:
		return "trans"
	case math.Abs(x-3.7) < 0.1:
		return "speaker"
	case math.Abs(x-3.1) < 0.1:
		return "paren"
	case math.Abs(x-2.5) < 0.1:
		return "dialog"
	case strings.HasPrefix(l.Text, "INT.") || strings.HasPrefix(l.Text, "EXT."):
		return "scene"
	}
	return "action"
}

func action(texts ...string) lex.Screenplay {
	var s lex.Screenplay
	for _, x := range texts {
		s = append(s, lex.Line{Type: lex.TypeAction, Contents: x})
	}
	return s
}

func numbered(format string, n int) []string {
	var out []string
	for k := 1; k <= n; k++ {
		out = append(out, fmt.Sprintf(format, k))
	}
	return out
}

// speech is a speech of many sentences, a few lines long.
const speech = "I never thought it would come to this. We had plans, you know. " +
	"Big ones. The house by the sea, the boat, the dog. And now look at us. " +
	"Standing here in the rain like two fools. Say something."

// subjects are what goes at a page's foot: each script part after the
// padding.
var subjects = map[string]lex.Screenplay{
	"scene": append(lex.Screenplay{{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY"}, {Type: lex.TypeEmpty}},
		action(numbered("House line %d.", 5)...)...),
	"speaker": {{Type: lex.TypeSpeaker, Contents: "ANNA"}, {Type: lex.TypeDialog, Contents: speech}},
	"paren": {{Type: lex.TypeSpeaker, Contents: "ANNA"}, {Type: lex.TypeParen, Contents: "(quietly)"},
		{Type: lex.TypeDialog, Contents: speech}},
	"trans": append(action(numbered("Before %d.", 3)...), lex.Line{Type: lex.TypeEmpty},
		lex.Line{Type: lex.TypeTrans, Contents: "CUT TO:"}),
	"styled": append(lex.Screenplay{{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY"}, {Type: lex.TypeEmpty}},
		action(numbered("House *line* **%d**.", 5)...)...),
	"dual": {{Type: lex.TypeDualOpen}, {Type: lex.TypeSpeaker, Contents: "ANNA"}, {Type: lex.TypeDialog, Contents: speech},
		{Type: lex.TypeDualNext}, {Type: lex.TypeSpeaker, Contents: "BOB"}, {Type: lex.TypeDialog, Contents: "Fine. Right. Okay."},
		{Type: lex.TypeDualClose}},
}

// script is a scene of pad lines of action, then the subject, then more.
func script(pad int, subject lex.Screenplay) lex.Screenplay {
	s := lex.Screenplay{{Type: lex.TypeScene, Contents: "INT. LEAD - DAY"}, {Type: lex.TypeEmpty}}
	s = append(s, action(numbered("Pad %d.", pad)...)...)
	s = append(s, lex.Line{Type: lex.TypeEmpty})
	s = append(s, subject...)
	s = append(s, lex.Line{Type: lex.TypeEmpty})
	for k := 0; k < 8; k++ {
		s = append(s, action(numbered(fmt.Sprintf("Tail %d-%%d.", k), 3)...)...)
		s = append(s, lex.Line{Type: lex.TypeEmpty})
	}
	return s
}

// tag is the paragraph a line of the scripts above is from ("Pad",
// "House", "Tail 3-"), "" for none.
var tagged = regexp.MustCompile(`^(Pad|House|Before|Tail \d+-)`)

// badBreaks are the ways the pages of a script break badly.
func badBreaks(pages [][]pdfin.PageLine, scale float64) []string {
	var bad []string
	count := map[string]map[int]int{} // a paragraph's lines on each page
	for p, page := range pages {
		if len(page) == 0 {
			continue
		}
		last, first := page[len(page)-1], page[0]
		if k := kind(last, scale); p < len(pages)-1 && (k == "scene" || k == "speaker" || k == "paren") {
			bad = append(bad, fmt.Sprintf("page %d ends with a %s: %q", p+1, k, last.Text))
		}
		if kind(first, scale) == "trans" {
			bad = append(bad, fmt.Sprintf("page %d starts with a transition", p+1))
		}
		for _, l := range page {
			if m := tagged.FindString(l.Text); m != "" {
				if count[m] == nil {
					count[m] = map[int]int{}
				}
				count[m][p]++
			}
		}
		// a speech broken over the page: (MORE) after a sentence, the
		// name again over the rest
		if kind(last, scale) == "dialog" && p < len(pages)-1 && len(pages[p+1]) > 0 && kind(pages[p+1][0], scale) == "dialog" {
			bad = append(bad, fmt.Sprintf("page %d breaks a speech without (MORE)", p+1))
		}
		if kind(last, scale) == "more" {
			before := page[len(page)-2].Text
			if !strings.ContainsAny(before[len(before)-1:], ".!?") {
				bad = append(bad, fmt.Sprintf("page %d breaks a speech within a sentence: %q", p+1, before))
			}
			dialog := 0
			for k := len(page) - 2; k >= 0 && kind(page[k], scale) == "dialog"; k-- {
				dialog++
			}
			next := pages[p+1]
			if len(next) == 0 || next[0].Text != "ANNA (CONT'D)" {
				bad = append(bad, fmt.Sprintf("page %d: no ANNA (CONT'D) after (MORE)", p+2))
			}
			after := 0
			for k := 1; k < len(next) && kind(next[k], scale) == "dialog"; k++ {
				after++
			}
			if dialog < 2 || after < 2 {
				bad = append(bad, fmt.Sprintf("page %d: a speech broken %d lines / %d", p+1, dialog, after))
			}
		}
	}
	for tag, pages := range count {
		if len(pages) < 2 {
			continue
		}
		for p, n := range pages {
			if n < 2 {
				bad = append(bad, fmt.Sprintf("%q: %d line on page %d", tag, n, p+1))
			}
		}
	}
	return bad
}

// Scene headings, characters and parentheticals at every line of a
// page's foot: none is the last on its page, nor a transition the first;
// a paragraph broken over a page leaves two lines on each; a speech
// breaks after a sentence with (MORE) and ANNA (CONT'D); dual dialogue
// is not broken. On Letter and on A4.
func TestPagination(t *testing.T) {
	for _, page := range []string{"letter", "a4"} {
		scale := 1.0
		if page == "a4" {
			scale = 8.27 / 8.5
		}
		for name, subject := range subjects {
			hit := false // whether the subject was at a page's foot once
			for pad := 40; pad <= 56; pad++ {
				pages := printedPages(t, script(pad, subject), rules.Default, page)
				for _, b := range badBreaks(pages, scale) {
					t.Errorf("%s, %s, %d lines before: %s", page, name, pad, b)
				}
				if name == "dual" {
					for p, pg := range pages {
						for _, l := range pg {
							if strings.Contains(l.Text, "Fine.") && (len(pg) == 0 || !strings.Contains(pg[0].Text+strings.Join(texts(pg), "|"), "ANNA")) {
								t.Errorf("%s, dual, %d lines before: page %d has BOB's line without the names", page, pad, p+1)
							}
						}
					}
				}
				hit = hit || len(pages) > 0 && moved(pages)
			}
			if !hit {
				t.Errorf("%s, %s: never at a page's foot", page, name)
			}
		}
	}
}

func texts(ls []pdfin.PageLine) []string {
	var out []string
	for _, l := range ls {
		out = append(out, l.Text)
	}
	return out
}

// moved reports whether a page ends short of its foot (something went
// over to the next).
func moved(pages [][]pdfin.PageLine) bool {
	for _, p := range pages[:len(pages)-1] {
		if len(p) > 0 && p[len(p)-1].Y < 9.9 {
			return true
		}
	}
	return false
}

// Without pagination (or with only some of it), the page fills to its
// foot as Lexington printed before: a heading can end it, a paragraph
// leave a line, a speech go on without (MORE).
func TestPaginationOff(t *testing.T) {
	for _, tc := range []struct {
		p       rules.Pagination
		subject string
		want    map[string]bool // the bad breaks that happen: by their words
	}{
		{rules.Pagination{}, "scene", map[string]bool{"ends with": true, "line on page": true}},
		{rules.Pagination{}, "speaker", map[string]bool{"ends with": true, "without (MORE)": true}},
		{rules.Pagination{Keep: true}, "scene", map[string]bool{"ends with": false, "line on page": true}},
		{rules.Pagination{Widows: true}, "scene", map[string]bool{"ends with": true, "line on page": false}},
		{rules.Pagination{Keep: true, Widows: true}, "paren", map[string]bool{"ends with": false, "(MORE)": false}},
	} {
		set := rules.Default.WithPagination(tc.p)
		got := map[string]bool{}
		for pad := 40; pad <= 56; pad++ {
			for _, b := range badBreaks(printedPages(t, script(pad, subjects[tc.subject]), set, ""), 1) {
				for words := range tc.want {
					got[words] = got[words] || strings.Contains(b, words)
				}
			}
		}
		for words, want := range tc.want {
			if got[words] != want {
				t.Errorf("%s, %s: %q %v, want %v", tc.p, tc.subject, words, got[words], want)
			}
		}
	}
}

// A forced page break is one: what is before it keeps with nothing.
func TestPaginationForcedBreak(t *testing.T) {
	for pad := 46; pad <= 54; pad++ {
		s := script(pad, lex.Screenplay{{Type: lex.TypeScene, Contents: "INT. LAST - DAY"}, {Type: lex.TypeEmpty},
			{Type: lex.TypeNewPage}, {Type: lex.TypeScene, Contents: "INT. NEXT - DAY"}})
		pages := printedPages(t, s, rules.Default, "")
		for p, page := range pages {
			for k, l := range page {
				if l.Text == "INT. NEXT - DAY" && k != 0 {
					t.Errorf("%d: INT. NEXT not at page %d's top", pad, p+1)
				}
			}
		}
	}
}
