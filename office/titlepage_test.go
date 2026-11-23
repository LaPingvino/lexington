package office

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// zipped are a zip's files by name.
func zipped(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range z.File {
		r, _ := f.Open()
		out[f.Name], _ = io.ReadAll(r)
		r.Close()
	}
	return out
}

// wellFormed fails unless every XML part reads as XML.
func wellFormed(t *testing.T, files map[string][]byte) {
	t.Helper()
	for name, data := range files {
		if !strings.HasSuffix(name, ".xml") && !strings.HasSuffix(name, ".rels") {
			continue
		}
		d := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := d.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
	}
}

// A template's title page in Word and ODT: its order, a field's size and
// alignment, the picture in the document (media, relation, manifest).
func TestTitlePageTemplateOffice(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var pic bytes.Buffer
	png.Encode(&pic, img)
	tpl := titlepage.Template{Page: rules.TitlePage{Keys: []rules.TitleKey{
		{Key: "Art", Place: "centre", Picture: true, Height: 1},
		{Key: "Title", Place: "centre", Size: 24},
		{Key: "Draft date", Place: "foot-right"},
	}}}
	s := fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(
		"Draft date: 1 May\nTitle: Rain\nArt: art.png\nAuthor: Ann\n\nINT. HOUSE - DAY\n\nRain.\n"))
	pics := map[string][]byte{"art.png": pic.Bytes()}

	var b bytes.Buffer
	if err := (&DOCXWriter{TitlePage: &tpl, TitlePictures: pics}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	files := zipped(t, b.Bytes())
	wellFormed(t, files)
	doc := string(files["word/document.xml"])
	if files["word/media/image1.png"] == nil || !strings.Contains(string(files["word/_rels/document.xml.rels"]), `Id="rIdimage1"`) ||
		!strings.Contains(doc, `r:embed="rIdimage1"`) || !strings.Contains(string(files["[Content_Types].xml"]), `Extension="png"`) {
		t.Errorf("the picture is not in the docx")
	}
	art, title, date, author := strings.Index(doc, "rIdimage1"), strings.Index(doc, ">Rain<"), strings.Index(doc, ">1 May<"), strings.Index(doc, ">Ann<")
	if !(art < title && title < author && author < date) {
		t.Errorf("order: art %d title %d author %d date %d", art, title, author, date)
	}
	if !strings.Contains(doc, `<w:sz w:val="48"/>`) || !strings.Contains(doc, `<w:jc w:val="right"/>`) {
		t.Errorf("the title's size or the date's alignment is missing")
	}

	b.Reset()
	if err := (&ODTWriter{TitlePage: &tpl, TitlePictures: pics}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	files = zipped(t, b.Bytes())
	wellFormed(t, files)
	content := string(files["content.xml"])
	if files["Pictures/image1.png"] == nil || !strings.Contains(string(files["META-INF/manifest.xml"]), "Pictures/image1.png") ||
		!strings.Contains(content, `xlink:href="Pictures/image1.png"`) {
		t.Errorf("the picture is not in the odt")
	}
	if !strings.Contains(content, `fo:font-size="24pt"`) || !strings.Contains(content, `fo:text-align="end"`) {
		t.Errorf("the title's size or the date's alignment is missing")
	}
}
