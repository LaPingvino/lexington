// Package trelby reads and writes Trelby's screenplays (.trelby): UTF-8
// text, a line of the script to a line of the file, each with two
// characters in front, how the line ends and its element's type. Its
// format is as Trelby's own fileformat.txt and its loader
// (trelby/screenplay.py) have it; no code of Trelby's is in this package.
//
// Trelby keeps less than Fountain says, and the writer leaves out what
// it has no place for: emphasis, dual dialogue (the two speeches one
// after the other), scene numbers (Trelby numbers scenes itself), page
// breaks, boneyards. A centred line is action, lyrics are dialogue. A
// first-level section is Trelby's act break; a deeper one ("## …") and a
// synopsis ("= …") are notes, which read back as what they were.
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
	var b lex.Builder
	var titles []titleString
	inSection := ""
	var text strings.Builder
	for i, l := range lines[1:] {
		if inSection != "" {
			if strings.TrimSpace(l) == "#End-"+inSection {
				inSection = ""
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
			add(&b, typ, text.String())
			text.Reset()
		default: // endSpace, and as Trelby reads one it does not know
			text.WriteString(" ")
		}
	}
	if text.Len() > 0 {
		add(&b, typeAction, text.String())
	}
	b.TitlePage(titlePage(titles))
	return b.Screenplay(), nil
}

// add adds an element: in capitals for the types Trelby prints so.
func add(b *lex.Builder, typ byte, text string) {
	t, caps := elementType(typ)
	text = strings.TrimSpace(text)
	if caps {
		text = strings.ToUpper(text)
	}
	text = lex.EscapeEmphasis(text) // Trelby's text is plain
	p := lex.Paragraph{Type: t, Text: text}
	switch t {
	case lex.TypeSection:
		p.Level = 1
	case lex.TypeNote:
		plain := lex.Unstyled(text)
		if m := noteSection.FindStringSubmatch(plain); m != nil {
			p = lex.Paragraph{Type: lex.TypeSection, Level: len(m[1]), Text: lex.EscapeEmphasis(m[2])}
			break
		}
		if m := noteSynopsis.FindStringSubmatch(plain); m != nil {
			b.Add(lex.Paragraph{Type: lex.TypeSynopsis, Text: m[1]})
			return
		}
		p = lex.Paragraph{Type: lex.TypeAction, Notes: []string{plain}}
	}
	b.Add(p)
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
		case !seenTitle && !creditLine.MatchString(strings.TrimSpace(lines[0])):
			add("Title", lines)
			seenTitle = true
		case creditLine.MatchString(strings.TrimSpace(lines[0])):
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
