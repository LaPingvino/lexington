package pdf

import (
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
// the characters its font (Courier Badi) lacks, which would show as
// boxes, and whether it has letters that join their neighbours that the
// PDF cannot shape (Syriac, N'Ko, Mongolian…: Joining_Type D, R or L; of
// Arabic only the letters whose joined forms Courier Badi has no
// character for, arabic.go), so they would print unjoined. Arabic is
// shaped, right to left is no problem (bidi.go). Printing the HTML (a
// browser's Print, Save as PDF) has neither problem.
func Unsupported(s lex.Screenplay) (missing []rune, shaping bool) {
	courierOnce.Do(func() { courier, _ = sfnt.Parse(font.GetFont(font.CourierPrimeName, "")) })
	var buf sfnt.Buffer
	seen := map[rune]bool{}
	shaper := shaperFor(font.CourierPrimeName, font.GetFont(font.CourierPrimeName, ""))
	for _, l := range lex.Printable(s, nil) {
		if shaper != nil && !shaping && hasArabic(string(l.Contents)) {
			if _, n := shaper.shape(string(l.Contents)); n > 0 {
				shaping = true
			}
		}
		for _, r := range string(l.Contents) {
			if seen[r] || unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) { // marks print as nothing
				continue
			}
			seen[r] = true
			if unicode.Is(joining, r) && !unicode.Is(unicode.Arabic, r) {
				shaping = true
			}
			if courier != nil {
				if g, err := courier.GlyphIndex(&buf, r); err == nil && g == 0 {
					missing = append(missing, r)
				}
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return missing, shaping
}
