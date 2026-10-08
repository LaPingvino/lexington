package office

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/LaPingvino/lexington/layout"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// DOCXWriter writes a Word document (Office Open XML).
type DOCXWriter struct {
	Elements rules.Set // the element rules; rules.Default if nil
	Page     string    // "letter" (the default), "a4" or "a5"
}

func twips(in float64) int { return int(math.Round(in * 1440)) }

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (w *DOCXWriter) Write(out io.Writer, s lex.Screenplay) error {
	d := prepare(s, w.Elements, GetPage(w.Page))
	z := zip.NewWriter(out)
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRels},
		{"word/_rels/document.xml.rels", docxDocumentRels},
		{"word/fontTable.xml", docxFontTable},
		{"docProps/core.xml", docxCore(d.title)},
		{"word/styles.xml", d.docxStyles()},
		{"word/header1.xml", docxHeader},
		{"word/document.xml", d.docxDocument()},
	}
	for _, p := range parts {
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

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n"

const docxContentTypes = xmlHeader + `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/>` +
	`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
	`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
	`<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>` +
	`<Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/>` +
	`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
	`</Types>`

const docxRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
	`</Relationships>`

const docxDocumentRels = xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
	`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
	`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>` +
	`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable" Target="fontTable.xml"/>` +
	`</Relationships>`

// Courier Prime as a fixed-pitch modern font, so that a word processor
// without it puts another monospaced font in its place
const docxFontTable = xmlHeader + `<w:fonts ` + wNS + `><w:font w:name="Courier Prime"><w:altName w:val="Courier New"/>` +
	`<w:family w:val="modern"/><w:pitch w:val="fixed"/></w:font></w:fonts>`

const wNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

// the page number at the top right, as in the PDF ("1.")
const docxHeader = xmlHeader + `<w:hdr ` + wNS + `><w:p><w:pPr><w:jc w:val="right"/></w:pPr>` +
	`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>` +
	`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>` +
	`<w:r><w:t>.</w:t></w:r></w:p></w:hdr>`

func docxCore(title string) string {
	return xmlHeader + `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + esc(title) + `</dc:title>` +
		`<dc:creator>Lexington</dc:creator></cp:coreProperties>`
}

func onOff(name string, on bool) string {
	if on {
		return "<w:" + name + "/>"
	}
	return `<w:` + name + ` w:val="0"/>`
}

func docxJc(a byte) string {
	switch a {
	case 'C':
		return `<w:jc w:val="center"/>`
	case 'R':
		return `<w:jc w:val="right"/>`
	}
	return ""
}

func (d *doc) docxStyles() string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<w:styles ` + wNS + `>`)
	b.WriteString(`<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Courier Prime" w:hAnsi="Courier Prime" w:cs="Courier Prime" w:eastAsia="Courier Prime"/>` +
		`<w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr></w:rPrDefault>` +
		`<w:pPrDefault><w:pPr><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="exact"/></w:pPr></w:pPrDefault></w:docDefaults>`)
	b.WriteString(`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>`)
	for _, st := range d.styles {
		fmt.Fprintf(&b, `<w:style w:type="paragraph" w:customStyle="1" w:styleId="%s"><w:name w:val="%s"/>`+
			`<w:basedOn w:val="Normal"/><w:qFormat/><w:pPr>`, st.ID, esc(st.Name))
		if st.KeepNext {
			b.WriteString(`<w:keepNext/>`)
		}
		fmt.Fprintf(&b, `<w:ind w:left="%d" w:right="%d"/>%s</w:pPr><w:rPr>`, twips(st.Left), twips(st.Right), docxJc(st.Align))
		if st.Font != "Courier Prime" {
			fmt.Fprintf(&b, `<w:rFonts w:ascii="%[1]s" w:hAnsi="%[1]s" w:cs="%[1]s"/>`, esc(st.Font))
		}
		if st.Bold {
			b.WriteString(`<w:b/>`)
		}
		if st.Italic {
			b.WriteString(`<w:i/>`)
		}
		if st.Underline {
			b.WriteString(`<w:u w:val="single"/>`)
		}
		if sz := size(st.Size); sz != 12 {
			fmt.Fprintf(&b, `<w:sz w:val="%d"/>`, int(sz*2))
		}
		b.WriteString(`</w:rPr></w:style>`)
	}
	b.WriteString(`</w:styles>`)
	return b.String()
}

func (d *doc) docxDocument() string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<w:document ` + wNS + `><w:body>`)
	lastTable := false
	for _, bl := range d.blocks {
		if bl.P.Column != 0 {
			d.docxDual(&b, bl)
			lastTable = true
			continue
		}
		d.docxParagraph(&b, bl.P, bl.BreakBefore, bl.SpaceBefore)
		lastTable = false
	}
	if lastTable {
		b.WriteString(`<w:p/>`)
	}
	l, r := d.pageMargins()
	fmt.Fprintf(&b, `<w:sectPr><w:headerReference w:type="default" r:id="rId2"/>`+
		`<w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="%d" w:footer="%d" w:gutter="0"/>`,
		twips(d.page.Width), twips(d.page.Height), twips(d.vscale), twips(r), twips(d.vscale), twips(l),
		twips(0.5*d.vscale), twips(0.5*d.vscale))
	if d.titlePage { // no number on the title page; the script starts at 1
		b.WriteString(`<w:pgNumType w:start="0"/><w:titlePg/>`)
	}
	b.WriteString(`</w:sectPr></w:body></w:document>`)
	return b.String()
}

func (d *doc) docxParagraph(b *strings.Builder, p layout.Paragraph, breakBefore bool, spaceBefore float64) {
	b.WriteString(`<w:p>`)
	st := d.byKey[p.Key]
	if st != nil || breakBefore || spaceBefore > 0 {
		b.WriteString(`<w:pPr>`)
		if st != nil {
			fmt.Fprintf(b, `<w:pStyle w:val="%s"/>`, st.ID)
		}
		if breakBefore {
			b.WriteString(`<w:pageBreakBefore/>`)
		}
		if p.SceneNumber != "" && st != nil {
			fmt.Fprintf(b, `<w:tabs><w:tab w:val="right" w:pos="%d"/></w:tabs>`, twips(d.textWidth()-st.Right))
		}
		if spaceBefore > 0 {
			fmt.Fprintf(b, `<w:spacing w:before="%d"/>`, twips(spaceBefore*d.vscale))
		}
		b.WriteString(`</w:pPr>`)
	}
	for _, sp := range p.Spans {
		b.WriteString(`<w:r>`)
		if st != nil && (sp.Bold != st.Bold || sp.Italic != st.Italic || sp.Underline != st.Underline) {
			b.WriteString(`<w:rPr>`)
			if sp.Bold != st.Bold {
				b.WriteString(onOff("b", sp.Bold))
			}
			if sp.Italic != st.Italic {
				b.WriteString(onOff("i", sp.Italic))
			}
			if sp.Underline != st.Underline {
				if sp.Underline {
					b.WriteString(`<w:u w:val="single"/>`)
				} else {
					b.WriteString(`<w:u w:val="none"/>`)
				}
			}
			b.WriteString(`</w:rPr>`)
		}
		fmt.Fprintf(b, `<w:t xml:space="preserve">%s</w:t></w:r>`, esc(sp.Text))
	}
	if p.SceneNumber != "" {
		fmt.Fprintf(b, `<w:r><w:tab/><w:t>%s</w:t></w:r>`, esc(p.SceneNumber))
	}
	b.WriteString(`</w:p>`)
}

func (d *doc) docxDual(b *strings.Builder, bl block) {
	if bl.BreakBefore {
		b.WriteString(`<w:p><w:pPr><w:pageBreakBefore/></w:pPr></w:p>`)
	}
	indent, col, gap := d.dualTable()
	fmt.Fprintf(b, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblInd w:w="%d" w:type="dxa"/>`+
		`<w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar>`+
		`</w:tblPr><w:tblGrid><w:gridCol w:w="%[3]d"/><w:gridCol w:w="%[4]d"/><w:gridCol w:w="%[3]d"/></w:tblGrid><w:tr>`,
		twips(2*col+gap), twips(indent), twips(col), twips(gap))
	cell := func(w float64, ps []layout.Paragraph) {
		fmt.Fprintf(b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/></w:tcPr>`, twips(w))
		if len(ps) == 0 {
			b.WriteString(`<w:p/>`)
		}
		for _, p := range ps {
			d.docxParagraph(b, p, false, 0)
		}
		b.WriteString(`</w:tc>`)
	}
	cell(col, bl.Dual[0])
	cell(gap, nil)
	cell(col, bl.Dual[1])
	b.WriteString(`</w:tr></w:tbl>`)
}
