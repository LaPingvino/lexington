package html

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// The title page as HTML, laid out by its template (HTMLWriter.TitlePage,
// package titlepage): a block for each place, each field in its type.
// On screen the centred block stands in the middle, the foot's at the
// foot; printed, each is where the PDF prints it (the CSS in
// htmlTemplateString).

// placeClass is the class of a place's block.
var placeClass = map[string]string{
	rules.PlaceTop:       "title-top",
	rules.PlaceCentre:    "title-centre",
	rules.PlaceFootLeft:  "title-meta",
	rules.PlaceFootRight: "title-meta-right",
	rules.PlaceFree:      "title-free",
}

// titlePageHTML is the title page of s, opened (the page break after it
// closes it), and the rows it takes, [0, to); "" and 0 without one.
func (h *HTMLWriter) titlePageHTML(s lex.Screenplay) (template.HTML, int) {
	_, to := titlepage.Rows(s)
	if to == 0 {
		return "", 0
	}
	tpl := titlepage.Standard()
	if h.TitlePage != nil {
		tpl = *h.TitlePage
	}
	set := h.elements()
	var b strings.Builder
	b.WriteString(`<div class="title-page">`)
	if !tpl.IsStandard() {
		fmt.Fprintf(&b, `<style>@media print { .title-centre { top: calc(%.3fin - 1in + 12pt); } }</style>`, tpl.CentreAt())
	}
	for _, bl := range tpl.Layout(s) {
		style := ""
		if bl.Place == rules.PlaceFree {
			e := bl.Fields[0].Entry
			style = freeStyle(e)
		}
		fmt.Fprintf(&b, `<div class="%s"%s>`, placeClass[bl.Place], style)
		for _, f := range bl.Fields {
			b.WriteString(h.titleField(set, f))
		}
		b.WriteString(`</div>`)
	}
	return template.HTML(b.String()), to
}

// freeStyle is where a free place stands: as printed, from the title
// page's box (an inch in from the paper's top and foot, 1.5 in from its
// left).
func freeStyle(e rules.TitleKey) string {
	css := []string{"position: absolute", "margin: 0"}
	if e.Y < 0 {
		css = append(css, fmt.Sprintf("bottom: calc(%.3fin - 1in)", -e.Y))
	} else {
		css = append(css, fmt.Sprintf("top: calc(%.3fin - 1in)", e.Y))
	}
	switch a := strings.ToUpper(e.Align); {
	case a == "C" && e.X == 0:
		css = append(css, "left: 0", "right: 0", "text-align: center")
	case a == "C":
		css = append(css, fmt.Sprintf("left: calc(%.3fin - 1.5in)", e.X), "transform: translateX(-50%)", "text-align: center")
	case a == "R":
		css = append(css, fmt.Sprintf("left: calc(%.3fin - 1.5in)", e.X), "transform: translateX(-100%)", "text-align: end")
	default:
		css = append(css, fmt.Sprintf("left: calc(%.3fin - 1.5in)", e.X), "text-align: start")
	}
	return styleAttr(css)
}

// titleField is one field: a picture, or its lines (the title's an h1).
func (h *HTMLWriter) titleField(set rules.Set, f titlepage.Field) string {
	e := f.Entry
	var css []string
	if e.Before > 0 {
		css = append(css, fmt.Sprintf("margin-top: %gpt", e.Before*12))
	}
	if f.Picture != "" {
		img := []string{fmt.Sprintf("height: %gin", titlepage.PictureHeight(e)), "max-width: 100%", "object-fit: contain"}
		return fmt.Sprintf(`<p class="title-picture"%s><img src="%s" alt="%s"%s></p>`, styleAttr(css),
			template.HTMLEscapeString(h.pictureSrc(f.Picture)), template.HTMLEscapeString(f.Alt), styleAttr(img))
	}
	fm := titlepage.Format(set, e)
	base := titlepage.Format(set, rules.TitleKey{Place: e.Place})
	if e.Size > 0 && fm.Size != base.Size {
		css = append(css, fmt.Sprintf("font-size: %gpt", fm.Size))
	}
	if strings.ContainsAny(e.Style, "bB") {
		css = append(css, "font-weight: bold")
	}
	if strings.ContainsAny(e.Style, "iI") {
		css = append(css, "font-style: italic")
	}
	if strings.ContainsAny(e.Style, "uU") {
		css = append(css, "text-decoration: underline")
	}
	if e.Caps {
		css = append(css, "text-transform: uppercase")
	}
	if e.Font != "" {
		css = append(css, "font-family: "+cssFont(e.Font))
	}
	switch strings.ToUpper(e.Align) {
	case "L":
		css = append(css, "text-align: start")
	case "C":
		css = append(css, "text-align: center")
	case "R":
		css = append(css, "text-align: end")
	}
	var lines []string
	for _, l := range f.Lines {
		lines = append(lines, string(processInlineMarkup(fm.Prefix+l+fm.Postfix)))
	}
	if strings.EqualFold(f.Key, "Title") {
		return `<h1` + styleAttr(css) + `>` + strings.Join(lines, "<br>") + `</h1>`
	}
	var b strings.Builder
	for i, l := range lines {
		c := css
		if i > 0 { // the space before is the field's, not each line's
			c = nil
			for _, s := range css {
				if !strings.HasPrefix(s, "margin-top") {
					c = append(c, s)
				}
			}
		}
		b.WriteString(`<p` + styleAttr(c) + `>` + l + `</p>`)
	}
	return b.String()
}

// pictureSrc is a picture's address: its file's bytes as a data: URL
// when the writer has them (TitlePictures), so the page is whole; else
// the file's name, relative to the page.
func (h *HTMLWriter) pictureSrc(name string) string {
	if data, ok := h.TitlePictures[name]; ok && len(data) > 0 {
		return "data:" + http.DetectContentType(data) + ";base64," + base64.StdEncoding.EncodeToString(data)
	}
	return name
}

// cssFont is a font setting as CSS names it.
func cssFont(f string) string {
	switch strings.ToLower(f) {
	case "courierprime", "courier":
		return `"Courier Prime", "Courier New", monospace`
	case "helvetica":
		return "Helvetica, Arial, sans-serif"
	case "times":
		return `"Times New Roman", Times, serif`
	}
	return `"` + strings.Trim(f, `"`) + `"`
}

func styleAttr(css []string) string {
	if len(css) == 0 {
		return ""
	}
	return ` style="` + template.HTMLEscapeString(strings.Join(css, "; ")) + `"`
}
