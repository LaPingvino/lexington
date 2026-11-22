package pdf

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Drawing again from a page (a program showing the pages while the script
// is written: OpenLoco's way of redrawing only what an edit invalidated).
// Each recorded page has its start (DrawnPage.Start): where its text
// starts in the script, with what the pages before leave to it (the
// scene it continues and its CONTINUEDs so far, the page numbers, the
// rows pagination looks back at). A Resume draws the script again from
// such a start, as it is now, and is asked at each later page's start
// whether to go on: when a page starts as it did before (PageStart.Same)
// the pages from there are the drawing's before, and the drawing can
// stop. Reflow does that after an edit; Redraw draws the edited page
// alone, for a view to show at once.
//
// A start can be drawn from when the rows up to it are as they were, it
// is in the script's body, and it is not in a dual dialogue, nor within
// a row that does not break as it is printed again from its words (a
// heading, a row with a prefix, a line reordered for its direction,
// emphasis broken over the pages). Locked pages and revisions are
// always drawn whole.

// ErrNotResumable: the drawing cannot be made from that start (the script
// changed before it, or it is not a place one can start from).
var ErrNotResumable = errors.New("pdf: the pages cannot be drawn again from that start")

// PageStart is where a page's text starts in the script.
type PageStart struct {
	// Page is the page's number in the PDF (as DrawnPage.Number).
	Page int
	// Line is the script's line its first text is from (lex.Line.Source;
	// 0 if not known), Word the word of that row it starts at.
	Line, Word int
	at         *resumeAt // nil: the drawing cannot start here
}

// Resumable reports whether a drawing can start at s.
func (s PageStart) Resumable() bool { return s.at != nil }

// resumeAt is what drawing from a start needs.
type resumeAt struct {
	rows   lex.Screenplay // the printed screenplay the start is in (Tree.F)
	row    int            // the row of rows the text starts in
	rest   *lex.Line      // its rest, when the page starts within it
	block  int            // the rest's colour block
	prefix []lex.Line     // rows the writer put in at the page's top (NAME (CONT'D))
	ctx    []lex.Line     // the rows before, as pagination looks back on them
	ctxBlk []int          // their colour blocks
	state  carried
}

// carried is what the pages before a page leave to it.
type carried struct {
	bodyFirst   int // the body's first page (the document's numbering)
	inScene     bool
	scene       string
	rtl         bool
	contd       int
	continuing  bool
	lastsection int
}

// Resume draws a screenplay from a page's start (PDFWriter.Resume,
// cli.Options.Resume): the drawing (PDFWriter.Record) has that page and
// those after it, numbered as in the whole; no PDF is made.
type Resume struct {
	// From is a page's start in a drawing of the screenplay (as it was
	// before an edit after it); the zero PageStart draws from the start.
	From PageStart
	// Stop, if set, is asked as each later page starts, with where it
	// starts: true ends the drawing before that page (Drawing.Next).
	Stop func(PageStart) bool
}

// Same reports whether the pages drawn from s are those drawn from o (of
// the same settings): s and o start the same page number with the same
// carried over, and the script from there on is the same. A drawing made
// again after an edit can stop at a start that is Same as the old
// drawing's start of that page: the rest is the old drawing's.
func (s PageStart) Same(o PageStart) bool {
	a, b := s.at, o.at
	if a == nil || b == nil || s.Page != o.Page || s.Word != o.Word || a.state != b.state ||
		(a.rest == nil) != (b.rest == nil) || !sameRows(a.prefix, b.prefix) || !sameRows(a.ctx, b.ctx) {
		return false
	}
	from := a.row
	if a.rest != nil {
		if !sameRow(*a.rest, *b.rest) {
			return false
		}
		from++
	}
	return sameRows(a.rows[from:], b.rows[b.row+from-a.row:])
}

// sameRow reports whether two rows print the same (their lines and
// colour blocks' numbers aside: an edit before them moves those).
func sameRow(a, b lex.Line) bool {
	a.Source, b.Source, a.Block, b.Block = 0, 0, 0, 0
	return a == b
}

func sameRows(a, b []lex.Line) bool {
	if len(a) != len(b) {
		return false
	}
	for i := len(a) - 1; i >= 0; i-- { // from the end: a renumbered heading differs early
		if !sameRow(a[i], b[i]) {
			return false
		}
	}
	return true
}

// of is s in rows, the same up to it.
func (s PageStart) of(rows lex.Screenplay) PageStart {
	if s.at != nil {
		at := *s.at
		at.rows = rows
		s.at = &at
	}
	return s
}

// rows are the printed rows a drawing was drawn from (nil if not known).
func (d Drawing) rows() lex.Screenplay {
	for _, p := range d.Pages {
		if p.Start.at != nil {
			return p.Start.at.rows
		}
	}
	if d.Next != nil && d.Next.at != nil {
		return d.Next.at.rows
	}
	return nil
}

// rebased is s, a start of a drawing's page after start o, in the drawing
// whose start n is Same as o: its rows and lines moved as n's are.
func (s PageStart) rebased(o, n PageStart) PageStart {
	if s.Line > 0 && n.Line > 0 && o.Line > 0 {
		s.Line += n.Line - o.Line
	}
	if s.at != nil && o.at != nil && n.at != nil {
		at := *s.at
		at.rows, at.row = n.at.rows, s.at.row+n.at.row-o.at.row
		s.at = &at
	}
	return s
}

// loopState is where Render is in the rows, for the pages' starts.
type loopState struct {
	rows        lex.Screenplay
	info        []rowInfo
	i, next     int // the row being printed; the row a page begun now starts with
	block       string
	lastsection int
}

// pageStarted notes the start of the page just begun (pageBegins, before
// it applies what the page continues), and asks the Resume whether to go
// on.
func (t *Tree) pageStarted() {
	if t.starts == nil {
		return
	}
	page := t.PDF.PageNo()
	st := t.startHere(page)
	t.starts[page] = st
	if r := t.resume; r != nil && r.Stop != nil && t.stopAt == 0 && page > 1 && t.resumable() && r.Stop(st) {
		t.stopAt = page
	}
}

// resumable reports whether the tree draws pages one can start from.
func (t *Tree) resumable() bool { return t.revs == nil && t.lock == nil && t.measure == nil }

// startHere is the start of page (the PDF's), begun now.
func (t *Tree) startHere(page int) PageStart {
	st := PageStart{Page: page + t.offset}
	l := &t.loop
	rows, info := l.rows, l.info
	s := t.state()
	at := &resumeAt{rows: t.F, state: carried{bodyFirst: s.bodyFirst, inScene: s.inScene, scene: s.scene, rtl: s.rtl,
		contd: s.contd, continuing: s.continuing, lastsection: l.lastsection}}
	if at.state.bodyFirst == 0 && t.offset == 0 { // the body's first page, after the title page
		at.state.bodyFirst = page
	}
	at.state.bodyFirst += t.offset
	ok := t.resumable() && l.block == "" && !t.DualDialogue && rows != nil &&
		!(page == 1 && t.offset == 0 && hasTitlePage(t.F)) // (the title page)
	n := l.next
	if p := t.printing; ok && p != nil && p.on {
		lines := 0
		for _, sp := range p.spans {
			lines += int(math.Round((sp.bottom - sp.top) / lineHeight))
		}
		if lines > 0 { // within the row: from the word its next line starts with
			n = l.i
			row, in := rows[n], info[n]
			w := t.wordAtLine(row, lines)
			st.Word = in.word + w
			if in.origin >= 0 {
				st.Line = t.F[in.origin].Source
			}
			cut := fieldOffset(row.Contents, w)
			f, _ := t.format(row.Type, row.Contents)
			// (styled text Write wraps, which wrap follows as near as it
			// does only: not from within it)
			if !ok || in.origin < 0 || w == 0 || cut >= len(row.Contents) || f.Prefix != "" || !flows(row.Type) && row.Type != lex.TypeDialog ||
				strings.ContainsAny(row.Contents, "*_") || needsBidi(row.Contents, t.lineRTL(row.Contents)) {
				return st
			}
			rest := row
			rest.Contents = strings.TrimSpace(row.Contents[cut:])
			at.row, at.rest, at.block = in.origin, &rest, in.block
			at.ctx, at.ctxBlk = lookback(rows, info, n)
			st.at = at
			return st
		}
	}
	m := n // the first row from the script after what the writer put in
	for m < len(rows) && info[m].origin < 0 {
		m++
	}
	if m == len(rows) {
		return st
	}
	in := info[m]
	st.Line, st.Word = t.F[in.origin].Source, in.word
	if !ok {
		return st
	}
	for _, r := range rows[n:m] {
		switch r.Type {
		case typeLock, typeBreak:
			return st
		}
	}
	at.row, at.prefix = in.origin, slices.Clone(rows[n:m])
	if in.word > 0 {
		rest := rows[m]
		at.rest, at.block = &rest, in.block
	}
	at.ctx, at.ctxBlk = lookback(rows, info, n)
	st.at = at
	return st
}

// lookback are the rows before row n that pagination looks back at: the
// paragraph or speech the rows before it are in, the blank lines between,
// the element before them.
func lookback(rows lex.Screenplay, info []rowInfo, n int) ([]lex.Line, []int) {
	k := n
	for k > 0 {
		k--
		switch rows[k].Type {
		case lex.TypeEmpty, typeMore, typeBreak, lex.TypeDialog, lex.TypeParen:
			continue
		}
		if continues(rows, k) {
			continue
		}
		break
	}
	ctx := slices.Clone(rows[k:n])
	blocks := make([]int, len(ctx))
	for j := range ctx {
		blocks[j] = info[k+j].block
	}
	return ctx, blocks
}

// resumeRows are the rows to draw from the Resume's start, their
// information, and the row to start at; ErrNotResumable if the script
// changed before the start.
func (t *Tree) resumeRows(rows lex.Screenplay, info []rowInfo) (lex.Screenplay, []rowInfo, int, error) {
	at := t.resume.From.at
	if at == nil || !t.resumable() || at.row >= len(t.F) || at.row >= len(at.rows) || len(at.ctx) != len(at.ctxBlk) {
		return nil, nil, 0, ErrNotResumable
	}
	if !slices.Equal(t.F[:at.row+1], at.rows[:at.row+1]) {
		return nil, nil, 0, ErrNotResumable
	}
	gen := func(block int) rowInfo { return rowInfo{block: block, origin: -1} }
	var out lex.Screenplay
	var outInfo []rowInfo
	for j, r := range at.ctx {
		out, outInfo = append(out, r), append(outInfo, gen(at.ctxBlk[j]))
	}
	start := len(out)
	from := at.row
	block := info[from].block
	for _, r := range at.prefix {
		out, outInfo = append(out, r), append(outInfo, gen(block))
	}
	if at.rest != nil {
		out = append(out, *at.rest)
		outInfo = append(outInfo, rowInfo{block: at.block, origin: from, word: t.resume.From.Word})
		from++
	}
	out = append(out, rows[from:]...)
	outInfo = append(outInfo, info[from:]...)
	return out, outInfo, start, nil
}

// resumeState sets the state the pages before the start leave to it,
// and begins its page.
func (t *Tree) resumeState() {
	c := t.resume.From.at.state
	t.offset = t.resume.From.Page - 1
	s := t.state()
	s.bodyFirst = c.bodyFirst - t.offset
	s.inScene, s.scene, s.rtl, s.contd, s.continuing = c.inScene, c.scene, c.rtl, c.contd, c.continuing
	t.loop.lastsection = c.lastsection
	t.pageBegins()
}

// PageOf is the index of the page a line of the script prints on (the
// first, for a row broken over pages; for a line printing nothing, the
// page before the next that prints), 0 if none is known.
func (d Drawing) PageOf(line int) int {
	at := 0
	for i, p := range d.Pages {
		if p.First == 0 {
			continue
		}
		if p.First > line {
			break
		}
		at = i
		if p.Last >= line {
			return i
		}
	}
	return at
}

// Equal reports whether p and q look the same: their size and marks.
func (p DrawnPage) Equal(q DrawnPage) bool {
	return p.Width == q.Width && p.Height == q.Height && slices.Equal(p.Marks, q.Marks)
}

// pageIndex is the index of the page numbered n in d, -1 if none.
func (d Drawing) pageIndex(n int) int {
	for i, p := range d.Pages {
		if p.Number == n {
			return i
		}
	}
	return -1
}

// Reflowed says what Reflow did.
type Reflowed struct {
	From  int  // the index of the first page drawn again
	Drawn int  // how many were drawn
	Kept  bool // the pages after them are the old drawing's (it stopped where a page started as before)
}

// Reflow is old, a drawing of a script, made again for the script as
// edited from line on (line in the text as edited): it draws the pages
// again from the page before the one the line is on (an edit at a page's
// top can take lines back to the page before: pagination looks ahead)
// and stops at a page that starts as before (PageStart.Same); the rest is
// old's. draw draws the script as edited with a Resume (nil: whole). A
// start that cannot be drawn from is passed for the one before; ctx
// cancels it between pages (its error then).
func Reflow(ctx context.Context, old Drawing, line int, draw func(*Resume) (Drawing, error)) (Drawing, Reflowed, error) {
	stop := func(s PageStart) bool {
		if ctx.Err() != nil {
			return true
		}
		i := old.pageIndex(s.Page)
		return i >= 0 && s.Same(old.Pages[i].Start)
	}
	from := max(old.PageOf(line)-1, 0)
	for {
		for from > 0 && !old.Pages[from].Start.Resumable() {
			from--
		}
		r := &Resume{Stop: stop}
		if from > 0 {
			r.From = old.Pages[from].Start
		}
		d, err := draw(r)
		if errors.Is(err, ErrNotResumable) && from > 0 {
			from--
			continue
		}
		if err == nil {
			err = ctx.Err()
		}
		if err != nil {
			return old, Reflowed{From: from}, err
		}
		out := Drawing{Fonts: d.Fonts, Pages: slices.Clone(old.Pages[:from])}
		if rows := d.rows(); rows != nil { // the pages before: of the rows as edited (the same before them)
			for i := range out.Pages {
				out.Pages[i].Start = out.Pages[i].Start.of(rows)
			}
		}
		out.Pages = append(out.Pages, d.Pages...)
		if len(out.Fonts) == 0 {
			out.Fonts = old.Fonts
		}
		got := Reflowed{From: from, Drawn: len(d.Pages)}
		if d.Next != nil {
			if j := old.pageIndex(d.Next.Page); j >= 0 {
				o := old.Pages[j].Start
				shift := 0
				if o.Line > 0 && d.Next.Line > 0 {
					shift = d.Next.Line - o.Line
				}
				for _, p := range old.Pages[j:] {
					p.Start = p.Start.rebased(o, *d.Next)
					if p.First > 0 {
						p.First += shift
					}
					if p.Last > 0 {
						p.Last += shift
					}
					out.Pages = append(out.Pages, p)
				}
				out.Pages[from+len(d.Pages)].Start = *d.Next
				got.Kept = true
			}
		}
		return out, got, nil
	}
}

// Redraw is old with the pages that lines first to last of the script as
// edited print on drawn again (from the first's start, else the page
// before's), as if the pages after them started where they did: what a
// view shows at once, while Reflow finds the rest. It gives the index of
// the first page drawn and how many were.
func Redraw(old Drawing, first, last int, draw func(*Resume) (Drawing, error)) (Drawing, int, int, error) {
	last = max(first, last)
	stop := func(s PageStart) bool { return s.Line == 0 || s.Line > last }
	from := old.PageOf(first)
	for {
		for from > 0 && !old.Pages[from].Start.Resumable() {
			from--
		}
		r := &Resume{Stop: stop}
		if from > 0 {
			r.From = old.Pages[from].Start
		}
		d, err := draw(r)
		if errors.Is(err, ErrNotResumable) && from > 0 {
			from--
			continue
		}
		if err != nil {
			return old, from, 0, err
		}
		out := Drawing{Fonts: old.Fonts, Pages: slices.Clone(old.Pages)}
		n := 0
		for _, p := range d.Pages {
			if i := from + n; i < len(out.Pages) {
				out.Pages[i] = p
				n++
			}
		}
		return out, from, n, nil
	}
}
