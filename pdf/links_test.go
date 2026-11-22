package pdf_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/LaPingvino/lexington/pdf"
	"github.com/phpdave11/gofpdf"
)

// linked draws two pages with links: an address over a cell's text, an
// internal link to a place on page 2 made before the place is set, and a
// rectangle linked to an address; through a Recorder if rec.
func linked(t *testing.T, rec bool) ([]byte, pdf.Drawing) {
	t.Helper()
	f := gofpdf.New("P", "pt", "A4", "")
	var d pdf.Drawer = f
	var r *pdf.Recorder
	if rec {
		r = pdf.NewRecorder(f)
		d = r
	}
	d.SetFont("helvetica", "", 12)
	d.AddPage()
	to := d.AddLink()
	d.SetXY(72, 100)
	d.CellFormat(200, 20, "An address", "", 0, "L", false, 0, "https://example.org/")
	d.SetXY(72, 140)
	d.CellFormat(0, 20, "Page two", "", 1, "C", false, to, "")
	d.LinkString(72, 200, 50, 10, "mailto:someone@example.org")
	d.AddPage()
	d.SetY(300)
	d.SetLink(to, -1, -1)
	d.Link(72, 72, 10, 10, to)
	var b bytes.Buffer
	if err := d.Output(&b); err != nil {
		t.Fatal(err)
	}
	if r == nil {
		return b.Bytes(), pdf.Drawing{}
	}
	return b.Bytes(), r.Drawing()
}

// The Recorder keeps the links where gofpdf puts them, its PDF the same
// byte for byte; an internal link's place is known once it is set, after
// the link.
func TestRecorderLinks(t *testing.T) {
	pdf.FixDate()
	plain, _ := linked(t, false)
	recorded, d := linked(t, true)
	if !bytes.Equal(plain, recorded) {
		t.Errorf("the PDF differs when recorded (%d bytes, %d recorded)", len(plain), len(recorded))
	}
	if len(d.Pages) != 2 {
		t.Fatalf("%d pages", len(d.Pages))
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.01 }
	one := d.Pages[0].Links
	if len(one) != 3 {
		t.Fatalf("page 1's links: %+v", one)
	}
	if l := one[0]; l.URL != "https://example.org/" || l.Page != 0 || !near(l.X, 72+2.835) /* the cell margin */ || !near(l.Y, 104) || !near(l.H, 12) || l.W <= 0 {
		t.Errorf("the address's link: %+v", l)
	}
	if l := one[1]; l.URL != "" || l.Page != 2 || !near(l.DestY, 300) || l.X <= 72 || !near(l.Y, 144) || l.W <= 0 { // centred
		t.Errorf("the internal link: %+v", l)
	}
	if l := one[2]; l.URL != "mailto:someone@example.org" || !near(l.X, 72) || !near(l.Y, 200) || !near(l.W, 50) || !near(l.H, 10) {
		t.Errorf("the rectangle's link: %+v", l)
	}
	if two := d.Pages[1].Links; len(two) != 1 || two[0].Page != 2 || !near(two[0].DestY, 300) {
		t.Errorf("page 2's links: %+v", two)
	}
	if !d.Pages[0].Equal(d.Pages[0]) || len(d.Pages[1].Marks) != 0 {
		t.Error("a link is a mark")
	}
}
