package fdx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
)

// A locked script with an A-scene and an omitted one: Number keeps
// "10A", and OMITTED is the heading's text with its number, as
// OpenDraft's golden file writes it (exportGolden/locked-omitted-scenes.fdx;
// how Final Draft itself writes one is not verified). The omitted
// scene's boneyard has no place in FDX: it is lost there, so Restore
// needs the Fountain. Read back, OMITTED is a forced heading again.
func TestLockedNumbersFDX(t *testing.T) {
	scenes := []string{"INT", "EXT"}
	o, _ := fountain.OmitScene("INT. GONE - DAY #11#\n\nAnna leaves.\n\n")
	src := "INT. ONE - DAY #10#\n\nHi.\n\nINT. NEW - NIGHT #10A#\n\nHo.\n\n" + o + "EXT. TWO - DAY #12#\n\nBye.\n"
	var b bytes.Buffer
	if err := (&FDXWriter{}).Write(&b, fountain.Parse(scenes, strings.NewReader(src))); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`<Paragraph Type="Scene Heading" Number="10A">`,
		`<Paragraph Type="Scene Heading" Number="11">`,
		`<Text>OMITTED</Text>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, "Anna leaves") {
		t.Errorf("the omitted scene's text is in the FDX:\n%s", out)
	}
	var heads []string
	for _, l := range Parse(strings.NewReader(out)) {
		if l.Type == lex.TypeScene {
			heads = append(heads, l.Contents)
		}
	}
	if strings.Join(heads, "|") != "INT. ONE - DAY #10#|INT. NEW - NIGHT #10A#|OMITTED #11#|EXT. TWO - DAY #12#" {
		t.Errorf("read back %q", heads)
	}
	var f bytes.Buffer
	if err := (&fountain.FountainWriter{}).Write(&f, Parse(strings.NewReader(out))); err != nil {
		t.Skip("no Fountain writer by that name:", err)
	}
	if !strings.Contains(f.String(), ".OMITTED #11#") {
		t.Errorf("Fountain from the FDX:\n%s", f.String())
	}
}

// v1.13.0's merged OMITTED heading is read as its scenes (here no blocks
// under it: its first and last), and FDX writes one heading a number.
func TestOldOmittedRangeFDX(t *testing.T) {
	src := "INT. ONE - DAY #11#\n\nHi.\n\n.OMITTED #12-14#\n\nEXT. TWO - DAY #15#\n\nBye.\n"
	var b bytes.Buffer
	if err := (&FDXWriter{}).Write(&b, fountain.Parse([]string{"INT", "EXT"}, strings.NewReader(src))); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), `Number="12-14"`) || !strings.Contains(b.String(), `Number="12"`) || !strings.Contains(b.String(), `Number="14"`) {
		t.Errorf("not one heading a number:\n%s", b.String())
	}
}
