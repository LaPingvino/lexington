package pdfin

import "testing"

// Hebrew as a text layer prints it, in the order it is seen, becomes the
// order it is written: numbers and Latin kept, brackets turned.
func TestAsWritten(t *testing.T) {
	for seen, want := range map[string]string{
		reversed("שלום עולם"):         "שלום עולם",
		"35" + reversed("בסדר, "):     "בסדר, 35",
		"(הקספה)":                     "(הפסקה)",
		"\u202B70" + reversed("גיל "): "גיל 70",
		"ABC " + reversed("שלום"):     "שלום ABC",
	} {
		if got := asWritten(seen); got != want {
			t.Errorf("asWritten(%q) = %q, want %q", seen, got, want)
		}
	}
	if got := tidy("הומה , שילוב ( הפסקה ) ג ' ורג"); got != "הומה, שילוב (הפסקה) ג'ורג" {
		t.Errorf("tidy: %q", got)
	}
}

// A right-to-left page's lines are laid out from the other edge, and a
// scene number before a heading's words makes it a heading.
func TestMirror(t *testing.T) {
	ls := []line{ln(1, 1, at(5.0, reversed("פנים. סדנה. יום.")), at(7.0, ".1"))}
	if !rtlPage(ls) {
		t.Fatal("not seen as right to left")
	}
	mirror(ls, 8.5, true)
	if r := ls[0].Runs; r[0].Text != "1." || r[1].Text != "פנים. סדנה. יום." || r[0].X > r[1].X {
		t.Errorf("mirrored: %+v", r)
	}
}
