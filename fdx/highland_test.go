package fdx

import (
	"strings"
	"testing"
	"time"

	"github.com/LaPingvino/lexington/lex"
)

// Highland's directives in Final Draft: the header and footer as its
// HeaderAndFooter (%p its Page # label), SERIES and TIMESTAMP as they
// print, a TOC as its sections, the rest left out.
func TestHighlandDirectivesFDX(t *testing.T) {
	old := lex.Today
	lex.Today = func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local) }
	defer func() { lex.Today = old }()
	out, doc := writeFDX(t, "{{HEADER: Draft | %p.}}\n{{FOOTER: %none}}\n\n{{TOC}}\n\n# Act {{SERIES}}\n\nINT. ROOM - DAY\n\nTake {{SERIES}} on {{TIMESTAMP}}. {{%m: x}}\n")
	if got := strings.Join(summary(doc.Content.Paragraphs), " / "); got != "Action:Act 1 / Scene Heading:INT. ROOM - DAY / Action:Take 2 on Sep 28, 2026." {
		t.Errorf("paragraphs: %s", got)
	}
	hf := doc.HeaderAndFooter
	if hf == nil || hf.HeaderVisible != "Yes" || hf.HeaderFirstPage != "Yes" || hf.FooterVisible != "No" {
		t.Fatalf("header and footer: %+v", hf)
	}
	if !strings.Contains(out, `<Header><Paragraph Alignment="Left"`) ||
		!strings.Contains(hf.Header.Inner, `<Text>Draft</Text><Text>&#x9;</Text><Text>&#x9;</Text><DynamicLabel Type="Page #"/><Text>.</Text>`) {
		t.Errorf("header: %s", hf.Header.Inner)
	}
	if _, doc := writeFDX(t, "INT. ROOM - DAY\n"); doc.HeaderAndFooter != nil {
		t.Error("a header and footer without directives")
	}
}
