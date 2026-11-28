package osf

import (
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/formattest"
)

// testdata/fadein5.fadein is a script Fade In 5 saved (5.0.15): OSF 5.0.
const fadeIn5 = "testdata/fadein5.fadein"

func fadeIn5Doc(t *testing.T) string {
	t.Helper()
	doc, err := ReadFadeIn(fadeIn5)
	if err != nil {
		t.Fatal(err)
	}
	return string(doc)
}

// Fade In 5's file reads as its script: emphasis, dual dialogue, the note
// on the centred line a line of notes before it, the title page.
func TestReadFadeIn5(t *testing.T) {
	got := strings.Join(formattest.Rows(parse(t, fadeIn5Doc(t)), nil), "\n")
	for _, w := range []string{
		"Title: Lexington Check",
		"Contact: example@example.org",
		"action: FADE IN:\nscene: INT. KITCHEN - MORNING",
		"action: A small kitchen. Sunlight. **Bold action**, *italic action*, _underlined action_ and ***bold italic***.",
		"dualspeaker_open: \nspeaker: MARIA\ndialog: The one on the table.\ndualspeaker_next: \nspeaker: JONAS\ndialog: Oh. That one.\ndualspeaker_close: ",
		"note: [[A note for the writer: this note should survive or be dropped cleanly.]]\ncenter: THE END OF THE BEGINNING",
		"trans: CUT TO:\nscene: INT. KITCHEN - LATER",
		"action: FADE OUT.",
	} {
		if !strings.Contains(got, w) {
			t.Errorf("no %q in\n%s", w, got)
		}
	}
}

// Written again in 5.0, the default, Fade In 5's file is as Fade In wrote
// it: its version, settings, styles, paragraphs and title page; and it
// reads back as the same script.
func TestWriteFadeIn5(t *testing.T) {
	orig := fadeIn5Doc(t)
	s := parse(t, orig)
	out := string((&OSFWriter{}).Document(s))
	if v := attrOf([]byte(out), "version"); v != "50" {
		t.Errorf("version %q, want 50", v)
	}
	if !regexp.MustCompile(`<info uuid="[0-9A-F-]+" draft_uuid="[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}"/>`).MatchString(out) {
		t.Errorf("no Fade In 5 info in\n%s", out)
	}
	part := func(doc, from, to string) string {
		_, rest, _ := strings.Cut(doc, from)
		in, _, _ := strings.Cut(rest, to)
		return in
	}
	for _, p := range [][2]string{{"<settings", "/>"}, {"<styles>", "</styles>"}, {"<paragraphs>", "</paragraphs>"}, {"<titlepage>", "</titlepage>"}} {
		if a, b := part(orig, p[0], p[1]), part(out, p[0], p[1]); a != b {
			t.Errorf("%s differs from Fade In's:\n%s\n---\n%s", p[0], b, a)
		}
	}
	formattest.Same(t, "5.0 back", formattest.Rows(s, nil), formattest.Rows(parse(t, out), nil))
}
