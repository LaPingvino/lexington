package fdx_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fdx"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
)

func lockScript() lex.Screenplay {
	var b strings.Builder
	b.WriteString("Title: Locked\n\n")
	for k := 1; k <= 12; k++ {
		fmt.Fprintf(&b, "INT. ROOM %d - DAY #%d#\n\n", k, k)
		for j := 1; j <= 3; j++ {
			fmt.Fprintf(&b, "Anna%d%d walks the wall %d%da and counts the windows %d%db one by one, as if "+
				"each %d%dc could tell her *something* new %d%dd about the house.\n\n", k, j, k, j, k, j, k, j, k, j)
		}
		fmt.Fprintf(&b, "ANNA\nLine%d one. Line%d two is long, longer than one would think. Line%d three. "+
			"Line%d four goes on and on about the rain and the garden and the kitchen.\n\n", k, k, k, k)
		if k%3 == 0 {
			fmt.Fprintf(&b, "BEN\nYes%d.\n\nANNA ^\nNo%d.\n\n", k, k)
		}
	}
	return fountain.ParseLines(fountain.Scene, strings.NewReader(b.String()))
}

// TestLockedPagesRoundTrip: a lock written as <LockedPages> reads back as
// the same pages, at the same places.
func TestLockedPagesRoundTrip(t *testing.T) {
	// (a script as the FDX reader reads it: Fountain's dual dialogue
	// comes back with a blank line more, which a lock would show)
	var plain bytes.Buffer
	if err := (&fdx.FDXWriter{}).Write(&plain, lockScript()); err != nil {
		t.Fatal(err)
	}
	s, _ := fdx.ParseWithError(bytes.NewReader(plain.Bytes()))
	lock := (&pdf.PDFWriter{}).LockPages(s)
	if len(lock.Pages) < 5 {
		t.Fatalf("%d pages", len(lock.Pages))
	}
	lock.Pages[3].Label = "4A" // as if locked again with an A-page
	var buf bytes.Buffer
	if err := (&fdx.FDXWriter{Lock: &lock}).Write(&buf, s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<LockedPage LevelIndex="3" LockLevel="1" PageNumber="4A" Position="`) {
		t.Fatalf("no page 4A in\n%s", buf.String()[:min(3000, buf.Len())])
	}
	back, err := fdx.ParseLockedPages(bytes.NewReader(buf.Bytes()))
	if err != nil || back == nil {
		t.Fatal(err, back)
	}
	if len(back.Pages) != len(lock.Pages) {
		t.Fatalf("%d pages back, %d written", len(back.Pages), len(lock.Pages))
	}
	read, _ := fdx.ParseWithError(bytes.NewReader(buf.Bytes()))
	for i := range lock.Pages {
		w, b := lock.Pages[i], back.Pages[i]
		if w.Label != b.Label || w.Words != b.Words || w.Scene != b.Scene {
			t.Errorf("page %d: %v, written %v", i, b, w)
		}
	}
	// the pages print the same from the FDX
	got := (&pdf.PDFWriter{Lock: back}).Pages(read)
	var labels []string
	for _, p := range got {
		labels = append(labels, p.Label)
	}
	var want []string
	for _, p := range lock.Pages {
		want = append(want, p.Label)
	}
	if !slices.Equal(labels, want) {
		t.Errorf("printed from the FDX: %v, want %v", labels, want)
	}
}

// TestLockedPagesRead: Final Draft's form, a page within a speech after a
// sentence, a stale position past the end left out.
func TestLockedPagesRead(t *testing.T) {
	file := `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>
<FinalDraft DocumentType="Script" Template="No" Version="1">
  <Content>
    <Paragraph Number="1" Type="Scene Heading"><Text>INT. STUDIO - DAY</Text></Paragraph>
    <Paragraph Type="Action"><Text>MF sits.</Text></Paragraph>
    <Paragraph Type="Character"><Text>MF</Text></Paragraph>
    <Paragraph Type="Dialogue"><Text>First sentence here. Second sentence starts the page.</Text></Paragraph>
  </Content>
  <LockedPages>
    <LockedPage LevelIndex="0" LockLevel="0" PageNumber="1" Position="0"/>
    <LockedPage LevelIndex="1" LockLevel="0" PageNumber="2" Position="` + fmt.Sprint(len("INT. STUDIO - DAY")+1+len("MF sits.")+1+len("MF")+1+len("First sentence here. ")) + `"/>
    <LockedPage LevelIndex="2" LockLevel="0" PageNumber="3" Position="99999"/>
  </LockedPages>
</FinalDraft>`
	lock, err := fdx.ParseLockedPages(strings.NewReader(file))
	if err != nil || lock == nil {
		t.Fatal(err)
	}
	if len(lock.Pages) != 2 || lock.Pages[1].Label != "2" || !strings.HasPrefix(lock.Pages[1].Words, "second sentence starts") ||
		lock.Pages[1].Before != "studio day mf sits mf first sentence here" || lock.Pages[1].Scene != "1" {
		t.Fatalf("read %+v", lock.Pages)
	}
}
