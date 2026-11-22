package pdf_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/lexington/cli"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

// featureScript is a made-up feature-length screenplay (some 110 pages):
// scenes of action paragraphs over several lines, speeches long enough
// to break with (MORE), parentheticals, emphasis, dual dialogue,
// transitions, sections, a forced page break.
func featureScript(seed uint64, scenes int) string {
	r := rand.New(rand.NewPCG(seed, 7))
	words := strings.Fields(`the a an old new dark quiet long small door window light rain city night car table glass
		hand eyes street room voice letter train morning shadow river bridge coat key phone silence moment wall floor
		looks turns waits walks opens closes holds reaches smiles listens stops runs falls watches remembers finds
		slowly again never always almost nothing everything somewhere outside inside behind across under`)
	sentence := func(n int) string {
		w := make([]string, n)
		for i := range w {
			w[i] = words[r.IntN(len(words))]
			if r.IntN(80) == 0 {
				w[i] = "*" + w[i] + "*"
			}
		}
		w[0] = strings.ToUpper(w[0][:1]) + w[0][1:]
		return strings.Join(w, " ") + [...]string{".", ".", ".", "?", "!"}[r.IntN(5)]
	}
	sentences := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = sentence(4 + r.IntN(9))
		}
		return strings.Join(s, " ")
	}
	names := []string{"ANNA", "BORIS", "CARLA", "DMITRI", "ELENA", "FRANK"}
	places := []string{"KITCHEN", "STATION", "ROOFTOP", "OFFICE", "CAR", "RIVERBANK", "HALLWAY"}
	var b strings.Builder
	b.WriteString("Title: A Feature\nCredit: written by\nAuthor: Nobody\n\n")
	for sc := 1; sc <= scenes; sc++ {
		if sc%25 == 1 {
			fmt.Fprintf(&b, "# ACT %d\n\n", sc/25+1)
		}
		fmt.Fprintf(&b, "%s. %s - %s\n\n", [...]string{"INT", "EXT"}[r.IntN(2)], places[r.IntN(len(places))], [...]string{"DAY", "NIGHT"}[r.IntN(2)])
		for range 1 + r.IntN(3) {
			for range 1 + r.IntN(3) { // a paragraph's lines
				b.WriteString(sentences(1+r.IntN(3)) + "\n")
			}
			b.WriteString("\n")
		}
		for range 2 + r.IntN(5) {
			name := names[r.IntN(len(names))]
			if r.IntN(12) == 0 { // dual dialogue
				fmt.Fprintf(&b, "%s\n%s\n\n%s ^\n%s\n\n", name, sentences(1), names[r.IntN(len(names))], sentences(1))
				continue
			}
			b.WriteString(name + "\n")
			if r.IntN(4) == 0 {
				b.WriteString("(" + strings.ToLower(strings.TrimSuffix(sentence(2+r.IntN(3)), ".")) + ")\n")
			}
			n := 1 + r.IntN(3)
			if r.IntN(6) == 0 {
				n = 6 + r.IntN(6) // a long speech
			}
			b.WriteString(sentences(n) + "\n\n")
			if r.IntN(5) == 0 {
				b.WriteString(sentences(1+r.IntN(2)) + "\n\n")
			}
		}
		switch r.IntN(10) {
		case 0:
			b.WriteString("CUT TO:\n\n")
		case 1:
			b.WriteString("> THE NEXT DAY <\n\n")
		}
		if sc == scenes/2 {
			b.WriteString("===\n\n")
		}
	}
	return b.String()
}

// drawer draws a script with o: from a Resume, or whole (nil).
func drawer(t testing.TB, text string, o cli.Options) func(*pdf.Resume) (pdf.Drawing, error) {
	s := fountain.ParseLines(rules.DefaultConf().Scenes["en"], strings.NewReader(text))
	dir := t.TempDir()
	return func(r *pdf.Resume) (pdf.Drawing, error) {
		var d pdf.Drawing
		o := o
		o.To, o.Record, o.Resume = "pdf", &d, r
		o.Output, o.ConfigFile = filepath.Join(dir, "out.pdf"), filepath.Join(dir, "none.toml")
		err := cli.Write(context.Background(), s, o)
		return d, err
	}
}

func whole(t testing.TB, draw func(*pdf.Resume) (pdf.Drawing, error)) pdf.Drawing {
	t.Helper()
	d, err := draw(nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type resumeCase struct {
	name string
	text string
	o    cli.Options
}

func resumeCases() []resumeCase {
	feature, shorter := featureScript(1, 150), featureScript(11, 50)
	return []resumeCase{
		{"feature", feature, cli.Options{}},
		{"feature-continueds", shorter, cli.Options{Continueds: true, Number: "count"}},
		{"feature-a4", shorter, cli.Options{Page: "a4"}},
		{"feature-keep", shorter, cli.Options{Pagination: "keep"}},
		{"feature-none", shorter, cli.Options{Pagination: "none"}},
		{"feature-line-tint", shorter, cli.Options{ColorStyle: "line", ColorTint: true}},
		{"feature-marker-block", featureScript(2, 40), cli.Options{ColorStyle: "marker", ColorScope: "block"}},
		{"complex", readFile("../testdata/input/complex_screenplay.fountain"), cli.Options{}},
		{"hebrew", strings.Repeat(hebrewFidelity+"\n", 30), cli.Options{SceneIn: "he"}},
		{"colours", strings.Repeat(colourScript, 20), cli.Options{ColorStyle: "background"}},
	}
}

func readFile(name string) string {
	b, err := os.ReadFile(name)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// samePages fails unless got is want: each page's number, look, lines
// and start.
func samePages(t *testing.T, what string, got, want []pdf.DrawnPage) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d pages, want %d", what, len(got), len(want))
	}
	for i := range got {
		g, w := got[i], want[i]
		if g.Number != w.Number || !g.Equal(w) || g.First != w.First || g.Last != w.Last {
			for k := range min(len(g.Marks), len(w.Marks)) {
				if g.Marks[k] != w.Marks[k] {
					t.Logf("mark %d: %+v, want %+v", k, g.Marks[k], w.Marks[k])
					break
				}
			}
			t.Fatalf("%s: page %d (%d) is not as drawn whole (number %d, lines %d-%d; want %d-%d; marks %d, want %d)",
				what, i, w.Number, g.Number, g.First, g.Last, w.First, w.Last, len(g.Marks), len(w.Marks))
		}
		if !sameStart(g.Start, w.Start) {
			t.Fatalf("%s: page %d (%d) starts at %+v, want %+v", what, i, w.Number, g.Start, w.Start)
		}
	}
}

// sameStart reports whether a and b are the same start.
func sameStart(a, b pdf.PageStart) bool {
	return a.Resumable() == b.Resumable() && (!a.Resumable() || a.Same(b)) && a.Page == b.Page && a.Line == b.Line && a.Word == b.Word
}

// A drawing made from a page's start is the whole drawing's from there:
// from every start, three pages and where the next starts; from a few,
// to the end.
func TestResumeDrawsTheSamePages(t *testing.T) {
	for _, c := range resumeCases() {
		t.Run(c.name, func(t *testing.T) {
			draw := drawer(t, c.text, c.o)
			all := whole(t, draw)
			resumable := 0
			for i := 1; i < len(all.Pages); i++ {
				from := all.Pages[i].Start
				if !from.Resumable() {
					continue
				}
				resumable++
				end := i + 3
				d, err := draw(&pdf.Resume{From: from, Stop: func(s pdf.PageStart) bool { return s.Page > all.Pages[min(end, len(all.Pages))-1].Number }})
				if err != nil {
					t.Fatalf("from page %d: %v", from.Page, err)
				}
				samePages(t, fmt.Sprintf("from page %d", from.Page), d.Pages, all.Pages[i:min(end, len(all.Pages))])
				if end < len(all.Pages) {
					if d.Next == nil || !sameStart(*d.Next, all.Pages[end].Start) {
						t.Fatalf("from page %d: stopped at %+v, want %+v", from.Page, d.Next, all.Pages[end].Start)
					}
				} else if d.Next != nil {
					t.Fatalf("from page %d: stopped at %+v, before the end", from.Page, d.Next)
				}
				if i == 1 || i == len(all.Pages)/2 { // to the end
					d, err := draw(&pdf.Resume{From: from})
					if err != nil {
						t.Fatal(err)
					}
					samePages(t, fmt.Sprintf("from page %d to the end", from.Page), d.Pages, all.Pages[i:])
				}
			}
			t.Logf("%d pages, %d resumable", len(all.Pages), resumable)
			if strings.HasPrefix(c.name, "feature") && resumable < len(all.Pages)*2/3 {
				t.Errorf("only %d of %d pages can be drawn from", resumable, len(all.Pages))
			}
		})
	}
}

// An edit on a page leaves the start of the page before alone: the
// drawing is made from there, the rows before unchanged; one before the
// start is refused.
func TestResumeRefusesAnEditBefore(t *testing.T) {
	text := featureScript(3, 30)
	all := whole(t, drawer(t, text, cli.Options{}))
	from := all.Pages[5].Start
	lines := strings.Split(text, "\n")
	before := slicesClone(lines)
	before[from.Line-3] += " More."
	if _, err := drawer(t, strings.Join(before, "\n"), cli.Options{})(&pdf.Resume{From: from}); !errors.Is(err, pdf.ErrNotResumable) {
		t.Fatalf("an edit before the start: %v, want ErrNotResumable", err)
	}
	after := slicesClone(lines)
	after[all.Pages[6].First+2] += " More."
	if _, err := drawer(t, strings.Join(after, "\n"), cli.Options{})(&pdf.Resume{From: from}); err != nil {
		t.Fatalf("an edit after the start: %v", err)
	}
}

// edits are edits of a script's lines: a word typed, a line typed or
// deleted, a scene heading typed (the numbers after it move), a long
// paragraph (a page more), a speaker's name deleted (its dialogue is
// action then), the first line of a page.
type edit struct {
	name string
	do   func(lines []string, at int) []string
}

var edits = []edit{
	{"word", func(l []string, at int) []string {
		for ; at < len(l) && strings.TrimSpace(l[at]) == ""; at++ {
		}
		if at < len(l) {
			l[at] += " again"
		}
		return l
	}},
	{"line", func(l []string, at int) []string {
		return insert(l, at, "", "He waits by the window for a while.", "")
	}},
	{"delete", func(l []string, at int) []string {
		if at < len(l) {
			return append(l[:at], l[at+1:]...)
		}
		return l
	}},
	{"heading", func(l []string, at int) []string { return insert(l, at, "", "EXT. NEW PLACE - DAY", "") }},
	{"paragraph", func(l []string, at int) []string {
		return insert(l, at, "", strings.Repeat("A long paragraph of action, written on and on to fill the page. ", 60), "")
	}},
	{"no speaker", func(l []string, at int) []string {
		for i := at; i < len(l); i++ {
			if s := strings.TrimSpace(l[i]); s != "" && s == strings.ToUpper(s) && !strings.Contains(s, ".") && i+1 < len(l) && l[i+1] != "" {
				l[i] = strings.ToLower(s) + " says."
				break
			}
		}
		return l
	}},
}

func insert(l []string, at int, add ...string) []string {
	at = min(at, len(l))
	return append(l[:at], append(add, l[at:]...)...)
}

func slicesClone(l []string) []string { return append([]string(nil), l...) }

// changedLine is the first line (from 1) where a and b differ.
func changedLine(a, b string) int {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	i := 0
	for i < len(la) && i < len(lb) && la[i] == lb[i] {
		i++
	}
	return i + 1
}

// A drawing reflowed after an edit is the drawing made whole of the
// script as edited — edit after edit, each from the drawing reflowed
// before it — and it is drawn again only from the page before the edit
// to where the pages start as they did.
func TestReflowIsTheWholeDrawing(t *testing.T) {
	for _, c := range []resumeCase{
		{"feature", featureScript(4, 150), cli.Options{}},
		{"feature-continueds", featureScript(5, 80), cli.Options{Continueds: true, Number: "count"}},
		{"feature-a4-line", featureScript(6, 60), cli.Options{Page: "a4", ColorStyle: "line"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := rand.New(rand.NewPCG(9, uint64(len(c.text))))
			text := c.text
			d := whole(t, drawer(t, text, c.o))
			kept := 0
			for n := 0; n < 12; n++ {
				if testing.Short() && n == 6 {
					break
				}
				e := edits[n%len(edits)]
				lines := strings.Split(text, "\n")
				at := 10 + r.IntN(len(lines)-20)
				if n%len(edits) == 0 && n > 0 { // a page's first line
					p := d.Pages[1+r.IntN(len(d.Pages)-2)]
					at = max(p.Start.Line-1, 1)
				}
				edited := strings.Join(e.do(lines, at), "\n")
				line := changedLine(text, edited)
				draw := drawer(t, edited, c.o)
				begun := time.Now()
				got, how, err := pdf.Reflow(context.Background(), d, line, draw)
				took := time.Since(begun)
				if err != nil {
					t.Fatal(err)
				}
				want := whole(t, draw)
				for i := range min(len(got.Pages), len(want.Pages)) {
					if got.Pages[i].First != want.Pages[i].First {
						t.Logf("page %d: got start %+v lines %d, want %+v %d; old %+v %d; how %+v", i, got.Pages[i].Start, got.Pages[i].First,
							want.Pages[i].Start, want.Pages[i].First, d.Pages[min(i, len(d.Pages)-1)].Start, d.Pages[min(i, len(d.Pages)-1)].First, how)
						break
					}
				}
				samePages(t, fmt.Sprintf("edit %d (%s at line %d)", n, e.name, line), got.Pages, want.Pages)
				if how.Kept {
					kept++
				}
				t.Logf("edit %d (%s at line %d, page %d): drawn %d pages from %d, the rest kept: %v (%v)", n, e.name, line,
					d.PageOf(line)+1, how.Drawn, how.From+1, how.Kept, took)
				text, d = edited, got
			}
			if !testing.Short() && kept < 6 {
				t.Errorf("only %d of 12 edits kept the pages after them", kept)
			}
		})
	}
}

// Redraw draws the edited page alone, from its start: the pages after it
// stay; a word typed in a line that stays on its page is drawn as the
// whole drawing has it.
func TestRedrawDrawsTheEditedPage(t *testing.T) {
	text := featureScript(7, 60)
	d := whole(t, drawer(t, text, cli.Options{}))
	page := d.Pages[20]
	lines := strings.Split(text, "\n")
	at := page.First + 3
	for strings.TrimSpace(lines[at-1]) == "" || lines[at-1] == strings.ToUpper(lines[at-1]) {
		at++
	}
	lines[at-1] = strings.Replace(lines[at-1], " ", " zebra ", 1)
	draw := drawer(t, strings.Join(lines, "\n"), cli.Options{})
	got, from, n, err := pdf.Redraw(d, at, at, draw)
	if err != nil {
		t.Fatal(err)
	}
	if from != 20 || n < 1 || n > 2 || len(got.Pages) != len(d.Pages) {
		t.Fatalf("drew %d pages from %d (%d pages), want page 20", n, from, len(got.Pages))
	}
	found := false
	for _, m := range got.Pages[20].Marks {
		if tx, ok := m.(pdf.Text); ok && strings.Contains(tx.Text, "zebra") {
			found = true
		}
	}
	if !found {
		t.Fatal("the word typed is not on the page redrawn")
	}
	for i := range d.Pages {
		if (i < from || i >= from+n) && !got.Pages[i].Equal(d.Pages[i]) {
			t.Fatalf("page %d drawn again", i)
		}
	}
	want := whole(t, draw)
	if !want.Pages[20].Equal(got.Pages[20]) && want.Pages[21].Start.Same(d.Pages[21].Start) {
		t.Fatal("the page redrawn is not the whole drawing's")
	}
}

// Reflow stops between pages when its context is cancelled: the old
// drawing, the context's error.
func TestReflowCancels(t *testing.T) {
	text := featureScript(8, 80)
	d := whole(t, drawer(t, text, cli.Options{}))
	lines := strings.Split(text, "\n")
	edited := strings.Join(insert(lines, 40, "", strings.Repeat("Long. ", 400), ""), "\n")
	ctx, cancel := context.WithCancel(context.Background())
	pages := 0
	draw := drawer(t, edited, cli.Options{})
	got, _, err := pdf.Reflow(ctx, d, 41, func(r *pdf.Resume) (pdf.Drawing, error) {
		stop := r.Stop
		r.Stop = func(s pdf.PageStart) bool {
			if pages++; pages == 3 {
				cancel() // typing again
			}
			return stop(s)
		}
		return draw(r)
	})
	if !errors.Is(err, context.Canceled) || len(got.Pages) != len(d.Pages) || pages != 3 {
		t.Fatalf("cancelled: %v, %d pages, asked %d times", err, len(got.Pages), pages)
	}
}

// PageOf is the page a line prints on.
func TestPageOf(t *testing.T) {
	text := featureScript(1, 20)
	d := whole(t, drawer(t, text, cli.Options{}))
	for i, p := range d.Pages {
		if p.First == 0 {
			continue
		}
		if got := d.PageOf(p.First); got != i && d.Pages[got].Last < p.First {
			t.Errorf("line %d: page %d, want %d", p.First, got, i)
		}
		if p.Start.Resumable() && p.Start.Line > p.First {
			t.Errorf("page %d starts at line %d, its first line %d", p.Number, p.Start.Line, p.First)
		}
	}
	if d.PageOf(1) != 0 || d.PageOf(1<<20) != len(d.Pages)-1 {
		t.Errorf("PageOf(1) %d, PageOf(the end) %d", d.PageOf(1), d.PageOf(1<<20))
	}
}

// BenchmarkRedraw: a word typed on page 60 of a feature, its page drawn
// again.
func BenchmarkRedraw(b *testing.B) {
	text := featureScript(1, 150)
	d := whole(b, drawer(b, text, cli.Options{}))
	lines := strings.Split(text, "\n")
	at := d.Pages[60].First + 2
	lines[at-1] += " again"
	draw := drawer(b, strings.Join(lines, "\n"), cli.Options{})
	b.ResetTimer()
	for b.Loop() {
		if _, _, _, err := pdf.Redraw(d, at, at, draw); err != nil {
			b.Fatal(err)
		}
	}
}

// colourful is a feature with colour notes on its acts and on some
// scenes' headings.
func colourful(seed uint64, scenes int) string {
	colours := []string{"red", "blue", "green", "yellow"}
	lines := strings.Split(featureScript(seed, scenes), "\n")
	n := 0
	for i, l := range lines {
		if strings.HasPrefix(l, "# ACT") || (strings.HasPrefix(l, "INT. ") || strings.HasPrefix(l, "EXT. ")) && n%4 == 0 {
			lines[i] += " [[" + colours[n%len(colours)] + "]]"
		}
		if strings.HasPrefix(l, "INT. ") || strings.HasPrefix(l, "EXT. ") {
			n++
		}
	}
	return strings.Join(lines, "\n")
}

// A heading's colour changed (its act or scene runs over pages after
// it): the pages after are drawn in the new colour too.
func TestReflowFollowsAColour(t *testing.T) {
	for _, o := range []cli.Options{
		{ColorStyle: "line"},
		{ColorStyle: "marker", ColorScope: "block"},
		{ColorStyle: "background", ColorScope: "block"},
	} {
		t.Run(o.ColorStyle, func(t *testing.T) {
			text := colourful(12, 80)
			d := whole(t, drawer(t, text, o))
			lines := strings.Split(text, "\n")
			for k, from := range []string{"[[red]]", "[[blue]]"} {
				at := -1
				for i := len(lines) / 3 * (1 - k); i < len(lines); i++ {
					if strings.Contains(lines[i], from) {
						at = i
						break
					}
				}
				if at < 0 {
					t.Fatalf("no %s", from)
				}
				lines[at] = strings.Replace(lines[at], from, "[[green]]", 1)
				edited := strings.Join(lines, "\n")
				draw := drawer(t, edited, o)
				got, how, err := pdf.Reflow(context.Background(), d, at+1, draw)
				if err != nil {
					t.Fatal(err)
				}
				samePages(t, fmt.Sprintf("%s to green at line %d", from, at+1), got.Pages, whole(t, draw).Pages)
				t.Logf("%s at line %d: drawn %d from %d, kept %v", from, at+1, how.Drawn, how.From+1, how.Kept)
				text, d = edited, got
			}
		})
	}
}
