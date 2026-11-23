// Package office writes screenplays as word processor documents, DOCX
// (Word) and ODT (LibreOffice, OpenDocument), without pandoc: ready to be
// edited further, with every kind of element (scene heading, action,
// character, dialogue, ...) as a paragraph style of its own, so that
// restyling one restyles them all. Margins, alignment, styles, prefixes
// and postfixes come from the element rules (a preset), as in the PDF.
package office

import (
	"math"
	"strings"

	"github.com/LaPingvino/lexington/layout"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// Page is a paper size, in inches.
type Page struct {
	Name          string
	Width, Height float64
}

// Pages are the paper sizes: US Letter, A4 and A5 (half A4, a booklet
// when folded).
var Pages = map[string]Page{
	"letter": {"letter", 8.5, 11},
	"a4":     {"a4", 8.27, 11.69},
	"a5":     {"a5", 5.83, 8.27},
}

// GetPage is the paper size of that name (case-insensitive); Letter for
// an unknown or empty one.
func GetPage(name string) Page {
	if p, ok := Pages[strings.ToLower(name)]; ok {
		return p
	}
	return Pages["letter"]
}

// style is the paragraph style of an element.
type style struct {
	Key, ID, Name           string
	Left, Right             float64 // indents in inches from the text's edges (or a column's)
	Align                   byte
	Bold, Italic, Underline bool
	Font                    string
	Size                    float64
	KeepNext                bool // keep with the next paragraph
	KeepLines               bool // keep the paragraph's lines on one page
}

// block is a paragraph or a dual dialogue table.
type block struct {
	P    layout.Paragraph
	Dual [2][]layout.Paragraph
	// BreakBefore: a page break before; SpaceBefore in inches.
	BreakBefore bool
	SpaceBefore float64
	// KeepNext: keep with the next paragraph, beyond what the style
	// keeps (a blank line after a heading, the line before a transition)
	KeepNext bool
}

// doc is a screenplay prepared for a word processor.
type doc struct {
	page        Page
	scale       float64 // of horizontal positions: the page's width to Letter's
	vscale      float64
	margin      float64 // the text's left and right edges on the page, in inches
	rightMargin float64
	title       string
	titlePage   bool
	styles      []*style
	byKey       map[string]*style
	blocks      []block
	pagination  rules.Pagination
	pictures    map[int]*picture // the title page's, by their blocks
}

// Dual dialogue's columns on a Letter page, as in the PDF and the layout.
const (
	dualStart = 2.0
	dualWidth = 2.5
	dualGap   = 0.5
)

// The bar of a scene's color (layout.Colors.Bar): its width, and how far
// it is before the text, in inches.
const (
	barWidth = 0.04
	barGap   = 0.3
)

var styleNames = map[string]string{
	"scene":       "Scene Heading",
	"action":      "Action",
	"speaker":     "Character",
	"dialog":      "Dialogue",
	"paren":       "Parenthetical",
	"trans":       "Transition",
	"center":      "Centered",
	"lyrics":      "Lyrics",
	"section":     "Section",
	"printnotes":  "Note",
	"allcaps":     "All Caps",
	"title":       "Title Page",
	"meta":        "Title Page Details",
	"dualspeaker": "Dual Character",
	"dualdialog":  "Dual Dialogue",
	"dualparen":   "Dual Parenthetical",
}

func prepare(s lex.Screenplay, set rules.Set, page Page, tp *titlepage.Template, pictures map[string][]byte) *doc {
	tpl := titlepage.Standard()
	if tp != nil {
		tpl = *tp
	}
	if set == nil {
		set = rules.Default
	}
	d := &doc{page: page, scale: page.Width / layout.PageWidth, vscale: page.Height / 11,
		margin: layout.Margin(set), byKey: map[string]*style{}, pagination: set.Pagination()}
	d.rightMargin = math.Inf(1)
	for k, f := range set {
		if !strings.HasPrefix(k, "dual") && !rules.IsSwitch(k) && !f.Hide && f.Right < d.rightMargin {
			d.rightMargin = f.Right
		}
	}
	for _, l := range s {
		if l.Type == "Title" && d.title == "" {
			d.title = strings.TrimSpace(lex.Unstyled(lex.StripNotes(string(l.Contents))))
		}
		if l.Type == lex.TypeTitlePage {
			d.titlePage = true
		}
	}

	paras := layout.ParagraphsWith(s, set, tpl)
	brk, firstMeta := false, false
	for i := 0; i < len(paras); i++ {
		p := paras[i]
		switch {
		case p.PageBreak:
			brk = true
			continue
		case p.Type == "" && p.Block == "meta":
			firstMeta = true // the details go to the bottom of the title page
			continue
		}
		b := block{P: p, BreakBefore: brk}
		if p.Block == "meta" && p.Type != "" && firstMeta {
			b.SpaceBefore, firstMeta = 2, false
		}
		if p.Column != 0 {
			j := i
			for j < len(paras) && paras[j].Column != 0 && !paras[j].PageBreak {
				q := paras[j]
				if q.Type != "" || len(b.Dual[q.Column-1]) > 0 {
					b.Dual[q.Column-1] = append(b.Dual[q.Column-1], q)
				}
				if q.Type != "" {
					d.addStyle(set, q)
				}
				j++
			}
			// no trailing empty lines in a column: the blank line after the
			// block goes after the table
			blank := false
			for c := range b.Dual {
				for n := len(b.Dual[c]); n > 0 && b.Dual[c][n-1].Type == ""; n = len(b.Dual[c]) {
					b.Dual[c], blank = b.Dual[c][:n-1], true
				}
			}
			i = j - 1
			d.blocks = append(d.blocks, b)
			if blank {
				d.blocks = append(d.blocks, block{})
			}
			brk = false
			continue
		} else if p.Type != "" {
			d.addStyle(set, p)
		}
		d.blocks = append(d.blocks, b)
		brk = false
	}
	if d.pagination.Keep {
		d.keep()
	}
	d.titleSpaces(tpl)
	d.titlePictures(pictures)
	return d
}

// keepsWithNext are the elements kept with the paragraph after them
// (rules.Pagination.Keep): never the last on a page.
func keepsWithNext(key string) bool {
	switch key {
	case "scene", "section", "speaker", "paren", "dualspeaker", "dualparen":
		return true
	}
	return false
}

// keep keeps with the next paragraph what the styles cannot: the blank
// lines after a heading, a name or a parenthetical (or the heading keeps
// with the blank line only), the line before a transition and the blank
// lines between (a page does not start with one), the dialogue before a
// parenthetical in the same speech (a speech goes over whole: Word and
// Writer have no (MORE) and CONT'D).
func (d *doc) keep() {
	blank := func(k int) bool { b := d.blocks[k]; return b.P.Type == "" && b.P.Column == 0 }
	for k, b := range d.blocks {
		if b.P.Column != 0 {
			continue
		}
		switch {
		case keepsWithNext(b.P.Key):
			for n := k + 1; n < len(d.blocks) && blank(n); n++ {
				d.blocks[n].KeepNext = true
			}
		case b.P.Type == lex.TypeTrans:
			for n := k - 1; n >= 0; n-- {
				d.blocks[n].KeepNext = true
				if !blank(n) {
					break
				}
			}
		case b.P.Type == lex.TypeDialog && k+1 < len(d.blocks):
			if next := d.blocks[k+1].P; next.Column == 0 && (next.Type == lex.TypeParen || next.Type == lex.TypeDialog) {
				d.blocks[k].KeepNext = true
			}
		}
	}
}

// addStyle adds the style of p's element, from its first paragraph.
func (d *doc) addStyle(set rules.Set, p layout.Paragraph) {
	if d.byKey[p.Key] != nil {
		return
	}
	f := set.Get(p.Key)
	name := styleNames[p.Key]
	if name == "" {
		name = strings.ToUpper(p.Key[:1]) + p.Key[1:]
	}
	align := p.Align
	if p.TitleField != nil { // (a field's own alignment is its paragraph's)
		align = titleStyleAlign(set, p.Key)
	}
	st := &style{Key: p.Key, Name: name, ID: strings.ReplaceAll(name, " ", ""), Align: align,
		Bold: strings.Contains(f.Style, "b"), Italic: strings.Contains(f.Style, "i"),
		Underline: strings.Contains(f.Style, "u"), Font: fontName(f.Font), Size: f.Size,
		KeepNext:  d.pagination.Keep && keepsWithNext(p.Key),
		KeepLines: d.pagination.Keep && (keepsWithNext(p.Key) || p.Key == "trans" || p.Key == "dialog" || p.Key == "dualdialog")}
	if p.Column != 0 { // within its column
		start := d.columnStart(p.Column)
		st.Left = (p.Left - start) * d.scale
		st.Right = math.Max(0, (layout.PageWidth-p.Right-start-dualWidth)*-d.scale)
	} else {
		st.Left = (p.Left - d.margin) * d.scale
		st.Right = (p.Right - d.rightMargin) * d.scale
	}
	st.Left, st.Right = math.Max(0, st.Left), math.Max(0, st.Right)
	d.styles = append(d.styles, st)
	d.byKey[p.Key] = st
}

// columnStart is where a dual dialogue column starts on a Letter page.
func (d *doc) columnStart(column int) float64 {
	return dualStart + float64(column-1)*(dualWidth+dualGap)
}

// pageMargins are the page's left and right margins, in inches.
func (d *doc) pageMargins() (float64, float64) {
	return d.margin * d.scale, d.rightMargin * d.scale
}

// textWidth is the width between the page's margins, in inches.
func (d *doc) textWidth() float64 {
	l, r := d.pageMargins()
	return d.page.Width - l - r
}

// dualTable is the dual dialogue table's indent from the left margin and
// its column and gap widths, in inches.
func (d *doc) dualTable() (indent, col, gap float64) {
	return math.Max(0, (dualStart-d.margin)*d.scale), dualWidth * d.scale, dualGap * d.scale
}

func fontName(f string) string {
	switch f {
	case "", "CourierPrime", "Courier Prime", "Courier":
		return "Courier Prime"
	}
	return f
}

func size(s float64) float64 {
	if s == 0 {
		return 12
	}
	return s
}
