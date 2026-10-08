package office

import (
	"archive/zip"
	"fmt"
	"hash/crc32"
	"io"
	"strings"

	"github.com/LaPingvino/lexington/layout"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// ODTWriter writes an OpenDocument text (LibreOffice, Word, Google Docs).
type ODTWriter struct {
	Elements rules.Set // the element rules; rules.Default if nil
	Page     string    // "letter" (the default), "a4" or "a5"
}

func (w *ODTWriter) Write(out io.Writer, s lex.Screenplay) error {
	d := prepare(s, w.Elements, GetPage(w.Page))
	z := zip.NewWriter(out)
	// the mimetype first, uncompressed and without a data descriptor (as
	// CreateHeader would add), or LibreOffice does not load the file
	const mimetype = "application/vnd.oasis.opendocument.text"
	f, err := z.CreateRaw(&zip.FileHeader{Name: "mimetype", Method: zip.Store,
		CRC32: crc32.ChecksumIEEE([]byte(mimetype)), CompressedSize64: uint64(len(mimetype)),
		UncompressedSize64: uint64(len(mimetype))})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, mimetype); err != nil {
		return err
	}
	content, auto := d.odtContent()
	for _, p := range []struct{ name, body string }{
		{"META-INF/manifest.xml", odtManifest},
		{"meta.xml", odtMeta(d.title)},
		{"styles.xml", d.odtStyles()},
		{"content.xml", odtDocument("content", "", auto, `<office:body><office:text>`+content+`</office:text></office:body>`)},
	} {
		f, err := z.Create(p.name)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			return err
		}
	}
	return z.Close()
}

const odtNS = `xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
	`xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" ` +
	`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" ` +
	`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
	`xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0" ` +
	`xmlns:svg="urn:oasis:names:tc:opendocument:xmlns:svg-compatible:1.0" ` +
	`xmlns:dc="http://purl.org/dc/elements/1.1/" ` +
	`xmlns:meta="urn:oasis:names:tc:opendocument:xmlns:meta:1.0" office:version="1.3"`

const odtManifest = xmlHeader + `<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.3">` +
	`<manifest:file-entry manifest:full-path="/" manifest:version="1.3" manifest:media-type="application/vnd.oasis.opendocument.text"/>` +
	`<manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>` +
	`<manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/>` +
	`<manifest:file-entry manifest:full-path="meta.xml" manifest:media-type="text/xml"/>` +
	`</manifest:manifest>`

func odtMeta(title string) string {
	return xmlHeader + `<office:document-meta ` + odtNS + `><office:meta><dc:title>` + esc(title) +
		`</dc:title><meta:generator>Lexington</meta:generator></office:meta></office:document-meta>`
}

// odtDocument is a content.xml or styles.xml, in ODF's order: the fonts,
// the (named) styles, the automatic styles, then the body or master pages.
func odtDocument(kind, styles, auto, body string) string {
	return xmlHeader + `<office:document-` + kind + ` ` + odtNS + `>` +
		`<office:font-face-decls><style:font-face style:name="Courier Prime" svg:font-family="'Courier Prime'" ` +
		`style:font-family-generic="modern" style:font-pitch="fixed"/></office:font-face-decls>` +
		styles + `<office:automatic-styles>` + auto + `</office:automatic-styles>` + body + `</office:document-` + kind + `>`
}

func in(v float64) string { return fmt.Sprintf("%.3fin", v) }

func odtAlign(a byte) string {
	switch a {
	case 'C':
		return "center"
	case 'R':
		return "end"
	}
	return "start"
}

// odtText is a text style's properties; with all, also the ones that
// switch a style off (for spans that undo their paragraph's style).
func odtText(bold, italic, underline, all bool) string {
	var b strings.Builder
	if bold {
		b.WriteString(` fo:font-weight="bold"`)
	} else if all {
		b.WriteString(` fo:font-weight="normal"`)
	}
	if italic {
		b.WriteString(` fo:font-style="italic"`)
	} else if all {
		b.WriteString(` fo:font-style="normal"`)
	}
	if underline {
		b.WriteString(` style:text-underline-style="solid" style:text-underline-width="auto" style:text-underline-color="font-color"`)
	} else if all {
		b.WriteString(` style:text-underline-style="none"`)
	}
	return b.String()
}

func (d *doc) odtStyles() string {
	var b strings.Builder
	b.WriteString(`<office:styles><style:default-style style:family="paragraph">` +
		`<style:paragraph-properties fo:line-height="12pt" fo:margin-top="0in" fo:margin-bottom="0in"/>` +
		`<style:text-properties style:font-name="Courier Prime" fo:font-size="12pt"/></style:default-style>` +
		`<style:style style:name="Standard" style:family="paragraph" style:class="text"/>`)
	for _, st := range d.styles {
		var props, text string
		if st.KeepNext {
			props = ` fo:keep-with-next="always"`
		}
		text = odtText(st.Bold, st.Italic, st.Underline, false)
		if st.Font != "Courier Prime" {
			text += fmt.Sprintf(` style:font-name="%s"`, esc(st.Font))
		}
		if sz := size(st.Size); sz != 12 {
			text += fmt.Sprintf(` fo:font-size="%gpt"`, sz)
		}
		fmt.Fprintf(&b, `<style:style style:name="%s" style:display-name="%s" style:family="paragraph" style:parent-style-name="Standard">`+
			`<style:paragraph-properties fo:margin-left="%s" fo:margin-right="%s" fo:text-align="%s"%s/>`+
			`<style:text-properties%s/></style:style>`,
			st.ID, esc(st.Name), in(st.Left), in(st.Right), odtAlign(st.Align), props, text)
	}
	b.WriteString(`<style:style style:name="Header" style:family="paragraph" style:parent-style-name="Standard">` +
		`<style:paragraph-properties fo:text-align="end"/></style:style></office:styles>`)
	l, r := d.pageMargins()
	auto := fmt.Sprintf(`<style:page-layout style:name="pm1"><style:page-layout-properties fo:page-width="%s" fo:page-height="%s" `+
		`fo:margin-top="%s" fo:margin-bottom="%s" fo:margin-left="%s" fo:margin-right="%s"/>`+
		`<style:header-style><style:header-footer-properties fo:min-height="0in" fo:margin-bottom="%s"/></style:header-style></style:page-layout>`,
		in(d.page.Width), in(d.page.Height), in(0.5*d.vscale), in(d.vscale), in(l), in(r), in(0.25*d.vscale))
	// the page number at the top right ("1."), not on the title page
	master := `<office:master-styles><style:master-page style:name="Standard" style:page-layout-name="pm1">` +
		`<style:header><text:p text:style-name="Header"><text:page-number text:select-page="current"/>.</text:p></style:header></style:master-page>` +
		`<style:master-page style:name="First_20_Page" style:display-name="First Page" style:page-layout-name="pm1" style:next-style-name="Standard"/>` +
		`</office:master-styles>`
	return odtDocument("styles", b.String(), auto, master)
}

// odtContent is the document's body and the automatic styles it uses.
func (d *doc) odtContent() (string, string) {
	var body, auto strings.Builder
	autos := map[string]bool{}
	addAuto := func(name, def string) {
		if !autos[name] {
			autos[name] = true
			auto.WriteString(def)
		}
	}
	first, numbered, n := true, false, 0
	para := func(p layout.Paragraph, breakBefore bool, spaceBefore float64) {
		st := d.byKey[p.Key]
		parent := "Standard"
		if st != nil {
			parent = st.ID
		}
		name := parent
		var props, master, tabs string
		if breakBefore {
			props += ` fo:break-before="page"`
			if d.titlePage && !numbered { // the script after the title page starts at 1
				numbered = true
				props += ` style:page-number="1"`
				master = ` style:master-page-name="Standard"`
			}
		}
		if spaceBefore > 0 {
			props += ` fo:margin-top="` + in(spaceBefore*d.vscale) + `"`
		}
		if first && d.titlePage {
			master = ` style:master-page-name="First_20_Page"`
		}
		if p.SceneNumber != "" && st != nil {
			tabs = `<style:tab-stops><style:tab-stop style:position="` + in(d.textWidth()-st.Left-st.Right) +
				`" style:type="right"/></style:tab-stops>`
		}
		if props != "" || master != "" || tabs != "" {
			n++
			name = fmt.Sprintf("P%d", n)
			addAuto(name, fmt.Sprintf(`<style:style style:name="%s" style:family="paragraph" style:parent-style-name="%s"%s>`+
				`<style:paragraph-properties%s>%s</style:paragraph-properties></style:style>`, name, parent, master, props, tabs))
		}
		first = false
		fmt.Fprintf(&body, `<text:p text:style-name="%s">`, name)
		for _, sp := range p.Spans {
			t := odtEsc(sp.Text)
			if st != nil && (sp.Bold != st.Bold || sp.Italic != st.Italic || sp.Underline != st.Underline) {
				ts := fmt.Sprintf("T_%t_%t_%t", sp.Bold, sp.Italic, sp.Underline)
				addAuto(ts, `<style:style style:name="`+ts+`" style:family="text"><style:text-properties`+
					odtText(sp.Bold, sp.Italic, sp.Underline, true)+`/></style:style>`)
				fmt.Fprintf(&body, `<text:span text:style-name="%s">%s</text:span>`, ts, t)
			} else {
				body.WriteString(t)
			}
		}
		if p.SceneNumber != "" {
			body.WriteString(`<text:tab/>` + odtEsc(p.SceneNumber))
		}
		body.WriteString(`</text:p>`)
	}
	tables := 0
	for _, bl := range d.blocks {
		if bl.P.Column == 0 {
			para(bl.P, bl.BreakBefore, bl.SpaceBefore)
			continue
		}
		if bl.BreakBefore {
			para(layout.Paragraph{}, true, 0)
		}
		indent, col, gap := d.dualTable()
		addAuto("Dual", fmt.Sprintf(`<style:style style:name="Dual" style:family="table"><style:table-properties style:width="%s" fo:margin-left="%s" table:align="left"/></style:style>`+
			`<style:style style:name="Dual.A" style:family="table-column"><style:table-column-properties style:column-width="%s"/></style:style>`+
			`<style:style style:name="Dual.B" style:family="table-column"><style:table-column-properties style:column-width="%s"/></style:style>`+
			`<style:style style:name="Dual.cell" style:family="table-cell"><style:table-cell-properties fo:padding="0in" fo:border="none"/></style:style>`,
			in(2*col+gap), in(indent), in(col), in(gap)))
		tables++
		fmt.Fprintf(&body, `<table:table table:name="Dual%d" table:style-name="Dual">`+
			`<table:table-column table:style-name="Dual.A"/><table:table-column table:style-name="Dual.B"/>`+
			`<table:table-column table:style-name="Dual.A"/><table:table-row>`, tables)
		for _, ps := range [][]layout.Paragraph{bl.Dual[0], nil, bl.Dual[1]} {
			body.WriteString(`<table:table-cell table:style-name="Dual.cell" office:value-type="string">`)
			if len(ps) == 0 {
				body.WriteString(`<text:p/>`)
			}
			for _, p := range ps {
				para(p, false, 0)
			}
			body.WriteString(`</table:table-cell>`)
		}
		body.WriteString(`</table:table-row></table:table>`)
	}
	return body.String(), auto.String()
}

// odtEsc escapes text and keeps runs of spaces, which ODF would collapse.
func odtEsc(s string) string {
	s = esc(s)
	var b strings.Builder
	spaces := 0
	flush := func() {
		if spaces > 0 {
			b.WriteByte(' ')
			if spaces > 1 {
				fmt.Fprintf(&b, `<text:s text:c="%d"/>`, spaces-1)
			}
			spaces = 0
		}
	}
	for _, r := range s {
		if r == ' ' {
			spaces++
			continue
		}
		flush()
		b.WriteRune(r)
	}
	flush()
	return b.String()
}
