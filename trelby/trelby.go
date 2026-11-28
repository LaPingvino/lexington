// Package trelby reads and writes Trelby's screenplays (.trelby): UTF-8
// text, a line of the script to a line of the file, each with two
// characters in front, how the line ends and its element's type. Its
// format is as Trelby's own fileformat.txt and its loader
// (trelby/screenplay.py) have it; no code of Trelby's is in this package.
//
// Trelby's text is plain: it styles a whole element type at once (its
// config's Element/<type>/Export/Bold, Italic, Underlined) and a title
// string as a whole (its b, i, u flags), and it has no dual dialogue. So
// emphasis and dual dialogue are kept the way Trelby's own Fountain
// import keeps them when its "Remove unsupported formatting markup" is
// off: Fountain's markers in the text (*italics*, **bold**, _underline_,
// \* for a plain asterisk) and a caret after the second speaker of dual
// dialogue ("BOB ^"), the two speeches one after the other. Trelby
// shows and prints these as they are, and its Fountain export writes
// them out unchanged, so they come back as emphasis and dual dialogue
// there as here. The reader reads Fountain's markers in every Trelby
// file (a file of Trelby's own with "*69" or snake_case keeps them as
// text, as Fountain would), and an element type its config exports bold,
// italic or underlined as styled throughout (written again, with the
// markers on each element and no config). A title string styled in part
// is written plain, Trelby styling it only as a whole. Notes within dual
// dialogue are written after it, so the caret pairs the two speeches.
//
// Trelby keeps less than Fountain says, and the writer leaves out what
// else it has no place for: scene numbers (Trelby numbers scenes
// itself), page breaks, boneyards. Trelby cannot centre a line: no
// element type or setting of its does (its own Fountain import indents
// one with spaces), so a centred line is action with Fountain's markers
// kept ("> THE END <"), as emphasis is, and reads back as centred; lyrics
// are dialogue. A first-level section is Trelby's act break; a deeper one
// ("## …") and a synopsis ("= …") are notes, which read back as what
// they were. Notes, sections and synopses are plain text.
package trelby

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// The element types: the second character of a line.
const (
	typeScene    = '\\'
	typeAction   = '.'
	typeChar     = '_'
	typeDialog   = ':'
	typeParen    = '('
	typeTrans    = '/'
	typeShot     = '='
	typeNote     = '%'
	typeActBreak = '@'
)

// How a line ends: the first character.
const (
	endSpace  = '>' // a space
	endSpace2 = '+' // two spaces
	endNone   = '&' // nothing (a word broken)
	endForced = '|' // a line break
	endLast   = '.' // the element's last line
)

func elementType(c byte) (lex.ElementType, bool) {
	switch c {
	case typeScene:
		return lex.TypeScene, true
	case typeAction:
		return lex.TypeAction, false
	case typeChar:
		return lex.TypeSpeaker, true
	case typeDialog:
		return lex.TypeDialog, false
	case typeParen:
		return lex.TypeParen, false
	case typeTrans:
		return lex.TypeTrans, true
	case typeShot:
		return lex.TypeAction, true
	case typeNote:
		return lex.TypeNote, false
	case typeActBreak:
		return lex.TypeSection, true
	}
	return lex.TypeAction, false // as Trelby reads a type it does not know
}

// noteSection and noteSynopsis are the notes a deeper section and a
// synopsis are written as.
var (
	noteSection  = regexp.MustCompile(`^(#{2,})\s+(.*)$`)
	noteSynopsis = regexp.MustCompile(`^=\s+(.*)$`)
)

// Parse reads a Trelby screenplay.
func Parse(r io.Reader) (lex.Screenplay, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var lines []string
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) > 0 {
		lines[0] = strings.TrimPrefix(lines[0], "\ufeff")
	}
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "#Version ") {
		return nil, fmt.Errorf("not a Trelby screenplay: no #Version line")
	}
	version, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(lines[0], "#Version ")))
	started := version < 3 // before 3, no #Start-Script line
	var rd reader
	var titles []titleString
	inSection := ""
	var text strings.Builder
	for i, l := range lines[1:] {
		if inSection != "" {
			if strings.TrimSpace(l) == "#End-"+inSection {
				inSection = ""
			} else if inSection == "Config" {
				rd.config(l)
			}
			continue
		}
		if strings.HasPrefix(l, "#") {
			key, val, _ := strings.Cut(l[1:], " ")
			switch {
			case strings.HasPrefix(key, "Begin-"):
				inSection = strings.TrimPrefix(key, "Begin-")
			case key == "Title-String":
				if t, ok := parseTitleString(val, len(titles)); ok {
					titles = append(titles, t)
				}
			case key == "Start-Script":
				started = true
			}
			continue
		}
		if !started || len(l) < 2 {
			if started && l != "" {
				return nil, fmt.Errorf("line %d is too short", i+2)
			}
			continue
		}
		end, typ, body := l[0], l[1], l[2:]
		text.WriteString(strings.ReplaceAll(body, "\t", "|"))
		switch end {
		case endNone:
		case endForced:
			text.WriteString("\n")
		case endSpace2:
			text.WriteString("  ")
		case endLast:
			rd.add(typ, text.String())
			text.Reset()
		default: // endSpace, and as Trelby reads one it does not know
			text.WriteString(" ")
		}
	}
	if text.Len() > 0 {
		rd.add(typeAction, text.String())
	}
	var b lex.Builder
	b.TitlePage(titlePage(titles))
	for _, p := range rd.paras {
		b.Add(p)
	}
	return b.Screenplay(), nil
}

// reader gathers the elements as paragraphs, with the styles the
// config gives the element types.
type reader struct {
	paras  []lex.Paragraph
	styles map[byte]string // the b, i, u an element type is exported in
}

// configTypes are the config's names of the element types that are
// styled as text (notes and act breaks are not).
var configTypes = map[string]byte{
	"Scene": typeScene, "Action": typeAction, "Character": typeChar, "Dialogue": typeDialog,
	"Parenthetical": typeParen, "Transition": typeTrans, "Shot": typeShot,
}

// configStyle is a config line on an element type's exported style.
var configStyle = regexp.MustCompile(`^Element/([^/]+)/Export/(Bold|Italic|Underlined):True\s*$`)

func (r *reader) config(l string) {
	m := configStyle.FindStringSubmatch(l)
	if m == nil {
		return
	}
	typ, ok := configTypes[m[1]]
	if !ok {
		return
	}
	if r.styles == nil {
		r.styles = map[byte]string{}
	}
	r.styles[typ] += strings.ToLower(m[2][:1])
}

// centred is a centred line as Fountain marks it: "> THE END <".
var centred = regexp.MustCompile(`^>\s*([^\n]*?)\s*<$`)

// add adds an element: in capitals for the types Trelby prints so, its
// Fountain markers read as emphasis, a speaker's caret as dual dialogue.
func (r *reader) add(typ byte, text string) {
	t, caps := elementType(typ)
	text = strings.TrimSpace(text)
	if caps {
		text = strings.ToUpper(text)
	}
	p := lex.Paragraph{Type: t, Text: text}
	switch t {
	case lex.TypeSection:
		p.Level = 1
		p.Text = lex.EscapeEmphasis(text) // an act break is plain
	case lex.TypeNote:
		if m := noteSection.FindStringSubmatch(text); m != nil {
			p = lex.Paragraph{Type: lex.TypeSection, Level: len(m[1]), Text: lex.EscapeEmphasis(m[2])}
			break
		}
		if m := noteSynopsis.FindStringSubmatch(text); m != nil {
			p = lex.Paragraph{Type: lex.TypeSynopsis, Text: m[1]}
			break
		}
		p = lex.Paragraph{Type: lex.TypeAction, Notes: []string{text}}
	case lex.TypeAction:
		lines := strings.Split(text, "\n")
		for i, l := range lines {
			m := centred.FindStringSubmatch(strings.TrimSpace(l))
			if m == nil {
				break
			}
			if lines[i] = m[1]; i == len(lines)-1 {
				p = lex.Paragraph{Type: lex.TypeCenter, Text: strings.Join(lines, "\n")}
			}
		}
	case lex.TypeSpeaker:
		if cue, ok := strings.CutSuffix(text, "^"); ok {
			p.Text = strings.TrimSpace(cue)
			if first := r.dualFirst(); first >= 0 {
				r.paras[first].Dual = lex.DualFirst
				p.Dual = lex.DualSecond
			} // a caret with no speech before it to pair with: an ordinary speech
		}
	}
	if f := r.styles[typ]; f != "" && p.Text != "" {
		p.Text = styled(p.Text, f)
	}
	r.paras = append(r.paras, p)
}

// dualFirst is the speaker of the speech just before, if it is not in
// dual dialogue already: -1 if there is none.
func (r *reader) dualFirst() int {
	for i := len(r.paras) - 1; i >= 0; i-- {
		switch p := r.paras[i]; p.Type {
		case lex.TypeDialog, lex.TypeParen:
			continue
		case lex.TypeSpeaker:
			if p.Dual == lex.NotDual {
				return i
			}
		}
		return -1
	}
	return -1
}

// styled is text in the style flags give (b, i, u), but for the part of
// it it has already: each line on its own, as Fountain's emphasis does
// not go over a line break.
func styled(text, flags string) string {
	has := styleFlags(text)
	bold := strings.Contains(flags, "b") && !strings.Contains(has, "b")
	italic := strings.Contains(flags, "i") && !strings.Contains(has, "i")
	underline := strings.Contains(flags, "u") && !strings.Contains(has, "u")
	if !bold && !italic && !underline {
		return text
	}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			lines[i] = mark(l, bold, italic, underline)
		}
	}
	return strings.Join(lines, "\n")
}

// titleString is a #Title-String: where it is and how it looks, and its
// lines.
type titleString struct {
	x, y                  float64
	centered, right       bool
	bold, italic, underln bool
	lines                 []string
	order                 int
}

// parseTitleString reads "x,y,size,flags,font,,text", the text's line
// breaks escaped as \n and its backslashes as \\.
func parseTitleString(v string, order int) (titleString, bool) {
	a := strings.SplitN(v, ",", 7)
	if len(a) != 7 {
		return titleString{}, false
	}
	t := titleString{order: order}
	t.x, _ = strconv.ParseFloat(a[0], 64)
	t.y, _ = strconv.ParseFloat(a[1], 64)
	t.centered = strings.Contains(a[3], "c")
	t.right = strings.Contains(a[3], "r")
	t.bold = strings.Contains(a[3], "b")
	t.italic = strings.Contains(a[3], "i")
	t.underln = strings.Contains(a[3], "u")
	t.lines = unescape(a[6])
	return t, true
}

func unescape(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				out = append(out, cur.String())
				cur.Reset()
				i++
				continue
			case '\\':
				cur.WriteByte('\\')
				i++
				continue
			}
		}
		cur.WriteByte(s[i])
	}
	return append(out, cur.String())
}

func escape(lines []string) string {
	for i, l := range lines {
		lines[i] = strings.ReplaceAll(l, `\`, `\\`)
	}
	return strings.Join(lines, `\n`)
}

// authorY is where the writer puts the credit and the authors.
const authorY = 90

// creditLine is a credit's line ("by", "Written by").
var creditLine = regexp.MustCompile(`(?i)\bby:?$`)

// titlePage reads the title strings as Fountain's fields, as the writer
// places them: centred, the title first, then "by" and the authors, the
// source and notes, the copyright low on the page (the authors without a
// credit where the credit would be); on the left the
// contact, on the right the draft (alone on the right, the contact).
func titlePage(ts []titleString) []lex.TitleField {
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].y < ts[j].y || ts[i].y == ts[j].y && ts[i].x < ts[j].x })
	var out []lex.TitleField
	add := func(key string, lines []string) {
		v := strings.TrimSpace(strings.Join(lines, "\n"))
		if v != "" {
			out = append(out, lex.TitleField{Key: key, Value: v})
		}
	}
	seenTitle, seenSource := false, false
	left := false // a block on the left: then the one on the right is the draft
	for _, t := range ts {
		left = left || !t.centered && !t.right
	}
	for _, t := range ts {
		lines := t.lines
		if t.bold || t.italic || t.underln {
			for i, l := range lines {
				if l = lex.EscapeEmphasis(strings.TrimSpace(l)); l != "" {
					lines[i] = mark(l, t.bold, t.italic, t.underln)
				}
			}
		} else {
			for i, l := range lines {
				lines[i] = lex.EscapeEmphasis(l)
			}
		}
		switch {
		case t.right && left:
			add("Draft date", lines)
		case t.right: // alone, as Trelby's own sample has its agent
			add("Contact", lines)
		case !t.centered:
			add("Contact", lines)
		case !seenTitle && !creditLine.MatchString(strings.TrimSpace(lex.Unstyled(lines[0]))):
			add("Title", lines)
			seenTitle = true
		case creditLine.MatchString(strings.TrimSpace(lex.Unstyled(lines[0]))):
			add("Credit", lines[:1])
			add("Author", lines[1:])
		case t.y >= authorY && t.y < authorY+10:
			add("Author", lines) // where the writer puts the authors without a credit
		case t.y >= 200:
			add("Copyright", lines)
		case !seenSource:
			add("Source", lines)
			seenSource = true
		default:
			add("Notes", lines)
		}
	}
	return out
}

func mark(s string, bold, italic, underline bool) string {
	switch {
	case bold && italic:
		s = "***" + s + "***"
	case bold:
		s = "**" + s + "**"
	case italic:
		s = "*" + s + "*"
	}
	if underline {
		s = "_" + s + "_"
	}
	return s
}
