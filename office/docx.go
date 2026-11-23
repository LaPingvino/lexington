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
	"github.com/LaPingvino/lexington/titlepage"
)

// DOCXWriter writes a Word document (Office Open XML).
type DOCXWriter struct {
	Elements rules.Set // the element rules; rules.Default if nil
	Page     string    // "letter" (the default), "a4" or "a5"
	// TitlePage lays the title page out (nil: Fountain's); TitlePictures
	// are its pictures' files, by the names its fields give.
	TitlePage     *titlepage.Template
	TitlePictures map[string][]byte
}

func twips(in float64) int { return int(math.Round(in * 1440)) }

func esc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

func (w *DOCXWriter) Write(out io.Writer, s lex.Screenplay) error {
	d := prepare(s, w.Elements, GetPage(w.Page), w.TitlePage, w.TitlePictures)
	z := zip.NewWriter(out)
	parts := []struct{ name, body string }{
		{"[Content_Types].xml", docxContentTypes},
		{"_rels/.rels", docxRels},
		{"word/_rels/document.xml.rels", d.docxDocumentRels()},
		{"word/fontTable.xml", docxFontTable},
		{"docProps/core.xml", docxCore(d.title)},
		{"word/styles.xml", d.docxStyles()},
		{"word/header1.xml", docxHeader},
		{"word/document.xml", d.docxDocument()},
	}
	for _, pic := range d.pictureList() {
		parts = append(parts, struct{ name, body string }{"word/media/" + pic.name, string(pic.data)})
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
	`<Default Extension="png" ContentType="image/png"/>` +
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

// docxDocumentRels are the document's parts: its styles, header, fonts
// and the title page's pictures (rIdImg1, …).
func (d *doc) docxDocumentRels() string {
	var b strings.Builder
	b.WriteString(xmlHeader + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable" Target="fontTable.xml"/>`)
	for _, pic := range d.pictureList() {
		fmt.Fprintf(&b, `<Relationship Id="rId%s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`,
			strings.TrimSuffix(pic.name, ".png"), pic.name)
	}
	b.WriteString(`</Relationships>`)
	return b.String()
}

// docxPicture is a title page picture's paragraph: the picture inline,
// as large as the template has it.
func (d *doc) docxPicture(b *strings.Builder, bl block, pic *picture) {
	b.WriteString(`<w:p><w:pPr>`)
	if bl.BreakBefore {
		b.WriteString(`<w:pageBreakBefore/>`)
	}
	fmt.Fprintf(b, `<w:spacing w:before="%d" w:line="240" w:lineRule="auto"/><w:ind w:left="0" w:right="0"/>%s</w:pPr>`,
		twips(bl.SpaceBefore*d.vscale), docxJc(bl.P.Align))
	cx, cy := int(pic.width*914400), int(pic.height*914400)
	id := strings.TrimSuffix(strings.TrimPrefix(pic.name, "image"), ".png")
	fmt.Fprintf(b, `<w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%[1]d" cy="%[2]d"/>`+
		`<wp:docPr id="%[3]s" name="Picture %[3]s" descr="%[4]s"/>`+
		`<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">`+
		`<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%[3]s" name="%[5]s"/><pic:cNvPicPr/></pic:nvPicPr>`+
		`<pic:blipFill><a:blip r:embed="rIdimage%[3]s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>`+
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%[1]d" cy="%[2]d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>`+
		`</pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`, cx, cy, id, esc(pic.alt), pic.name)
}

// Courier Prime as a fixed-pitch modern font, so that a word processor
// without it puts another monospaced font in its place
const docxFontTable = xmlHeader + `<w:fonts ` + wNS + `><w:font w:name="Courier Prime"><w:altName w:val="Courier New"/>` +
	`<w:family w:val="modern"/><w:pitch w:val="fixed"/></w:font></w:fonts>`

const wNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"`

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
		`<w:pPrDefault><w:pPr>` + onOff("widowControl", d.pagination.Widows) + `<w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="exact"/></w:pPr></w:pPrDefault></w:docDefaults>`)
	b.WriteString(`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/></w:style>`)
	for _, st := range d.styles {
		fmt.Fprintf(&b, `<w:style w:type="paragraph" w:customStyle="1" w:styleId="%s"><w:name w:val="%s"/>`+
			`<w:basedOn w:val="Normal"/><w:qFormat/><w:pPr>`, st.ID, esc(st.Name))
		if st.KeepNext {
			b.WriteString(`<w:keepNext/>`)
		}
		if st.KeepLines {
			b.WriteString(`<w:keepLines/>`)
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
	for i, bl := range d.blocks {
		if bl.P.Column != 0 {
			d.docxDual(&b, bl)
			lastTable = true
			continue
		}
		if pic := d.pictures[i]; pic != nil {
			d.docxPicture(&b, bl, pic)
			lastTable = false
			continue
		}
		d.docxParagraph(&b, bl.P, bl.BreakBefore, bl.SpaceBefore, bl.KeepNext)
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

func (d *doc) docxParagraph(b *strings.Builder, p layout.Paragraph, breakBefore bool, spaceBefore float64, keepNext bool) {
	b.WriteString(`<w:p>`)
	st := d.byKey[p.Key]
	bar := p.Bar != "" && p.Column == 0
	ownAlign := p.TitleField != nil && st != nil && p.Align != st.Align
	if st != nil || breakBefore || spaceBefore > 0 || bar || p.Background != "" || keepNext {
		b.WriteString(`<w:pPr>`)
		if st != nil {
			fmt.Fprintf(b, `<w:pStyle w:val="%s"/>`, st.ID)
		}
		if keepNext {
			b.WriteString(`<w:keepNext/>`)
		}
		if breakBefore {
			b.WriteString(`<w:pageBreakBefore/>`)
		}
		if bar { // a left border: Word draws it before the paragraph's indent
			// (in its color's pattern: the nearest of Word's borders)
			style := "single"
			if p.Pattern != "" {
				style = lex.BarPattern(p.Pattern).Name
			}
			fmt.Fprintf(b, `<w:pBdr><w:left w:val="%s" w:sz="%d" w:space="%d" w:color="%s"/></w:pBdr>`,
				style, int(math.Round(barWidth*72*8)), int(math.Round(barGap*72)), hexVal(p.Bar))
		}
		if p.Background != "" {
			fmt.Fprintf(b, `<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, hexVal(p.Background))
		}
		if p.SceneNumber != "" && st != nil {
			fmt.Fprintf(b, `<w:tabs><w:tab w:val="right" w:pos="%d"/></w:tabs>`, twips(d.textWidth()-st.Right))
		}
		sz, _ := titleRun(p, st)
		switch {
		case sz > 12: // a line as high as its type
			fmt.Fprintf(b, `<w:spacing w:before="%d" w:line="%d" w:lineRule="atLeast"/>`, twips(spaceBefore*d.vscale), int(sz*20))
		case spaceBefore > 0:
			fmt.Fprintf(b, `<w:spacing w:before="%d"/>`, twips(spaceBefore*d.vscale))
		}
		if ownAlign {
			if jc := docxJc(p.Align); jc != "" {
				b.WriteString(jc)
			} else {
				b.WriteString(`<w:jc w:val="left"/>`)
			}
		}
		b.WriteString(`</w:pPr>`)
	}
	sz, font := titleRun(p, st)
	for _, sp := range p.Spans {
		b.WriteString(`<w:r>`)
		if p.Color != "" || p.Marker != "" || sz > 0 || font != "" || st != nil && (sp.Bold != st.Bold || sp.Italic != st.Italic || sp.Underline != st.Underline) {
			b.WriteString(`<w:rPr>`)
			if font != "" {
				fmt.Fprintf(b, `<w:rFonts w:ascii="%[1]s" w:hAnsi="%[1]s" w:cs="%[1]s"/>`, esc(font))
			}
			if st != nil && sp.Bold != st.Bold {
				b.WriteString(onOff("b", sp.Bold))
			}
			if st != nil && sp.Italic != st.Italic {
				b.WriteString(onOff("i", sp.Italic))
			}
			if p.Color != "" {
				fmt.Fprintf(b, `<w:color w:val="%s"/>`, hexVal(p.Color))
			}
			if sz > 0 { // (before u and shd: the schema's order)
				fmt.Fprintf(b, `<w:sz w:val="%[1]d"/><w:szCs w:val="%[1]d"/>`, int(sz*2))
			}
			if st != nil && sp.Underline != st.Underline {
				if sp.Underline {
					b.WriteString(`<w:u w:val="single"/>`)
				} else {
					b.WriteString(`<w:u w:val="none"/>`)
				}
			}
			if p.Marker != "" { // a highlighter: the run's shading
				fmt.Fprintf(b, `<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, hexVal(p.Marker))
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

// hexVal is a #rrggbb color as Word has it: RRGGBB.
func hexVal(c string) string { return strings.ToUpper(strings.TrimPrefix(c, "#")) }

func (d *doc) docxDual(b *strings.Builder, bl block) {
	if bl.BreakBefore {
		b.WriteString(`<w:p><w:pPr><w:pageBreakBefore/></w:pPr></w:p>`)
	}
	indent, col, gap := d.dualTable()
	fmt.Fprintf(b, `<w:tbl><w:tblPr><w:tblW w:w="%d" w:type="dxa"/><w:tblInd w:w="%d" w:type="dxa"/>`+
		`<w:tblLayout w:type="fixed"/><w:tblCellMar><w:left w:w="0" w:type="dxa"/><w:right w:w="0" w:type="dxa"/></w:tblCellMar>`+
		`</w:tblPr><w:tblGrid><w:gridCol w:w="%[3]d"/><w:gridCol w:w="%[4]d"/><w:gridCol w:w="%[3]d"/></w:tblGrid><w:tr>`,
		twips(2*col+gap), twips(indent), twips(col), twips(gap))
	if d.pagination.Keep { // dual dialogue is not broken over pages
		b.WriteString(`<w:trPr><w:cantSplit/></w:trPr>`)
	}
	cell := func(w float64, ps []layout.Paragraph) {
		fmt.Fprintf(b, `<w:tc><w:tcPr><w:tcW w:w="%d" w:type="dxa"/></w:tcPr>`, twips(w))
		if len(ps) == 0 {
			b.WriteString(`<w:p/>`)
		}
		for _, p := range ps {
			d.docxParagraph(b, p, false, 0, false)
		}
		b.WriteString(`</w:tc>`)
	}
	cell(col, bl.Dual[0])
	cell(gap, nil)
	cell(col, bl.Dual[1])
	b.WriteString(`</w:tr></w:tbl>`)
}
