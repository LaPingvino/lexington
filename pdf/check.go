package pdf

import (
	"sort"
	"sync"
	"unicode"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/unicode/bidi"

	"github.com/LaPingvino/lexington/font"
	"github.com/LaPingvino/lexington/lex"
)

var (
	courier     *sfnt.Font
	courierOnce sync.Once
)

// Unsupported is what the PDF cannot print of a script as it is written:
// the characters its font (Courier Badi) lacks, which would show as
// boxes, and whether it has right-to-left text (Hebrew, Arabic), which
// the PDF lays out left to right with unjoined letters. Printing the HTML
// (a browser's Print, Save as PDF) has neither problem.
func Unsupported(s lex.Screenplay) (missing []rune, rtl bool) {
	courierOnce.Do(func() { courier, _ = sfnt.Parse(font.GetFont(font.CourierPrimeName, "")) })
	var buf sfnt.Buffer
	seen := map[rune]bool{}
	for _, l := range s {
		for _, r := range string(l.Contents) {
			if seen[r] || unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) { // marks print as nothing
				continue
			}
			seen[r] = true
			if p, _ := bidi.LookupRune(r); p.Class() == bidi.R || p.Class() == bidi.AL {
				rtl = true
			}
			if courier != nil {
				if g, err := courier.GlyphIndex(&buf, r); err == nil && g == 0 {
					missing = append(missing, r)
				}
			}
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	return missing, rtl
}
