package fdx

import (
	"encoding/xml"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Highland's {{HEADER}} and {{FOOTER}} in Final Draft (lex/highland.go):
// the script's first of each is the document's <HeaderAndFooter>, as
// Final Draft has one for all the pages; its left, centre and right
// segments are one paragraph, tabbed to the tab stops Final Draft's own
// header has (centre 4.50, right 7.25). %p is Final Draft's "Page #"
// label; %pp the last page's number when the writer knows it
// (FDXWriter.LastPage); %# (the section) and {{PAGENUMBER}} have no
// equivalent and are left out. With no header, Final Draft's own (the
// page number, top right, not on the first page) is not written either:
// Final Draft puts its default in.

// FdxHeaderAndFooter is a <HeaderAndFooter>.
type FdxHeaderAndFooter struct {
	FooterFirstPage string    `xml:"FooterFirstPage,attr"`
	FooterVisible   string    `xml:"FooterVisible,attr"`
	HeaderFirstPage string    `xml:"HeaderFirstPage,attr"`
	HeaderVisible   string    `xml:"HeaderVisible,attr"`
	StartingPage    string    `xml:"StartingPage,attr"`
	Header          FdxHFText `xml:"Header"`
	Footer          FdxHFText `xml:"Footer"`
}

// FdxHFText is a <Header> or <Footer>: its paragraph as XML.
type FdxHFText struct {
	Inner string `xml:",innerxml"`
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// headerAndFooter is the <HeaderAndFooter> for a header and a footer
// (nil: none of either).
func headerAndFooter(header, footer *lex.Directive, lastPage string) *FdxHeaderAndFooter {
	if header == nil && footer == nil {
		return nil
	}
	on := func(d *lex.Directive) bool { return d != nil && !d.Off }
	hf := &FdxHeaderAndFooter{
		HeaderFirstPage: yesNo(on(header)), HeaderVisible: yesNo(on(header)),
		FooterFirstPage: yesNo(on(footer)), FooterVisible: yesNo(on(footer)),
		StartingPage: "1",
	}
	hf.Header.Inner = hfParagraph(header, lastPage)
	hf.Footer.Inner = hfParagraph(footer, lastPage)
	return hf
}

func hfParagraph(d *lex.Directive, lastPage string) string {
	var b strings.Builder
	b.WriteString(`<Paragraph Alignment="Left" FirstIndent="0.00" Leading="Regular" LeftIndent="1.25" RightIndent="-1.00" SpaceBefore="0" Spacing="1" StartsNewPage="No">`)
	text := func(s string) {
		if s == "" {
			return
		}
		b.WriteString("<Text>")
		xml.EscapeText(&b, []byte(s))
		b.WriteString("</Text>")
	}
	if d != nil && !d.Off {
		for i, seg := range [][]lex.PagePart{d.Left, d.Center, d.Right} {
			if i > 0 {
				text("\t")
			}
			for _, p := range seg {
				switch p.Kind {
				case lex.PartText:
					text(p.Text)
				case lex.PartPage:
					b.WriteString(`<DynamicLabel Type="Page #"/>`)
				case lex.PartPages:
					text(lastPage)
				}
			}
		}
	} else {
		text(" ")
	}
	b.WriteString(`<Tabstops><Tabstop Position="4.50" Type="Center"/><Tabstop Position="7.25" Type="Right"/></Tabstops></Paragraph>`)
	return b.String()
}
