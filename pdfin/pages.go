package pdfin

import (
	"fmt"
	"os"

	"github.com/LaPingvino/lexington/internal/pdf"
)

// PageLine is a line of text as printed on a page: in inches from the
// page's left edge (where it starts) and top (its baseline).
type PageLine struct {
	X, Y float64
	Text string
}

// Pages are the text lines of each page of the PDF file at path, top to
// bottom, as they are printed (nothing left out, nothing classified): to
// see where a page breaks.
func Pages(path string) (pages [][]PageLine, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	defer func() {
		if p := recover(); p != nil { // the PDF library panics on some files
			err = fmt.Errorf("reading the PDF: %v", p)
		}
	}()
	doc, err := pdf.NewReader(f, st.Size())
	if err != nil {
		return nil, err
	}
	for i := 1; i <= doc.NumPage(); i++ {
		var out []PageLine
		if p := doc.Page(i); !p.V.IsNull() {
			for _, l := range lines(i, p) {
				out = append(out, PageLine{X: l.x(), Y: l.Y, Text: l.text()})
			}
		}
		pages = append(pages, out)
	}
	return pages, nil
}
