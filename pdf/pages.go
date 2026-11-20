package pdf

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Locked pages in print (lex.PageLock): each locked page after the first
// starts where its anchor is found, with a page break of its own (a
// typeLock row, put in before the render); text that does not fit before
// the next locked page goes on to A-pages, named between the two
// (lex.PageLabelBetween); a page whose anchor is gone gives its number to
// the page before the next one found ("12-13"). The labels are printed
// once the text is laid out, as "12A." where the page number goes (none on
// the first page, as before).
//
// Scenes' CONTINUEDs (PDFWriter.Continueds): a scene broken over pages has
// "(CONTINUED)" on the line under its last text on the page, ending where
// a transition ends, and "CONTINUED:" at the top of the next (at the
// heading's place, its number in both margins; "CONTINUED: (2)" on the
// scene's second page after its first, and on), as screenwriting software
// puts them in a shooting script (johnaugust.com/?p=3959: "most
// screenwriting software will offer to put CONTINUED: at the top left of
// the next page, next to the scene number"; the bottom's "(CONTINUED)" as
// in Charles Deemer's and SimplyScripts' format guides; the "(2)" is
// Final Draft's way as remembered, not seen in a source). They take
// lines: a page keeps its last line free for the bottom one always (so
// that a page's text does not depend on what follows it), and a page that
// continues a scene starts two lines down.

// The row a locked page starts with (its label in Contents).
const typeLock = "_lock"

// rowInfo is what is known of a row while rendering: its colour block,
// and where it came from in the screenplay (origin: its row in t.F, -1 for
// a row the writer put in; word: the word of that row it starts at, for
// the rest of a row broken in two).
type rowInfo struct {
	block, origin, word int
}

func rowInfos(rows []lex.Line) []rowInfo {
	blocks := lex.ColorBlocks(rows)
	out := make([]rowInfo, len(rows))
	for i := range rows {
		out[i] = rowInfo{block: blocks[i], origin: i}
	}
	return out
}

// pageState follows the pages as they are made.
type pageState struct {
	bodyFirst int                 // the first page of the body (after the title page)
	locks     map[int]string      // the pages a locked page began, by PDF page: its label
	starts    map[int]lex.PagePos // each body page's first text (its origin row and word)
	firstRev  map[int]string      // the revision of that text's row
	top       float64             // where the text starts on the page topPage
	topPage   int
	// CONTINUED
	inScene    bool   // a scene's text is being printed
	scene      string // its number
	contd      int    // its pages so far after its first
	continuing bool   // the page being made continues the scene
	marks      []contMark
	lastPage   int     // the last text printed: its page
	lastBottom float64 // and foot
	heading    bool    // a scene heading is being printed
}

// contMark is a CONTINUED to draw: at the top of a page or the foot.
type contMark struct {
	page  int
	y     float64
	top   bool
	scene string
	nth   int
}

func (t *Tree) state() *pageState {
	if t.pages == nil {
		t.pages = &pageState{locks: map[int]string{}, starts: map[int]lex.PagePos{}, firstRev: map[int]string{}}
	}
	return t.pages
}

// textTop is where text starts on the page the PDF is on.
func (t *Tree) textTop() float64 {
	_, top, _, _ := t.PDF.GetMargins()
	if s := t.pages; s != nil && s.topPage == t.PDF.PageNo() && s.top > 0 {
		return s.top
	}
	return top
}

// pageBegins is the header of each new page: a page that continues a
// scene starts lower, for its CONTINUED:.
func (t *Tree) pageBegins() {
	s := t.state()
	_, top, _, _ := t.PDF.GetMargins()
	y := top
	if t.continueds && s.continuing {
		s.contd++
		s.marks = append(s.marks, contMark{page: t.PDF.PageNo(), y: top, top: true, scene: s.scene, nth: s.contd})
		y += 2 * lineHeight
	}
	s.continuing = false
	s.top, s.topPage = y, t.PDF.PageNo()
	t.PDF.SetY(y)
}

// breaking notes that the page breaks here: whether the next page
// continues the scene, and the (CONTINUED) at this one's foot, at y (0:
// under the last text on the page).
func (t *Tree) breaking(cont bool, y float64) {
	s := t.state()
	cont = cont && s.inScene
	if t.continueds && cont {
		if y == 0 {
			y = t.PDF.GetY()
			if s.lastPage == t.PDF.PageNo() {
				y = s.lastBottom
			}
		}
		s.marks = append(s.marks, contMark{page: t.PDF.PageNo(), y: y})
	}
	s.continuing = cont
}

// newPage starts a new page here; cont: the scene goes on on it.
func (t *Tree) newPage(cont bool) {
	t.breaking(cont, 0)
	t.PDF.AddPage()
}

// headingAhead tells whether the next row from i that prints is a scene
// heading or a section (or the script ends): a page break before it
// continues no scene.
func headingAhead(rows lex.Screenplay, i int) bool {
	for k := i; k < len(rows); k++ {
		switch rows[k].Type {
		case lex.TypeEmpty, typeLock, typeBreak, lex.TypeNote, lex.TypeSynopsis:
			continue
		case lex.TypeScene, lex.TypeSection, lex.TypeNewPage:
			return true
		}
		return false
	}
	return true
}

// textPrinted follows the body's text as printed: its last line (for a
// (CONTINUED) under it), and the first text of each page (for a lock).
func (t *Tree) textPrinted(row lex.Line, info rowInfo, spans []span) {
	if len(spans) == 0 {
		return
	}
	s := t.state()
	last := spans[len(spans)-1]
	s.lastPage, s.lastBottom = last.page, last.bottom
	if info.origin < 0 || lex.PageText(row) == "" && row.Type != lex.TypeSection && row.Type != lex.TypeDualOpen {
		return
	}
	lines := 0
	for n, sp := range spans {
		if _, ok := s.starts[sp.page]; !ok && sp.page >= s.bodyFirst {
			word := info.word
			if n > 0 { // the row went on from the page before: after its lines there
				word += t.wordAtLine(row, lines)
			}
			s.starts[sp.page] = lex.PagePos{Row: info.origin, Word: word}
			s.firstRev[sp.page] = row.Revision
		}
		lines += int(math.Round((sp.bottom - sp.top) / lineHeight))
	}
}

// wordAtLine is the word (of the row's text) its line n starts with, as
// the row wraps.
func (t *Tree) wordAtLine(row lex.Line, n int) int {
	f, printed := t.format(row.Type, row.Contents)
	pageWidth, _ := t.PDF.GetPageSize()
	text, starts := t.wrap(f, printed, pageWidth-f.Left-f.Right)
	if n >= len(starts) {
		return len(strings.Fields(lex.PageText(row)))
	}
	w := len(strings.Fields(text[:starts[n]])) - len(strings.Fields(f.Prefix))
	return max(0, w)
}

// fieldOffset is the byte where the n-th field of s starts (len(s) if
// none).
func fieldOffset(s string, n int) int {
	k, in := 0, false
	for i, r := range s {
		space := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		if !space && !in {
			if k == n {
				return i
			}
			k++
		}
		in = !space
	}
	return len(s)
}

// lockRows puts in the locked pages' breaks: before each page's first
// row, or within it (a speech with (MORE) and its CONT'D). A row that
// does not break (a heading, a dual dialogue) breaks before it; a break
// before a row goes before the blank lines and sections in front of it.
// It notes the pages found (found[i] for the lock's page i).
func (t *Tree) lockRows(rows lex.Screenplay, info []rowInfo) (lex.Screenplay, []rowInfo, []bool) {
	pos := lex.LocatePages(rows, t.lock)
	found := make([]bool, len(pos))
	if len(found) > 0 {
		found[0] = true
	}
	for i := len(pos) - 1; i >= 1; i-- {
		at := pos[i]
		if at == lex.Missing {
			continue
		}
		found[i] = true
		label := t.lock.Pages[i].Label
		r, w := at.Row, at.Word
		// within a dual dialogue: before it
		for k := r; k >= 0; k-- {
			if rows[k].Type == lex.TypeDualClose && k < r {
				break
			}
			if rows[k].Type == lex.TypeDualOpen {
				r, w = k, 0
				break
			}
		}
		row := rows[r]
		if w > 0 && !(flows(row.Type) || row.Type == lex.TypeDialog) {
			w = 0
		}
		speech := ""
		if row.Type == lex.TypeDialog || row.Type == lex.TypeParen {
			speech = speechOf(rows, r)
		}
		at0 := 0
		if w > 0 {
			at0 = fieldOffset(row.Contents, w)
			if c := row.Contents[:at0]; strings.Count(c, "*")%2 != 0 || strings.Count(c, "_")%2 != 0 {
				at0 = 0 // within emphasis: the row whole
			}
			if at0 >= len(row.Contents) {
				at0 = 0
			}
		}
		mark := func(typ, text string) lex.Line {
			return lex.Line{Type: typ, Contents: text, Color: row.Color, Block: row.Block}
		}
		gen := rowInfo{block: info[r].block, origin: -1}
		lock := mark(typeLock, label)
		var add []lex.Line
		var addInfo []rowInfo
		if speech != "" {
			add = []lex.Line{mark(typeMore, moreText), lock, mark(lex.TypeSpeaker, contd(speech))}
			addInfo = []rowInfo{gen, gen, gen}
		} else {
			add, addInfo = []lex.Line{lock}, []rowInfo{gen}
		}
		if at0 == 0 {
			k := r
			if speech == "" {
				for k > 0 && (rows[k-1].Type == lex.TypeEmpty || rows[k-1].Type == lex.TypeSection ||
					rows[k-1].Type == lex.TypeSynopsis || rows[k-1].Type == lex.TypeNote) {
					k--
				}
			}
			rows = slices.Insert(slices.Clone(rows), k, add...)
			info = slices.Insert(slices.Clone(info), k, addInfo...)
			continue
		}
		first, rest := row, row
		first.Contents, rest.Contents = strings.TrimSpace(row.Contents[:at0]), strings.TrimSpace(row.Contents[at0:])
		restInfo := info[r]
		restInfo.word += w
		out := append(append(append(slices.Clone(rows[:r]), first), add...), rest)
		rows = append(out, rows[r+1:]...)
		outInfo := append(append(append(slices.Clone(info[:r]), info[r]), addInfo...), restInfo)
		info = append(outInfo, info[r+1:]...)
	}
	return rows, info, found
}

// contd is a speaker's name with (CONT'D).
func contd(speaker string) string {
	name := strings.TrimSpace(lex.StripNotes(speaker))
	if !strings.HasSuffix(strings.ToUpper(name), contdText[1:]) {
		name += contdText
	}
	return name
}

// labels are the body's pages' numbers as printed, by PDF page, with the
// locked page each page's text belongs to (its span: the index of the
// lock's page; -1 without a lock).
func (t *Tree) labels() (map[int]string, map[int]int) {
	s := t.state()
	n := t.PDF.PageCount()
	out, spans := map[int]string{}, map[int]int{}
	if t.lock == nil || len(t.lock.Pages) == 0 {
		for p := s.bodyFirst; p <= n; p++ {
			out[p], spans[p] = strconv.Itoa(p-s.bodyFirst+1), -1
		}
		return out, spans
	}
	pages := t.lock.Pages
	index, taken := map[string]int{}, map[string]bool{}
	for i, lp := range pages {
		index[lp.Label] = i
		taken[lp.Label] = true
	}
	nextFound := func(i int) int { // the next page found after the lock's page i
		for j := i + 1; j < len(pages); j++ {
			if t.found[j] {
				return j
			}
		}
		return len(pages)
	}
	cur, prev := 0, ""
	for p := s.bodyFirst; p <= n; p++ {
		label := ""
		if p == s.bodyFirst {
			label = pages[0].Label
		} else if l, ok := s.locks[p]; ok {
			label, cur = l, index[l]
		} else {
			next := ""
			if j := nextFound(cur); j < len(pages) {
				next = pages[j].Label
			}
			label = lex.PageLabelBetween(prev, next, taken)
			taken[label] = true
		}
		prev = label
		// the last page before the next found: the missing ones' numbers
		if _, ok := s.locks[p+1]; ok || p == n {
			if j := nextFound(cur); j > cur+1 {
				label += "-" + pages[j-1].Label
			}
		}
		out[p], spans[p] = label, cur
	}
	return out, spans
}

// drawPages prints the pages' numbers and the CONTINUEDs, once the text
// is laid out.
func (t *Tree) drawPages() {
	pdf := t.PDF
	s := t.state()
	now := pdf.PageNo()
	defer pdf.SetPage(now)
	left, top, right, _ := pdf.GetMargins()
	defer pdf.SetMargins(left, top, right)
	pdf.SetMargins(0, top, 0)
	auto, foot := pdf.GetAutoPageBreak()
	pdf.SetAutoPageBreak(false, 0)
	defer pdf.SetAutoPageBreak(auto, foot)
	cell := pdf.GetCellMargin()
	defer pdf.SetCellMargin(cell)
	pdf.SetCellMargin(0)
	pdf.SetTextColor(0, 0, 0)
	pageWidth, _ := pdf.GetPageSize()
	if s.bodyFirst > 0 {
		labels, _ := t.labels()
		for p := s.bodyFirst + 1; p <= pdf.PageCount(); p++ {
			pdf.SetPage(p)
			pageFont(pdf, "CourierPrime", 12)
			pdf.SetXY(pageWidth-1, 0.5)
			pdf.Cell(0, 0, labels[p]+".")
		}
	}
	scene := t.Rules.Get(lex.TypeScene)
	trans := t.Rules.Get(lex.TypeTrans)
	for _, m := range s.marks {
		pdf.SetPage(m.page)
		fontName, _ := t.Fonts.font(scene.Font, "CONTINUED")
		pageFont(pdf, fontName, scene.Size)
		if m.top {
			text := "CONTINUED:"
			if m.nth > 1 {
				text += " (" + strconv.Itoa(m.nth) + ")"
			}
			pdf.SetXY(scene.Left, m.y)
			pdf.CellFormat(pageWidth-scene.Left-scene.Right, lineHeight, text, "", 0, "L", false, 0, "")
			if m.scene != "" {
				_, n := t.Fonts.font(scene.Font, m.scene)
				pdf.SetXY(scene.Left-0.85, m.y)
				pdf.CellFormat(0.6, lineHeight, n, "", 0, "R", false, 0, "")
				pdf.SetXY(pageWidth-scene.Right+0.25, m.y)
				pdf.CellFormat(0.6, lineHeight, n, "", 0, "L", false, 0, "")
			}
			continue
		}
		w := pdf.GetStringWidth("(CONTINUED)")
		pdf.SetXY(pageWidth-trans.Right-w, m.y)
		pdf.CellFormat(w, lineHeight, "(CONTINUED)", "", 0, "L", false, 0, "")
	}
}

// Page is a page of the script as printed.
type Page struct {
	// Label is its number: 12, 12A, 12-13 (a locked page's text gone).
	Label string
	// Revision is the latest set on it, "" if it is not a revised page.
	Revision string
	// New is a page the lock has not got (an A-page, or one after the
	// last locked page).
	New bool
}

// Pages are the script's pages after its title page, as printed, with
// their numbers and revisions.
func (p *PDFWriter) Pages(screenplay lex.Screenplay) []Page {
	t := p.tree(screenplay)
	if t.revs == nil {
		t.revs = &lex.Revisions{}
	}
	t.Render()
	labels, _ := t.labels()
	revised := t.revisedPages()
	var out []Page
	for n := t.state().bodyFirst; n <= t.PDF.PageCount(); n++ {
		_, locked := t.pages.locks[n]
		out = append(out, Page{Label: labels[n], Revision: revised[n],
			New: t.lock != nil && !locked && n != t.pages.bodyFirst})
	}
	return out
}

// LockPages are the pages of a screenplay as printed now, locked: each
// page's number and where it starts. With a lock already, its pages keep
// their numbers and the A-pages theirs (Final Draft's Lock A Pages).
func (p *PDFWriter) LockPages(screenplay lex.Screenplay) lex.PageLock {
	t := p.tree(screenplay)
	t.Render()
	labels, _ := t.labels()
	s := t.state()
	var starts []lex.PagePos
	var names []string
	for n := s.bodyFirst; n <= t.PDF.PageCount(); n++ {
		at, ok := s.starts[n]
		if !ok {
			continue // a page with no text of the script's (a forced break's)
		}
		starts = append(starts, at)
		names = append(names, strings.SplitN(labels[n], "-", 2)[0])
	}
	l := lex.PageLock{Paper: strings.ToLower(t.paper), Continueds: t.continueds}
	for i, a := range lex.PageAnchors(t.F, starts) {
		a.Label = names[i]
		l.Pages = append(l.Pages, a)
	}
	return l
}

// bodyPrinted follows a row of the body just printed: the scene it is in,
// its text (textPrinted).
func (t *Tree) bodyPrinted(row lex.Line, info rowInfo, spans []span) {
	s := t.state()
	switch row.Type {
	case lex.TypeScene:
		_, s.scene = lex.SceneNumber(row.Contents)
		s.inScene, s.contd = true, 0
	case lex.TypeSection:
		s.inScene = false
	}
	t.textPrinted(row, info, spans)
}
