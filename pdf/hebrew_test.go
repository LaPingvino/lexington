package pdf

import (
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/LaPingvino/lexington/font"
)

func courierHas() func(rune) bool {
	return glyphTest(font.GetFont(font.CourierPrimeName, ""))
}

func TestComposeHebrewPairs(t *testing.T) {
	has := courierHas()
	n := 0
	for r := rune(0xFB1D); r <= 0xFB4F; r++ {
		d := norm.NFD.String(string(r))
		if d == string(r) {
			continue
		}
		if !has(r) {
			t.Errorf("%U: not in Courier Badi", r)
			continue
		}
		n++
		if got := composeHebrew(d, has); got != string(r) {
			t.Errorf("%U: %q composes to %q", r, d, got)
		}
	}
	if n != 34 { // the canonical forms: FB1D, FB1F, FB2A–FB4E less the unassigned
		t.Errorf("%d presentation forms composed, want 34", n)
	}
}

func TestComposeHebrew(t *testing.T) {
	has := courierHas()
	for _, c := range []struct{ in, want string }{
		{"שׁ", "שׁ"},
		{"שׂ", "שׂ"},
		{"שּׁ", "שּׁ"}, // dagesh, shin dot
		{"שּׁ", "שּׁ"}, // the other order
		{"שָּׂ", "שָּׂ"},
		{"וּ", "וּ"},
		{"וֹ", "וֹ"},
		{"שָׁלוֹם", "שָׁלוֹם"}, // qamats left over after the form
		{"שָׁ", "שָׁ"},        // shin dot after the vowel
		{"בָּ", "בָּ"},
		{"אַ", "אַ"},
		{"תּוֹרָה", "תּוֹרָה"},
		{"חֹ", "חֹ"},       // no form: as written
		{"ש", "ש"},         // no points
		{"abc é", "abc é"}, // not Hebrew
		{"ּ", "ּ"},         // a mark alone
	} {
		if got := composeHebrew(c.in, has); got != c.want {
			t.Errorf("composeHebrew(%q) = %+q, want %+q", c.in, got, c.want)
		}
	}
}

func TestComposeHebrewOnlyWithGlyph(t *testing.T) {
	none := func(rune) bool { return false }
	if got := composeHebrew("שׁוּ", none); got != "שׁוּ" {
		t.Errorf("composed without the glyphs: %+q", got)
	}
	onlyShin := func(r rune) bool { return r == 0xFB2A }
	// shin + dagesh + shin dot: FB2C missing, FB2A taken, dagesh follows
	if got := composeHebrew("שּׁ", onlyShin); got != "שּׁ" {
		t.Errorf("fallback to a smaller form: %+q", got)
	}
}
