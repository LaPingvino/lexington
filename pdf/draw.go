package pdf

import (
	"image/color"
	"io"
	"math"
	"strings"

	"github.com/phpdave11/gofpdf"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/encoding/charmap"
)

// Drawer is what Lexington's PDF writers draw with: the gofpdf calls
// they make. *gofpdf.Fpdf is one; a Recorder is another, which draws
// through gofpdf and keeps what it drew (a Drawing), so that a program
// can show the pages without reading the PDF.
type Drawer interface {
	AddPage()
	AddUTF8FontFromBytes(family, style string, b []byte)
	SetFont(family, style string, size float64)
	GetFontSize() (pt, unit float64)
	GetStringWidth(s string) float64
	GetStringSymbolWidth(s string) int
	SplitText(text string, w float64) []string
	UnicodeTranslatorFromDescriptor(cp string) func(string) string

	Cell(w, h float64, text string)
	CellFormat(w, h float64, text, border string, ln int, align string, fill bool, link int, linkURL string)
	MultiCell(w, h float64, text, border, align string, fill bool)
	Write(h float64, text string)
	Ln(h float64)
	Rect(x, y, w, h float64, style string)

	SetTextColor(r, g, b int)
	SetFillColor(r, g, b int)
	SetDrawColor(r, g, b int)
	SetLineWidth(w float64)
	SetAlpha(alpha float64, blend string)

	GetX() float64
	GetY() float64
	SetX(x float64)
	SetY(y float64)
	SetXY(x, y float64)
	GetMargins() (left, top, right, bottom float64)
	SetMargins(left, top, right float64)
	SetLeftMargin(m float64)
	SetRightMargin(m float64)
	GetCellMargin() float64
	SetCellMargin(m float64)
	GetAutoPageBreak() (auto bool, margin float64)
	SetAutoPageBreak(auto bool, margin float64)
	GetPageSize() (w, h float64)
	PageNo() int
	PageCount() int
	SetPage(n int)

	SetHeaderFuncMode(f func(), home bool)
	SetAcceptPageBreakFunc(f func() bool)
	Bookmark(text string, level int, y float64)
	SetTitle(title string, utf8 bool)
	Output(w io.Writer) error
}

var (
	_ Drawer = (*gofpdf.Fpdf)(nil)
	_ Drawer = (*Recorder)(nil)
)

// Drawing is a document's pages as a Recorder saw them drawn. Lengths
// are in points, from the page's top left corner.
type Drawing struct {
	Pages []DrawnPage
	// Fonts are the fonts' files by their key (Text.Font): the family in
	// lower case and the style, "B", "I" or "BI" ("courierprimeBI"). The
	// standard fonts (courier, helvetica, times) have none.
	Fonts map[string][]byte
}

// DrawnPage is a page: its size and what is drawn on it, in the order
// drawn (a band multiplied over text comes after the text).
type DrawnPage struct {
	// Number is the page's number in the PDF made (from 1); a PDF of
	// revised pages only leaves the others out of Pages.
	Number        int
	Width, Height float64
	Marks         []Mark
}

// A Mark is a Text or a Box.
type Mark interface{ mark() }

// Paint is how a mark is painted: its opacity and blend mode ("Normal",
// "Multiply"), as SetAlpha set them.
type Paint struct {
	Alpha float64
	Blend string
}

// Text is a run of text as placed: X and Y the start of its baseline;
// the text in the order it reads on the page (a right-to-left line is
// in visual order already) and the characters printed (the Hebrew
// presentation forms), in UTF-8 also for a standard font.
type Text struct {
	X, Y  float64
	Font  string  // the key of its font in the Drawing's Fonts
	Size  float64 // in points
	Width float64 // as gofpdf measured it
	Color color.RGBA
	Paint
	Text string
}

// Box is a rectangle filled, stroked (its outline LineWidth wide, on the
// edges) or both; a cell's border and an underline are boxes too.
type Box struct {
	X, Y, W, H  float64
	Fill        bool
	FillColor   color.RGBA
	Stroke      bool
	StrokeColor color.RGBA
	LineWidth   float64
	Paint
}

func (Text) mark() {}
func (Box) mark()  {}

// Recorder draws through gofpdf and records what is drawn, page by page:
// gofpdf measures, breaks and places everything as it would alone (its
// PDF is the same, byte for byte), and the Recorder notes where text and
// boxes landed. MultiCell and Write, which place their lines inside
// gofpdf, it runs itself, line by line, as gofpdf does.
type Recorder struct {
	*gofpdf.Fpdf
	family, style      string // the font, as gofpdf keys it
	underline          bool
	ws                 float64 // word spacing of a justified line, a standard font's
	utf8               map[string]bool
	drawing            Drawing
	upos, uthick       map[string]float64 // underline position and thickness, in 1/1000 em
	cp1252             func(string) string
	defaultW, defaultH float64
}

// NewRecorder records what is drawn on f from now on: make it before
// adding fonts or pages.
func NewRecorder(f *gofpdf.Fpdf) *Recorder {
	w, h := f.GetPageSize()
	k := f.GetConversionRatio()
	return &Recorder{Fpdf: f, utf8: map[string]bool{}, upos: map[string]float64{}, uthick: map[string]float64{},
		drawing: Drawing{Fonts: map[string][]byte{}}, defaultW: w * k, defaultH: h * k}
}

// Drawing is what was drawn, a page for each of the PDF's.
func (r *Recorder) Drawing() Drawing {
	r.page(r.PageCount())
	d := r.drawing
	d.Pages = d.Pages[:r.PageCount()]
	return d
}

// fontKey is gofpdf's key for a font.
func fontKey(family, style string) string {
	style = strings.ToUpper(style)
	if style == "IB" {
		style = "BI"
	}
	return strings.ToLower(strings.ReplaceAll(family, " ", "#20")) + style
}

func (r *Recorder) AddUTF8FontFromBytes(family, style string, b []byte) {
	r.Fpdf.AddUTF8FontFromBytes(family, style, b)
	key := fontKey(family, style)
	if _, ok := r.drawing.Fonts[key]; ok || r.Err() {
		return
	}
	r.drawing.Fonts[key] = b
	r.utf8[strings.ToLower(strings.ReplaceAll(family, " ", "#20"))] = true
	if f, err := sfnt.Parse(b); err == nil {
		if p := f.PostTable(); p != nil && f.UnitsPerEm() != 0 {
			em := float64(f.UnitsPerEm())
			r.upos[key] = math.Round(float64(p.UnderlinePosition) * 1000 / em)
			r.uthick[key] = math.Round(float64(p.UnderlineThickness) * 1000 / em)
		}
	}
}

func (r *Recorder) SetFont(family, style string, size float64) {
	r.Fpdf.SetFont(family, style, size)
	if r.Err() {
		return
	}
	if family != "" {
		r.family = strings.ToLower(strings.ReplaceAll(family, " ", "#20"))
		if r.family == "arial" {
			r.family = "helvetica"
		}
	}
	style = strings.ToUpper(style)
	r.underline = strings.Contains(style, "U")
	style = strings.NewReplacer("U", "", "S", "").Replace(style)
	if style == "IB" {
		style = "BI"
	}
	r.style = style
}

// isUTF8 tells whether the font is one added from a file (not a
// standard one, whose text is in code page 1252).
func (r *Recorder) isUTF8() bool { return r.utf8[r.family] }

func (r *Recorder) key() string {
	if r.family == "zapfdingbats" || r.family == "symbol" {
		return r.family
	}
	return r.family + r.style
}

// page is the recorded page n (from 1), and those before it.
func (r *Recorder) page(n int) *DrawnPage {
	for len(r.drawing.Pages) < n {
		r.drawing.Pages = append(r.drawing.Pages, DrawnPage{Number: len(r.drawing.Pages) + 1,
			Width: r.defaultW, Height: r.defaultH})
	}
	if n < 1 {
		return nil
	}
	p := &r.drawing.Pages[n-1]
	if len(p.Marks) == 0 {
		w, h := r.GetPageSize()
		k := r.GetConversionRatio()
		p.Width, p.Height = w*k, h*k
	}
	return p
}

func (r *Recorder) add(m Mark) {
	if p := r.page(r.PageNo()); p != nil {
		p.Marks = append(p.Marks, m)
	}
}

func rgba(r, g, b int) color.RGBA { return color.RGBA{uint8(r), uint8(g), uint8(b), 255} }

func (r *Recorder) paint() Paint {
	a, blend := r.GetAlpha()
	return Paint{Alpha: a, Blend: blend}
}

// box records a rectangle in the document's unit; style as Rect's.
func (r *Recorder) box(x, y, w, h float64, style string) {
	k := r.GetConversionRatio()
	style = strings.ToUpper(style)
	b := Box{X: x * k, Y: y * k, W: w * k, H: h * k, Paint: r.paint()}
	switch style {
	case "F":
		b.Fill = true
	case "FD", "DF", "B":
		b.Fill, b.Stroke = true, true
	default:
		b.Stroke = true
	}
	b.FillColor = rgba(r.GetFillColor())
	b.StrokeColor = rgba(r.GetDrawColor())
	b.LineWidth = r.GetLineWidth() * k
	r.add(b)
}

// line records a stroked line as the box it covers.
func (r *Recorder) line(x1, y1, x2, y2 float64) {
	k := r.GetConversionRatio()
	lw := r.GetLineWidth()
	x, y, w, h := x1-lw/2, y1, lw, y2-y1
	if y1 == y2 {
		x, y, w, h = x1, y1-lw/2, x2-x1, lw
	}
	r.add(Box{X: x * k, Y: y * k, W: w * k, H: h * k, Fill: true, FillColor: rgba(r.GetDrawColor()), Paint: r.paint()})
}

func (r *Recorder) Rect(x, y, w, h float64, style string) {
	r.Fpdf.Rect(x, y, w, h, style)
	if !r.Err() {
		r.box(x, y, w, h, style)
	}
}

func (r *Recorder) Cell(w, h float64, text string) {
	r.CellFormat(w, h, text, "", 0, "L", false, 0, "")
}

// CellFormat draws the cell through gofpdf, and records it where gofpdf
// put it: after a page break, on the new page, below what its header
// printed.
func (r *Recorder) CellFormat(w, h float64, text, border string, ln int, align string, fill bool, link int, linkURL string) {
	x := r.GetX()
	r.Fpdf.CellFormat(w, h, text, border, ln, align, fill, link, linkURL)
	if r.Err() {
		return
	}
	y := r.GetY()
	if ln > 0 {
		y -= h
	}
	if w == 0 {
		pw, _ := r.GetPageSize()
		_, _, right, _ := r.GetMargins()
		w = pw - right - x
	}
	border = strings.ToUpper(border)
	if fill || border == "1" {
		style := "D"
		if fill {
			style = "F"
			if border == "1" {
				style = "FD"
			}
		}
		r.box(x, y, w, h, style)
	}
	if border != "" && border != "1" {
		if strings.Contains(border, "L") {
			r.line(x, y, x, y+h)
		}
		if strings.Contains(border, "T") {
			r.line(x, y, x+w, y)
		}
		if strings.Contains(border, "R") {
			r.line(x+w, y, x+w, y+h)
		}
		if strings.Contains(border, "B") {
			r.line(x, y+h, x+w, y+h)
		}
	}
	if text == "" {
		return
	}
	cm := r.GetCellMargin()
	_, size := r.GetFontSize()
	sw := r.GetStringWidth(text)
	var dx, dy float64
	switch {
	case strings.Contains(align, "R"):
		dx = w - cm - sw
	case strings.Contains(align, "C"):
		dx = (w - sw) / 2
	default:
		dx = cm
	}
	switch {
	case strings.Contains(align, "T"):
		dy = (size - h) / 2
	case strings.Contains(align, "B"):
		dy = (h - size) / 2
	case strings.Contains(align, "A"):
		d := r.GetFontDesc("", "")
		descent := -0.19 * size
		if d.Descent != 0 {
			descent = float64(d.Descent) * size / float64(d.Ascent-d.Descent)
		}
		dy = (h-size)/2 - descent
	}
	base := y + dy + .5*h + .3*size
	k := r.GetConversionRatio()
	pt, _ := r.GetFontSize()
	t := Text{Font: r.key(), Size: pt, Color: rgba(r.GetTextColor()), Paint: r.paint()}
	if r.isUTF8() && align == "J" {
		// justified: the words spread to fill the cell (gofpdf's TJ)
		wmax := int(math.Ceil((w - 2*cm) * 1000 / size))
		words := strings.Split(text, " ")
		shift := 0.0
		if len(words) > 1 {
			shift = float64(wmax-r.GetStringSymbolWidth(text)) / float64(len(words)-1)
		}
		at := x + dx
		for i, word := range words {
			ww := r.GetStringWidth(word)
			if word != "" {
				t.X, t.Y, t.Width, t.Text = at*k, base*k, ww*k, word
				r.add(t)
			}
			at += ww
			if i+1 < len(words) {
				at += r.GetStringWidth(" ") + shift/1000*size
			}
		}
	} else {
		width := sw
		if !r.isUTF8() && r.ws != 0 {
			width += r.ws * float64(strings.Count(text, " "))
		}
		shown := text
		if !r.isUTF8() {
			shown = r.fromCP1252(text)
		}
		t.X, t.Y, t.Width, t.Text = (x+dx)*k, base*k, width*k, shown
		r.add(t)
	}
	if r.underline {
		up, ut := -100.0, 50.0 // the standard fonts'
		if u, ok := r.uthick[r.key()]; ok {
			up, ut = r.upos[r.key()], u
		}
		r.add(Box{X: (x + dx) * k, Y: (base - up/1000*size) * k, W: sw * k, H: ut / 1000 * size * k,
			Fill: true, FillColor: t.Color, Paint: t.Paint})
	}
}

func (r *Recorder) fromCP1252(s string) string {
	out, err := charmap.Windows1252.NewDecoder().String(s)
	if err != nil {
		return s
	}
	return out
}

// chars are the text's characters as gofpdf counts them: runes in a
// font from a file, bytes in a standard one.
func (r *Recorder) chars(s string) []string {
	var cs []string
	if r.isUTF8() {
		for _, c := range s {
			cs = append(cs, string(c))
		}
		return cs
	}
	for i := 0; i < len(s); i++ {
		cs = append(cs, s[i:i+1])
	}
	return cs
}

// MultiCell is gofpdf's MultiCell, its lines drawn by the Recorder's
// CellFormat.
func (r *Recorder) MultiCell(w, h float64, text, border, align string, fill bool) {
	if r.Err() {
		return
	}
	if align == "" {
		align = "J"
	}
	if w == 0 {
		pw, _ := r.GetPageSize()
		_, _, right, _ := r.GetMargins()
		w = pw - right - r.GetX()
	}
	cm := r.GetCellMargin()
	_, size := r.GetFontSize()
	wmax := int(math.Ceil((w - 2*cm) * 1000 / size))
	cs := r.chars(strings.ReplaceAll(text, "\r", ""))
	nb := len(cs)
	if r.isUTF8() {
		for nb > 0 && cs[nb-1] == "\n" {
			nb--
		}
	} else if nb > 0 && cs[nb-1] == "\n" {
		nb--
	}
	cs = cs[:nb]
	part := func(a, b int) string { return strings.Join(cs[a:b], "") }
	b, b2 := "0", ""
	if border != "" {
		if border == "1" {
			border, b, b2 = "LTRB", "LRT", "LR"
		} else {
			if strings.Contains(border, "L") {
				b2 += "L"
			}
			if strings.Contains(border, "R") {
				b2 += "R"
			}
			b = b2
			if strings.Contains(border, "T") {
				b = b2 + "T"
			}
		}
	}
	noSpacing := func() {
		if r.ws > 0 {
			r.ws = 0
			r.SetWordSpacing(0)
		}
	}
	widths := map[string]int{}
	sep, i, j, l, ls, ns, nl := -1, 0, 0, 0, 0, 0, 1
	for i < nb {
		c := cs[i]
		if c == "\n" {
			noSpacing()
			a := align
			if r.isUTF8() && a == "J" {
				a = "L"
			}
			r.CellFormat(w, h, part(j, i), b, 2, a, fill, 0, "")
			i++
			sep, j, l, ns = -1, i, 0, 0
			nl++
			if border != "" && nl == 2 {
				b = b2
			}
			continue
		}
		if c == " " || isChinese(c) {
			sep, ls = i, l
			ns++
		}
		cw, ok := widths[c]
		if !ok {
			cw = r.GetStringSymbolWidth(c)
			widths[c] = cw
		}
		l += cw
		if l > wmax {
			if sep == -1 {
				if i == j {
					i++
				}
				noSpacing()
				r.CellFormat(w, h, part(j, i), b, 2, align, fill, 0, "")
			} else {
				if align == "J" && !r.isUTF8() {
					// (a file's font justifies by moving its words: CellFormat)
					r.ws = 0
					if ns > 1 {
						r.ws = float64((wmax-ls)/1000) * size / float64(ns-1)
					}
					r.SetWordSpacing(r.ws)
				}
				r.CellFormat(w, h, part(j, sep), b, 2, align, fill, 0, "")
				i = sep + 1
			}
			sep, j, l, ns = -1, i, 0, 0
			nl++
			if border != "" && nl == 2 {
				b = b2
			}
		} else {
			i++
		}
	}
	noSpacing()
	if border != "" && strings.Contains(border, "B") {
		b += "B"
	}
	if r.isUTF8() && align == "J" {
		align = ""
	}
	r.CellFormat(w, h, part(j, i), b, 2, align, fill, 0, "")
	left, _, _, _ := r.GetMargins()
	r.SetX(left)
}

// isChinese is gofpdf's test of a character MultiCell may break after.
func isChinese(c string) bool {
	for _, r := range c {
		return (r >= 0x2E80 && r <= 0x2EFF) || (r >= 0x2F00 && r <= 0x2FDF) || (r >= 0x2FF0 && r <= 0x2FFF) ||
			(r >= 0x3000 && r <= 0x303F) || (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF) ||
			(r >= 0x3100 && r <= 0x312F) || (r >= 0x3130 && r <= 0x318F) || (r >= 0x3190 && r <= 0x319F) ||
			(r >= 0x31A0 && r <= 0x31BF) || (r >= 0x31C0 && r <= 0x31EF) || (r >= 0x31F0 && r <= 0x31FF) ||
			(r >= 0x3200 && r <= 0x32FF) || (r >= 0x3300 && r <= 0x33FF) || (r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0x4DC0 && r <= 0x4DFF) || (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0xA000 && r <= 0xA48F) ||
			(r >= 0xA490 && r <= 0xA4CF) || (r >= 0xA4D0 && r <= 0xA4FF) || (r >= 0xAC00 && r <= 0xD7AF) ||
			(r >= 0xF900 && r <= 0xFAFF) || (r >= 0xFE30 && r <= 0xFE4F) || (r >= 0xFF00 && r <= 0xFFEF)
	}
	return false
}

// Write is gofpdf's Write, its pieces drawn by the Recorder's CellFormat.
func (r *Recorder) Write(h float64, text string) {
	if r.Err() {
		return
	}
	pw, _ := r.GetPageSize()
	left, _, right, _ := r.GetMargins()
	cm := r.GetCellMargin()
	_, size := r.GetFontSize()
	w := pw - right - r.GetX()
	wmax := (w - 2*cm) * 1000 / size
	s := strings.ReplaceAll(text, "\r", "")
	cs := r.chars(s)
	nb := len(cs)
	if r.isUTF8() && nb == 1 && s == " " {
		r.SetX(r.GetX() + r.GetStringWidth(s))
		return
	}
	part := func(a, b int) string { return strings.Join(cs[a:b], "") }
	widths := map[string]float64{}
	width := func(c string) float64 {
		if v, ok := widths[c]; ok {
			return v
		}
		v := float64(r.GetStringSymbolWidth(c))
		if v == 0 && r.isUTF8() {
			v = 65535 // a zero-width mark: gofpdf's Write counts its marker
		}
		widths[c] = v
		return v
	}
	sep, i, j, l, nl := -1, 0, 0, 0.0, 1
	firstLine := func() {
		if nl == 1 {
			r.SetX(left)
			w = pw - right - r.GetX()
			wmax = (w - 2*cm) * 1000 / size
		}
	}
	for i < nb {
		c := cs[i]
		if c == "\n" {
			r.CellFormat(w, h, part(j, i), "", 2, "", false, 0, "")
			i++
			sep, j, l = -1, i, 0
			firstLine()
			nl++
			continue
		}
		if c == " " {
			sep = i
		}
		l += width(c)
		if l > wmax {
			if sep == -1 {
				if r.GetX() > left {
					r.SetXY(left, r.GetY()+h)
					w = pw - right - r.GetX()
					wmax = (w - 2*cm) * 1000 / size
					i++
					nl++
					continue
				}
				if i == j {
					i++
				}
				r.CellFormat(w, h, part(j, i), "", 2, "", false, 0, "")
			} else {
				r.CellFormat(w, h, part(j, sep), "", 2, "", false, 0, "")
				i = sep + 1
			}
			sep, j, l = -1, i, 0
			firstLine()
			nl++
		} else {
			i++
		}
	}
	if i != j {
		r.CellFormat(l/1000*size, h, part(j, nb), "", 0, "", false, 0, "")
	}
}
