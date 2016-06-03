package main

import "github.com/jung-kurt/gofpdf"

var tr func(string) string

func line(pdf *gofpdf.Fpdf, jump, width float64, text string) {
	pdf.SetX(jump)
	pdf.MultiCell(width, 0.19, tr(text), "", "aligned", false)
}

func main() {
	pdf := gofpdf.New("P", "in", "Letter", "")
  tr = pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("courier", "", 12)
	pdf.SetMargins(1, 1, 1)
	pdf.SetXY(1, 1)
  line(pdf, 1.5, 6, "FADE IN")
  line(pdf, 1.5, 6, "")
  line(pdf, 1.5, 6, "A RIVER")
  line(pdf, 1.5, 6, "")
  line(pdf, 1.5, 6, "We’re underwater, watching a fat catfish swim along.")
  line(pdf, 1.5, 6, "")
  line(pdf, 1.5, 6, "This is The Beast.")
  line(pdf, 1.5, 6, "")
  line(pdf, 4.2, 3.3, "EDWARD (V.O.)")
  line(pdf, 2.9, 3.3, "There are some fish that cannot be caught.  It’s not that they’re faster or stronger than other fish.  They’re just touched by something extra.  Call it luck.  Call it grace.  One such fish was The Beast.")
	err := pdf.OutputFileAndClose("hello.pdf")
	if err != nil {
		panic(err)
	}
}
