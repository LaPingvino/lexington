package pdf

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/phpdave11/gofpdf"

	"github.com/LaPingvino/lexington/font"
)

// fonts resolves the rules' Font settings to the PDF's fonts:
//   - "" or Courier (Prime, Badi): the embedded Courier Badi;
//   - Helvetica (or Arial), Times: the PDF's standard fonts, which cover
//     Western European text (cp1252); a line with other characters is
//     printed in Courier Badi instead;
//   - the path of a TrueType or OpenType file, with its -Bold, -Italic and
//     -BoldItalic siblings if there are; an unreadable file is Courier.
type fonts struct {
	pdf       *gofpdf.Fpdf
	translate func(string) string
	files     map[string]string // a file's path to its name in the PDF
}

func newFonts(pdf *gofpdf.Fpdf) *fonts {
	return &fonts{pdf: pdf, translate: pdf.UnicodeTranslatorFromDescriptor(""), files: map[string]string{}}
}

// core are the standard fonts by the names one can give them.
var core = map[string]string{
	"helvetica": "Helvetica", "arial": "Helvetica", "sans": "Helvetica", "sans-serif": "Helvetica",
	"times": "Times", "times new roman": "Times", "serif": "Times",
}

// font is the PDF font for a Font setting and the text as that font
// needs it.
func (f *fonts) font(setting, text string) (string, string) {
	if f == nil {
		return font.CourierPrimeName, text
	}
	if name, ok := core[strings.ToLower(strings.TrimSpace(setting))]; ok {
		if !cp1252(text) {
			return font.CourierPrimeName, text
		}
		return name, f.translate(text)
	}
	if ext := strings.ToLower(filepath.Ext(setting)); ext == ".ttf" || ext == ".otf" {
		if name := f.file(setting); name != "" {
			return name, text
		}
	}
	return font.GetFontName(setting), text
}

// file registers a font file and its style siblings; "" if unreadable.
func (f *fonts) file(path string) string {
	if name, ok := f.files[path]; ok {
		return name
	}
	regular, err := os.ReadFile(path)
	if err != nil {
		f.files[path] = ""
		return ""
	}
	name := "file:" + path
	base := strings.TrimSuffix(path, filepath.Ext(path))
	base = strings.TrimSuffix(strings.TrimSuffix(base, "-Regular"), "Regular")
	for style, suffixes := range map[string][]string{
		"B": {"-Bold", "Bold"}, "I": {"-Italic", "Italic"}, "BI": {"-BoldItalic", "BoldItalic"},
	} {
		b := regular // without a sibling the regular face does
		for _, s := range suffixes {
			if data, err := os.ReadFile(base + s + filepath.Ext(path)); err == nil {
				b = data
				break
			}
		}
		f.pdf.AddUTF8FontFromBytes(name, style, b)
	}
	f.pdf.AddUTF8FontFromBytes(name, "", regular)
	f.files[path] = name
	return name
}

// cp1252 reports whether the standard fonts can print text.
func cp1252(text string) bool {
	for _, r := range text {
		if r < 0x80 || (r >= 0xA0 && r <= 0xFF) {
			continue
		}
		if !strings.ContainsRune("€‚ƒ„…†‡ˆ‰Š‹ŒŽ‘’“”•–—˜™š›œžŸ", r) {
			return false
		}
	}
	return true
}
