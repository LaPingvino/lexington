package pdf

import (
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// The title page, drawn whole by its template (PDFWriter.TitlePage;
// package titlepage): each field where its key puts it, whatever the
// order the fields are written in.

// footTop is how far above the page's foot the foot's blocks start, in
// inches; a block too long for it is moved up to end footRoom above
// the foot.
const (
	footTop  = 2.0
	footRoom = 0.5
)

// template is the tree's title page template, Fountain's if none.
func (t *Tree) template() titlepage.Template {
	if t.title != nil {
		return *t.title
	}
	return titlepage.Standard()
}

// drawTitlePage draws the title page: rows are its titlepage row and the
// rows up to the page break after it.
func (t *Tree) drawTitlePage(rows lex.Screenplay) {
	tpl := t.template()
	blocks := tpl.Layout(rows)
	auto, margin := t.PDF.GetAutoPageBreak()
	t.PDF.SetAutoPageBreak(false, margin) // a big title or a free place never breaks the page
	defer t.PDF.SetAutoPageBreak(auto, margin)
	_, height := t.PDF.GetPageSize()
	for _, b := range blocks {
		for _, f := range b.Fields {
			if strings.EqualFold(f.Key, "Title") && f.Picture == "" {
				t.PDF.SetTitle(lex.Unstyled(strings.Join(f.Lines, " ")), true)
				break
			}
		}
	}
	for _, b := range blocks {
		var y float64
		switch b.Place {
		case rules.PlaceTop:
			y = 1 * t.vsc()
		case rules.PlaceCentre: // its first line one under Centre, as Fountain's always printed
			y = tpl.CentreAt()*t.vsc() + lineHeight
		case rules.PlaceFootLeft, rules.PlaceFootRight:
			y = height - footTop
			if h := t.titleBlockHeight(b); y+h > height-footRoom {
				y = max(height-footRoom-h, 0)
			}
		case rules.PlaceFree:
			e := b.Fields[0].Entry
			y = e.Y * t.vsc()
			if e.Y < 0 {
				y = height + e.Y*t.vsc()
			}
		}
		t.PDF.SetY(y)
		for _, f := range b.Fields {
			t.drawTitleField(f)
		}
	}
}

// titleFormat is the format a field prints in, its margins those of
// its place.
func (t *Tree) titleFormat(e rules.TitleKey) rules.Format {
	f := titlepage.Format(t.Rules, e)
	if e.Place != rules.PlaceFree {
		return f
	}
	width, _ := t.PDF.GetPageSize()
	x := e.X * t.sc()
	switch f.Align {
	case "L":
		f.Left = x
	case "R":
		f.Right = width - x
	case "C":
		if e.X != 0 {
			d := max(min(x, width-x)-0.25, 0.5)
			f.Left, f.Right = x-d, width-x-d
		}
	}
	return f
}

// titleLineHeight is the height of a line in a format's type.
func titleLineHeight(f rules.Format) float64 {
	if f.Size > 12 {
		return lineHeight * f.Size / 12
	}
	return lineHeight
}

// titleBlockHeight is how high a block prints.
func (t *Tree) titleBlockHeight(b titlepage.Block) float64 {
	width, _ := t.PDF.GetPageSize()
	h := 0.0
	for _, f := range b.Fields {
		fm := t.titleFormat(f.Entry)
		h += f.Entry.Before * lineHeight
		if f.Picture != "" {
			_, ph := t.titlePictureSize(f, fm)
			h += ph
			continue
		}
		for _, l := range f.Lines {
			_, printed := t.titleText(fm, f.Entry, l)
			h += float64(t.lines(fm, printed, width-fm.Left-fm.Right)) * titleLineHeight(fm)
		}
	}
	return h
}

// titleText is a line of a field as printed: in capitals if its entry
// says so, the format's prefix and postfix around it.
func (t *Tree) titleText(f rules.Format, e rules.TitleKey, line string) (rules.Format, string) {
	return f, f.Prefix + titlepage.Caps(e, line) + f.Postfix
}

// drawTitleField draws a field where the PDF is, and moves down past it.
func (t *Tree) drawTitleField(f titlepage.Field) {
	fm := t.titleFormat(f.Entry)
	if f.Entry.Before > 0 {
		t.PDF.SetY(t.PDF.GetY() + f.Entry.Before*lineHeight)
	}
	if f.Picture != "" {
		t.drawTitlePicture(f, fm)
		return
	}
	tt := *t
	tt.lh = titleLineHeight(fm)
	for _, l := range f.Lines {
		_, printed := t.titleText(fm, f.Entry, l)
		tt.prf(fm, printed, false)
	}
}

// titlePictureSize is the size a field's picture prints in: its
// entry's height (scaled to the paper), made smaller to fit between the
// format's margins.
func (t *Tree) titlePictureSize(f titlepage.Field, fm rules.Format) (w, h float64) {
	width, _ := t.PDF.GetPageSize()
	room := width - fm.Left - fm.Right
	h = titlepage.PictureHeight(f.Entry) * t.vsc()
	pw, ph, err := ImageSize(t.titlePics[f.Picture])
	if err != nil || pw == 0 || ph == 0 {
		return min(room, h*4/3), h
	}
	w = h * float64(pw) / float64(ph)
	if w > room {
		w, h = room, room*float64(ph)/float64(pw)
	}
	return w, h
}

// drawTitlePicture draws a field's picture where the PDF is, aligned in
// its format's margins; a picture that cannot be read as a quiet box with
// its name.
func (t *Tree) drawTitlePicture(f titlepage.Field, fm rules.Format) {
	width, _ := t.PDF.GetPageSize()
	w, h := t.titlePictureSize(f, fm)
	x := fm.Left
	switch fm.Align {
	case "C":
		x = fm.Left + (width-fm.Left-fm.Right-w)/2
	case "R":
		x = width - fm.Right - w
	}
	y := t.PDF.GetY()
	name := "titlepage:" + f.Picture
	ok := false
	if data, there := t.titlePics[f.Picture]; there {
		_, _, err := AddImage(t.PDF, name, data)
		ok = err == nil
	}
	if ok {
		DrawImage(t.PDF, name, x, y, w, h)
	} else {
		t.PDF.SetDrawColor(170, 170, 170)
		t.PDF.SetLineWidth(0.01)
		t.PDF.Rect(x, y, w, h, "D")
		t.PDF.SetTextColor(120, 120, 120)
		t.PDF.SetFont("CourierPrime", "", 10)
		t.PDF.SetXY(x, y+h/2-lineHeight/2)
		t.PDF.CellFormat(w, lineHeight, f.Picture, "", 0, "C", false, 0, "")
		t.PDF.SetTextColor(0, 0, 0)
		t.PDF.SetDrawColor(0, 0, 0)
	}
	t.PDF.SetXY(0, y+h)
}
