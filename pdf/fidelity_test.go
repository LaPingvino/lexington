package pdf_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/lexington/cli"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/report"
	"github.com/LaPingvino/lexington/rules"
)

// A drawn case: a script and how it is printed.
type drawnCase struct {
	name   string
	input  string // a file, or the script itself if it has a new line
	o      cli.Options
	script lex.Screenplay // printed through pdf.PDFWriter instead, if set
	w      *pdf.PDFWriter
	make   func(io.Writer) (pdf.Drawing, error) // a report's PDF, if set
}

const colourScript = "Title: Colours\n\n# ACT ONE [[blue]]\n\nINT. PUB - NIGHT [[red]]\n\nAnna sits at the bar. **Bold** and *italic* and _underlined_ words.\n\nANNA\nHi there, it is a long line of dialogue that wraps over two lines at least.\n\nEXT. ROAD - DAY [[green]]\n\nBOB\nBye.\n\nINT. HOUSE - DAY\n\nNothing.\n"

const hebrewFidelity = `פנים. חנות ספרים - יום #1#

דני (40) עומד ליד הקופה.

He looks at the sign: "פתוח" and smiles.

@מִרְיָם
(בשקט)
שָׁלוֹם, דָּנִי!

ANNA ^
שלום, all good?

> חתך ל:
`

// drawnCases are the examples, the test inputs, and right to left,
// colours, revisions, styles, A4.
func drawnCases(t *testing.T) []drawnCase {
	var cs []drawnCase
	presets := map[string]string{"bd": "bd", "comic": "comic", "musical": "musical", "radio": "radio",
		"stageplay": "stageplay", "transcript": "transcript"}
	files, _ := filepath.Glob("../examples/*.fountain")
	inputs, _ := filepath.Glob("../testdata/input/*.fountain")
	for _, f := range append(files, inputs...) {
		base := strings.TrimSuffix(filepath.Base(f), ".fountain")
		o := cli.Options{Input: f}
		if p, ok := presets[base]; ok {
			o.Elements = p
		}
		cs = append(cs, drawnCase{name: base, input: f, o: o})
	}
	cs = append(cs,
		drawnCase{name: "inverse", input: "../examples/radio.fountain", o: cli.Options{Elements: "inverse"}},
		drawnCase{name: "a4", input: "../testdata/input/complex_screenplay.fountain", o: cli.Options{Page: "a4", Number: "count"}},
		drawnCase{name: "continueds", input: "../examples/tv-episode.fountain", o: cli.Options{Continueds: true, Number: "count"}},
		drawnCase{name: "hebrew", input: hebrewFidelity, o: cli.Options{SceneIn: "he"}},
		drawnCase{name: "hebrew-a5", input: hebrewFidelity + "\n" + hebrewFidelity, o: cli.Options{SceneIn: "he", Page: "a5"}},
	)
	for _, style := range []string{"line", "marker", "background", "text"} {
		cs = append(cs, drawnCase{name: "colour-" + style, input: colourScript, o: cli.Options{ColorStyle: style}})
	}
	cs = append(cs,
		drawnCase{name: "colour-marker-block", input: colourScript, o: cli.Options{ColorStyle: "marker", ColorScope: "block"}},
		drawnCase{name: "colour-tint-patterns", input: colourScript, o: cli.Options{ColorStyle: "line", ColorTint: true, ColorPatterns: true}},
	)
	marks := map[int]string{lineOf(t, "room 2,"): "Blue", lineOf(t, "room 5."): "Pink", lineOf(t, "room 9,"): "Blue"}
	revised := revisedScript(t, marks)
	cs = append(cs,
		drawnCase{name: "revisions", script: revised, w: &pdf.PDFWriter{Revisions: testSets}},
		drawnCase{name: "revisions-pages-only", script: revised, w: &pdf.PDFWriter{Revisions: pagesOnly(true, false)}},
		drawnCase{name: "revisions-colours", script: revised, w: &pdf.PDFWriter{Revisions: pagesOnly(false, true)}},
	)
	return append(cs, reportCases(t)...)
}

// reportCases are the production reports' PDFs: breakdown sheets, the
// one-liner, the Day Out of Days.
func reportCases(t *testing.T) []drawnCase {
	data, err := os.ReadFile("../report/testdata/shoot.fountain")
	if err != nil {
		t.Fatal(err)
	}
	r := report.Make(fountain.Parse(rules.DefaultConf().Scenes["en"], bytes.NewReader(data)), report.Options{})
	b := report.Breakdown{Elements: map[int][]report.Element{
		1: {{Category: "Props", Name: "revolver"}, {Category: "Cast Members", Name: "Carl"}, {Category: "Wardrobe", Name: "raincoat"}},
		2: {{Category: "Vehicles", Name: "taxi"}, {Category: "Animals", Name: "dog"}, {Category: "Background Actors", Name: "neighbours"}},
	}}
	day := func(d int) time.Time { return time.Date(2026, 12, d, 0, 0, 0, 0, time.UTC) }
	s := report.Schedule{Days: []report.Day{
		{ID: "d1", Date: day(1), Call: "07:00", Notes: "Kovač house; the kitchen", Scenes: []int{4, 1}},
		{ID: "d2", Date: day(2), Scenes: []int{2}},
		{ID: "d3", Date: day(4), Call: "6:30", Scenes: []int{5}},
	}}
	return []drawnCase{
		{name: "report-sheets", make: func(w io.Writer) (pdf.Drawing, error) { return report.DrawSheetsPDF(w, "Shoot", r.Sheets(b), "a4") }},
		{name: "report-oneliner", make: func(w io.Writer) (pdf.Drawing, error) {
			return report.DrawOneLinerPDF(w, "Shoot", r, b, s, report.StripOptions{}, "letter")
		}},
		{name: "report-dood", make: func(w io.Writer) (pdf.Drawing, error) { return report.DrawDOODPDF(w, "Shoot", r, b, s, "") }},
	}
}

func pagesOnly(only, colours bool) *lex.Revisions {
	r := *testSets
	r.PagesOnly, r.PageColors = only, colours
	return &r
}

// draw prints a case, recording it: the PDF and its drawing.
func (c drawnCase) draw(t *testing.T, record bool) ([]byte, pdf.Drawing) {
	t.Helper()
	var d pdf.Drawing
	out := filepath.Join(t.TempDir(), c.name+".pdf")
	if c.make != nil {
		var buf bytes.Buffer
		if record {
			d, err := c.make(&buf)
			if err != nil {
				t.Fatal(err)
			}
			return buf.Bytes(), d
		}
		var plain bytes.Buffer
		if _, err := c.make(&plain); err != nil {
			t.Fatal(err)
		}
		return plain.Bytes(), d
	}
	if c.script != nil {
		w := *c.w
		w.OutputFile = out
		if record {
			w.Record = &d
		}
		if err := w.Write(nil, c.script); err != nil {
			t.Fatal(err)
		}
	} else {
		o := c.o
		o.Input, o.Output, o.To = c.input, out, "pdf"
		o.ConfigFile = filepath.Join(t.TempDir(), "none.toml")
		if strings.Contains(c.input, "\n") {
			o.Input = filepath.Join(t.TempDir(), c.name+".fountain")
			if err := os.WriteFile(o.Input, []byte(c.input), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if record {
			o.Record = &d
		}
		if err := cli.Convert(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b, d
}

// The PDF made through a Recorder is the PDF made without one, byte for
// byte: recording changes nothing of what gofpdf draws.
func TestRecorderKeepsThePDF(t *testing.T) {
	pdf.FixDate()
	for _, c := range drawnCases(t) {
		t.Run(c.name, func(t *testing.T) {
			plain, _ := c.draw(t, false)
			recorded, d := c.draw(t, true)
			if !bytes.Equal(plain, recorded) {
				t.Errorf("the PDF differs when recorded (%d bytes, %d recorded)", len(plain), len(recorded))
			}
			if len(d.Pages) == 0 {
				t.Error("nothing recorded")
			}
		})
	}
}

// fidelityDPI is the resolution the pages are compared at.
const fidelityDPI = 100

// The recorded pages, painted by the Rasterizer, look like the PDF's
// pages as pdftoppm paints them: per page, the ink of each lies on the
// ink of the other (within a pixel), and where there is colour the
// colours agree.
func TestDrawingLooksLikeThePDF(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("no pdftoppm")
	}
	worst := 1.0
	for _, c := range drawnCases(t) {
		t.Run(c.name, func(t *testing.T) {
			data, d := c.draw(t, true)
			shots := pdftoppm(t, data)
			if len(shots) != len(d.Pages) {
				t.Fatalf("%d pages drawn, the PDF has %d", len(d.Pages), len(shots))
			}
			r := pdf.NewRasterizer(&d)
			for i, want := range shots {
				got := r.Page(i, fidelityDPI/72.0)
				m := compare(got, want)
				worst = min(worst, m.score())
				if !m.ok() {
					saveFailure(t, c.name, i, got, want)
					t.Errorf("page %d: %v", i+1, m)
				}
			}
		})
	}
	t.Logf("worst page's score: %.4f", worst)
}

// The comparison sees a difference: a line moved by 0.05in, a run left
// out, a colour changed.
func TestFidelityCatchesDifferences(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("no pdftoppm")
	}
	c := drawnCase{name: "colour", input: colourScript, o: cli.Options{ColorStyle: "marker"}}
	data, d := c.draw(t, true)
	want := pdftoppm(t, data)[1] // the first page after the title page
	page := d.Pages[1]
	texts := []int{}
	boxes := []int{}
	for i, m := range page.Marks {
		switch m.(type) {
		case pdf.Text:
			texts = append(texts, i)
		case pdf.Box:
			boxes = append(boxes, i)
		}
	}
	mutate := func(name string, f func(marks []pdf.Mark) []pdf.Mark) {
		marks := f(append([]pdf.Mark(nil), page.Marks...))
		m := pdf.Drawing{Pages: []pdf.DrawnPage{{Width: page.Width, Height: page.Height, Marks: marks}}, Fonts: d.Fonts}
		if r := compare(m.Image(0, fidelityDPI/72.0), want); r.ok() {
			t.Errorf("%s: not seen (%v)", name, r)
		}
	}
	mutate("a line moved", func(marks []pdf.Mark) []pdf.Mark {
		t := marks[texts[2]].(pdf.Text)
		t.Y += 3.6
		marks[texts[2]] = t
		return marks
	})
	mutate("a run left out", func(marks []pdf.Mark) []pdf.Mark {
		return append(marks[:texts[3]:texts[3]], marks[texts[3]+1:]...)
	})
	mutate("a colour changed", func(marks []pdf.Mark) []pdf.Mark {
		b := marks[boxes[0]].(pdf.Box)
		b.FillColor.R, b.FillColor.B = b.FillColor.B, b.FillColor.R
		marks[boxes[0]] = b
		return marks
	})
}

// pdftoppm paints a PDF's pages.
func pdftoppm(t *testing.T, data []byte) []image.Image {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("pdftoppm", "-r", fmt.Sprint(fidelityDPI), "-png", in, filepath.Join(dir, "p")).CombinedOutput(); err != nil {
		t.Fatalf("pdftoppm: %v %s", err, out)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "p-*.png"))
	var imgs []image.Image
	for _, f := range files { // (p-1, p-2 … sort as numbers do: zero-padded)
		r, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		imgs = append(imgs, img)
	}
	return imgs
}

// match is how two pages compare: the share of each one's ink that lies
// on (a pixel from) the other's, and of the coloured pixels whose
// colours agree.
type match struct {
	ink, inkBack, colour float64
	inkPixels            int
	size                 string
}

func (m match) score() float64 { return min(m.ink, m.inkBack, m.colour) }

// ok is the tolerance: 99.5% of the ink and of the colour (a line moved
// by 0.05in, or one run left out, on a page of 5000 ink pixels is 98%).
func (m match) ok() bool { return m.size == "" && m.score() >= 0.995 }

func (m match) String() string {
	if m.size != "" {
		return m.size
	}
	return fmt.Sprintf("ink drawn on the PDF's %.4f, the PDF's on drawn %.4f, colour %.4f (%d ink pixels)",
		m.ink, m.inkBack, m.colour, m.inkPixels)
}

func rgb(img image.Image, x, y int) (int, int, int) {
	r, g, b, _ := img.At(x, y).RGBA()
	return int(r >> 8), int(g >> 8), int(b >> 8)
}

// inkOf is where a page has ink: dark pixels.
func inkOf(img image.Image) [][]bool {
	b := img.Bounds()
	ink := make([][]bool, b.Dy())
	for y := range ink {
		ink[y] = make([]bool, b.Dx())
		for x := range ink[y] {
			r, g, bl := rgb(img, b.Min.X+x, b.Min.Y+y)
			ink[y][x] = (r+g+bl)/3 < 140
		}
	}
	return ink
}

// covered is the share of a's ink with b's ink within a pixel.
func covered(a, b [][]bool) (float64, int) {
	n, hit := 0, 0
	for y := range a {
		for x := range a[y] {
			if !a[y][x] {
				continue
			}
			n++
		near:
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if yy, xx := y+dy, x+dx; yy >= 0 && yy < len(b) && xx >= 0 && xx < len(b[yy]) && b[yy][xx] {
						hit++
						break near
					}
				}
			}
		}
	}
	if n == 0 {
		return 1, 0
	}
	return float64(hit) / float64(n), n
}

func compare(got, want image.Image) match {
	gb, wb := got.Bounds(), want.Bounds()
	if d := gb.Dx() - wb.Dx(); d < -1 || d > 1 || gb.Dy()-wb.Dy() < -1 || gb.Dy()-wb.Dy() > 1 {
		return match{size: fmt.Sprintf("size %v, the PDF's %v", gb.Size(), wb.Size())}
	}
	gi, wi := inkOf(got), inkOf(want)
	var m match
	var n int
	m.ink, n = covered(gi, wi)
	m.inkBack, m.inkPixels = covered(wi, gi)
	if n == 0 && m.inkPixels > 0 || n > 0 && m.inkPixels == 0 {
		m.ink, m.inkBack = 0, 0
	}
	// colour: the pixels coloured (not grey) in either, away from the
	// ink's edges, agree within 40 a channel
	w, h := min(gb.Dx(), wb.Dx()), min(gb.Dy(), wb.Dy())
	coloured, agree := 0, 0
	for y := 2; y < h-2; y++ {
		for x := 2; x < w-2; x++ {
			r1, g1, b1 := rgb(got, gb.Min.X+x, gb.Min.Y+y)
			r2, g2, b2 := rgb(want, wb.Min.X+x, wb.Min.Y+y)
			if spread(r1, g1, b1) < 24 && spread(r2, g2, b2) < 24 {
				continue
			}
			if inkNear(gi, x, y) || inkNear(wi, x, y) {
				continue
			}
			coloured++
			// within the colours about it (an edge may fall a pixel away,
			// or be painted in part)
			lo, hi := [3]int{255, 255, 255}, [3]int{}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					r2, g2, b2 := rgb(want, wb.Min.X+x+dx, wb.Min.Y+y+dy)
					for k, v := range [3]int{r2, g2, b2} {
						lo[k], hi[k] = min(lo[k], v), max(hi[k], v)
					}
				}
			}
			in := true
			for k, v := range [3]int{r1, g1, b1} {
				in = in && v >= lo[k]-40 && v <= hi[k]+40
			}
			if in {
				agree++
			}
		}
	}
	m.colour = 1
	if coloured > 0 {
		m.colour = float64(agree) / float64(coloured)
	}
	return m
}

func inkNear(ink [][]bool, x, y int) bool {
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if ink[y+dy][x+dx] {
				return true
			}
		}
	}
	return false
}

func spread(r, g, b int) int { return max(r, g, b) - min(r, g, b) }

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// saveFailure keeps both pages of a failure, under $LEXINGTON_FIDELITY.
func saveFailure(t *testing.T, name string, i int, got, want image.Image) {
	dir := os.Getenv("LEXINGTON_FIDELITY")
	if dir == "" {
		return
	}
	for k, img := range map[string]image.Image{"drawn": got, "pdf": want} {
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%d-%s.png", name, i+1, k)))
		if err != nil {
			t.Log(err)
			return
		}
		png.Encode(f, img)
		f.Close()
	}
}
