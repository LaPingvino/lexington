package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Production revisions in print (lex.Revisions): a line with a Revision
// gets its set's mark (*) in the right margin on each line it prints on,
// as Final Draft puts it, at 7.75" on Letter (its <Revisions
// Location="7.75">; right of a numbered heading's number when that is
// further). A page with a mark gets a header with the latest set on it
// and its date, on the line of the page number, ending before it
// (Wikipedia's Shooting script: "A slug (header) appears at the top of
// every revision page, aligned vertically with the page number"); on
// coloured paper too, if asked (a tint over the page, multiplied, so the
// text stays black). With PagesOnly only the changed pages are written,
// numbered as in the whole script.
//
// A deleted line has no row to mark: the caller marks the line after it
// (Accolade does), so the page it was on is a changed page.

// revising is what the render found of the revisions.
type revising struct {
	marks []revMark
	dual  string // the latest set of a dual dialogue being buffered
}

// revMark is a revised row as printed: its set and where.
type revMark struct {
	set  string
	span span
	numberEnd float64 // a numbered heading's: where its right number ends (0: none)
}

// revisionLocation is where the mark stands on Letter, in inches from
// the page's left edge (Final Draft's default).
const revisionLocation = 7.75

// revised notes the spans a row was printed on, when it has a revision.
func (t *Tree) revised(row lex.Line, spans []span) {
	if t.revs == nil || row.Revision == "" || len(spans) == 0 {
		return
	}
	m := revMark{set: row.Revision}
	if row.Type == lex.TypeScene {
		if _, number := lex.SceneNumber(row.Contents); number != "" {
			f := t.Rules.Get(lex.TypeScene)
			pageWidth, _ := t.PDF.GetPageSize()
			fontName, n := t.Fonts.font(f.Font, number)
			t.PDF.SetFont(fontName, "", f.Size)
			m.numberEnd = pageWidth - f.Right + 0.25 + t.PDF.GetStringWidth(n)
		}
	}
	for _, s := range spans {
		m.span = s
		t.revising.marks = append(t.revising.marks, m)
	}
}

// later is the later of two sets in the order of the Revisions' sets (a
// set not among them is earlier than any that is).
func (t *Tree) later(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	_, i, _ := t.revs.Set(a)
	_, j, _ := t.revs.Set(b)
	if j > i {
		return b
	}
	return a
}

// revisedPages are the pages with a mark, each with the latest set on it.
func (t *Tree) revisedPages() map[int]string {
	out := map[int]string{}
	if t.revising == nil {
		return out
	}
	for _, m := range t.revising.marks {
		out[m.span.page] = t.later(out[m.span.page], m.set)
	}
	return out
}

// drawRevisions draws the marks, the headers and the paper of the
// changed pages, once the text is laid out.
func (t *Tree) drawRevisions() {
	if t.revs == nil || t.revising == nil || len(t.revising.marks) == 0 {
		return
	}
	pdf := t.PDF
	now := pdf.PageNo()
	defer pdf.SetPage(now)
	left, top, right, _ := pdf.GetMargins()
	defer pdf.SetMargins(left, top, right)
	pdf.SetMargins(0, top, 0)
	auto, foot := pdf.GetAutoPageBreak()
	pdf.SetAutoPageBreak(false, 0) // writing in the margins makes no page
	defer pdf.SetAutoPageBreak(auto, foot)
	pageWidth, pageHeight := pdf.GetPageSize()
	x := pageWidth - (8.5-revisionLocation)*t.sc()
	f := t.Rules.Get(lex.TypeAction)
	fontName, _ := t.Fonts.font(f.Font, "*")
	pdf.SetTextColor(0, 0, 0)
	cell := pdf.GetCellMargin()
	pdf.SetCellMargin(0) // the mark's left edge where it is put
	defer pdf.SetCellMargin(cell)
	for _, m := range t.revising.marks {
		s, _, _ := t.revs.Set(m.set)
		pdf.SetPage(m.span.page)
		pdf.SetFont(fontName, "", 12)
		at := x
		if m.numberEnd > 0 {
			at = max(x, m.numberEnd+0.1)
		}
		for y := m.span.top; y+lineHeight/2 < m.span.bottom; y += lineHeight {
			pdf.SetXY(at, y)
			pdf.CellFormat(0.2, lineHeight, s.MarkOf(), "", 0, "L", false, 0, "")
		}
	}
	for page, set := range t.revisedPages() {
		s, _, ok := t.revs.Set(set)
		if !ok {
			s = lex.RevisionSet{Name: set}
		}
		pdf.SetPage(page)
		if c, ok := lex.RevisionPaper(s.Name); ok && t.revs.PageColors {
			pdf.SetAlpha(1, "Multiply")
			pdf.SetFillColor(int(c[0]), int(c[1]), int(c[2]))
			pdf.Rect(0, 0, pageWidth, pageHeight, "F")
			pdf.SetAlpha(1, "Normal")
		}
		if h := t.revs.HeaderFor(s); h != "" {
			pdf.SetFont("CourierPrime", "", 12)
			w := pdf.GetStringWidth(h)
			// ending a little before the page number (at 1" from the right)
			pdf.SetXY(pageWidth-1-0.25-w, 0.5)
			pdf.Cell(w, 0, h)
		}
	}
}

// RevisedPages are the pages of a screenplay with a change on them, as
// printed and numbered (the first after the title page is 1), with the
// latest set on each: what Revisions.PagesOnly keeps.
func (p *PDFWriter) RevisedPages(screenplay lex.Screenplay) map[int]string {
	t := p.tree(screenplay)
	t.revs = p.revisionsOrEmpty()
	t.Render()
	offset := 0
	if hasTitlePage(t.F) {
		offset = 1
	}
	out := map[int]string{}
	for page, set := range t.revisedPages() {
		out[page-offset] = set
	}
	return out
}

func (p *PDFWriter) revisionsOrEmpty() *lex.Revisions {
	if p.Revisions != nil {
		return p.Revisions
	}
	return &lex.Revisions{}
}

// ErrNoRevisedPages is PagesOnly's error when no page has a change.
var ErrNoRevisedPages = errors.New("no page has a revision on it")

var (
	kidsList  = regexp.MustCompile(`(?s)1 0 obj\n<</Type /Pages\n/Kids \[([0-9 R]*)\]\n/Count (\d+)`)
	kidObject = regexp.MustCompile(`(\d+) 0 R `)
)

// onlyPages is a PDF gofpdf wrote with only the pages keep tells (by
// number from 1) in its page tree: the others stay in the file, unshown.
// The page tree is written over in place, padded with spaces, so that
// the cross-reference's offsets hold.
func onlyPages(data []byte, keep func(page int) bool) ([]byte, error) {
	m := kidsList.FindSubmatchIndex(data)
	if m == nil {
		return nil, errors.New("the PDF's page tree was not found")
	}
	kids := kidObject.FindAllSubmatch(data[m[2]:m[3]], -1)
	var b strings.Builder
	n := 0
	for i, k := range kids {
		if keep(i + 1) {
			fmt.Fprintf(&b, "%s 0 R ", k[1])
			n++
		}
	}
	if n == 0 {
		return nil, ErrNoRevisedPages
	}
	list := b.String()
	count := strconv.Itoa(n)
	out := bytes.Clone(data)
	copy(out[m[2]:m[3]], list+strings.Repeat(" ", m[3]-m[2]-len(list)))
	copy(out[m[4]:m[5]], count+strings.Repeat(" ", m[5]-m[4]-len(count)))
	return out, nil
}
