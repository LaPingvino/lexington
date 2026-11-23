package pdf

import (
	"slices"
	"unicode"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/unicode/norm"
)

// Hebrew presentation forms. A point left over is placed by the font's
// GPOS (bidiPrint, fontShaper.markOffsets), but the points inside a
// letter (a dagesh centred in the cell hides in vav's stem, a shuruk,
// zayin's, yod's and final nun's) are best printed as one glyph: the
// presentation forms U+FB1D–FB4F (letter + dagesh or mapiq, shin + shin or
// sin dot, vav + holam, alef + patah…) have the points placed for their
// letter, so a letter with such points prints as its form when the font
// has one. They are composition exclusions, so NFC never makes them; the
// table is their canonical decompositions. The points of a letter are
// matched in any order (real text has dagesh and shin dot either way
// round); the form taking the most of them wins, and the points it does
// not take follow it in their order.

// hebrewForm is one presentation form and the points it stands for.
type hebrewForm struct {
	form  rune
	marks []rune // sorted
}

// hebrewForms are the forms by their letter, those with more points
// first.
var hebrewForms = func() map[rune][]hebrewForm {
	out := map[rune][]hebrewForm{}
	for r := rune(0xFB1D); r <= 0xFB4F; r++ {
		d := []rune(norm.NFD.String(string(r)))
		if len(d) < 2 || d[0] == r { // compatibility forms (wide letters, ligatures) stay
			continue
		}
		marks := slices.Clone(d[1:])
		slices.Sort(marks)
		out[d[0]] = append(out[d[0]], hebrewForm{r, marks})
	}
	for _, fs := range out {
		slices.SortStableFunc(fs, func(a, b hebrewForm) int { return len(b.marks) - len(a.marks) })
	}
	return out
}()

// composeHebrew prints each Hebrew letter with points as its
// presentation form where has(form) says the font has that glyph.
func composeHebrew(text string, has func(rune) bool) string {
	rs := []rune(text)
	if !slices.ContainsFunc(rs, func(r rune) bool { return hebrewForms[r] != nil }) {
		return text
	}
	out := make([]rune, 0, len(rs))
	for i := 0; i < len(rs); {
		r := rs[i]
		j := i + 1
		for j < len(rs) && unicode.Is(unicode.Mn, rs[j]) {
			j++
		}
		marks := rs[i+1 : j]
		forms := hebrewForms[r]
		if len(forms) == 0 || len(marks) == 0 {
			out = append(out, rs[i:j]...)
			i = j
			continue
		}
		var best *hebrewForm
		for k := range forms {
			if containsAll(marks, forms[k].marks) && has(forms[k].form) {
				best = &forms[k]
				break
			}
		}
		if best == nil {
			out = append(out, rs[i:j]...)
			i = j
			continue
		}
		out = append(out, best.form)
		taken := slices.Clone(best.marks)
		for _, m := range marks {
			if k := slices.Index(taken, m); k >= 0 {
				taken = slices.Delete(taken, k, k+1)
				continue
			}
			out = append(out, m)
		}
		i = j
	}
	return string(out)
}

// containsAll tells whether marks has every one of want (each as often).
func containsAll(marks, want []rune) bool {
	left := slices.Clone(marks)
	for _, w := range want {
		k := slices.Index(left, w)
		if k < 0 {
			return false
		}
		left = slices.Delete(left, k, k+1)
	}
	return true
}

// glyphTest is a font's "has a glyph for" test, from its file; nil when
// the font cannot be read (then nothing is composed).
func glyphTest(data []byte) func(rune) bool {
	f, err := sfnt.Parse(data)
	if err != nil {
		return nil
	}
	var buf sfnt.Buffer
	return func(r rune) bool {
		g, err := f.GlyphIndex(&buf, r)
		return err == nil && g != 0
	}
}
