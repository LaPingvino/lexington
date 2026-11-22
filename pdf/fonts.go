package pdf

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/lex"
)

// fonts resolves the rules' Font settings to the PDF's fonts:
//   - "" or Courier (Prime, Badi): the embedded Courier Badi;
//   - Helvetica (or Arial), Times: the PDF's standard fonts, which cover
//     Western European text (cp1252); a line with other characters is
//     printed in Courier Badi instead;
//   - the path of a TrueType or OpenType file, with its -Bold, -Italic and
//     -BoldItalic siblings if there are; an unreadable file is Courier.
//
// In the embedded and file fonts a Hebrew letter with points prints as
// its presentation form when the font has it (composeHebrew).
type fonts struct {
	pdf       Drawer
	translate func(string) string
	files     map[string]string          // a file's path to its name in the PDF
	has       map[string]func(rune) bool // a font's name to its glyph test
	data      map[string][]byte          // a file font's name to its regular face
}

func newFonts(pdf Drawer) *fonts {
	return &fonts{pdf: pdf, translate: pdf.UnicodeTranslatorFromDescriptor(""),
		files: map[string]string{}, has: map[string]func(rune) bool{}, data: map[string][]byte{}}
}

// compose is text as the (embedded or file) font name prints it: Hebrew
// letters with points as the presentation forms it has, Arabic shaped
// by the font's own features (arabic.go).
func (f *fonts) compose(name, text string) string {
	has, ok := f.has[name]
	if !ok {
		has = glyphTest(font.GetFont(name, ""))
		f.has[name] = has
	}
	if has == nil {
		return text
	}
	text = composeHebrew(text, has)
	if hasArabic(text) {
		data, ok := f.data[name]
		if !ok {
			data = font.GetFont(name, "")
		}
		if s := shaperFor(name, data); s != nil {
			text, _ = s.shape(text)
		}
	}
	return text
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
			return font.CourierPrimeName, f.compose(font.CourierPrimeName, text)
		}
		return name, f.translate(text)
	}
	if ext := strings.ToLower(filepath.Ext(setting)); ext == ".ttf" || ext == ".otf" {
		if name := f.file(setting); name != "" {
			return name, f.compose(name, text)
		}
	}
	name := font.GetFontName(setting)
	return name, f.compose(name, text)
}

// spans are a text's emphasis (lex.Emphasis), each span's text as the
// font for the setting needs it, and that font.
func (f *fonts) spans(setting, text string) (string, []lex.Span) {
	spans := lex.Emphasis(text)
	name, _ := f.font(setting, lex.Unstyled(text))
	if f == nil {
		return name, spans
	}
	for i := range spans {
		if name == "Helvetica" || name == "Times" {
			spans[i].Text = f.translate(spans[i].Text)
		} else {
			spans[i].Text = f.compose(name, spans[i].Text)
		}
	}
	return name, spans
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
		AddFont(f.pdf, name, style, b)
	}
	AddFont(f.pdf, name, "", regular)
	f.files[path] = name
	f.has[name] = glyphTest(regular)
	f.data[name] = regular
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
