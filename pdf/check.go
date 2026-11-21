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
// boxes, and whether it has letters that join their neighbours (Arabic,
// Syriac, N'Ko, Mongolian…: Joining_Type D, R or L), which need shaping
// the PDF does not do, so they would print unjoined. Right to left alone
// is no problem (Hebrew prints as written: bidi.go). Printing the HTML
// (a browser's Print, Save as PDF) has neither problem.
func Unsupported(s lex.Screenplay) (missing []rune, shaping bool) {
	courierOnce.Do(func() { courier, _ = sfnt.Parse(font.GetFont(font.CourierPrimeName, "")) })
	var buf sfnt.Buffer
	seen := map[rune]bool{}
	for _, l := range lex.Printable(s, nil) {
		for _, r := range string(l.Contents) {
			if seen[r] || unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) { // marks print as nothing
				continue
			}
			seen[r] = true
			if unicode.Is(joining, r) {
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
