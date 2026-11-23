package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-text/typesetting/di"
	gofont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/LaPingvino/lexington/font"
)

// TestArabicShapedAsHarfBuzz is the proof of the shaping: for every line
// of Hekaya's own samples, in each of Courier Badi's faces, the glyphs the
// PDF prints (the characters compose gives, through the font's cmap, as
// gofpdf maps them) are the glyphs HarfBuzz shapes the line into with the
// font's own features, in logical order (joiners, which print nothing,
// left out).
func TestArabicShapedAsHarfBuzz(t *testing.T) {
	files, _ := filepath.Glob("../testdata/hekaya/*.hekaya")
	if len(files) == 0 {
		t.Fatal("no samples")
	}
	var lines []string
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if hasArabic(l) {
				lines = append(lines, strings.TrimSpace(l))
			}
		}
	}
	lines = append(lines, "لا إله إلا الله", "سَمِيرٌ يَمْشِي", "يا سمير!", "(بهدوء) مش عايز؟", "مي\u200cخواهم")
	for _, style := range []string{"", "B", "I", "BI"} {
		data := font.GetFont(font.CourierPrimeName, style)
		s := newArabicShaper(data)
		face, _ := gofont.ParseTTF(bytes.NewReader(data))
		sf, _ := sfnt.Parse(data)
		var buf sfnt.Buffer
		var sh shaping.HarfbuzzShaper
		checked := 0
		for _, l := range lines {
			got, n := s.shape(l)
			if n != 0 {
				t.Errorf("%s %q: %d letters not shaped", style, l, n)
			}
			var printed []gofont.GID
			for _, r := range got {
				if r == 0x200C || r == 0x200D {
					continue
				}
				g, _ := sf.GlyphIndex(&buf, r)
				printed = append(printed, gofont.GID(g))
			}
			// HarfBuzz over each Arabic run, the rest as is
			var want []gofont.GID
			rs := []rune(l)
			for i := 0; i < len(rs); {
				if !arabicRune(rs[i]) {
					g, _ := sf.GlyphIndex(&buf, rs[i])
					want = append(want, gofont.GID(g))
					i++
					continue
				}
				j := arabicRunEnd(rs, i)
				out := sh.Shape(shaping.Input{Text: rs[i:j], RunEnd: j - i, Direction: di.DirectionRTL, Face: face,
					Size: fixed.I(12), Script: language.Arabic, Language: language.NewLanguage("ar")})
				for k := len(out.Glyphs) - 1; k >= 0; k-- {
					if g := out.Glyphs[k]; !(rs[i+g.ClusterIndex] == 0x200C || rs[i+g.ClusterIndex] == 0x200D) {
						want = append(want, g.GlyphID)
					}
				}
				i = j
			}
			if !equalGIDs(printed, want) {
				t.Errorf("%s %q:\nprinted %v\nshaped  %v", style, l, printed, want)
			}
			checked++
		}
		if checked < 100 {
			t.Errorf("only %d lines", checked)
		}
	}
}

func equalGIDs(a, b []gofont.GID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestArabicForms(t *testing.T) {
	s := shaperFor(font.CourierPrimeName, font.GetFont(font.CourierPrimeName, ""))
	for in, want := range map[string]string{
		"قهوة":    "\uFED7\uFEEC\uFEEE\u0629", // initial, medial, final waw, teh marbuta alone (waw does not join on)
		"لا":      "\uFEFB",                   // lam-alef
		"سمير ١٢": "\uFEB3\uFEE4\uFEF4\uFEAE ١٢",
		"Hello":   "Hello",
		// vowelled: the marks stay after their letters, the letters joined
		"سِمْسِمْ": "\uFEB3\u0650\uFEE4\u0652\uFEB4\u0650\uFEE2\u0652",
	} {
		if got, _ := s.shape(in); got != want {
			t.Errorf("%q: %+q, want %+q", in, got, want)
		}
	}
}
