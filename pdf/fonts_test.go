package pdf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/phpdave11/gofpdf"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

func TestFontSettings(t *testing.T) {
	f := newFonts(gofpdf.New("P", "in", "Letter", ""))
	dir := t.TempDir()
	file := filepath.Join(dir, "Mine-Regular.ttf")
	os.WriteFile(file, font.GetFont("CourierPrime", ""), 0o644)
	os.WriteFile(filepath.Join(dir, "Mine-Bold.ttf"), font.GetFont("CourierPrime", "B"), 0o644)
	for _, c := range []struct{ setting, text, want string }{
		{"", "Hello", font.CourierPrimeName},
		{"Courier", "Hello", font.CourierPrimeName},
		{"Helvetica", "Hello, café", "Helvetica"},
		{"Arial", "“Quotes” – fine", "Helvetica"},
		{"Times", "Hello", "Times"},
		{"Helvetica", "Привет", font.CourierPrimeName}, // not in the standard fonts
		{file, "Привет", "file:" + file},
		{filepath.Join(dir, "missing.ttf"), "Hello", font.CourierPrimeName},
		{"Comic Sans", "Hello", font.CourierPrimeName},
	} {
		if got, _ := f.font(c.setting, c.text); got != c.want {
			t.Errorf("font(%q, %q) = %q, want %q", c.setting, c.text, got, c.want)
		}
	}
}

// The default lyrics are in Helvetica, as the rules always said.
func TestLyricsInHelvetica(t *testing.T) {
	out := filepath.Join(t.TempDir(), "lyrics.pdf")
	s := fountain.Parse(nil, strings.NewReader("ANNA\n~Who took the biscuit?\n"))
	if err := (&PDFWriter{OutputFile: out, Elements: rules.Default}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(out)
	if !strings.Contains(string(b), "/BaseFont /Helvetica") {
		t.Error("no Helvetica in the PDF")
	}
}
