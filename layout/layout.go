// Package layout lays a screenplay out as printed lines, the way Lexington
// prints it, for front-ends that draw it themselves (a preview, a
// terminal): every line with its column and width in characters (Courier,
// ten to the inch), its alignment, and its text in styled spans; page
// breaks; the title page's title block and the meta block below it; dual
// dialogue as two columns. The rules (margins, alignment, style, prefix
// and postfix, hidden elements) are those of the PDF writer.
package layout

import (
	"math"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// The page: US Letter, Courier 12 pt at ten characters to the inch.
const (
	PageWidth    = 8.5 // inches
	CharsPerInch = 10
)

// Dual dialogue's columns, in characters from the action's margin, as
// afterwriting, Better Fountain and screenplain print them (2.5" columns
// at 2" and 5" from the page's edge): where the first starts, their
// width, and how far the second starts from the first. Within a column
// the dual rules' left margins, counted from 1", indent the paren (3)
// and the speaker (5).
const (
	DualStart = 5
	DualWidth = 25
	DualGap   = 30
)

// Span is styled text in a line.
type Span = lex.Span

// Line is a printed line.
type Line struct {
	// Type is the element the line belongs to (lex.TypeAction, ...),
	// "" for an empty line.
	Type lex.ElementType
	// Block is "title" or "meta" on the title page, otherwise "".
	Block string
	// Indent and Width are the line's column and width in characters,
	// counted from the leftmost margin of the set (the action's in a
	// screenplay).
	Indent, Width int
	// Align is 'L', 'C' or 'R', within Indent and Width.
	Align byte
	// Spans are the line's text; the whole line is their Text.
	Spans []Span
	// PageBreak: a new page starts here (the line has no text).
	PageBreak bool
	// Column of dual dialogue: 0 none, 1 left, 2 right.
	Column int
	// SceneNumber of a scene heading (Fountain's #1A#), printed in the
	// margins; on the heading's first line only.
	SceneNumber string
	// Colors of the line, as #rrggbb (or ""), when the rules print
	// colors (rules.Set.ShowsColors), in their style (ColorStyle).
	Colors
}

// Colors are how a line or paragraph shows its scene's color
// (lex.ColorFor, readable on the page), as #rrggbb, "" for none: the
// text's color, a marker behind the text, a band behind the line, a bar
// in the left margin (on every line of a scene or section with a color,
// so that the lines' bars make one).
type Colors struct {
	Color, Marker, Background, Bar string
	// Pattern, when bars have patterns: the color whose pattern
	// (lex.BarPattern) the bar has.
	Pattern string
}

// colors are a line's colors in a scheme: of the block it is in (its
// scene or section, which a color on a section covers whole), a heading
// being its block's first line.
func colors(sch lex.ColorScheme, color string, heading bool) Colors {
	hex := func(style lex.ColorStyle) string {
		c, ok := lex.ColorFor(style, color)
		if !ok {
			return ""
		}
		return lex.HexOf(c)
	}
	var out Colors
	switch sch.Style {
	case lex.ColorLine:
		out.Bar = hex(lex.ColorLine)
		if sch.Patterns && out.Bar != "" {
			out.Pattern = color
		}
		if sch.Tint {
			out.Background = hex(lex.ColorTint)
		}
	case lex.ColorMarker:
		if heading || sch.Block {
			out.Marker = hex(lex.ColorMarker)
		}
	case lex.ColorBackground:
		if heading || sch.Block {
			out.Background = hex(lex.ColorBackground)
		}
	case lex.ColorText:
		if heading {
			out.Color = hex(lex.ColorText)
		}
	}
	return out
}

// Text is the line's text without styles.
func (l Line) Text() string {
	var b strings.Builder
	for _, s := range l.Spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

// Padded is the line as fixed-width text: indented and aligned with
// spaces, Width characters wide from the left margin (a renderer that
// only has a monospace font needs nothing else).
func (l Line) Padded() string {
	text := l.Text()
	n := len([]rune(text))
	pad := 0
	switch l.Align {
	case 'C':
		pad = max(0, (l.Width-n)/2)
	case 'R':
		pad = max(0, l.Width-n)
	}
	return strings.Repeat(" ", l.Indent+pad) + text
}

// Paragraph is an element of the script as it is printed, before it is
// broken into lines: for writers that leave that to a word processor
// (DOCX, ODT) or lay it out themselves.
type Paragraph struct {
	// Type is the element (lex.TypeAction, ...); "" for an empty line
	// or a page break.
	Type lex.ElementType
	// Block is "title" or "meta" on the title page, otherwise "".
	Block string
	// Key is the rule the paragraph follows: its type, the block on the
	// title page, "dualspeaker" etc. in dual dialogue.
	Key string
	// Left and Right are the margins in inches from the edges of a US
	// Letter page (PageWidth); in dual dialogue, those of its column.
	Left, Right float64
	// Align is 'L', 'C' or 'R'.
	Align byte
	// Spans are the text, styled; none for an empty line.
	Spans []Span
	// PageBreak: a new page starts here.
	PageBreak bool
	// Column of dual dialogue: 0 none, 1 left, 2 right.
	Column int
	// SceneNumber of a scene heading (Fountain's #1A#).
	SceneNumber string
	// Colors of the line, as #rrggbb (or ""), when the rules print
	// colors (rules.Set.ShowsColors), in their style (ColorStyle).
	Colors
}

// Text is the paragraph's text without styles.
func (p Paragraph) Text() string {
	return Line{Spans: p.Spans}.Text()
}

// Margin is the leftmost margin of a set's elements, in inches: the
// action's in a screenplay, further left in radio (its names).
func Margin(set rules.Set) float64 {
	if set == nil {
		set = rules.Default
	}
	base := set.Get("action").Left
	for k, f := range set {
		if !strings.HasPrefix(k, "dual") && !rules.IsSwitch(k) && !f.Hide && f.Left < base {
			base = f.Left
		}
	}
	return base
}

// Paragraphs are a screenplay's printed elements with a set of rules
// (rules.Default if nil): hidden elements and Fountain's notes left out
// (lex.Printable; lines of notes are printed with the set's printnotes
// rule, if it has one), prefixes and postfixes added, scene headings and names in capitals, Fountain's
// emphasis as styled spans.
func Paragraphs(s lex.Screenplay, set rules.Set) []Paragraph {
	if set == nil {
		set = rules.Default
	}
	s = lex.Printable(s, set.Hides)
	var out []Paragraph
	var ids []int // the color block of each paragraph (lex.ColorBlocks)
	add := func(p Paragraph, id int) {
		out = append(out, p)
		ids = append(ids, id)
	}
	block, column := "", 0
	sch := set.ColorScheme()
	blocks, colorOf := lex.ColorBlocks(s), map[int]string{} // the block each row is in, and its color
	for _, row := range s {
		if row.Block != 0 && colorOf[row.Block] == "" {
			colorOf[row.Block] = row.Color
		}
	}
	for i, row := range s {
		t := row.Type
		if !set.ShowsColors() {
			row.Color, blocks[i] = "", 0
		}
		bar := Colors{}
		if block == "" && t != lex.TypeTitlePage && t != lex.TypeNewPage && blocks[i] != 0 {
			bar = colors(sch, colorOf[blocks[i]], false)
		}
		switch t {
		case lex.TypeNewPage:
			block = ""
			add(Paragraph{PageBreak: true}, 0)
			continue
		case lex.TypeTitlePage:
			block = "title"
			continue
		case "metasection":
			block = "meta"
			add(Paragraph{Block: block}, 0)
			continue
		case lex.TypeDualOpen:
			column = 1
			continue
		case lex.TypeDualNext:
			column = 2
			continue
		case lex.TypeDualClose:
			column = 0
			continue
		case lex.TypeEmpty:
			add(Paragraph{Block: block, Column: column, Colors: bar}, blocks[i])
			continue
		}
		key := string(t)
		if t == lex.TypeNote {
			key = rules.KeyPrintNotes.String()
		} else if block != "" {
			key = block // title page fields: the title or meta format
		} else if column != 0 && (t == lex.TypeSpeaker || t == lex.TypeDialog || t == lex.TypeParen) {
			key = "dual" + string(t)
		}
		f := set.Get(key)
		if f.Hide && block == "" {
			continue
		}
		contents, number := string(row.Contents), ""
		switch t {
		case lex.TypeScene:
			contents, number = lex.SceneNumber(contents)
		case "section":
			contents = lex.SectionText(contents)
		}
		text := strings.TrimSpace(f.Prefix + contents + f.Postfix)
		if t == lex.TypeScene || t == lex.TypeSpeaker {
			text = strings.ToUpper(text)
		}
		left, right := f.Left, f.Right
		if column != 0 {
			// two columns side by side (see DualStart); the dual rules'
			// left margins, from 1", indent within a column
			start := Margin(rules.Default) + float64(DualStart+(column-1)*DualGap)/CharsPerInch
			left = start + math.Max(0, f.Left-1.0)
			right = PageWidth - start - float64(DualWidth)/CharsPerInch
		}
		align := byte('L')
		if a := strings.ToUpper(f.Align); a == "C" || a == "R" {
			align = a[0]
		}
		base := Span{Bold: strings.Contains(f.Style, "b"), Italic: strings.Contains(f.Style, "i"),
			Underline: strings.Contains(f.Style, "u")}
		cs := bar
		if t == lex.TypeScene || t == lex.TypeSection {
			cs = colors(sch, row.Color, true)
		}
		id := blocks[i]
		if block != "" {
			id = 0
		}
		add(Paragraph{Type: t, Block: block, Key: key, Left: left, Right: right, Align: align,
			Spans: emphasis(text, base), Column: column, SceneNumber: number, Colors: cs}, id)
	}
	// a blank line at a block's edge, or a page's, shows no color
	lex.TrimBlocks(ids, func(i int) bool { return out[i].Type != "" && strings.TrimSpace(out[i].Text()) != "" },
		func(i int) bool { return out[i].PageBreak })
	for i := range out {
		if out[i].Type == "" && ids[i] == 0 {
			out[i].Colors = Colors{}
		}
	}
	return out
}

// Lay lays out a screenplay with a set of rules (rules.Default if nil).
func Lay(s lex.Screenplay, set rules.Set) []Line {
	base := Margin(set)
	var out []Line
	for _, p := range Paragraphs(s, set) {
		switch {
		case p.PageBreak:
			out = append(out, Line{PageBreak: true})
			continue
		case p.Type == "":
			out = append(out, Line{Block: p.Block, Column: p.Column, Colors: p.Colors})
			continue
		}
		indent := int(math.Round((p.Left - base) * CharsPerInch))
		width := int(math.Round((PageWidth - p.Left - p.Right) * CharsPerInch))
		number := p.SceneNumber
		for _, spans := range wrap(p.Spans, width) {
			out = append(out, Line{Type: p.Type, Block: p.Block, Indent: indent, Width: width, Align: p.Align,
				Spans: spans, Column: p.Column, SceneNumber: number, Colors: p.Colors})
			number = ""
		}
	}
	return out
}

// emphasis reads Fountain's emphasis (lex.Emphasis) into spans on top
// of the base style.
func emphasis(text string, base Span) []Span {
	spans := lex.Emphasis(text)
	for i, s := range spans {
		s.Bold, s.Italic, s.Underline = s.Bold || base.Bold, s.Italic || base.Italic, s.Underline || base.Underline
		spans[i] = s
	}
	return merge(spans)
}

// wrap breaks styled text into lines of at most width characters, at
// spaces (a longer word gets a line of its own); line breaks are kept.
func wrap(spans []Span, width int) [][]Span {
	var lines [][]Span
	var line []Span
	n := 0
	flush := func() {
		lines = append(lines, merge(line))
		line, n = nil, 0
	}
	var cur []Span
	curLen := 0
	addWord := func() {
		if curLen == 0 {
			return
		}
		if n > 0 && n+1+curLen > width {
			flush()
		}
		if n > 0 { // the space takes the style of the text before it
			sp := line[len(line)-1]
			sp.Text = " "
			line = append(line, sp)
			n++
		}
		line = append(line, cur...)
		n += curLen
		cur, curLen = nil, 0
	}
	for _, s := range spans {
		for _, r := range s.Text {
			switch {
			case r == '\n':
				addWord()
				flush()
			case unicode.IsSpace(r):
				addWord()
			default:
				if len(cur) > 0 && sameStyle(cur[len(cur)-1], s) {
					cur[len(cur)-1].Text += string(r)
				} else {
					c := s
					c.Text = string(r)
					cur = append(cur, c)
				}
				curLen++
			}
		}
	}
	addWord()
	flush()
	return lines
}

func sameStyle(a, b Span) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline
}

// merge joins neighbouring spans of the same style.
func merge(spans []Span) []Span {
	var out []Span
	for _, s := range spans {
		if len(out) > 0 && sameStyle(out[len(out)-1], s) {
			out[len(out)-1].Text += s.Text
		} else {
			out = append(out, s)
		}
	}
	return out
}
