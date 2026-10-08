package office

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

func script(t *testing.T) lex.Screenplay {
	t.Helper()
	f, err := os.Open("../examples/tv-episode.fountain")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	return fountain.Parse(rules.DefaultConf().Scenes["en"], f)
}

// parts writes the document and returns its zip's parts, checking that
// every XML part is well-formed.
func parts(t *testing.T, w interface {
	Write(io.Writer, lex.Screenplay) error
}) (map[string]string, []*zip.File) {
	t.Helper()
	var buf bytes.Buffer
	if err := w.Write(&buf, script(t)); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range z.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		out[f.Name] = string(b)
		if strings.HasSuffix(f.Name, ".xml") || strings.HasSuffix(f.Name, ".rels") {
			d := xml.NewDecoder(bytes.NewReader(b))
			for {
				if _, err := d.Token(); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("%s: %v", f.Name, err)
				}
			}
		}
	}
	return out, z.File
}

func TestDOCX(t *testing.T) {
	p, _ := parts(t, &DOCXWriter{Page: "a5"})
	doc, styles := p["word/document.xml"], p["word/styles.xml"]
	for _, want := range []string{`w:styleId="Character"`, `w:styleId="Dialogue"`, `w:styleId="SceneHeading"`,
		`w:styleId="DualCharacter"`, `<w:name w:val="Scene Heading"/>`} {
		if !strings.Contains(styles, want) {
			t.Errorf("styles lack %s", want)
		}
	}
	for _, want := range []string{
		`<w:pgSz w:w="8395" w:h="11909"/>`,           // A5
		`INT. OFFICE KITCHEN - DAY</w:t>`,            // the heading, without its number
		`<w:tab/><w:t>1</w:t>`,                       // the number at the right
		`<w:tbl>`,                                    // dual dialogue
		`<w:pageBreakBefore/>`,                       // === and after the title page
		`<w:titlePg/>`, `<w:pgNumType w:start="0"/>`, // no number on the title page
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document lacks %s", want)
		}
	}
	if strings.Contains(doc, "#1#") || strings.Contains(doc, "# ACT") {
		t.Error("scene number or section marks in the text")
	}
}

func TestODT(t *testing.T) {
	p, files := parts(t, &ODTWriter{})
	if files[0].Name != "mimetype" || files[0].Method != zip.Store || files[0].Flags&0x8 != 0 ||
		p["mimetype"] != "application/vnd.oasis.opendocument.text" {
		t.Errorf("mimetype must come first, stored, without a data descriptor: %+v", files[0].FileHeader)
	}
	styles, content := p["styles.xml"], p["content.xml"]
	for _, want := range []string{`style:name="Character" style:display-name="Character"`,
		`fo:page-width="8.500in"`, `<text:page-number`} {
		if !strings.Contains(styles, want) {
			t.Errorf("styles lack %s", want)
		}
	}
	// ODF wants the named styles before the automatic ones
	if strings.Index(styles, "<office:styles>") > strings.Index(styles, "<office:automatic-styles>") {
		t.Error("office:styles after office:automatic-styles")
	}
	for _, want := range []string{`<table:table `, `fo:break-before="page"`, `style:page-number="1"`,
		`INT. OFFICE KITCHEN - DAY<text:tab/>1`} {
		if !strings.Contains(content, want) {
			t.Errorf("content lacks %s", want)
		}
	}
}

// A preset's styles reach the document: the musical's song headings in
// bold underline, its lyrics indented.
func TestPresetStyles(t *testing.T) {
	m, _ := rules.GetPreset("musical")
	d := prepare(fountain.Parse(nil, strings.NewReader("ANNA\n~A LYRIC\n\n> 3. SONG <\n")), m.Elements, GetPage(m.Page))
	c, l := d.byKey["center"], d.byKey["lyrics"]
	if c == nil || !c.Bold || !c.Underline || c.Align != 'C' {
		t.Errorf("center %+v", c)
	}
	if l == nil || l.Left <= 0 {
		t.Errorf("lyrics %+v", l)
	}
	if d.page.Name != "a5" {
		t.Errorf("page %+v", d.page)
	}
}
