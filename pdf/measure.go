package pdf

import (
	"math"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"

	"github.com/phpdave11/gofpdf"
)

// SceneLength is where a scene is printed and how long it is, as the PDF
// writer lays the script out (its pagination, its paper).
type SceneLength struct {
	// Page is the page its heading is printed on, as numbered: the first
	// page after the title page is 1.
	Page int
	// Pages is its length in pages: from the top of its heading to the
	// top of the next scene's heading, or to the foot of the script's
	// last line. The room the pagination leaves at a page's foot (a
	// heading kept with what follows it) is the scene's before it, as a
	// ruler laid on the printed page measures it; the rest of a page
	// after a forced break (===, an act's end) is no one's. A page is the
	// lines that fit on it.
	Pages float64
	// Eighths is Pages in eighths of a page (lex.EighthsOf).
	Eighths lex.Eighths
}

// Measure lays a screenplay out as Write prints it (the same rules and
// paper; nothing is written) and measures its scenes: one SceneLength for
// each scene heading of the screenplay, in order, OMITTED ones too (a
// run printed as one, MergeOmitted: each on the run's page, the first
// with its length).
func (p *PDFWriter) Measure(screenplay lex.Screenplay) []SceneLength {
	t := p.tree(screenplay)
	t.measure = &measuring{}
	t.Render()
	m := t.measure
	_, top, _, _ := t.PDF.GetMargins()
	_, h := t.PDF.GetPageSize()
	_, foot := t.PDF.GetAutoPageBreak()
	page := math.Floor((h-foot-top+1e-6)/lineHeight) * lineHeight // the lines that fit
	offset := 0
	if hasTitlePage(t.F) {
		offset = 1
	}
	at := func(p place) float64 { return float64(p.page) + (p.y-top)/page }
	out := make([]SceneLength, len(m.starts))
	for i, start := range m.starts {
		end := m.end
		if i+1 < len(m.starts) {
			end = m.starts[i+1]
		}
		length := at(end) - at(start)
		for _, g := range m.gaps {
			if g.scene == i {
				length -= at(g.to) - at(g.from)
			}
		}
		length = max(0, length)
		out[i] = SceneLength{Page: start.page - offset, Pages: length, Eighths: lex.EighthsOf(length)}
	}
	if p.MergeOmitted { // a run printed as one: each of its headings on its page
		var each []SceneLength
		for i, n := range lex.OmittedRunSizes(screenplay) {
			for k := 0; k < n && i < len(out); k++ {
				l := out[i]
				if k > 0 {
					l.Pages, l.Eighths = 0, 0
				}
				each = append(each, l)
			}
		}
		out = each
	}
	return out
}

// Measure measures a screenplay's scenes as Letter paper and rules (nil:
// rules.Default) print them (see PDFWriter.Measure).
func Measure(screenplay lex.Screenplay, elements rules.Set) []SceneLength {
	return (&PDFWriter{Elements: elements}).Measure(screenplay)
}

func hasTitlePage(s lex.Screenplay) bool {
	return len(s) > 0 && s[0].Type == lex.TypeTitlePage
}

// place is a place on a page.
type place struct {
	page int
	y    float64
}

// measuring follows where scenes start and the script's text ends.
type measuring struct {
	starts []place // the scenes' headings' tops
	end    place   // the foot of the last text printed
	gaps   []gap   // the pages' rests left by forced breaks
}

// gap is the room a forced page break leaves blank in a scene.
type gap struct {
	scene    int
	from, to place
}

// forcedBreak notes the rest of the page a forced break leaves, from the
// last text printed to the next page's top, in the scene it is in.
func (m *measuring) forcedBreak(pdf *gofpdf.Fpdf) {
	if m == nil || len(m.starts) == 0 {
		return
	}
	_, top, _, _ := pdf.GetMargins()
	start := m.starts[len(m.starts)-1]
	from := m.end
	if from.page < start.page || (from.page == start.page && from.y < start.y) {
		from = start // nothing printed in the scene yet
	}
	m.gaps = append(m.gaps, gap{scene: len(m.starts) - 1, from: from, to: place{pdf.PageNo() + 1, top}})
}

// scene marks a scene starting where the PDF is now (its heading not yet
// printed: heading puts it where the heading landed).
func (m *measuring) scene(pdf *gofpdf.Fpdf) {
	if m != nil {
		m.starts = append(m.starts, place{pdf.PageNo(), pdf.GetY()})
	}
}

// heading puts the scene's start where its heading was printed (a heading
// that went over to the next page by itself).
func (m *measuring) heading(spans []span) {
	if m == nil || len(spans) == 0 || len(m.starts) == 0 {
		return
	}
	m.starts[len(m.starts)-1] = place{spans[0].page, spans[0].top}
	m.printed(spans)
}

// printed follows text printed in the script's body.
func (m *measuring) printed(spans []span) {
	if m == nil || len(spans) == 0 {
		return
	}
	last := spans[len(spans)-1]
	if e := (place{last.page, last.bottom}); e.page > m.end.page || (e.page == m.end.page && e.y > m.end.y) {
		m.end = e
	}
}
