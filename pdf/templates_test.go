package pdf

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// titleTexts are the title page's texts, by their text.
func titleTexts(t *testing.T, src string, w PDFWriter) (map[string]Text, DrawnPage) {
	t.Helper()
	var d Drawing
	w.Record = &d
	if _, err := w.Render(fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(src))); err != nil {
		t.Fatal(err)
	}
	out := map[string]Text{}
	for _, m := range d.Pages[0].Marks {
		if tx, ok := m.(Text); ok {
			tx.X, tx.Y, tx.Width = tx.X/72, tx.Y/72, tx.Width/72 // (inches)
			out[strings.TrimSpace(tx.Text)] = tx
		}
	}
	return out, d.Pages[0]
}

// The keys' places do not follow the order they are written in.
func TestTitlePageByKey(t *testing.T) {
	src := "Contact:\n    Next Level\nTitle: MIXED\nDraft date: 1/20\nAuthor: Someone\n\nEXT. PLACE - DAY\n\nAction.\n"
	tx, _ := titleTexts(t, src, PDFWriter{})
	title, author, contact, date := tx["MIXED"], tx["Someone"], tx["Next Level"], tx["1/20"]
	if title.Y < 4 || title.Y > 4.6 || author.Y <= title.Y {
		t.Errorf("the title block: title %.2f, author %.2f", title.Y, author.Y)
	}
	if contact.Y < 8.9 || date.Y <= contact.Y || contact.X > 1.6 {
		t.Errorf("the foot: contact %.2f,%.2f date %.2f", contact.X, contact.Y, date.Y)
	}
	if title.X < 2 { // centred
		t.Errorf("the title is not centred: x %.2f", title.X)
	}
}

// A template's size, capitals, place on the right and free position;
// its picture drawn.
func TestTitlePageTemplate(t *testing.T) {
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	img.Set(1, 1, color.RGBA{255, 0, 0, 255})
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	tpl := titlepage.Template{Page: rules.TitlePage{Keys: []rules.TitleKey{
		{Key: "Art", Place: "centre", Picture: true, Height: 1},
		{Key: "Title", Place: "centre", Size: 24, Caps: true, Style: "b"},
		{Key: "Draft date", Place: "foot-right"},
		{Key: "Registration", Place: "free", X: 7.5, Y: 0.5, Align: "R"},
	}}}
	src := "Title: little\nArt: art.png\nDraft date: 1/20\nRegistration: WGA 1\n\nEXT. PLACE - DAY\n"
	tx, page := titleTexts(t, src, PDFWriter{TitlePage: &tpl, TitlePictures: map[string][]byte{"art.png": b.Bytes()}})
	title := tx["LITTLE"]
	if title.Size != 24 {
		t.Errorf("title: %+v", title)
	}
	if d := tx["1/20"]; d.X+d.Width < 7.3 || d.Y < 8.9 {
		t.Errorf("draft date not at the foot's right: %+v", d)
	}
	if r := tx["WGA 1"]; r.Y > 1 || r.X+r.Width > 7.55 || r.X+r.Width < 7.4 {
		t.Errorf("registration not at its place: %+v", r)
	}
	var pic *Image
	for _, m := range page.Marks {
		if im, ok := m.(Image); ok {
			im.X, im.Y, im.W, im.H = im.X/72, im.Y/72, im.W/72, im.H/72
			pic = &im
		}
	}
	if pic == nil || pic.H < 0.99 || pic.H > 1.01 || pic.W < 1.99 || pic.Y > title.Y {
		t.Errorf("the picture: %+v (title at %.2f)", pic, title.Y)
	}
}
