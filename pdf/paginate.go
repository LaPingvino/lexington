package pdf

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/LaPingvino/lexington/lekhan"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Pagination: where a page may break, as screenplay software has it
// (rules.Pagination). Before each row of the script's body the page
// ahead is measured, from where the PDF is now down to the first line
// that no longer fits, with every place on the way where a break may
// fall; the last of those is where the page breaks. When it is before
// the row, a new page starts; when it is in it, the row breaks there; when
// it is further on, the row prints as it is (the rows after it decide
// again). Without any, the page simply fills to its foot.
//
// Breaks may fall where the page fills up at a blank line, and before a
// line of text, except:
//   - after a scene heading, a section, a character's name or a
//     parenthetical (through the blank lines after it), and within them
//     (Keep); after one, the next paragraph keeps its first two lines
//     with it too;
//   - before a transition: it goes over with what is before it (Keep);
//   - within dual dialogue;
//   - after a paragraph's first line or before its last (Widows);
//   - within a speech, unless it breaks after a sentence with (MORE)
//     below it and NAME (CONT'D) above the rest (More; between words
//     only when nothing else on the page can break); without More, a
//     speech goes over whole when it can.
//
// Consecutive rows of one element (Fountain's lines of action without a
// blank line between) are one paragraph.

// Rows the pagination puts in the script: (MORE), and a page break that
// is not one of the script's own (no new page numbering).
const (
	typeMore  = "_more"
	typeBreak = "_break"
)

// The marks of a speech broken over pages (as pdfin reads them back).
const (
	moreText  = "(MORE)"
	contdText = " (CONT'D)"
)

// keepsWithNext are the elements that are never the last on a page.
func keepsWithNext(t lex.ElementType) bool {
	switch t {
	case lex.TypeScene, lex.TypeSection, lex.TypeSpeaker, lex.TypeParen:
		return true
	}
	return false
}

// unbreakable are the elements that are not broken over pages.
func unbreakable(t lex.ElementType) bool {
	return keepsWithNext(t) || t == lex.TypeTrans
}

// room is how many lines still fit on the page.
func (t *Tree) room() int {
	_, h := t.PDF.GetPageSize()
	_, margin := t.PDF.GetAutoPageBreak()
	return int(math.Floor((h - margin - t.PDF.GetY() + 1e-6) / lineHeight))
}

// atTop reports whether nothing is printed on the page yet.
func (t *Tree) atTop() bool {
	return t.PDF.GetY() <= t.textTop()+1e-6
}

// format is the format a row prints in and its text as printed (with the
// prefix and postfix, without a scene's number).
func (t *Tree) format(a, text string) (rules.Format, string) {
	if a == lex.TypeNote {
		a = rules.KeyPrintNotes.String()
	}
	f := t.Rules.Get(a)
	switch a {
	case "section":
		text = lex.SectionText(text)
	case "scene":
		text, _ = lex.SceneNumber(text)
	}
	return f, f.Prefix + text + f.Postfix
}

// height is how many lines a row prints.
func (t *Tree) height(a, text string) int {
	switch a {
	case lex.TypeEmpty, typeMore:
		return 1
	case lex.TypePicture:
		return t.pictureLines(text)
	}
	key := [2]string{a, text}
	if n, ok := t.heights[key]; ok {
		return n
	}
	f, printed := t.format(a, text)
	pageWidth, _ := t.PDF.GetPageSize()
	n := t.lines(f, printed, pageWidth-f.Left-f.Right)
	if t.heights == nil {
		t.heights = map[[2]string]int{}
	}
	t.heights[key] = n
	return n
}

// lines is how many lines text wraps to in a format and a width, as
// MultiCell wraps it (styled text, which Write wraps, as near as does).
func (t *Tree) lines(f rules.Format, text string, width float64) int {
	_, starts := t.wrap(f, text, width)
	return len(starts)
}

// wrap is the text as it is wrapped (styles taken out) and the byte each
// of its lines starts at, as lines counts them.
func (t *Tree) wrap(f rules.Format, text string, width float64) (string, []int) {
	fontName, spans := t.Fonts.spans(f.Font, strings.TrimRight(text, "\r\n"))
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Text)
	}
	s := strings.TrimRight(strings.ReplaceAll(b.String(), "\r", ""), "\n")
	if strings.TrimSpace(s) == "" {
		return s, []int{0}
	}
	t.PDF.SetFont(fontName, f.Style, f.Size)
	var chars []string // as MultiCell takes them: runes, or bytes in a standard font
	if fontName == "Helvetica" || fontName == "Times" {
		for i := 0; i < len(s); i++ {
			chars = append(chars, s[i:i+1])
		}
	} else {
		for _, r := range s {
			chars = append(chars, string(r))
		}
	}
	offsets := make([]int, len(chars)+1) // the byte each char starts at
	for k, c := range chars {
		offsets[k+1] = offsets[k] + len(c)
	}
	// a line printed piece by piece is measured by its cells, one that
	// MultiCell prints as gofpdf measures it
	core := fontName == "Helvetica" || fontName == "Times" || !needsBidi(text, t.lineRTL(text))
	starts := t.breaks(chars, width, core)
	for k, i := range starts {
		starts[k] = offsets[i]
	}
	return s, starts
}

// breaks are the chars each line starts at, of chars wrapped in width
// in the PDF's font, as MultiCell wraps them: at the last space that
// fits, or within a word that does not, and at each new line. Unless
// core (a standard font, or a line MultiCell prints), what takes no cell
// (zeroCell) has no width, Devanagari takes its cells by the Lekhan width,
// and a word breaks between its clusters.
func (t *Tree) breaks(chars []string, width float64, core bool) []int {
	pdf := t.PDF
	_, size := pdf.GetFontSize()
	wmax := int(math.Ceil((width - 2*pdf.GetCellMargin()) * 1000 / size))
	widths := map[string]int{}
	starts := []int{0}
	// Devanagari by its clusters (Lekhan width): a cluster's cells on its
	// runes, a cell as wide as a space
	var rs []rune
	var shares []int
	cell := pdf.GetStringSymbolWidth(" ")
	if !core {
		rs = make([]rune, len(chars))
		for k, c := range chars {
			rs[k], _ = utf8.DecodeRuneInString(c)
		}
		shares = lekhan.Cells(rs)
	}
	sep, i, j, l := -1, 0, 0, 0
	for i < len(chars) {
		c := chars[i]
		if c == "\n" {
			i++
			sep, j, l = -1, i, 0
			starts = append(starts, i)
			continue
		}
		if c == " " {
			sep = i
		}
		w, ok := widths[c]
		if !core && lekhan.Devanagari(rs[i]) {
			w, ok = shares[i]*cell, true
		}
		if !ok {
			w = pdf.GetStringSymbolWidth(c)
			if r, _ := utf8.DecodeRuneInString(c); !core && len(c) > 1 && zeroCell(r) {
				w = 0 // no cell (a joiner, a direction mark), as Accolade counts
			}
			widths[c] = w
		}
		l += w
		if l > wmax {
			if sep == -1 {
				// within a word: not inside a cluster (a conjunct, a letter
				// and its marks), unless it is the line's only one
				for k := i; !core && k > j+1; k-- {
					if lekhan.Boundary(rs, k) {
						i = k
						break
					}
				}
				if i == j {
					i++
				}
			} else {
				i = sep + 1
			}
			sep, j, l = -1, i, 0
			starts = append(starts, i)
			continue
		}
		i++
	}
	return starts
}

// dualHeight is how many lines the dual dialogue that starts at row i
// prints, and the row it ends at.
func (t *Tree) dualHeight(rows lex.Screenplay, i int) (int, int) {
	sc := t.sc()
	col, height := 0, [2]int{}
	j := i + 1
	for ; j < len(rows) && rows[j].Type != lex.TypeDualClose; j++ {
		r := rows[j]
		switch r.Type {
		case lex.TypeDualNext:
			col = 1
			continue
		case lex.TypeNewPage, lex.TypeDualOpen:
			return max(height[0], height[1]), j - 1
		}
		key := r.Type
		switch key {
		case lex.TypeSpeaker, lex.TypeDialog, lex.TypeParen:
			key = "dual" + key
		}
		f := t.Rules.Get(key)
		height[col] += t.lines(f, r.Contents, 2.5*sc-(f.Left-1.0*sc))
	}
	return max(height[0], height[1]), j
}

// A breakAt is a place where the page may break: before row (lines
// into it: 0), or in a speech at a sentence's end (at: the byte where the
// rest starts), with (MORE).
type breakAt struct {
	row, lines, at int
	more           bool
	speaker        string // the speech's, for its CONT'D
}

// sentenceEnd is where a sentence ends and the next starts.
var sentenceEnd = regexp.MustCompile(`[.!?…]+["'”’)\]]*\s+`)

// flows are the elements whose consecutive lines (rows without a blank
// line between) make one paragraph, which may break between its lines.
func flows(t lex.ElementType) bool {
	switch t {
	case lex.TypeAction, lex.TypeDialog, lex.TypeCenter, lex.TypeLyrics, lex.TypeNote, "allcaps":
		return true
	}
	return false
}

// continues reports whether row j goes on with the paragraph of the row
// before it.
func continues(rows lex.Screenplay, j int) bool {
	return j > 0 && flows(rows[j].Type) && rows[j-1].Type == rows[j].Type
}

// keptBefore reports whether the last element before row j that prints
// keeps with what follows it.
func keptBefore(rows lex.Screenplay, j int) bool {
	for k := j - 1; k >= 0; k-- {
		if rows[k].Type != lex.TypeEmpty {
			return keepsWithNext(rows[k].Type)
		}
	}
	return false
}

// transitionAfter reports whether the next element after row j that
// prints is a transition.
func transitionAfter(rows lex.Screenplay, j int) bool {
	for k := j + 1; k < len(rows); k++ {
		if rows[k].Type != lex.TypeEmpty {
			return rows[k].Type == lex.TypeTrans
		}
	}
	return false
}

// speechOf is the name of the speech row j is in, "" if none.
func speechOf(rows lex.Screenplay, j int) string {
	for k := j; k >= 0; k-- {
		switch rows[k].Type {
		case lex.TypeSpeaker:
			if k < j {
				return rows[k].Contents
			}
		case lex.TypeDialog, lex.TypeParen:
		default:
			return ""
		}
	}
	return ""
}

// paginate decides, before row i, where the page breaks (see above), and
// acts on it: a new page now, a row to break, or rows to put in for a
// speech broken over pages. It returns the rows as they are now.
//
//nolint:gocyclo,funlen // one walk down the page, case by case
func (t *Tree) paginate(rows lex.Screenplay, blocks []rowInfo, i int) (lex.Screenplay, []rowInfo) {
	p := t.pagination
	if p == (rules.Pagination{}) {
		return rows, blocks
	}
	room, top := t.room(), t.atTop()
	used := 0
	var best, fallback *breakAt
	consider := func(b breakAt, last bool) {
		if b.row == i && b.lines == 0 && b.at == 0 && top {
			return // a break before what is at a page's top already
		}
		if last {
			fallback = &b
		} else {
			best = &b
		}
	}
	// the paragraph the row is in: its lines before the row, all its
	// lines, whether it follows what keeps with the next
	paraOff, paraLen, afterKeep := 0, 0, false
	prevKeep := p.Keep && keptBefore(rows, i)
walk:
	for j := i; j < len(rows); j++ {
		r := rows[j]
		if r.Type == lex.TypeEmpty || r.Type == typeMore {
			if used++; used > room {
				if !prevKeep && r.Type == lex.TypeEmpty && !(p.Keep && transitionAfter(rows, j)) {
					return rows, blocks // the page breaks at a blank line: as it would
				}
				break
			}
			continue
		}
		switch r.Type {
		case lex.TypeNewPage, lex.TypeTitlePage, typeBreak, typeLock:
			return rows, blocks // it all fits before the page ends
		case lex.TypeDualOpen:
			h, end := t.dualHeight(rows, j)
			if !prevKeep {
				consider(breakAt{row: j}, false)
			}
			if used += h; used > room {
				break walk
			}
			j, prevKeep = end, false
			continue
		case lex.TypeDualNext, lex.TypeDualClose:
			continue
		}
		if t.shouldSkipElement(r, "") {
			continue
		}
		h := t.height(r.Type, r.Contents)
		if j == i || !continues(rows, j) {
			start := j
			for continues(rows, start) {
				start--
			}
			paraOff, paraLen = 0, 0
			for k := start; k < len(rows) && (k == start || continues(rows, k)); k++ {
				n := t.height(rows[k].Type, rows[k].Contents)
				if k < j {
					paraOff += n
				}
				paraLen += n
			}
			afterKeep = p.Keep && keptBefore(rows, start)
		}
		speech := speechOf(rows, j)
		inSpeech := speech != ""
		// may the page break before line o of the paragraph?
		least := 1
		if p.Widows || afterKeep {
			least = 2
		}
		within := func(o int) bool { return o >= least && (!p.Widows || paraLen-o >= 2) }
		// before the row
		switch {
		case paraOff > 0 && !within(paraOff):
		case p.Keep && (prevKeep || r.Type == lex.TypeTrans):
		case inSpeech && p.More:
			if used+1 <= room {
				consider(breakAt{row: j, more: true, speaker: speech}, false)
			}
		default:
			consider(breakAt{row: j}, inSpeech && p.Keep)
		}
		// in the row (also when it fits: a transition after it may need
		// some of it to go over with it)
		switch {
		case r.Type == lex.TypePicture: // a picture is never broken
		case p.Keep && unbreakable(r.Type):
		case inSpeech && p.More && r.Type == lex.TypeDialog:
			t.speechBreaks(r, j, used, room, speech, used+h > room, func(k, rest int) bool {
				return paraOff+k >= least && (!p.Widows || paraLen-paraOff-h+rest >= 2)
			}, consider)
		default:
			for m := 1; m < h && used+m <= room; m++ {
				if within(paraOff + m) {
					consider(breakAt{row: j, lines: m}, inSpeech && p.Keep)
				}
			}
		}
		if used += h; used > room {
			break
		}
		paraOff += h
		prevKeep = p.Keep && keepsWithNext(r.Type)
	}
	if used <= room {
		return rows, blocks // to the script's end
	}
	b := best
	if b == nil {
		b = fallback
	}
	if b == nil || b.row != i {
		return rows, blocks
	}
	return t.breakPage(rows, blocks, *b)
}

// speechBreaks considers the breaks in a speech's dialogue: after its
// sentences with (MORE) below them, where ok says the lines before the
// break (k) and after it in the row (rest) may break; else, as a last
// resort, between its words (when the row overflows the page).
func (t *Tree) speechBreaks(r lex.Line, j, used, room int, speech string, overflows bool, ok func(k, rest int) bool, consider func(breakAt, bool)) {
	text := r.Contents
	try := func(at int, last bool) bool {
		if strings.Count(text[:at], "*")%2 != 0 || strings.Count(text[:at], "_")%2 != 0 {
			return false // within emphasis
		}
		k := t.height(r.Type, strings.TrimSpace(text[:at]))
		if used+k+1 > room || !ok(k, t.height(r.Type, strings.TrimSpace(text[at:]))) {
			return false
		}
		consider(breakAt{row: j, at: at, more: true, speaker: speech}, last)
		return true
	}
	found := false
	for _, m := range sentenceEnd.FindAllStringIndex(text, -1) {
		if m[1] < len(text) && try(m[1], false) {
			found = true
		}
	}
	if found || !overflows {
		return
	}
	for at, c := range text {
		if c == ' ' && at > 0 {
			try(at+utf8.RuneLen(c), true)
		}
	}
}

// breakPage breaks the page at b, in row b.row, the row about to print.
func (t *Tree) breakPage(rows lex.Screenplay, blocks []rowInfo, b breakAt) (lex.Screenplay, []rowInfo) {
	i := b.row
	r := rows[i]
	switch {
	case b.more:
		// (MORE), the page's end, the name again, the rest of the speech
		name := contd(b.speaker)
		mark := func(typ, text string) lex.Line {
			return lex.Line{Type: typ, Contents: text, Color: r.Color, Block: r.Block}
		}
		add := []lex.Line{mark(typeMore, moreText), mark(typeBreak, ""), mark(lex.TypeSpeaker, name)}
		gen := rowInfo{block: blocks[i].block, origin: -1}
		addInfo := []rowInfo{gen, gen, gen}
		after, afterInfo := rows[i:], blocks[i:] // the rest: the row on
		if b.at > 0 {                            // the row split after a sentence
			first, rest := r, r
			first.Contents, rest.Contents = strings.TrimSpace(r.Contents[:b.at]), strings.TrimSpace(r.Contents[b.at:])
			if rtl := t.lineRTL(r.Contents); t.lineRTL(rest.Contents) != rtl { // the speech's direction kept
				mark := lex.LRM
				if rtl {
					mark = lex.RLM
				}
				rest.Contents = string(mark) + rest.Contents
			}
			add = append([]lex.Line{first}, add...)
			addInfo = append([]rowInfo{blocks[i]}, addInfo...)
			after = append(lex.Screenplay{rest}, rows[i+1:]...)
			restInfo := blocks[i]
			restInfo.word += len(strings.Fields(r.Contents[:b.at]))
			afterInfo = append([]rowInfo{restInfo}, blocks[i+1:]...)
		}
		out := append(append(slices.Clone(rows[:i]), add...), after...)
		return out, append(append(slices.Clone(blocks[:i]), addInfo...), afterInfo...)
	case b.lines == 0:
		t.newPage(!headingAhead(rows, i))
	default:
		// the row breaks after b.lines lines: the page's foot raised to
		// there until it breaks (Render's page break function puts it back)
		_, h := t.PDF.GetPageSize()
		_, margin := t.PDF.GetAutoPageBreak()
		t.foot = margin
		t.PDF.SetAutoPageBreak(true, h-(t.PDF.GetY()+float64(b.lines)*lineHeight+0.001))
	}
	return rows, blocks
}
