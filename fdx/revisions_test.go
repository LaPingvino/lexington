package fdx

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/lexington/lex"
)

func TestRevisionsWrittenAndRead(t *testing.T) {
	s := lex.Screenplay{
		{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY #1#"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeAction, Contents: "She runs.", Revision: "Blue"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeSpeaker, Contents: "BOB"},
		{Type: lex.TypeDialog, Contents: "Wait for me.", Revision: "Pink"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeAction, Contents: "Nothing changed here."},
	}
	r := &lex.Revisions{Sets: []lex.RevisionSet{
		{Name: "White", Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "Blue", Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)},
		{Name: "Pink", Date: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)},
	}}
	var buf bytes.Buffer
	if err := (&FDXWriter{Revisions: r}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		`<Revisions ActiveSet="2" Location="7.75"`,
		`ID="1" Mark="*" Name="Blue Rev. 20/11/26" PageColor="#C6C6EDEDFEFE"`,
		`ID="2" Mark="*" Name="Pink Rev. 25/11/26"`,
		`<Text RevisionID="1">She runs.</Text>`,
		`<Text RevisionID="2">Wait for me.</Text>`,
		`<Text>Nothing changed here.</Text>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in\n%s", want, out)
		}
	}
	if strings.Contains(out, "White Rev") || strings.Contains(out, "DeletedText") {
		t.Error("White written as a set, or deletions written")
	}
	back, sets, err := ParseRevisions(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range back {
		if l.Revision != "" {
			got[l.Contents] = l.Revision
		}
	}
	if len(got) != 2 || got["She runs."] != "Blue" || got["Wait for me."] != "Pink" {
		t.Errorf("read back %v", got)
	}
	if len(sets) != 2 || sets[0].Name != "Blue" || !sets[0].Date.Equal(r.Sets[1].Date) || sets[1].Name != "Pink" {
		t.Errorf("sets %+v", sets)
	}
	// without sets, as before: no Revisions, no RevisionID on the text
	buf.Reset()
	if err := (&FDXWriter{}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "<Revisions") || strings.Contains(buf.String(), `<Text RevisionID`) {
		t.Error("revisions written without sets")
	}
}

// finalDraftRevised is as Final Draft writes a revised script (its
// default sets' names, a deletion: the sample's form, cut down).
const finalDraftRevised = `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>
<FinalDraft DocumentType="Script" Template="No" Version="5">
  <Content>
    <Paragraph Type="Scene Heading"><Text RevisionID="0">INT. STUDIO</Text></Paragraph>
    <Paragraph Type="Action"><Text RevisionID="0">Show the product.</Text></Paragraph>
    <Paragraph Type="Character"><Text RevisionID="2">MF</Text><Text RevisionID="0"> (V.O.)</Text></Paragraph>
    <Paragraph Type="Dialogue"><Text RevisionID="0">I made this.</Text></Paragraph>
    <Paragraph Type="Action"><Text RevisionID="0">Title sequence</Text></Paragraph>
    <Paragraph Type="Action"><Text RevisionID="0">The end.</Text></Paragraph>
  </Content>
  <Revisions ActiveSet="2" Location="7.75" RevisionMode="Yes" RevisionsShown="Active" ShowAllMarks="No" ShowAllSets="No" ShowPageColor="No">
    <Revision Color="#000000000000" FullRevision="No" ID="1" Mark="*" Name="Blue Rev. (mm/dd/yy)" PageColor="#C6C6EDEDFEFE" Style=""/>
    <Revision Color="#000000000000" FullRevision="No" ID="2" Mark="*" Name="Pink Rev. (mm/dd/yy)" PageColor="#FDFDCCCCD4D4" Style=""/>
    <Revision Color="#000000000000" FullRevision="No" ID="9" Mark="*" Name="Tan Rev. (mm/dd/yy)" PageColor="#FDFDF1F1C7C7" Style=""/>
  </Revisions>
  <DeletedText>
    <DeletedTextLocation Offset="62" RevisionID="1"/>
  </DeletedText>
</FinalDraft>`

func TestFinalDraftRevisionsRead(t *testing.T) {
	s, sets, err := ParseRevisions(strings.NewReader(finalDraftRevised))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, l := range s {
		if l.Revision != "" {
			got[l.Contents] = l.Revision
		}
	}
	// "Title sequence" starts at 11+1+17+1+9+1+12+1 = 53; the deletion
	// at 62 is within it
	want := map[string]string{"MF (V.O.)": "Pink", "Title sequence": "Blue"}
	if len(got) != len(want) {
		t.Errorf("marked %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%q: %q, want %q", k, got[k], v)
		}
	}
	// the sets used, by ID: Tan is listed, not used
	if len(sets) != 2 || sets[0].Name != "Blue" || sets[1].Name != "Pink" || !sets[0].Date.IsZero() {
		t.Errorf("sets %+v", sets)
	}
	if lex.RevisionColor(sets[1].Name) != "Pink" {
		t.Errorf("colour of %q: %q", sets[1].Name, lex.RevisionColor(sets[1].Name))
	}
	// Parse is as ever, and keeps the marks too
	if p, err := ParseWithError(strings.NewReader(finalDraftRevised)); err != nil || len(p) != len(s) {
		t.Errorf("ParseWithError: %v, %d lines", err, len(p))
	}
}
