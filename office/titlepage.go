package office

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // the pictures read
	_ "image/jpeg"
	"image/png"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/LaPingvino/lexington/layout"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// The title page in a word processor, by its template (titlepage): its
// fields in the order the template places them (a word processor flows
// them: the centred block under the title, the foot's after it, a free
// place's with the foot's), in their size, font, style and alignment;
// its pictures in the document, as PNG.

// picture is a title page picture put in the document.
type picture struct {
	name          string // the file in the document (image1.png)
	data          []byte // PNG
	width, height float64
	alt           string
}

// titlePictures reads the pictures of the title page's paragraphs (by
// their files' names in files) into d; one that cannot be read is left
// out (its paragraph prints its name).
func (d *doc) titlePictures(files map[string][]byte) {
	for i := range d.blocks {
		tf := d.blocks[i].P.TitleField
		if tf == nil || tf.Picture == "" {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(files[tf.Picture]))
		if err != nil {
			continue
		}
		var b bytes.Buffer
		if png.Encode(&b, img) != nil {
			continue
		}
		size := img.Bounds().Size()
		if size.X == 0 || size.Y == 0 {
			continue
		}
		h := tf.Height * d.vscale
		w := h * float64(size.X) / float64(size.Y)
		if room := d.textWidth(); w > room {
			w, h = room, room*float64(size.Y)/float64(size.X)
		}
		if d.pictures == nil {
			d.pictures = map[int]*picture{}
		}
		d.pictures[i] = &picture{name: fmt.Sprintf("image%d.png", len(d.pictures)+1), data: b.Bytes(),
			width: w, height: h, alt: tf.Alt}
	}
}

// titleSpaces are the title page paragraphs' spaces before, in inches:
// the centred block's first starts where the template has it (Fountain's
// 4 in down, a line under), each field the lines before it its entry
// asks for.
func (d *doc) titleSpaces(tpl titlepage.Template) {
	top, centred := 0.0, false
	for i := range d.blocks {
		b := &d.blocks[i]
		tf := b.P.TitleField
		if tf == nil {
			continue
		}
		before := tf.Before / 6
		switch tf.Entry.Place {
		case rules.PlaceTop:
			top += before + lineInches(tf)
		case rules.PlaceCentre:
			if !centred {
				centred = true
				before += max(tpl.CentreAt()-1-top, 1.0/6)
			}
		}
		b.SpaceBefore += before
	}
}

// lineInches is how high a title paragraph's line is, roughly.
func lineInches(tf *layout.TitleField) float64 {
	if tf.Picture != "" {
		return tf.Height
	}
	return max(size(tf.Size), 12) / 72
}

// titleRun is the run properties a title page paragraph's text has
// beyond its style's: its size and font.
func titleRun(p layout.Paragraph, st *style) (sz float64, font string) {
	tf := p.TitleField
	if tf == nil || st == nil {
		return 0, ""
	}
	if s := size(tf.Size); s != size(st.Size) {
		sz = s
	}
	if f := fontName(tf.Font); tf.Font != "" && f != st.Font {
		font = f
	}
	return sz, font
}

// titleStyleAlign is the alignment of the title page's styles: the
// set's title format's (centred) and its meta format's.
func titleStyleAlign(set rules.Set, key string) byte {
	place := rules.PlaceCentre
	if key == "meta" {
		place = rules.PlaceFootLeft
	}
	if a := strings.ToUpper(titlepage.Format(set, rules.TitleKey{Place: place}).Align); a == "C" || a == "R" {
		return a[0]
	}
	return 'L'
}

// pictureList are the pictures in the document's order.
func (d *doc) pictureList() []*picture {
	var out []*picture
	for i := range d.blocks {
		if p := d.pictures[i]; p != nil {
			out = append(out, p)
		}
	}
	return out
}
