package celtx

import (
	"archive/zip"
	"bytes"
	"cmp"
	"fmt"
	"hash/crc32"
	"hash/fnv"
	"html"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/LaPingvino/lexington/lex"
)

// CeltxWriter writes a screenplay as a Celtx project: project.rdf and
// the script's HTML, stored in a zip as Celtx stores its own.
type CeltxWriter struct {
	// Tags are the script's breakdown, and LineTags the words tagged by
	// the rows' Source line (fountain.ParseLines), as fdx.FDXWriter has
	// them: those of Cast, Props and Sound are written as Celtx's.
	Tags     *lex.TagSet
	LineTags map[int][]lex.LineTag
	// Now is the time the project is saved at (zero: now).
	Now time.Time
}

// departments are the Celtx departments the writer writes tags of: the
// category, its type in project.rdf and its class in the script.
var departments = []struct{ category, dept, class string }{
	{lex.CastCategory, "Cast", "cast"},
	{"Props", "Props", "props"},
	{"Sound", "Sound", "sound"},
}

// scriptFile is the script's HTML's name in the zip.
const scriptFile = "script-lex.html"

// Write writes the project's zip.
func (w *CeltxWriter) Write(out io.Writer, s lex.Screenplay) error {
	pw := newProjectWriter(w, s)
	body := pw.script(s)
	z := zip.NewWriter(out)
	now := cmp.Or(w.Now, time.Now())
	if err := store(z, scriptFile, body, now); err != nil {
		return err
	}
	if err := store(z, ManifestName, pw.manifest(), now); err != nil {
		return err
	}
	return z.Close()
}

// store puts a file in the zip uncompressed, its sizes in its header (no
// data descriptor), as Celtx stores its files.
func store(z *zip.Writer, name string, data []byte, now time.Time) error {
	h := &zip.FileHeader{Name: name, Method: zip.Store, Modified: now, CreatorVersion: 20, ReaderVersion: 20,
		CRC32: crc32.ChecksumIEEE(data), CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data))}
	// CreateRaw takes the header as it is: the MS-DOS date and time too
	local := now.Local()
	h.ModifiedDate = uint16(max(local.Year()-1980, 0)<<9 | int(local.Month())<<5 | local.Day())
	h.ModifiedTime = uint16(local.Hour()<<11 | local.Minute()<<5 | local.Second()/2)
	f, err := z.CreateRaw(h)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// projectWriter makes the project's ids and keeps what the manifest
// lists: the scenes, the items tagged and where.
type projectWriter struct {
	w      *CeltxWriter
	seed   uint64
	n      int
	title  string
	scenes []scene
	items  []item
	itemOf map[string]int // an element's ID → its item
}

// scene is a scene heading written: its id in the script, its URI, its
// heading and index card, the items tagged in it.
type scene struct {
	id, res, title, card string
	items                []int
}

// item is a breakdown item: its URI, type and name.
type item struct{ res, dept, name string }

func newProjectWriter(w *CeltxWriter, s lex.Screenplay) *projectWriter {
	h := fnv.New64a()
	for _, l := range s {
		h.Write([]byte(l.Type + "\x00" + l.Contents + "\x00"))
	}
	return &projectWriter{w: w, seed: h.Sum64(), itemOf: map[string]int{}}
}

// id is the next id of the project: twelve letters and digits, as
// Celtx's are, the same for the same script.
func (pw *projectWriter) id(n int) string {
	const digits = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	pw.n++
	h := fnv.New64a()
	fmt.Fprintf(h, "%d/%d", pw.seed, pw.n)
	v := h.Sum64()
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[v%uint64(len(digits))]
		v /= uint64(len(digits))
		if v == 0 {
			v = h.Sum64() ^ uint64(i+1)*0x9e3779b97f4a7c15
		}
	}
	return string(b)
}

func (pw *projectWriter) res() string { return "http://celtx.com/res/" + pw.id(12) }

// titleMeta is the head's meta of a title page key, as the reader reads
// them (titleMetas).
func titleMeta(key string) string {
	for _, m := range titleMetas {
		if strings.EqualFold(m.key, key) {
			return m.meta
		}
	}
	return ""
}

// script is the script's HTML.
func (pw *projectWriter) script(s lex.Screenplay) []byte {
	title, paras := lex.Paragraphs(s)
	var b bytes.Buffer
	b.WriteString(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01//EN" "http://www.w3.org/TR/html4/strict.dtd">` + "\n<html>\n<head>\n")
	metas := map[string]string{}
	for _, f := range title {
		v := strings.TrimSpace(lex.Unstyled(f.Value))
		if strings.EqualFold(f.Key, "Title") {
			pw.title = strings.Join(strings.Fields(v), " ")
		} else if m := titleMeta(f.Key); m != "" && v != "" {
			metas[m] = strings.TrimSpace(metas[m] + "\n" + v)
		}
	}
	b.WriteString("  <title>" + html.EscapeString(pw.title) + "</title>\n")
	b.WriteString(`  <link rel="stylesheet" type="text/css" href="chrome://celtx/content/editor.css">` + "\n")
	b.WriteString(`  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">` + "\n")
	b.WriteString(`  <link href="chrome://celtx/content/style/film/USLetter/Normal.css" type="text/css" rel="stylesheet">` + "\n")
	for _, m := range titleMetas {
		if v, ok := metas[m.meta]; ok {
			b.WriteString(`  <meta content="` + attrEscape(v) + `" name="` + m.meta + `">` + "\n")
		}
	}
	b.WriteString("</head>\n<body>\n")
	inSpeech := false
	for _, p := range paras {
		for _, n := range p.Before {
			pw.notePara(&b, n)
		}
		if p.Type == lex.TypeLyrics && !inSpeech {
			p.Type = lex.TypeAction // lyrics of their own: action with their ~s
			p.Text = lyricLines(p.Text)
		} else if p.Type == lex.TypeLyrics {
			p.Text = lyricLines(p.Text)
		}
		switch p.Type {
		case lex.TypeSpeaker:
			inSpeech = true
		case lex.TypeParen, lex.TypeDialog, lex.TypeLyrics:
		default:
			inSpeech = false
		}
		pw.para(&b, p)
	}
	b.WriteString("</body>\n</html>\n")
	return b.Bytes()
}

// lyricLines are lyrics' lines with Fountain's ~, as Celtx has no
// lyrics: dialogue (in a speech) or action that reads back as lyrics.
func lyricLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = "~" + strings.TrimSpace(l)
	}
	return strings.Join(lines, "\n")
}

// notePara is a paragraph of a note alone.
func (pw *projectWriter) notePara(b *bytes.Buffer, note string) {
	if note = strings.TrimSpace(note); note == "" {
		return
	}
	b.WriteString(`<p class="action">` + pw.note(note) + "</p>\n")
}

// note is a note's span, as Celtx writes one.
func (pw *projectWriter) note(text string) string {
	date := cmp.Or(pw.w.Now, time.Now()).Format("January 2, 2006 3:04:05 PM")
	return `<span date="` + date + `" id="` + pw.id(12) + `" text="` + attrEscape(text) + `" class="note"></span>`
}

// classes are the paragraph classes of Lexington's types.
var classes = map[lex.ElementType]string{
	lex.TypeScene: "sceneheading", lex.TypeAction: "action", lex.TypeSpeaker: "character",
	lex.TypeDialog: "dialog", lex.TypeParen: "parenthetical", lex.TypeTrans: "transition",
	lex.TypeLyrics: "dialog", lex.TypeCenter: "action",
}

// para writes a paragraph.
func (pw *projectWriter) para(b *bytes.Buffer, p lex.Paragraph) {
	switch p.Type {
	case lex.TypeBoneyard:
		return
	case lex.TypeSection:
		if t := strings.TrimSpace(lex.Unstyled(p.Text)); t != "" {
			pw.notePara(b, strings.Repeat("#", max(p.Level, 1))+" "+t)
		}
		for _, n := range p.Notes {
			pw.notePara(b, n)
		}
		for _, s := range p.Synopses {
			pw.notePara(b, "= "+s)
		}
		return
	case lex.TypeSynopsis:
		pw.notePara(b, "= "+p.Text)
		return
	}
	if strings.TrimSpace(p.Text) == "" && len(p.Notes) == 0 {
		return
	}
	class, ok := classes[p.Type]
	if !ok {
		class = "action"
	}
	text := p.Text
	switch p.Type {
	case lex.TypeCenter:
		lines := strings.Split(text, "\n")
		for i, l := range lines {
			lines[i] = "> " + strings.TrimSpace(l) + " <"
		}
		text = strings.Join(lines, "\n")
	case lex.TypeSpeaker:
		if p.Dual == lex.DualSecond {
			text += " ^"
		}
	}
	var sc *scene
	open := `<p class="` + class + `">`
	if p.Type == lex.TypeScene {
		pw.scenes = append(pw.scenes, scene{id: pw.id(8), res: pw.res(), title: lex.Unstyled(text)})
		sc = &pw.scenes[len(pw.scenes)-1]
		sc.card = strings.TrimSpace(strings.Join(p.Synopses, "\n"))
		open = `<p id="` + sc.id + `" class="sceneheading">`
	} else if len(pw.scenes) > 0 {
		sc = &pw.scenes[len(pw.scenes)-1]
	}
	b.WriteString(open)
	lines := strings.Split(text, "\n")
	lineTags := pw.lineTags(p, lines)
	for i, l := range lines {
		if i > 0 {
			b.WriteString("<br>\n")
		}
		b.WriteString(pw.line(l, lineTags[i], sc))
	}
	for _, n := range p.Notes {
		if n = strings.TrimSpace(n); n != "" {
			b.WriteString(pw.note(n))
		}
	}
	b.WriteString("</p>\n")
}

// lineTags are the tags of each of a paragraph's lines: a speech's lines
// are rows of their own, with their own tags; the lines of another
// paragraph are one row, whose tags are found in the row's text and go
// on the line they are in, counted again there.
func (pw *projectWriter) lineTags(p lex.Paragraph, lines []string) [][]lex.LineTag {
	out := make([][]lex.LineTag, len(lines))
	if p.Type == lex.TypeCenter || len(pw.w.LineTags) == 0 {
		return out
	}
	if len(p.Sources) != 1 || len(lines) == 1 {
		for i := range lines {
			if i < len(p.Sources) && p.Sources[i] > 0 {
				out[i] = pw.w.LineTags[p.Sources[i]]
			}
		}
		return out
	}
	plains := make([]string, len(lines))
	for i, l := range lines {
		plains[i] = lex.Unstyled(strings.TrimSpace(l))
	}
	whole := strings.Join(plains, "\n")
	for _, t := range pw.w.LineTags[p.Sources[0]] {
		at := lex.TagIndex(whole, t.Text, t.At)
		if at < 0 {
			continue
		}
		line := strings.Count(whole[:at], "\n")
		start := strings.LastIndex(whole[:at], "\n") + 1
		t.At = strings.Count(strings.ToLower(whole[start:at]), strings.ToLower(t.Text))
		out[line] = append(out[line], t)
	}
	return out
}

// line is a line of a paragraph as HTML: its emphasis, its tags.
func (pw *projectWriter) line(text string, tags []lex.LineTag, sc *scene) string {
	spans := lex.Emphasis(strings.TrimSpace(text))
	var plain strings.Builder
	for _, s := range spans {
		plain.WriteString(s.Text)
	}
	// the tags' places: byte ranges of the plain text, and their items
	type place struct{ from, to, item int }
	var places []place
	for _, t := range tags {
		it := pw.itemFor(t.Elements)
		if it < 0 {
			continue
		}
		i := lex.TagIndex(plain.String(), t.Text, t.At)
		if i < 0 {
			continue
		}
		pl := place{i, i + len(t.Text), it}
		overlaps := false
		for _, o := range places {
			overlaps = overlaps || pl.from < o.to && o.from < pl.to
		}
		if !overlaps {
			places = append(places, pl)
			if sc != nil && !containsInt(sc.items, it) {
				sc.items = append(sc.items, it)
			}
		}
	}
	tagAt := func(off int) int {
		for k, p := range places {
			if off >= p.from && off < p.to {
				return k
			}
		}
		return -1
	}
	var b strings.Builder
	off, open := 0, -1
	for _, s := range spans {
		// cut the span where a tag begins or ends
		for len(s.Text) > 0 {
			k := tagAt(off)
			n := len(s.Text)
			for j := 1; j < len(s.Text); j++ {
				if tagAt(off+j) != k {
					n = j
					break
				}
			}
			if k != open {
				if open >= 0 {
					b.WriteString("</span>")
				}
				if k >= 0 {
					it := pw.items[places[k].item]
					b.WriteString(`<span ref="` + it.res + `" class="` + deptClass(it.dept) + `">`)
				}
				open = k
			}
			piece := lex.Span{Text: s.Text[:n], Bold: s.Bold, Italic: s.Italic, Underline: s.Underline}
			b.WriteString(styledHTML(piece))
			s.Text = s.Text[n:]
			off += n
		}
	}
	if open >= 0 {
		b.WriteString("</span>")
	}
	return b.String()
}

func containsInt(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

// itemFor is the item of the first of the elements whose category Celtx
// has a department for: made the first time; -1 none.
func (pw *projectWriter) itemFor(elements []string) int {
	if pw.w.Tags == nil {
		return -1
	}
	for _, id := range elements {
		if it, ok := pw.itemOf[id]; ok {
			return it
		}
		for _, e := range pw.w.Tags.Elements {
			if e.ID != id {
				continue
			}
			cat, ok := pw.w.Tags.Category(e.Category)
			if !ok {
				break
			}
			for _, d := range departments {
				if d.category == cat.Name {
					pw.items = append(pw.items, item{res: pw.res(), dept: d.dept, name: e.Name})
					pw.itemOf[id] = len(pw.items) - 1
					return len(pw.items) - 1
				}
			}
		}
	}
	return -1
}

func deptClass(dept string) string {
	for _, d := range departments {
		if d.dept == dept {
			return d.class
		}
	}
	return strings.ToLower(dept)
}

// styledHTML is a span's text as HTML, in <b>, <i> and <u>: a second
// space a no-break space, as Mozilla writes it.
func styledHTML(s lex.Span) string {
	t := html.EscapeString(s.Text)
	t = strings.ReplaceAll(t, "&#34;", `"`)
	t = strings.ReplaceAll(t, "&#39;", "'")
	t = strings.ReplaceAll(t, "  ", "&nbsp; ")
	if s.Underline {
		t = "<u>" + t + "</u>"
	}
	if s.Italic {
		t = "<i>" + t + "</i>"
	}
	if s.Bold {
		t = "<b>" + t + "</b>"
	}
	return t
}

// attrEscape is a text as an attribute's value in double quotes; its
// line breaks kept, as Celtx keeps them.
func attrEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;").Replace(s)
}

// rdfEscape is a text as an attribute's value in project.rdf (XML, whose
// readers make a line break in an attribute a space): its line breaks
// as character references.
func rdfEscape(s string) string {
	return strings.NewReplacer("\r", "", "\n", "&#10;", "\t", "&#9;").Replace(attrEscape(s))
}

// manifest is project.rdf: the project, its tree (a catalogue and the
// script), the script's scenes, the items tagged.
func (pw *projectWriter) manifest() []byte {
	project := "http://celtx.com/project/" + pw.id(12)
	components, root := pw.res(), pw.res()
	catalog, script := pw.res(), pw.res()
	name := cmp.Or(pw.title, "Untitled")
	var b bytes.Buffer
	seq := 0
	anon := func() string { seq++; return "rdf:#$lx" + strconv.Itoa(seq) }
	b.WriteString(`<?xml version="1.0"?>` + "\n")
	b.WriteString(`<RDF:RDF xmlns:dc="` + nsDC + `"` + "\n         " + `xmlns:cx="` + nsCX + `"` + "\n         " +
		`xmlns:RDF="` + nsRDF + `">` + "\n")
	date := cmp.Or(pw.w.Now, time.Now()).UTC().Format("2006-01-02T15:04:05Z")
	fmt.Fprintf(&b, "  <cx:Project RDF:about=\"%s\"\n                   cx:fileVersion=\"1.4\"\n                   dc:title=\"%s\"\n                   dc:modified=\"%s\">\n    <cx:components RDF:resource=\"%s\"/>\n  </cx:Project>\n",
		project, rdfEscape(name), date, components)
	fmt.Fprintf(&b, "  <RDF:Seq RDF:about=\"%s\">\n    <RDF:li RDF:resource=\"%s\"/>\n  </RDF:Seq>\n", components, root)
	fmt.Fprintf(&b, "  <RDF:Seq RDF:about=\"%s\">\n    <RDF:li RDF:resource=\"%s\"/>\n    <RDF:li RDF:resource=\"%s\"/>\n  </RDF:Seq>\n", root, catalog, script)
	fmt.Fprintf(&b, "  <RDF:Description RDF:about=\"%s\"\n                   cx:projectRoot=\"true\"\n                   dc:title=\"%s\" />\n", root, rdfEscape(name))

	// the catalogue: every item, and the items by department
	members, depts := anon(), anon()
	fmt.Fprintf(&b, "  <cx:Document RDF:about=\"%s\"\n                   dc:title=\"Master Catalog\">\n    <cx:doctype RDF:resource=\"%sCatalogDocument\"/>\n    <cx:filter RDF:resource=\"celtx:filter:all\"/>\n    <cx:members RDF:resource=\"%s\"/>\n    <cx:departments RDF:resource=\"%s\"/>\n  </cx:Document>\n",
		catalog, nsCX, members, depts)
	all := make([]int, len(pw.items))
	for i := range all {
		all[i] = i
	}
	pw.seqOf(&b, members, all)
	pw.deptLists(&b, depts, all, anon)

	// the script and its scenes
	scenes := anon()
	fmt.Fprintf(&b, "  <cx:Document RDF:about=\"%s\"\n                   dc:title=\"Screenplay\"\n                   cx:localFile=\"%s\">\n    <cx:doctype RDF:resource=\"%sScriptDocument\"/>\n    <cx:scenes RDF:resource=\"%s\"/>\n  </cx:Document>\n",
		script, scriptFile, nsCX, scenes)
	b.WriteString(`  <RDF:Seq RDF:about="` + scenes + `">` + "\n")
	for _, s := range pw.scenes {
		b.WriteString(`    <RDF:li RDF:resource="` + s.res + `"/>` + "\n")
	}
	b.WriteString("  </RDF:Seq>\n")
	for i, s := range pw.scenes {
		sm, mk := anon(), anon()
		fmt.Fprintf(&b, "  <RDF:Description RDF:about=\"%s\"\n                   cx:sceneid=\"%s\"\n                   dc:title=\"%s\"\n                   cx:ordinal=\"%d\"", s.res, s.id, rdfEscape(s.title), i+1)
		if s.card != "" {
			fmt.Fprintf(&b, "\n                   dc:description=\"%s\"", rdfEscape(s.card))
		}
		fmt.Fprintf(&b, ">\n    <cx:members RDF:resource=\"%s\"/>\n    <cx:markup RDF:resource=\"%s\"/>\n  </RDF:Description>\n", sm, mk)
		pw.deptLists(&b, sm, s.items, anon)
		pw.seqOf(&b, mk, s.items)
	}

	// the items
	for _, it := range pw.items {
		fmt.Fprintf(&b, "  <cx:%s RDF:about=\"%s\"\n                   dc:title=\"%s\" />\n", it.dept, it.res, rdfEscape(it.name))
	}
	b.WriteString("</RDF:RDF>\n")
	return b.Bytes()
}

// seqOf writes a sequence of items.
func (pw *projectWriter) seqOf(b *bytes.Buffer, about string, items []int) {
	b.WriteString(`  <RDF:Seq RDF:about="` + about + `">` + "\n")
	for _, i := range items {
		b.WriteString(`    <RDF:li RDF:resource="` + pw.items[i].res + `"/>` + "\n")
	}
	b.WriteString("  </RDF:Seq>\n")
}

// deptLists writes a sequence of the items' departments, each a sequence
// of its items and the DepartmentList it is, as Celtx keeps a scene's
// breakdown and its catalogue's.
func (pw *projectWriter) deptLists(b *bytes.Buffer, about string, items []int, anon func() string) {
	type list struct {
		about, dept string
		items       []int
	}
	var lists []list
	for _, d := range departments {
		var in []int
		for _, i := range items {
			if pw.items[i].dept == d.dept {
				in = append(in, i)
			}
		}
		if len(in) > 0 {
			lists = append(lists, list{anon(), d.dept, in})
		}
	}
	b.WriteString(`  <RDF:Seq RDF:about="` + about + `">` + "\n")
	for _, l := range lists {
		b.WriteString(`    <RDF:li RDF:resource="` + l.about + `"/>` + "\n")
	}
	b.WriteString("  </RDF:Seq>\n")
	for _, l := range lists {
		pw.seqOf(b, l.about, l.items)
		fmt.Fprintf(b, "  <cx:DepartmentList RDF:about=\"%s\"\n                   cx:size=\"%d\">\n    <cx:department RDF:resource=\"%s%s\"/>\n  </cx:DepartmentList>\n",
			l.about, len(l.items), nsCX, l.dept)
	}
}
