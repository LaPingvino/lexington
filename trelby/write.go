package trelby

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// TrelbyWriter writes a screenplay as a Trelby screenplay (format 3).
type TrelbyWriter struct{}

// Write writes the screenplay.
func (w *TrelbyWriter) Write(out io.Writer, s lex.Screenplay) error {
	bw := bufio.NewWriter(out)
	title, paras := lex.Paragraphs(s)
	bw.WriteString("\ufeff#Version 3\n")
	for _, t := range titleStrings(title) {
		bw.WriteString("#Title-String " + t + "\n")
	}
	bw.WriteString("#Header-String 1,0,r,,${PAGE}.\n#Header-Empty-Lines 1\n#Start-Script \n")
	n := 0
	// element writes an element's text as it is: Fountain's emphasis
	// markers in it, as Trelby has no styles in its text.
	element := func(typ byte, text string) {
		text = strings.TrimSpace(text)
		lines := strings.Split(text, "\n")
		for i, l := range lines {
			end := byte(endForced)
			if i == len(lines)-1 {
				end = endLast
			}
			bw.WriteString(string([]byte{end, typ}) + strings.ReplaceAll(strings.TrimRight(l, " \t"), "\t", " ") + "\n")
		}
		n++
	}
	// Notes within dual dialogue wait for its end: Trelby's caret pairs
	// the second speech only with a speech right before it.
	held, hold := []string(nil), false
	note := func(text string) {
		if hold {
			held = append(held, text)
		} else {
			element(typeNote, text)
		}
	}
	release := func() {
		hold = false
		for _, h := range held {
			element(typeNote, h)
		}
		held = nil
	}
	notes := func(ns []string) {
		for _, n := range ns {
			if n = strings.TrimSpace(n); n != "" {
				note(n)
			}
		}
	}
	for _, p := range paras {
		inSpeech := p.Type == lex.TypeParen || p.Type == lex.TypeDialog || p.Type == lex.TypeLyrics
		if hold && !inSpeech && !(p.Type == lex.TypeSpeaker && p.Dual == lex.DualSecond) {
			release()
		}
		notes(p.Before)
		if p.Type == lex.TypeSpeaker && p.Dual == lex.DualFirst {
			hold = true
		}
		switch p.Type {
		case lex.TypeBoneyard:
			continue
		case lex.TypeSection:
			if p.Level <= 1 {
				element(typeActBreak, lex.Unstyled(p.Text))
			} else {
				element(typeNote, strings.Repeat("#", p.Level)+" "+lex.Unstyled(p.Text))
			}
		case lex.TypeSynopsis:
			element(typeNote, "= "+lex.Unstyled(p.Text))
		default:
			text := p.Text
			if strings.TrimSpace(text) == "" {
				break
			}
			if p.Type == lex.TypeSpeaker && p.Dual == lex.DualSecond {
				text = strings.TrimSpace(text) + " ^" // as Fountain marks dual dialogue
			}
			element(typeOf(p.Type), needlessEscapes(text))
		}
		notes(p.Notes)
		for _, syn := range p.Synopses {
			note("= " + lex.Unstyled(syn))
		}
	}
	release()
	if n == 0 {
		element(typeAction, "") // Trelby reads no script without a line
	}
	return bw.Flush()
}

func typeOf(t lex.ElementType) byte {
	switch t {
	case lex.TypeScene:
		return typeScene
	case lex.TypeSpeaker:
		return typeChar
	case lex.TypeDialog, lex.TypeLyrics:
		return typeDialog
	case lex.TypeParen:
		return typeParen
	case lex.TypeTrans:
		return typeTrans
	}
	return typeAction
}

// titleStrings are the title page as #Title-Strings, placed as Trelby
// places its own (millimetres from the top left): the title, "by" and the
// authors, the source and notes centred; the contact at the bottom left,
// the draft at the bottom right, the copyright at the bottom centre.
func titleStrings(fields []lex.TitleField) []string {
	var out []string
	put := func(x, y float64, size int, flags, value string) {
		var lines []string
		for _, l := range strings.Split(value, "\n") {
			lines = append(lines, lex.Unstyled(strings.TrimSpace(l)))
		}
		flags += styleFlags(value) // styled as a whole; in part, plain
		out = append(out, fmt.Sprintf("%f,%f,%d,%s,Courier,,%s", x, y, size, flags, escape(lines)))
	}
	byKey := map[string][]string{}
	var order []string
	for _, f := range fields {
		k := strings.ToLower(f.Key)
		if k == "authors" {
			k = "author"
		}
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], f.Value)
	}
	join := func(k string) string { return strings.Join(byKey[k], "\n") }
	if v := join("title"); v != "" {
		put(0, 70, 24, "c", v)
	}
	if credit, author := join("credit"), join("author"); credit != "" || author != "" {
		v := author
		if credit != "" {
			v = credit + "\n\n" + author
		}
		put(0, authorY, 12, "c", strings.TrimSpace(v))
	}
	y := 120.0
	if v := join("source"); v != "" {
		put(0, y, 12, "c", v)
		y += 20
	}
	for _, k := range order {
		switch k {
		case "title", "credit", "author", "source", "contact", "draft date", "copyright":
			continue
		}
		put(0, y, 12, "c", join(k))
		y += 20
	}
	if v := join("copyright"); v != "" {
		put(0, 250, 12, "c", v)
	}
	if v := join("contact"); v != "" {
		put(15, 240, 12, "", v)
	}
	if v := join("draft date"); v != "" {
		put(200, 240, 12, "r", v)
	}
	return out
}

// styleFlags are the b, i and u of a value styled as a whole
// ("_**TITLE**_": bu): Trelby's flags for a title string, and what an
// element styled by its type has already.
func styleFlags(v string) string {
	spans := lex.Emphasis(v)
	bold, italic, underline := true, true, true
	text := false
	for _, s := range spans {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		text = true
		bold, italic, underline = bold && s.Bold, italic && s.Italic, underline && s.Underline
	}
	flags := ""
	if !text {
		return ""
	}
	if bold {
		flags += "b"
	}
	if italic {
		flags += "i"
	}
	if underline {
		flags += "u"
	}
	return flags
}

// needlessEscapes drops the backslashes before asterisks and underscores
// that read as text without them (snake\_case, 2 \* 3), Trelby showing
// them as they are.
func needlessEscapes(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	want := lex.Emphasis(text)
	for i := 0; i < len(text)-1; i++ {
		if text[i] != '\\' {
			continue
		}
		if c := text[i+1]; c != '*' && c != '_' {
			i++
			continue
		}
		if t := text[:i] + text[i+1:]; slices.Equal(lex.Emphasis(t), want) {
			text = t // the marker is now at i
		} else {
			i++
		}
	}
	return text
}
