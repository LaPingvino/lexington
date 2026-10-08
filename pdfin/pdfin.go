// Package pdfin reads a screenplay from a PDF: the text of a printed
// script (from Lexington, Final Draft, Highland, Fade In, ...), not a
// scanned one. It finds each line's position on the page and tells the
// elements apart by their indents, as a reader does: scene headings and
// action at the margin, dialogue about an inch in, parentheticals a bit
// further, character names about two inches in, transitions at the
// right. Page numbers, (MORE) and (CONT'D) at page breaks are dropped,
// scene numbers in the margins kept, the title page recognised.
package pdfin

import (
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/LaPingvino/lexington/internal/pdf"

	"github.com/LaPingvino/lexington/lex"
)

// run is text printed in one go on a line, in inches from the page's
// left edge.
type run struct {
	X, End float64 // where it starts and (about) ends
	Text   string
	Bold   bool
}

// line is a printed line: its runs from left to right.
type line struct {
	Page int
	Y    float64 // inches from the page's top
	Runs []run
}

func (l line) x() float64   { return l.Runs[0].X }
func (l line) end() float64 { return l.Runs[len(l.Runs)-1].End }
func (l line) text() string { return joinRuns(l.Runs) }
func (l line) upper() bool  { t := l.text(); return t == strings.ToUpper(t) && hasLetter(t) }
func (l line) bold() bool   { return l.Runs[0].Bold }
func joinRuns(rs []run) string {
	var p []string
	for _, r := range rs {
		p = append(p, r.Text)
	}
	return strings.Join(p, " ")
}

func hasLetter(s string) bool {
	for _, r := range s {
		if 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || r > 127 {
			return true
		}
	}
	return false
}

// ReadFile reads the screenplay in a PDF file.
func ReadFile(path string) (lex.Screenplay, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return Read(f, st.Size())
}

// Read reads the screenplay in a PDF.
func Read(r io.ReaderAt, size int64) (s lex.Screenplay, err error) {
	defer func() {
		if p := recover(); p != nil { // the PDF library panics on some files
			err = fmt.Errorf("reading the PDF: %v", p)
		}
	}()
	doc, err := pdf.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	var pages [][]line
	width := 8.5
	for i := 1; i <= doc.NumPage(); i++ {
		p := doc.Page(i)
		if p.V.IsNull() {
			continue
		}
		if box := p.V.Key("MediaBox"); box.Len() == 4 {
			width = box.Index(2).Float64() / 72
		}
		pages = append(pages, lines(i, p))
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages")
	}
	all := 0
	for _, p := range pages {
		all += len(p)
	}
	if all == 0 {
		return nil, fmt.Errorf("the PDF has no text: a scanned script needs OCR")
	}
	return classify(pages, width), nil
}

// lines are a page's text lines, top to bottom.
func lines(page int, p pdf.Page) []line {
	height := 11.0
	if box := p.V.Key("MediaBox"); box.Len() == 4 {
		height = box.Index(3).Float64() / 72
	}
	type glyph struct {
		x, y, size, w float64 // w: the glyph's width, 0 if the PDF does not say
		s             string
		bold          bool
	}
	var gs []glyph
	for _, t := range p.Content().Text {
		if t.S == "" || t.S == "\n" || t.S == "\r" || t.S == "\uFFFD" { // no Unicode for it: lost
			continue
		}
		gs = append(gs, glyph{t.X / 72, height - t.Y/72, t.FontSize, t.W / 72, t.S, strings.Contains(strings.ToLower(t.Font), "bold") || strings.HasSuffix(t.Font, "B")})
	}
	// rows: glyphs on (nearly) the same baseline, in the order printed
	sort.SliceStable(gs, func(i, j int) bool { return gs[i].y < gs[j].y-0.02 })
	var out []line
	for i := 0; i < len(gs); {
		j := i
		for j < len(gs) && math.Abs(gs[j].y-gs[i].y) < 0.03 {
			j++
		}
		row := gs[i:j]
		// runs in the order the PDF draws them: a glyph goes on the run if
		// it is where the run's text ends, or where the glyph before it was
		// (a run drawn from one position, glyphs without widths); then the
		// runs from left to right
		var runs []run
		est, last := -1.0, -1.0 // where the run's text should end; the glyph before
		for _, g := range row {
			em := g.size / 72 // the font size, in inches
			if em == 0 {
				em = 1.0 / 6
			}
			cw := 0.6 * em // Courier's width, where the PDF gives none
			onRun := math.Abs(g.x-last) < 0.02 || (g.x >= est-0.5*em && g.x <= est+0.9*em)
			if len(runs) == 0 || !onRun {
				if len(runs) > 0 && strings.TrimSpace(runs[len(runs)-1].Text) == "" {
					runs = runs[:len(runs)-1]
				}
				runs = append(runs, run{X: g.x, Bold: g.bold})
				est = g.x
			}
			r := &runs[len(runs)-1]
			// a glyph after the run's estimated end with a gap is a space
			// the PDF left out
			if g.x > est+0.15*em && !strings.HasSuffix(r.Text, " ") && g.s != " " {
				r.Text += " "
			}
			r.Text += g.s
			adv := g.w
			if adv <= 0 {
				adv = cw * float64(len([]rune(g.s)))
			}
			est = math.Max(est, g.x) + adv
			last = g.x
			r.End = est
		}
		sort.SliceStable(runs, func(a, b int) bool { return runs[a].X < runs[b].X })
		var clean []run
		for _, r := range runs {
			r.Text = strings.Join(strings.Fields(r.Text), " ")
			if r.Text != "" {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 {
			out = append(out, line{Page: page, Y: row[0].y, Runs: clean})
		}
		i = j
	}
	return out
}

var (
	pageNumber  = regexp.MustCompile(`^\(?\d+[A-Z]?\.?\)?$`)
	sceneNumber = regexp.MustCompile(`^[0-9]+[A-Z]{0,2}\.?$`)
	more        = regexp.MustCompile(`^\(MORE\)$`)
	contd       = regexp.MustCompile(`\s*\((CONT'D|CONT’D|CONTINUED|cont'd)\)\s*$`)
	continued   = regexp.MustCompile(`^\(?CONTINUED[:)]?\)?$|^CONTINUED: ?(\(\d+\))?$`)
	scenePrefix = regexp.MustCompile(`^(INT|EXT|EST|INT\.?/EXT|EXT\.?/INT|I/E)[ .]`)
	transition  = regexp.MustCompile(`(TO:|TO BLACK\.|OUT\.|IN:)$`)
	creditLine  = regexp.MustCompile(`(?i)^(written by|screenplay by|teleplay by|story by|by|an? .* by)$`)
	dateLike    = regexp.MustCompile(`(?i)\b(19|20)\d\d\b|\b(draft|revis)`)
)
