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
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
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
		s := newFontShaper(data)
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
					Size: fixed.I(12), Script: language.Arabic, Language: arabicLanguage})
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
		"قهوة":     "\uFED7\uFEEC\uFEEE\u0629", // initial, medial, final waw, teh marbuta alone (waw does not join on)
		"لا":       "\uFEFB",                   // lam-alef
		"لَا":      "\uFEFB\u064E",             // with a vowel on the lam (Courier Badi 1.109)
		"لـا":      "\uFEDF\u0640\uFE8E",       // a tatweel between: joined to it, no ligature
		"ل\u200dا": "\uFEDF\uFE8E",             // a ZWJ between: joined forms, the joiner printing nothing
		"سمير ١٢":  "\uFEB3\uFEE4\uFEF4\uFEAE ١٢",
		"Hello":    "Hello",
		// vowelled: the marks stay after their letters, the letters joined
		"سِمْسِمْ": "\uFEB3\u0650\uFEE4\u0652\uFEB4\u0650\uFEE2\u0652",
	} {
		if got, _ := s.shape(in); got != want {
			t.Errorf("%q: %+q, want %+q", in, got, want)
		}
	}
}

// A lam-alef takes one cell as the PDF prints it, also with vowel marks on
// the lam (Courier Badi 1.109's ligature over them); a tatweel or a joiner
// between keeps the two apart: in each style, the shaped text's cellWidth
// is its Lekhan width (lekhan.Width of the text as written, as Accolade
// counts it) and the shaper's advance.
func TestLamAlefCells(t *testing.T) {
	words := []string{"لا", "لَا", "لّا", "لٰا", "لّٰا", "لاَ", "لأ", "لِإ", "لَآ", "فَلَّا", "السَّلَامُ", "إلا",
		"لَـا", "لـا", "ل\u200dا", "ل\u200cا", "ل\u200dَا", "لٱ"}
	for _, style := range []string{"", "B", "I", "BI"} {
		data := font.GetFont(font.CourierPrimeName, style)
		pdf := NewFpdf("P", "in", "Letter")
		AddFont(pdf, "CourierPrime", style, data)
		pdf.SetFont("CourierPrime", style, 12)
		space := pdf.GetStringWidth(" ")
		sh := newFontShaper(data)
		for _, w := range words {
			want := cells(w)
			shaped, n := sh.shape(w)
			if n > 0 {
				t.Errorf("%s %q: %d letters unshaped", style, w, n)
			}
			if got := cellWidth(pdf, shaped, false); abs(got-float64(want)*space) > 1e-6 {
				t.Errorf("%s %q: printed as %+q, %.2f cells, want %d", style, w, shaped, got/space, want)
			}
			rs := []rune(w)
			out := sh.sh.Shape(shaping.Input{Text: rs, RunEnd: len(rs), Direction: di.DirectionRTL, Face: sh.face,
				Size: fixed.I(int(sh.face.Upem())), Script: language.Arabic, Language: arabicLanguage})
			if got := float64(out.Advance) / 64 / float64(sh.face.Upem()) * 12 / 72; abs(got-float64(want)*space) > 1e-3 {
				t.Errorf("%s %q: the font draws %.2f cells, want %d", style, w, got/space, want)
			}
		}
	}
	for w, want := range map[string]int{"لا": 1, "لَا": 1, "لّٰا": 1, "فَلَّا": 2, "لَـا": 3, "ل\u200dا": 2} {
		if cells(w) != want {
			t.Errorf("Lekhan width of %q: %d, want %d", w, cells(w), want)
		}
	}
}

// A vowel on the lam of a lam-alef sits on the lam's half of the
// ligature, one on the alef on the alef's, in each style: its place
// comes from the shaping of the text as written (fonts.marks), as both
// print as U+FEFB U+064E, and the printed form reshaped puts the fatha on
// the alef.
func TestLamAlefMarkOnItsStroke(t *testing.T) {
	pdf := NewFpdf("P", "in", "Letter")
	for _, s := range []string{"", "B", "I", "BI"} {
		AddFont(pdf, "CourierPrime", s, font.GetFont("CourierPrime", s))
	}
	tree := &Tree{PDF: pdf, Rules: rules.Default, Fonts: newFonts(pdf, nil)}
	for _, style := range []string{"", "B", "I", "BI"} {
		f := tree.Rules.Get(lex.TypeAction)
		f.Style = style
		_, lines := tree.visualLines(f, "لَا لاَ", true, 6)
		var marks []placed
		for _, p := range lines[0] {
			for _, m := range p.marks {
				if m.mark == "َ" {
					if r := []rune(p.text); m.at >= len(r) || r[m.at] != 0xFEFB {
						t.Errorf("%s: the fatha on %+q, want the lam-alef", style, p.text)
					}
					marks = append(marks, m)
				}
			}
		}
		if len(marks) != 2 {
			t.Fatalf("%s: fathas placed: %+v", style, marks)
		}
		// in visual order: لاَ (the second word) first, at the left
		onAlef, onLam := marks[0], marks[1]
		if onLam.dx < 0.05 || onAlef.dx > -0.05 {
			t.Errorf("%s: the fatha on the lam at %.3f em, on the alef at %.3f em: want the lam's half (> 0.05) and the alef's (< -0.05)",
				style, onLam.dx, onAlef.dx)
		}
	}
}
