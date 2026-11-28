package osf

import (
	"crypto/sha1"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// OSFWriter writes a screenplay as an OSF document.
//
// What OSF has no place for goes in its notes, which Fade In does not
// print either: a section (# Act One) and a synopsis not on a scene
// heading are notes on the paragraph after them, "# Act One" and "= …"; a
// boneyard is left out. A scene heading's synopses are its synopsis.
type OSFWriter struct {
	// Version is the OSF version written: "5.0" (the default, Fade In
	// 5's), "4.0", "2.0" or "1.2".
	Version string
}

// version is the writer's version as a number: 50, 40, 20 or 12.
func (w *OSFWriter) version() int {
	switch v := Version(w.Version); {
	case v == 0 || v >= 50:
		return 50
	case v >= 40:
		return 40
	case v >= 20:
		return 20
	}
	return 12
}

// Write writes the screenplay.
func (w *OSFWriter) Write(out io.Writer, s lex.Screenplay) error {
	_, err := out.Write(w.Document(s))
	return err
}

// Document is the screenplay as an OSF document.
func (w *OSFWriter) Document(s lex.Screenplay) []byte {
	v := w.version()
	title, paras := lex.Paragraphs(s)
	base := "basestyle"
	if v < 40 {
		base = "basestylename"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<document type="%s" version="%d">`+"\n", DocumentType, v)
	sum := sha1.Sum([]byte(fmt.Sprint(s)))
	uuid := fmt.Sprintf("%X-%X-%X-%X-%X", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
	switch {
	case v < 20:
		b.WriteString("  <info" + infoAttrs(title) + ` uuid="` + uuid + `"/>` + "\n")
	case v >= 50:
		// a draft's uuid as well, in lower case, as Fade In 5 has it (its
		// pagecount it counts itself)
		d := sha1.Sum(append([]byte("draft"), sum[:]...))
		d[6], d[8] = d[6]&0x0f|0x40, d[8]&0x3f|0x80 // a version 4 uuid's form
		draft := fmt.Sprintf("%x-%x-%x-%x-%x", d[0:4], d[4:6], d[6:8], d[8:10], d[10:16])
		b.WriteString(`  <info uuid="` + uuid + `" draft_uuid="` + draft + `"/>` + "\n")
	default:
		b.WriteString(`  <info uuid="` + uuid + `"/>` + "\n")
	}
	if v >= 50 {
		// Fade In 5's settings for a new script
		b.WriteString(`  <settings page_width="2159" page_height="2794" margin_top="317" margin_bottom="220" margin_left="317" margin_right="317" normal_linesperinch="6.0" element_spacing="1.00" break_on_sentences="false" dialogue_continues="true" dialogue_pagebreaks="true" cont_text="(cont'd)" more_text="(MORE)" scenes_continue="false" continued_text="CONTINUED" number_continued="true" scene_time_separator=" - " page_header="#." page_footer="" header_alignment="3" footer_alignment="3" header_first_page="false" footer_first_page="false" pages_locked="false" pages_combine="true" pagenumber_start="1" pagenumber_mode="1AB" revision="0" document_revision="-1" revision_mode="false" show_revisions="none" selected_revisions=""/>` + "\n")
	} else {
		b.WriteString(`  <settings page_width="2159" page_height="2794" margin_top="317" margin_bottom="220" margin_left="317" margin_right="317" normal_linesperinch="6.0" dialogue_continues="true" cont_text="(cont'd)" more_text="(MORE)" continued_text="CONTINUED" omitted_text="OMITTED"/>` + "\n")
	}
	writeStyles(&b, v, base)
	b.WriteString("  <paragraphs>\n")
	var pending []string // notes for the next paragraph
	for _, p := range paras {
		switch p.Type {
		case lex.TypeBoneyard:
			pending = append(pending, p.Before...)
			continue
		case lex.TypeSection:
			pending = append(append(pending, p.Before...), strings.Repeat("#", max(p.Level, 1))+" "+p.Text)
			pending = append(append(pending, p.Notes...), synopsisNotes(p.Synopses)...)
			continue
		case lex.TypeSynopsis:
			pending = append(append(pending, p.Before...), synopsisNotes([]string{p.Text})...)
			continue
		}
		p.Notes = append(append(pending, p.Before...), p.Notes...)
		pending = nil
		writePara(&b, v, base, p)
	}
	if len(pending) > 0 {
		writePara(&b, v, base, lex.Paragraph{Type: lex.TypeAction, Notes: pending})
	}
	b.WriteString("  </paragraphs>\n")
	if v >= 20 {
		writeTitlePage(&b, v, base, title)
	}
	writeLists(&b, paras)
	b.WriteString("</document>\n")
	return []byte(b.String())
}

func synopsisNotes(ss []string) []string {
	var out []string
	for _, s := range ss {
		out = append(out, "= "+s)
	}
	return out
}

// styleOf is the paragraph style a line is written in.
func styleOf(t lex.ElementType) string {
	switch t {
	case lex.TypeScene:
		return styleScene
	case lex.TypeSpeaker:
		return styleChar
	case lex.TypeParen:
		return styleParen
	case lex.TypeDialog:
		return styleDialog
	case lex.TypeTrans:
		return styleTrans
	case lex.TypeLyrics:
		return styleLyrics
	}
	return styleAction
}

func writeStyles(b *strings.Builder, v int, base string) {
	caps := func(on bool) string {
		switch {
		case v < 20 && on:
			return ` effects="1"`
		case v < 20:
			return ` effects="0"`
		case on:
			return ` allcaps="1"`
		}
		return ""
	}
	b.WriteString("  <styles>\n")
	b.WriteString(`    <style name="Normal Text" builtin="1" builtin_index="0" font="Courier Screenplay" size="12"/>` + "\n")
	if v >= 50 {
		// as Fade In 5 writes them
		b.WriteString(`    <style name="Scene Heading" builtin="1" builtin_index="1" basestyle="Normal Text" style_enter="Action" style_tab_after="Action" font="Courier Screenplay" size="12" spacebefore="2.0" keepwithnext="1" allcaps="1"/>
    <style name="Action" builtin="1" builtin_index="2" basestyle="Normal Text" style_enter="Action" style_tab_before="Character" font="Courier Screenplay" size="12" spacebefore="1.0"/>
    <style name="Character" builtin="1" builtin_index="3" basestyle="Normal Text" style_enter="Dialogue" style_tab_before="Action" style_tab_after="Parenthetical" font="Courier Screenplay" size="12" spacebefore="1.0" keepwithnext="1" leftindent="635" allcaps="1"/>
    <style name="Parenthetical" builtin="1" builtin_index="4" basestyle="Normal Text" style_enter="Dialogue" style_tab_before="Dialogue" style_tab_after="Dialogue" font="Courier Screenplay" size="12" keepwithnext="1" leftindent="508" rightindent="508"/>
    <style name="Dialogue" builtin="1" builtin_index="5" basestyle="Normal Text" style_enter="Action" style_tab_before="Parenthetical" style_tab_after="Parenthetical" font="Courier Screenplay" size="12" leftindent="330" rightindent="254"/>
    <style name="Transition" builtin="1" builtin_index="6" basestyle="Normal Text" style_enter="Scene Heading" style_tab_after="Action" font="Courier Screenplay" size="12" spacebefore="1.0" align="right" leftindent="1016" rightindent="127" allcaps="1"/>
    <style name="Shot" builtin="1" builtin_index="7" basestyle="Normal Text" style_enter="Action" style_tab_after="Action" font="Courier Screenplay" size="12" spacebefore="1.0" keepwithnext="1" allcaps="1"/>
    <style name="Lyrics" basestyle="Dialogue" style_enter="Dialogue" font="Courier Screenplay" size="12" italic="1" leftindent="330" rightindent="254"/>
    <header_style basestyle="Normal Text"/>
    <footer_style basestyle="Normal Text"/>
  </styles>
`)
		return
	}
	rows := []struct {
		name, extra string
		caps        bool
	}{
		{styleScene, ` style_enter="Action" spacebefore="2" keepwithnext="1"`, true},
		{styleAction, ` style_enter="Action" spacebefore="1"`, false},
		{styleChar, ` style_enter="Dialogue" spacebefore="1" keepwithnext="1" leftindent="635"`, true},
		{styleParen, ` style_enter="Dialogue" keepwithnext="1" leftindent="508" rightindent="508"`, false},
		{styleDialog, ` style_enter="Action" leftindent="330" rightindent="254"`, false},
		{styleTrans, ` style_enter="Scene Heading" spacebefore="1" align="right" leftindent="1016" rightindent="127"`, true},
		{styleShot, ` style_enter="Action" spacebefore="1" keepwithnext="1"`, true},
	}
	for i, r := range rows {
		fmt.Fprintf(b, `    <style name="%s" builtin="1" builtin_index="%d" %s="Normal Text" font="Courier Screenplay" size="12"%s%s/>`+"\n",
			r.name, i+1, base, r.extra, caps(r.caps))
	}
	italic := ` italic="1"`
	if v < 20 {
		italic = ""
	}
	fmt.Fprintf(b, `    <style name="%s" %s="Dialogue" style_enter="Dialogue" font="Courier Screenplay" size="12" leftindent="330" rightindent="254"%s/>`+"\n",
		styleLyrics, base, italic)
	b.WriteString("  </styles>\n")
}

func writePara(b *strings.Builder, v int, base string, p lex.Paragraph) {
	b.WriteString("    <para")
	if p.Type == lex.TypeScene && p.Number != "" {
		if v >= 40 {
			b.WriteString(` number="` + esc(p.Number) + `"`)
		} else {
			b.WriteString(` scene_number="` + esc(p.Number) + `"`)
		}
	}
	if len(p.Notes) > 0 {
		b.WriteString(` note="` + esc(strings.Join(p.Notes, "\n")) + `"`)
	}
	if len(p.Synopses) > 0 {
		b.WriteString(` synopsis="` + esc(strings.Join(p.Synopses, "\n")) + `"`)
	}
	b.WriteString(">\n")
	b.WriteString(`      <style ` + base + `="` + styleOf(p.Type) + `"`)
	if p.Type == lex.TypeCenter {
		b.WriteString(` align="center"`)
	}
	if p.Type == lex.TypeSpeaker && p.Dual == lex.DualFirst {
		b.WriteString(` dualdialogue="1"`)
	}
	if p.NewPage {
		b.WriteString(` pagebreakbefore="1"`)
	}
	b.WriteString("/>\n")
	writeText(b, v, p.Text)
	b.WriteString("    </para>\n")
}

// writeText writes a text's runs: 2.x and 4.0 a <text> a run, with its
// emphasis as attributes; 1.2 one <text>, its emphasis and line breaks as
// tags in it.
func writeText(b *strings.Builder, v int, text string) {
	spans := lex.Emphasis(text)
	if v < 20 {
		b.WriteString("      <text>" + esc(inlineText(spans)) + "</text>\n")
		return
	}
	for _, s := range spans {
		if s.Text == "" && len(spans) > 1 {
			continue
		}
		b.WriteString("      <text")
		if s.Bold {
			b.WriteString(` bold="1"`)
		}
		if s.Italic {
			b.WriteString(` italic="1"`)
		}
		if s.Underline {
			b.WriteString(` underline="1"`)
		}
		b.WriteString(">" + escText(s.Text) + "</text>\n")
	}
}

// inlineText is text as OSF 1.2 has it: its emphasis and line breaks as
// tags in it.
func inlineText(spans []lex.Span) string {
	var t strings.Builder
	for _, s := range spans {
		body := strings.ReplaceAll(strings.NewReplacer("<", "&lt;", ">", "&gt;").Replace(s.Text), "\n", "<br>")
		if body == "" {
			continue
		}
		open, close := "", ""
		if s.Bold {
			open, close = open+"<b>", "</b>"+close
		}
		if s.Italic {
			open, close = open+"<i>", "</i>"+close
		}
		if s.Underline {
			open, close = open+"<u>", "</u>"+close
		}
		t.WriteString(open + body + close)
	}
	return t.String()
}

// titleBookmark is the bookmark a title page key's paragraph has.
func titleBookmark(key string) (mark string, centered bool) {
	switch strings.ToLower(key) {
	case "title":
		return "Title", true
	case "credit":
		return "Credit", true
	case "author", "authors":
		return "Author", true
	case "source":
		return "Source", true
	case "copyright":
		return "Copyright", false
	case "draft date", "draft":
		return "Draft", false
	case "contact":
		return "Contact", false
	}
	return "Notes", false
}

// writeLists writes the script's characters and locations, as Fade In
// lists them for its completion.
func writeLists(b *strings.Builder, paras []lex.Paragraph) {
	var chars, places []string
	seen := map[string]bool{}
	for _, p := range paras {
		switch p.Type {
		case lex.TypeSpeaker:
			name := strings.TrimSpace(extension.ReplaceAllString(lex.Unstyled(p.Text), ""))
			if name != "" && !seen["c"+name] {
				seen["c"+name] = true
				chars = append(chars, name)
			}
		case lex.TypeScene:
			if m := location.FindStringSubmatch(lex.Unstyled(p.Text)); m != nil && !seen["l"+m[1]] {
				seen["l"+m[1]] = true
				places = append(places, m[1])
			}
		}
	}
	b.WriteString("  <lists>\n    <characters>\n")
	for _, c := range chars {
		b.WriteString(`      <character name="` + esc(c) + `"/>` + "\n")
	}
	b.WriteString("    </characters>\n    <locations>\n")
	for _, l := range places {
		b.WriteString(`      <location name="` + esc(l) + `"/>` + "\n")
	}
	b.WriteString("    </locations>\n  </lists>\n")
}

var (
	// extension is a name's (V.O.), (CONT'D)
	extension = regexp.MustCompile(`\s*\([^()]*\)\s*`)
	// location is a heading's place: after INT., before " - DAY"
	location = regexp.MustCompile(`^(?i)(?:int|ext|est|i/e|int\.?/ext)[.\s]+(.+?)(?:\s+[-–—]\s+.*)?$`)
)

func writeTitlePage(b *strings.Builder, v int, base string, fields []lex.TitleField) {
	b.WriteString("  <titlepage>\n")
	for _, f := range fields {
		mark, centered := titleBookmark(f.Key)
		b.WriteString("    <para")
		if mark != "" {
			b.WriteString(` bookmark="` + mark + `"`)
		}
		b.WriteString(">\n      <style " + base + `="Normal Text"`)
		if centered {
			b.WriteString(` align="center"`)
		}
		b.WriteString("/>\n")
		writeText(b, v, f.Value)
		b.WriteString("    </para>\n")
	}
	b.WriteString("  </titlepage>\n")
}

// infoAttrs are a 1.2 document's title page, as <info>'s attributes.
func infoAttrs(fields []lex.TitleField) string {
	var credit, author string
	var b strings.Builder
	set := func(attr, v string) { b.WriteString(" " + attr + `="` + esc(inlineText(lex.Emphasis(v))) + `"`) }
	for _, f := range fields {
		switch strings.ToLower(f.Key) {
		case "title":
			set("title", f.Value)
		case "credit":
			credit = f.Value
		case "author", "authors":
			author = f.Value
		case "source":
			set("subtitle", f.Value)
		case "draft date", "draft":
			set("drafts", f.Value)
		case "copyright":
			set("copyright", f.Value)
		case "contact":
			set("contact", f.Value)
		}
	}
	if credit != "" || author != "" {
		set("written_by", strings.TrimSpace(credit+"\n"+author))
	}
	return b.String()
}

// esc escapes a text for an attribute, its line breaks kept.
func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "\n", "&#10;", "\t", "&#9;")
	return r.Replace(s)
}

// escText escapes a text for an element's content.
func escText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
