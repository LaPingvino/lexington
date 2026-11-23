package pdf

import (
	"math"

	"github.com/LaPingvino/lexington/lex"
)

// Storyboard frames in a screenplay's PDF (PDFWriter.Pictures): each a
// TypePicture row, printed at the action's left margin, PictureLines
// high (less when it is wider than the action), never broken over pages.

// pictures are the frames' files and what is known of them.
type pictures struct {
	files map[string][]byte
	lines int             // a frame's height, in lines
	added map[string]bool // read into the PDF (true) or not readable (false)
}

// missingLines is how high a frame with no picture prints: a box with its
// name.
const missingLines = 3

// pictureSize is a frame's width and height in the document's unit, and
// whether its picture can be read.
func (t *Tree) pictureSize(name string) (w, h float64, ok bool) {
	f := t.Rules.Get(lex.TypeAction)
	pageWidth, _ := t.PDF.GetPageSize()
	width := pageWidth - f.Left - f.Right
	pw, ph, err := ImageSize(t.pictures.files[name])
	if err != nil || pw == 0 || ph == 0 {
		return width, missingLines * lineHeight, false
	}
	h = float64(t.pictures.lines) * lineHeight
	w = h * float64(pw) / float64(ph)
	if w > width {
		w, h = width, width*float64(ph)/float64(pw)
	}
	return w, h, true
}

// pictureLines is how many lines a frame takes.
func (t *Tree) pictureLines(name string) int {
	if t.pictures == nil {
		return 0
	}
	_, h, _ := t.pictureSize(name)
	return int(math.Ceil(h/lineHeight - 1e-6))
}

// printPicture prints a frame where the PDF is (on the next page if it
// does not fit), and moves down past it.
func (t *Tree) printPicture(name string) {
	if t.pictures == nil {
		return
	}
	n := t.pictureLines(name)
	if t.room() < n && !t.atTop() {
		t.newPage(false)
	}
	w, h, ok := t.pictureSize(name)
	x, y := t.Rules.Get(lex.TypeAction).Left, t.PDF.GetY()
	if ok {
		if added, seen := t.pictures.added[name]; !seen {
			_, _, err := AddImage(t.PDF, name, t.pictures.files[name])
			added = err == nil
			t.pictures.added[name] = added
			ok = added
		} else {
			ok = added
		}
	}
	if ok {
		DrawImage(t.PDF, name, x, y, w, h)
	} else { // a quiet box with the file's name
		t.PDF.SetDrawColor(170, 170, 170)
		t.PDF.SetLineWidth(0.01)
		t.PDF.Rect(x, y+0.02, w, h-0.04, "D")
		t.PDF.SetTextColor(120, 120, 120)
		t.PDF.SetXY(x, y+h/2-lineHeight/2)
		t.PDF.CellFormat(w, lineHeight, name, "", 0, "C", false, 0, "")
		t.PDF.SetTextColor(0, 0, 0)
		t.PDF.SetDrawColor(0, 0, 0)
	}
	t.PDF.SetXY(x, y+float64(n)*lineHeight)
}
