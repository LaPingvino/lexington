package pdf

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/rules"
)

// visual is a line as shown, left to right, in the direction given.
func visual(s string, rtl bool) string {
	rs := []rune(s)
	lv := levels(rs, rtl)
	var b strings.Builder
	for _, i := range visualOrder(lv, 0, len(rs)) {
		r := rs[i]
		if lv[i]%2 == 1 {
			r = mirror(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func reversed(s string) string {
	rs := []rune(s)
	for i, j := 0, len(rs)-1; i < j; i, j = i+1, j-1 {
		rs[i], rs[j] = rs[j], rs[i]
	}
	return string(rs)
}

// Lines are shown in visual order: right-to-left runs reversed, numbers
// and Latin in them kept, brackets turned; points before their letters.
func TestVisualOrder(t *testing.T) {
	for _, c := range []struct {
		text string
		rtl  bool
		want string
	}{
		{"שלום עולם", true, reversed("שלום עולם")},
		{"דני (40) עומד", true, "דמוע (40) ינד"},
		{"המחיר 120 ש\"ח (כולל מע\"מ), לא 100.", true, ".100 אל ,(מ\"עמ ללוכ) ח\"ש 120 ריחמה"},
		{"שלום The Old Man שלום", true, "םולש The Old Man םולש"},
		{"He said שלום 123 עולם ok", false, "He said םלוע 123 םולש ok"},
		{"He said \"שלום\" and smiled.", false, "He said \"םולש\" and smiled."},
		{"12.", true, ".12"},
		{"...", true, "..."},
		{"21.11.2026", true, "21.11.2026"},
		{"<כן>", true, "<ןכ>"},
		{"שָׁלוֹם", true, reversed("שָׁלוֹם")}, // the points before their letters
		{"Hello, world.", false, "Hello, world."},
	} {
		if got := visual(c.text, c.rtl); got != c.want {
			t.Errorf("visual(%q, rtl %v) = %q, want %q", c.text, c.rtl, got, c.want)
		}
	}
}

// A line reads right to left by its own letters or mark, a line with
// neither by the language's direction; it needs reordering only then, or
// with right-to-left letters in it.
func TestLineDirection(t *testing.T) {
	he, en := Tree{rtl: true}, Tree{}
	for _, c := range []struct {
		text      string
		tree      Tree
		rtl, bidi bool
	}{
		{"שלום", en, true, true},
		{"Shalom שלום", he, false, true},
		{"Hello.", he, false, false},
		{"...", he, true, true},
		{"...", en, false, false},
		{"\u200f2.", en, true, true},
		{"*שלום* there", en, true, true},
	} {
		rtl := c.tree.lineRTL(c.text)
		if bidi := needsBidi(c.text, rtl); rtl != c.rtl || bidi != c.bidi {
			t.Errorf("%q (language rtl %v): rtl %v, bidi %v; want %v, %v", c.text, c.tree.rtl, rtl, bidi, c.rtl, c.bidi)
		}
	}
	f := mirrored(rules.Default.Get("trans"))
	if f.Left != 1 || f.Right != 1.5 || f.Align != "L" {
		t.Errorf("a transition mirrored: %+v", f)
	}
}
