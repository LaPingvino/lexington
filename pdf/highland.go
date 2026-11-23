package pdf

import (
	"cmp"
	"strconv"

	"github.com/LaPingvino/lexington/lex"
)

// Highland's page directives in print (lex/highland.go): the header and
// footer, the page numbers {{PAGENUMBER}} sets, the section %# names, and
// the page numbers in a {{TOC}}. As Highland 2 prints them
// (highwater-too's screenplay.ts, MIT, has it from Highland's PDFs): a
// page has the header in effect at its first text and the footer in
// effect at its end; with a header, a page has no "2." of its own (the
// first page too has the header); a footer takes the page's last line;
// {{PAGENUMBER: n}} makes the page it is on n, and those after count on
// from it (not under a lock, whose pages keep their numbers). The header
// is where the page number is (half an inch down), the footer half an
// inch from the foot; the left segment starts at the inch's margin, the
// centre is centred, the right ends an inch from the right edge.
//
// A script with these is drawn whole (not again from a page's start,
// Resume): a page's numbers depend on all the pages.

// highland is the running state of the page directives, as the rows are
// printed.
type highland struct {
	on             bool // the script has page directives
	header, footer *lex.Directive
	section        string
	headerOf       map[int]*lex.Directive // by PDF page
	footerOf       map[int]*lex.Directive
	sectionOf      map[int]string
	firstText      map[int]bool   // the page has had its first text
	renumber       map[int]int    // pages {{PAGENUMBER}} numbers
	sections       int            // the sections so far
	waiting        []int          // sections whose page is that of the next text
	sectionPage    map[int]int    // each section's PDF page, by its number from 1
	toc            map[int]string // the TOC's pages, by section (the first drawing's)
}

func (t *Tree) hl() *highland {
	if t.highland == nil {
		t.highland = &highland{headerOf: map[int]*lex.Directive{}, footerOf: map[int]*lex.Directive{},
			sectionOf: map[int]string{}, firstText: map[int]bool{}, renumber: map[int]int{}, sectionPage: map[int]int{}}
	}
	return t.highland
}

// startHighland readies the page directives before the rows are drawn:
// a footer's line kept free.
func (t *Tree) startHighland() {
	h := t.hl()
	if !h.on {
		return
	}
	for _, l := range t.F {
		for _, d := range lex.LineDirectives(l) {
			if d.Kind == lex.DirectiveFooter && !d.Off {
				auto, margin := t.PDF.GetAutoPageBreak()
				t.PDF.SetAutoPageBreak(auto, margin+lineHeight)
				return
			}
		}
	}
}

// applyDirectives does a row's page directives, as it is about to be
// printed (from its start: info's word 0).
func (t *Tree) applyDirectives(row lex.Line, info rowInfo) {
	h := t.hl()
	if !h.on {
		return
	}
	if row.Type == lex.TypeSection {
		h.section = lex.SectionText(row.Contents)
		h.sections++
		h.waiting = append(h.waiting, h.sections)
	}
	if row.Directives == "" || info.word != 0 || info.origin < 0 {
		return
	}
	for _, d := range lex.LineDirectives(row) {
		switch d.Kind {
		case lex.DirectiveHeader:
			h.header = &d
		case lex.DirectiveFooter:
			h.footer = &d
		case lex.DirectivePageNumber:
			if d.HasNumber {
				h.renumber[t.PDF.PageNo()] = d.Number
			}
		}
	}
}

// highlandPage notes a page begun: the page before has the footer in
// effect now; this one the header and section, until its first text.
func (t *Tree) highlandPage() {
	h := t.hl()
	if !h.on {
		return
	}
	p := t.PDF.PageNo()
	if p > 1 {
		h.footerOf[p-1] = h.footer
	}
	h.headerOf[p], h.sectionOf[p] = h.header, h.section
}

// highlandText notes body text printed on the pages of spans.
func (t *Tree) highlandText(spans []span) {
	h := t.hl()
	if !h.on || len(spans) == 0 {
		return
	}
	for _, sp := range spans {
		if !h.firstText[sp.page] {
			h.firstText[sp.page] = true
			h.headerOf[sp.page], h.sectionOf[sp.page] = h.header, h.section
		}
	}
	for _, n := range h.waiting {
		h.sectionPage[n] = spans[0].page
	}
	h.waiting = nil
}

// endHighland notes the last page's footer.
func (t *Tree) endHighland() {
	if h := t.hl(); h.on {
		h.footerOf[t.PDF.PageCount()] = h.footer
	}
}

// numbered are the body's pages' numbers without a lock: from 1, or what
// {{PAGENUMBER}} makes them.
func (t *Tree) numbered(first, last int) map[int]string {
	out := map[int]string{}
	n := 1
	for p := first; p <= last; p++ {
		if v, ok := t.hl().renumber[p]; ok {
			n = v
		}
		out[p] = strconv.Itoa(n)
		n++
	}
	return out
}

// sectionLabels are the pages the sections are printed on, as numbered,
// by section (for a TOC: drawn once to know them).
func (t *Tree) sectionLabels() map[int]string {
	labels, _ := t.labels()
	out := map[int]string{}
	for n, p := range t.hl().sectionPage {
		out[n] = labels[p]
	}
	return out
}

// hasTOC tells whether a printable screenplay has a table of contents.
func hasTOC(s lex.Screenplay) bool {
	for _, l := range s {
		if l.TOC != 0 {
			return true
		}
	}
	return false
}

// tocPage prints a TOC line's page at its right, on the line just printed
// (spans).
func (t *Tree) tocPage(row lex.Line, spans []span) {
	if row.TOC == 0 || len(spans) == 0 {
		return
	}
	label := t.hl().toc[row.TOC]
	if label == "" {
		return
	}
	pdf := t.PDF
	last := spans[len(spans)-1]
	now := pdf.PageNo()
	x, y := pdf.GetX(), pdf.GetY()
	f := t.Rules.Get(lex.TypeAction)
	pageWidth, _ := pdf.GetPageSize()
	fontName, _ := t.Fonts.font(f.Font, label)
	pageFont(pdf, fontName, cmp.Or(f.Size, 12))
	w := pdf.GetStringWidth(label)
	pdf.SetPage(last.page)
	pdf.SetXY(pageWidth-f.Right-w, last.bottom-lineHeight)
	pdf.CellFormat(w, lineHeight, label, "", 0, "L", false, 0, "")
	pdf.SetPage(now)
	pdf.SetXY(x, y)
}

// drawPageTexts draws the header and footer of the body's page p (its
// number label, the last page's last), or the page's number as before;
// it reports whether it drew the header.
func (t *Tree) drawPageTexts(p int, label, last string, pageWidth, pageHeight float64) bool {
	h := t.hl()
	if !h.on {
		return false
	}
	pdf := t.PDF
	act := t.Rules.Get(lex.TypeAction)
	size := act.Size
	if size == 0 {
		size = 12
	}
	draw := func(d *lex.Directive, y float64) {
		if d == nil || d.Off {
			return
		}
		section := h.sectionOf[p]
		for i, seg := range [][]lex.PagePart{d.Left, d.Center, d.Right} {
			text := lex.FillPage(seg, label, last, section)
			if text == "" {
				continue
			}
			fontName, text := t.Fonts.font(act.Font, text)
			pageFont(pdf, fontName, size)
			core := fontName == "Helvetica" || fontName == "Times"
			w := cellWidth(pdf, text, core)
			x := 1.0
			switch i {
			case 1:
				x = (pageWidth - w) / 2
			case 2:
				x = pageWidth - 1 - w
			}
			pdf.SetXY(x, y)
			var sh *fontShaper
			if !core {
				sh = t.Fonts.shaper(fontName, "")
			}
			shapedCell(pdf, sh, fontName, "", size, w, 0, text, "L")
		}
	}
	draw(h.footerOf[p], pageHeight-0.5)
	if hd := h.headerOf[p]; hd != nil {
		draw(hd, 0.5)
		return true
	}
	return false
}
