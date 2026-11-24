package pdf

import (
	"slices"
	"sort"
	"sync"
	"unicode"

	"golang.org/x/image/font/sfnt"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/lex"
)

var (
	courier     *sfnt.Font
	courierOnce sync.Once
)

//go:generate go run gen_joining.go

// Unsupported is what the PDF cannot print of a script as it is written:
// the characters neither its font (Courier Badi) nor a font on the
// computer has (fallback.go), which print as blanks, and whether it has
// letters that join their neighbours that the PDF cannot shape (Syriac,
// N'Ko, Mongolian…: Joining_Type D, R or L; of Arabic only the letters
// whose joined forms Courier Badi has no character for, arabic.go), so
// they would print unjoined (Unshaped names their scripts). Arabic is
// shaped, right to left is no problem (bidi.go). Printing the HTML (a
// browser's Print, Save as PDF) has neither problem.
func Unsupported(s lex.Screenplay) (missing []rune, shaping bool) {
	missing, scripts := unsupported(s)
	return missing, len(scripts) > 0
}

// Unshaped are the scripts whose joining letters the PDF of a script
// cannot shape (Unsupported), by their Unicode names ("Syriac", "Nko"),
// sorted; none if it shapes all it has.
func Unshaped(s lex.Screenplay) []string {
	_, scripts := unsupported(s)
	return scripts
}

func unsupported(s lex.Screenplay) (missing []rune, scripts []string) {
	courierOnce.Do(func() { courier, _ = sfnt.Parse(font.GetFont(font.CourierPrimeName, "")) })
	var buf sfnt.Buffer
	seen := map[rune]bool{}
	unshaped := map[string]bool{}
	shaper := shaperFor(font.CourierPrimeName, font.GetFont(font.CourierPrimeName, ""))
	for _, l := range lex.Printable(s, nil) {
		if shaper != nil && !unshaped["Arabic"] && hasArabic(string(l.Contents)) {
			if _, n := shaper.shape(string(l.Contents)); n > 0 {
				unshaped["Arabic"] = true
			}
		}
		for _, r := range string(l.Contents) {
			if seen[r] || unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) { // marks print as nothing
				continue
			}
			seen[r] = true
			if unicode.Is(joining, r) && !unicode.Is(unicode.Arabic, r) {
				unshaped[scriptOf(r)] = true
			}
			if courier != nil {
				if g, err := courier.GlyphIndex(&buf, r); err == nil && g == 0 {
					missing = append(missing, r)
				}
			}
		}
	}
	if len(missing) > 0 { // those the computer's fonts have print
		has := systemHas(lacked(nil, missing))
		missing = slices.DeleteFunc(missing, func(r rune) bool { return has[r] })
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	for name := range unshaped {
		scripts = append(scripts, name)
	}
	slices.Sort(scripts)
	return missing, scripts
}

// scriptOf is the name of a character's Unicode script ("Common" if it
// has none of its own).
func scriptOf(r rune) string {
	for name, t := range unicode.Scripts {
		if name != "Common" && name != "Inherited" && unicode.Is(t, r) {
			return name
		}
	}
	return "Common"
}
