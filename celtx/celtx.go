// Package celtx reads and writes Celtx's projects (.celtx), as the
// desktop Celtx (2.x, on Mozilla's XUL, given up around 2012) saves
// them: a zip with project.rdf, an RDF/XML catalogue of the project's
// documents and breakdown, and each script as HTML (script-*.html), a
// paragraph a <p> whose class is its element. The format is as a project
// Celtx saved shows it; no code of Celtx's (CePL) or of Trelby's Celtx
// import (GPL) is in this package.
//
// The script's paragraphs are sceneheading, action, character, dialog,
// parenthetical, transition, shot (read as action, in capitals) and act
// (a stage play's act break, read as a first-level section); a class of
// another kind ("text", "") is action. Bold, italics and underline are
// <b>, <i> and <u> (or a span's style); a line break in a paragraph is
// <br>, and the HTML's own line breaks are spaces. A note is an empty
// <span class="note" text="…">, read at the end of its paragraph's line
// (a paragraph of notes alone is a line of notes); an attachment
// (<span class="media">) is passed over. The title page is the head's:
// <title>, the Author meta, CX.byline (the credit), DC.source, DC.rights
// (the copyright) and CX.contact. A scene's index card text (the
// scene's dc:description in project.rdf, matched by the heading's id) is
// its synopsis. Celtx numbers the scenes itself (scenenumber), so the
// numbers it writes are not read, and none are written.
//
// Breakdown tags are the script's <span ref="…" class="cast">: words
// tagged as the catalogue's item the ref names, of the department the
// item's type is (cx:Cast, cx:Props, cx:Sound…). They are read as
// Lexington's breakdown (lex.TagSet, ParseTags), as Final Draft's are.
// The writer writes the elements of the three departments a project
// Celtx saved shows, Cast, Props and Sound, and leaves the words of the
// other categories plain: their departments' names in Celtx are not
// known here.
//
// Celtx has no dual dialogue, no centred line, no lyrics and no outline,
// so they are kept as Trelby's are (package trelby): the second speaker
// of dual dialogue with a caret ("BOB ^"), a centred line as action with
// Fountain's markers ("> THE END <"), lyrics as dialogue. A section is a
// note of its #s ("# Act One"), a synopsis not on a scene heading a note
// "= …"; both read back as what they were. Page breaks, boneyards,
// revisions and the title page's other fields (Draft date, Notes) have no
// place in Celtx and are left out.
//
// A project holds several documents, a script among them or more than
// one (a screenplay and its stage adaptation): the reader reads the first
// script in the project's tree, or the one asked for (Project.Script).
// The writer writes a project of one screenplay, its catalogue and its
// scenes. Whether desktop Celtx opens what it writes is *unverified*: it
// follows what Celtx itself writes, but no Celtx has opened it yet.
package celtx

import (
	"archive/zip"
	"bytes"
	"cmp"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

// The namespaces of project.rdf.
const (
	nsRDF = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	nsCX  = "http://celtx.com/NS/v1/"
	nsDC  = "http://purl.org/dc/elements/1.1/"
)

// ManifestName is the name of the project's catalogue in the zip.
const ManifestName = "project.rdf"

// node is a subject of project.rdf: its types (cx:Document, RDF:Seq;
// the namespace and the name), its properties by "namespace name" (a
// resource's URI or a literal), and a sequence's items, in order. One
// subject can be in several elements (a Seq and the Folder it is): they
// are one node.
type node struct {
	types []string
	props map[string][]string
	items []string
	order int
}

func (n *node) prop(ns, name string) string {
	if v := n.props[ns+" "+name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (n *node) is(ns, name string) bool {
	for _, t := range n.types {
		if t == ns+" "+name {
			return true
		}
	}
	return false
}

// typeIn is the node's type's name in namespace ns ("" for none).
func (n *node) typeIn(ns string) string {
	for _, t := range n.types {
		if name, ok := strings.CutPrefix(t, ns+" "); ok {
			return name
		}
	}
	return ""
}

// graph is project.rdf's subjects by their URI (RDF:about).
type graph map[string]*node

func (g graph) get(about string) *node {
	n := g[about]
	if n == nil {
		n = &node{props: map[string][]string{}, order: len(g)}
		g[about] = n
	}
	return n
}

// parseRDF reads RDF/XML as Celtx writes it: the subjects at the top,
// their literals as attributes or elements, their resources as elements
// with RDF:resource, a sequence's items as RDF:li.
func parseRDF(data []byte) (graph, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	g := graph{}
	depth := 0
	var subject *node
	var prop *xml.StartElement
	var text strings.Builder
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch depth {
			case 1:
				if t.Name.Space != nsRDF || t.Name.Local != "RDF" {
					return nil, errors.New("not RDF: no RDF:RDF")
				}
			case 2:
				about := attr(t, nsRDF, "about")
				if about == "" {
					about = "#" + attr(t, nsRDF, "ID")
				}
				subject = g.get(about)
				if !(t.Name.Space == nsRDF && t.Name.Local == "Description") {
					subject.types = append(subject.types, t.Name.Space+" "+t.Name.Local)
				}
				for _, a := range t.Attr {
					if a.Name.Space != nsRDF && a.Name.Space != "xmlns" && a.Name.Space != "" {
						k := a.Name.Space + " " + a.Name.Local
						subject.props[k] = append(subject.props[k], a.Value)
					}
				}
			case 3:
				p := t.Copy()
				prop = &p
				text.Reset()
			}
		case xml.CharData:
			if depth == 3 {
				text.Write(t)
			}
		case xml.EndElement:
			if depth == 3 && prop != nil && subject != nil {
				v := attr(*prop, nsRDF, "resource")
				if v == "" {
					v = text.String()
				}
				if prop.Name.Space == nsRDF && prop.Name.Local == "li" {
					subject.items = append(subject.items, v)
				} else {
					k := prop.Name.Space + " " + prop.Name.Local
					subject.props[k] = append(subject.props[k], v)
				}
				prop = nil
			}
			depth--
		}
	}
	return g, nil
}

func attr(t xml.StartElement, ns, name string) string {
	for _, a := range t.Attr {
		if a.Name.Local == name && (a.Name.Space == ns || ns == "") {
			return a.Value
		}
	}
	return ""
}

// Script is one of a project's scripts.
type Script struct {
	// Title is its name in the project ("Screenplay").
	Title string
	// File is its HTML's name in the zip.
	File string
	// Kind is its document's type in Celtx, without "Document": Script
	// (a screenplay), Theatre (a stage play), and Celtx's others.
	Kind string
	res  string // its URI in project.rdf
}

// Project is a Celtx project as read: its title and scripts.
type Project struct {
	Title   string
	Scripts []Script
	files   map[string]*zip.File
	g       graph
}

// IsProject tells whether a file's bytes are a Celtx project: a zip with
// a project.rdf.
func IsProject(data []byte) bool {
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return false
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range z.File {
		if f.Name == ManifestName {
			return true
		}
	}
	return false
}

// ReadProject reads a project's zip: its catalogue and the scripts in it,
// in the order of the project's tree (folders in their place), then any
// script the tree does not hold.
func ReadProject(data []byte) (*Project, error) {
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a Celtx project (a zip): %w", err)
	}
	p := &Project{files: map[string]*zip.File{}}
	for _, f := range z.File {
		p.files[f.Name] = f
	}
	mf, ok := p.files[ManifestName]
	if !ok {
		return nil, errors.New("not a Celtx project: no " + ManifestName)
	}
	man, err := readFile(mf)
	if err != nil {
		return nil, err
	}
	if p.g, err = parseRDF(man); err != nil {
		return nil, fmt.Errorf("could not read %s: %w", ManifestName, err)
	}
	seen := map[string]bool{}
	var walk func(res string, depth int)
	walk = func(res string, depth int) {
		n, ok := p.g[res]
		if !ok || seen[res] || depth > 32 {
			return
		}
		seen[res] = true
		if s, ok := p.script(res, n); ok {
			p.Scripts = append(p.Scripts, s)
		}
		for _, it := range n.items {
			walk(it, depth+1)
		}
	}
	var subjects []string
	for res := range p.g {
		subjects = append(subjects, res)
	}
	sort.Slice(subjects, func(i, j int) bool { return p.g[subjects[i]].order < p.g[subjects[j]].order })
	for _, res := range subjects {
		n := p.g[res]
		if !n.is(nsCX, "Project") {
			continue
		}
		p.Title = n.prop(nsDC, "title")
		for _, c := range n.props[nsCX+" components"] {
			walk(c, 0)
		}
	}
	for _, res := range subjects { // the scripts outside the tree
		if !seen[res] {
			if s, ok := p.script(res, p.g[res]); ok {
				seen[res] = true
				p.Scripts = append(p.Scripts, s)
			}
		}
	}
	if len(p.Scripts) == 0 { // no catalogue of them: the script files
		var names []string
		for name := range p.files {
			if isScriptFile(name) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			p.Scripts = append(p.Scripts, Script{Title: strings.TrimSuffix(name, ".html"), File: name, Kind: "Script"})
		}
	}
	if len(p.Scripts) == 0 {
		return nil, errors.New("the Celtx project holds no script")
	}
	return p, nil
}

// script is the node as a script, if it is one in the zip: a document of
// a script's type, or with a script's file.
func (p *Project) script(res string, n *node) (Script, bool) {
	file := n.prop(nsCX, "localFile")
	if _, ok := p.files[file]; !ok {
		return Script{}, false
	}
	kind := strings.TrimPrefix(n.prop(nsCX, "doctype"), nsCX)
	switch kind {
	case "ScriptDocument", "TheatreDocument":
	default:
		if !isScriptFile(file) {
			return Script{}, false
		}
	}
	return Script{Title: n.prop(nsDC, "title"), File: file, Kind: strings.TrimSuffix(cmp.Or(kind, "Script"), "Document"), res: res}, true
}

func isScriptFile(name string) bool {
	base := path.Base(name)
	return strings.HasPrefix(base, "script") && strings.HasSuffix(base, ".html")
}

// Script is the script pick names: its number in Scripts, from 1, or its
// title or file name (any case); "" is the first.
func (p *Project) Script(pick string) (Script, error) {
	pick = strings.TrimSpace(pick)
	if pick == "" {
		return p.Scripts[0], nil
	}
	if n, err := strconv.Atoi(pick); err == nil {
		if n < 1 || n > len(p.Scripts) {
			return Script{}, fmt.Errorf("the project has %d scripts, not a script %d", len(p.Scripts), n)
		}
		return p.Scripts[n-1], nil
	}
	for _, s := range p.Scripts {
		if strings.EqualFold(s.Title, pick) || strings.EqualFold(s.File, pick) {
			return s, nil
		}
	}
	var names []string
	for _, s := range p.Scripts {
		names = append(names, strconv.Quote(s.Title))
	}
	return Script{}, fmt.Errorf("no script %q in the project: it has %s", pick, strings.Join(names, ", "))
}

func readFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
