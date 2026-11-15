package lex

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Span is styled text: a piece of a line with Fountain's emphasis.
type Span struct {
	Text      string
	Bold      bool
	Italic    bool
	Underline bool
}

// SameStyle reports whether two spans have the same style.
func (s Span) SameStyle(o Span) bool {
	return s.Bold == o.Bold && s.Italic == o.Italic && s.Underline == o.Underline
}

// Emphasis reads Fountain's emphasis (fountain.io/syntax#emphasis) in a
// text and returns it as spans, without the markers: *italics*, **bold**,
// ***bold italics*** and _underline_, which mix and nest
// ("_**THE END**_"). As in Markdown the spaces count: a marker opens only
// before text and closes only after it ("He dialed *69 and then *23" has
// none). A backslash makes a marker plain text ("**\*9765\***" is bold
// *9765*); other backslashes are text. Emphasis does not go over a line
// break; the line breaks are in the spans' text. An underscore within a
// word (snake_case) is no marker, an asterisk is (un*frigging*believable).
// Markers that are not closed are text.
func Emphasis(text string) []Span {
	var out []Span
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			out = appendSpan(out, Span{Text: "\n"})
		}
		for _, s := range emphasisLine(line) {
			out = appendSpan(out, s)
		}
	}
	if len(out) == 0 {
		out = []Span{{}}
	}
	return out
}

// Unstyled is the text without its emphasis markers and escapes.
func Unstyled(text string) string {
	if !strings.ContainsAny(text, `*_\`) {
		return text
	}
	var b strings.Builder
	for _, s := range Emphasis(text) {
		b.WriteString(s.Text)
	}
	return b.String()
}

// EscapeEmphasis escapes every emphasis marker in plain text, so that
// Emphasis reads it as it is.
func EscapeEmphasis(text string) string {
	if !strings.ContainsAny(text, "*_") {
		return text
	}
	r := strings.NewReplacer("*", `\*`, "_", `\_`)
	return r.Replace(text)
}

// appendSpan adds a span, joining it to the last one if they look alike.
func appendSpan(out []Span, s Span) []Span {
	if s.Text == "" {
		return out
	}
	if n := len(out); n > 0 && out[n-1].SameStyle(s) {
		out[n-1].Text += s.Text
		return out
	}
	return append(out, s)
}

const (
	styleBold = 1 << iota
	styleItalic
	styleUnderline
)

// node is a piece of a line: text, or a run of one marker (delim != 0)
// with rem of its characters not yet used as emphasis.
type node struct {
	text              string
	delim             byte
	rem               int
	canOpen, canClose bool
	style             int
}

func emphasisLine(line string) []Span {
	if !strings.ContainsAny(line, `*_\`) {
		return []Span{{Text: line}}
	}
	nodes := tokenize(line)
	match(nodes)
	var out []Span
	for _, n := range nodes {
		text := n.text
		if n.delim != 0 {
			text = strings.Repeat(string(n.delim), n.rem)
		}
		out = appendSpan(out, Span{Text: text, Bold: n.style&styleBold != 0,
			Italic: n.style&styleItalic != 0, Underline: n.style&styleUnderline != 0})
	}
	return out
}

// tokenize splits a line into text and marker runs, with escapes undone
// and whether each run can open or close emphasis.
func tokenize(line string) []node {
	var nodes []node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			nodes = append(nodes, node{text: text.String()})
			text.Reset()
		}
	}
	for i := 0; i < len(line); {
		c := line[i]
		switch {
		case c == '\\' && i+1 < len(line) && (line[i+1] == '*' || line[i+1] == '_'):
			text.WriteByte(line[i+1])
			i += 2
		case c == '*' || c == '_':
			j := i
			for j < len(line) && line[j] == c {
				j++
			}
			before, _ := utf8.DecodeLastRuneInString(line[:i])
			after, _ := utf8.DecodeRuneInString(line[j:])
			if i == 0 {
				before = ' '
			}
			if j == len(line) {
				after = ' '
			}
			n := node{delim: c, rem: j - i, canOpen: !unicode.IsSpace(after), canClose: !unicode.IsSpace(before)}
			if c == '_' { // not within a word
				n.canOpen = n.canOpen && !isWordRune(before)
				n.canClose = n.canClose && !isWordRune(after)
			}
			flush()
			nodes = append(nodes, n)
			i = j
		default:
			text.WriteByte(c)
			i++
		}
	}
	flush()
	return nodes
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// match pairs the marker runs as Markdown does: each closing run with the
// nearest open run of its marker before it, two asterisks (bold) at a
// time when both have them, else one (italics); the text between gets
// the style, and runs between them can no longer pair.
func match(nodes []node) {
	active := make([]bool, len(nodes))
	for i := range nodes {
		active[i] = nodes[i].delim != 0
	}
	for c := range nodes {
		closer := &nodes[c]
		if !active[c] || !closer.canClose {
			continue
		}
		for closer.rem > 0 {
			o := -1
			for k := c - 1; k >= 0; k-- {
				if active[k] && nodes[k].delim == closer.delim && nodes[k].canOpen && nodes[k].rem > 0 {
					o = k
					break
				}
			}
			if o < 0 {
				break
			}
			opener := &nodes[o]
			n, style := 1, styleItalic
			switch {
			case closer.delim == '_':
				style = styleUnderline
			case opener.rem >= 2 && closer.rem >= 2:
				n, style = 2, styleBold
			}
			opener.rem -= n
			closer.rem -= n
			for k := o + 1; k < c; k++ {
				nodes[k].style |= style
				active[k] = false
			}
			if opener.rem == 0 {
				active[o] = false
			}
		}
		if closer.rem == 0 || !closer.canOpen {
			active[c] = closer.rem > 0 && closer.canOpen
		}
	}
}
