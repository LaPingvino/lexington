package pdf

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

func TestTitlePageMetaFieldsArePrinted(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not available")
	}
	src := "Title: The Barn\nAuthor: Jane Smith\nContact:\n    Jane Smith\n    1 Writer's Lane\nDraft date: 1 May\n\nFADE IN:\n\nINT. BARN - DAY\n"
	screenplay := fountain.Parse(rules.DefaultConf().Scenes["en"], strings.NewReader(src))

	out := filepath.Join(t.TempDir(), "barn.pdf")
	w := &PDFWriter{OutputFile: out, Elements: rules.Default}
	if err := w.Write(nil, screenplay); err != nil {
		t.Fatal(err)
	}
	text, err := exec.Command("pdftotext", "-layout", out, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	pages := strings.Split(string(text), "\f")
	if len(pages) < 2 {
		t.Fatalf("expected a title page and a script page, got:\n%s", text)
	}
	for _, s := range []string{"The Barn", "Jane Smith", "1 Writer's Lane", "1 May"} {
		if !strings.Contains(pages[0], s) {
			t.Errorf("title page is missing %q:\n%s", s, pages[0])
		}
	}
	for _, s := range []string{"FADE IN:", "INT. BARN - DAY"} {
		if !strings.Contains(pages[1], s) {
			t.Errorf("first script page is missing %q:\n%s", s, pages[1])
		}
	}
}
