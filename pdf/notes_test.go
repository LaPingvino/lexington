package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// The outline (bookmarks) of sections and scenes leaves their notes out,
// as SectionText and SceneNumber do.
func TestOutlineWithoutNotes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.pdf")
	s := lex.Screenplay{
		{Type: lex.TypeSection, Contents: "# Dominik 1 [[blue]]"},
		{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY [[red]] #12#"},
		{Type: lex.TypeNote, Contents: "[[A standalone note]]"},
		{Type: lex.TypeAction, Contents: "Rain."},
	}
	if err := (&PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, m := range regexp.MustCompile(`/Title \(([^)]*)\)`).FindAllSubmatch(data, -1) {
		titles = append(titles, string(bytes.ReplaceAll(bytes.TrimPrefix(m[1], []byte{0xfe, 0xff}), []byte{0}, nil)))
	}
	if got := strings.Join(titles, "|"); got != "Dominik 1|INT. HOUSE - DAY" {
		t.Errorf("outline %q", got)
	}
}
