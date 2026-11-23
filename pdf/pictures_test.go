package pdf_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

// framesScript is a scene with a storyboard frame under its action, a
// speech with one under it, and a frame whose file is missing.
const framesScript = `INT. HOUSE - DAY

She opens the door.
[[board: door.png, duration: 2.50]]

BOB
Hello.
[[board: bob.png]]

Rain.
[[board: missing.png]]
`

func drawFrames(t *testing.T, src string, pics map[string][]byte) ([]byte, pdf.Drawing) {
	t.Helper()
	var d pdf.Drawing
	w := &pdf.PDFWriter{Elements: rules.Default, Pictures: pics, Record: &d}
	data, err := w.Render(fountain.ParseLines(fountain.Scene, strings.NewReader(src)))
	if err != nil {
		t.Fatal(err)
	}
	return data, d
}

func marks[T pdf.Mark](d pdf.Drawing) (out []T) {
	for _, p := range d.Pages {
		for _, m := range p.Marks {
			if x, ok := m.(T); ok {
				out = append(out, x)
			}
		}
	}
	return out
}

func pageText(p pdf.DrawnPage) string {
	var b strings.Builder
	for _, m := range p.Marks {
		if x, ok := m.(pdf.Text); ok {
			b.WriteString(x.Text + "\n")
		}
	}
	return b.String()
}

// Storyboard frames print under their lines when asked, as pictures two
// inches high at the action's margin; a missing one as a box with its
// name; without Pictures the PDF is as before, the notes left out.
func TestPrintFrames(t *testing.T) {
	pdf.FixDate()
	png := encoded(t, "png", picture(160, 90))
	plain, _ := drawFrames(t, framesScript, nil)
	var none pdf.Drawing
	same, err := (&pdf.PDFWriter{Elements: rules.Default, Record: &none}).Render(fountain.ParseLines(fountain.Scene, strings.NewReader(framesScript)))
	if err != nil || !bytes.Equal(plain, same) {
		t.Fatal("without pictures the PDF changed")
	}
	if strings.Contains(pageText(none.Pages[0]), "board") {
		t.Error("a board note printed without Pictures")
	}
	_, d := drawFrames(t, framesScript, map[string][]byte{"door.png": png, "bob.png": png})
	imgs := marks[pdf.Image](d)
	if len(imgs) != 2 {
		t.Fatalf("%d pictures drawn, want 2", len(imgs))
	}
	if h := imgs[0].H; h < 142 || h > 143 { // 12 lines of 0.165in, in points
		t.Errorf("a frame %.1fpt high", h)
	}
	if imgs[0].X != 1.5*72 {
		t.Errorf("a frame at x %.1f, not the action's margin", imgs[0].X)
	}
	text := pageText(d.Pages[0])
	if !strings.Contains(text, "missing.png") || strings.Contains(text, "board") {
		t.Errorf("page text %q", text)
	}
	var texts []pdf.Text
	for _, m := range d.Pages[0].Marks {
		if x, ok := m.(pdf.Text); ok {
			texts = append(texts, x)
		}
	}
	// the frame is under its line and above the next
	var door, bob float64
	for _, x := range texts {
		switch strings.TrimSpace(x.Text) {
		case "She opens the door.":
			door = x.Y
		case "BOB":
			bob = x.Y
		}
	}
	if !(door < imgs[0].Y && imgs[0].Y+imgs[0].H < bob) {
		t.Errorf("the frame (%.0f–%.0f) not between its line (%.0f) and the next (%.0f)", imgs[0].Y, imgs[0].Y+imgs[0].H, door, bob)
	}
}

// A frame that does not fit at a page's foot goes to the next page whole.
func TestFrameNotBroken(t *testing.T) {
	png := encoded(t, "png", picture(160, 90))
	var b strings.Builder
	b.WriteString("INT. HOUSE - DAY\n\n")
	for i := 0; i < 40; i++ {
		b.WriteString("A line of action.\n\n")
	}
	for i := 0; i < 4; i++ {
		b.WriteString("Look.\n[[board: f.png]]\n\n")
	}
	_, d := drawFrames(t, b.String(), map[string][]byte{"f.png": png})
	for _, im := range marks[pdf.Image](d) {
		if im.Y+im.H > 11*72-0.5*72+1 {
			t.Errorf("a frame runs to %.0fpt, past the page's foot", im.Y+im.H)
		}
	}
	if n := len(marks[pdf.Image](d)); n != 4 {
		t.Errorf("%d frames", n)
	}
}
