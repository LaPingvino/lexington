package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

func render(t *testing.T, src string) string {
	t.Helper()
	screenplay := fountain.Parse(rules.DefaultConf().Scenes["en"], strings.NewReader(src))
	var buf bytes.Buffer
	if err := (&HTMLWriter{Elements: rules.Default}).Write(&buf, screenplay); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestTitlePageMetaFields(t *testing.T) {
	page := render(t, "Title: The Barn\nAuthor: Jane Smith\nContact:\n    Jane Smith\n    1 Lane\nDraft date: 1 May\n\nFADE IN:\n\nINT. BARN - DAY\n")
	meta := `<div class="title-meta"><p>Jane Smith</p><p>1 Lane</p><p>1 May</p></div>`
	if !strings.Contains(page, meta) {
		t.Errorf("missing title meta block %q in:\n%s", meta, page)
	}
	if n := strings.Count(page, `<div class="newpage">`); n != 1 {
		t.Errorf("%d page breaks, want 1 after the title page", n)
	}
	if open, closed := strings.Count(page, "<div"), strings.Count(page, "</div>"); open != closed {
		t.Errorf("unbalanced divs: %d opened, %d closed", open, closed)
	}
}

func TestScriptTextIsEscaped(t *testing.T) {
	page := render(t, "INT. LAB - NIGHT\n\nThe screen reads <script>alert(1)</script> & **BOOM**.\n")
	if strings.Contains(page, "<script>alert") {
		t.Error("script text was emitted as HTML")
	}
	if !strings.Contains(page, "&lt;script&gt;alert(1)&lt;/script&gt; &amp; <b>BOOM</b>.") {
		t.Errorf("escaped text with markup not found in:\n%s", page)
	}
}
