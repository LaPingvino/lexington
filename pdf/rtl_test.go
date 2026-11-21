package pdf_test

import (
	"math"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
)

const hebrewScript = `פנים. חנות ספרים - יום #1#

דני (40) עומד ליד הקופה.

He looks at the sign: "פתוח" and smiles.

@מִרְיָם
(בשקט)
שָׁלוֹם, דָּנִי!

...

> חתך ל:
`

// printRTL prints a script, its language right to left or not, and reads
// its pages back as printed (left to right, as seen).
func printRTL(t *testing.T, src string, rtl bool) [][]pdfin.PageLine {
	t.Helper()
	return printRTLWith(t, src, rtl, rules.Default)
}

func printRTLWith(t *testing.T, src string, rtl bool, set rules.Set) [][]pdfin.PageLine {
	t.Helper()
	s := fountain.Parse(rules.DefaultConf().Scenes["he"], strings.NewReader(src))
	file := filepath.Join(t.TempDir(), "rtl.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: set, RTL: rtl}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	return pages
}

// seen is a line of Hebrew as it is seen on the page: reversed.
func seen(s string) string {
	rs := []rune(s)
	slices.Reverse(rs)
	return string(rs)
}

// width is how wide a line of Courier at 12 pt prints, in inches: a
// tenth of an inch a character, the points none.
func width(s string) float64 {
	n := 0
	for _, r := range s {
		if !unicode.Is(unicode.Mn, r) {
			n++
		}
	}
	return float64(n) / 10
}

// A Hebrew script prints from the right: each line in visual order,
// ending at the right margin of its mirrored element (the action's 1.5",
// the speaker's 3.7"), the transition at the left margin; a Latin line
// in it stays on the left with its Hebrew word turned; a line of
// neutrals takes the language's direction.
func TestHebrewPrintsRightToLeft(t *testing.T) {
	pages := printRTL(t, hebrewScript, true)
	if len(pages) != 1 {
		t.Fatalf("%d pages", len(pages))
	}
	const near = 0.06 // the cell's margin, rounding
	page := pages[0]
	find := func(text string) pdfin.PageLine {
		for _, l := range page {
			if l.Text == text || strings.Contains(l.Text, text) {
				return l
			}
		}
		t.Fatalf("no line %q on the page: %v", text, page)
		return pdfin.PageLine{}
	}
	ends := func(l pdfin.PageLine, text string, at float64) {
		t.Helper()
		if end := l.X + width(text); math.Abs(end-at) > near {
			t.Errorf("%q ends at %.2f\", want %.2f\"", l.Text, end, at)
		}
	}
	starts := func(l pdfin.PageLine, at float64) {
		t.Helper()
		if math.Abs(l.X-at) > near {
			t.Errorf("%q starts at %.2f\", want %.2f\"", l.Text, l.X, at)
		}
	}
	heading := find(seen("פנים. חנות ספרים - יום"))
	if !strings.HasPrefix(heading.Text, "1 ") || !strings.HasSuffix(heading.Text, " 1") {
		t.Errorf("the heading's numbers: %q", heading.Text)
	}
	ends(find("ינד"), "ינד (40) דמוע דיל הפוקה.", 8.5-1.5)
	if l := find("ינד"); l.Text != ".הפוקה דיל דמוע (40) ינד" {
		t.Errorf("an action line in visual order: %q", l.Text)
	}
	starts(find("He looks"), 1.5)
	if l := find("He looks"); l.Text != `He looks at the sign: "חותפ" and smiles.` {
		t.Errorf("a Latin line with a Hebrew word: %q", l.Text)
	}
	ends(find(seen("מִרְיָם")), "מִרְיָם", 8.5-3.7)
	ends(find("(טקשב)"), "(בשקט)", 8.5-3.1)
	// pointed letters print as their presentation forms (shin + shin dot,
	// vav + holam, dalet + dagesh) and read back as those; the rest of a
	// letter's points stand before it, as seen
	ends(find("!\u05d9\u05b4\u05e0\u05b8\ufb33 ,\u05dd\ufb4b\u05dc\u05b8\ufb2a"), "שָׁלוֹם, דָּנִי!", 8.5-2.5)
	ends(find("..."), "...", 8.5-1.5)
	starts(find(seen("חתך ל:")), 1.0)

	ltr := printRTL(t, hebrewScript, false)[0]
	for _, l := range ltr {
		if l.Text == "..." && math.Abs(l.X-1.5) > near {
			t.Errorf("a line of neutrals in a left-to-right language starts at %.2f\"", l.X)
		}
	}
}

// A script with no right-to-left letters prints the same whether its
// language reads right to left or not, but for its lines of neutrals.
func TestLeftToRightUnchanged(t *testing.T) {
	src := "INT. HOUSE - DAY #1#\n\nAnna walks in, *slowly*, and looks around the room for a long, long while.\n\n" +
		"ANNA\n(quietly)\nHello? Is anyone here?\n\nBEN ^\nYes.\n\n> CUT TO:\n"
	a, b := printRTL(t, src, false), printRTL(t, src, true)
	if !slices.EqualFunc(a, b, slices.Equal) {
		t.Errorf("printed differently:\n%v\n%v", a, b)
	}
}

// A dual dialogue whose first speaker reads right to left is mirrored:
// the first speaker on the right.
func TestHebrewDualDialogue(t *testing.T) {
	pages := printRTL(t, "@מרים\nאני הולכת.\n\n@דני ^\nאני נשאר.\n", true)
	var names pdfin.PageLine
	for _, l := range pages[0] {
		if strings.Contains(l.Text, "םירמ") {
			names = l
		}
	}
	if names.Text != "ינד םירמ" {
		t.Fatalf("the names as seen: %q (%v)", names.Text, pages[0])
	}
	// Dani's column (the second) at 1"-3.5", Miriam's at 4"-6.5"; the
	// names 0.5" in from their column's right edge
	if math.Abs(names.X+width("ינד")-3.0) > 0.06 {
		t.Errorf("the second speaker ends at %.2f\", want 3\"", names.X+width("ינד"))
	}
}

// A right-to-left paragraph broken over a page goes on from the right
// on the next, its lines in visual order (one with Latin words in it
// too): with the pagination's break and with gofpdf's own.
func TestHebrewParagraphOverPages(t *testing.T) {
	filler := strings.Repeat("שורה של פעולה שממלאת את העמוד.\n\n", 25)
	para := strings.Repeat("דני עומד ליד הקופה ומסדר ספרים. ", 4) + "The Old Man and the Sea " +
		strings.Repeat("ועוד ספר אחד על המדף. ", 12)
	for _, pagination := range []string{"on", "off"} {
		p, err := rules.ParsePagination(pagination)
		if err != nil {
			t.Fatal(err)
		}
		pages := printRTLWith(t, filler+para+"\n", true, rules.Default.WithPagination(p))
		if len(pages) != 2 {
			t.Fatalf("pagination %s: %d pages", pagination, len(pages))
		}
		lines := 0
		for n, page := range pages {
			for _, l := range page {
				if strings.Contains(l.Text, seen("העמוד")) || l.Text == "2." {
					continue
				}
				lines++
				if end := l.X + width(l.Text); math.Abs(end-7.0) > 0.06 {
					t.Errorf("pagination %s, page %d: %q ends at %.2f\"", pagination, n+1, l.Text, end)
				}
			}
		}
		if lines != 8 || len(pages[1]) < 3 {
			t.Errorf("pagination %s: %d lines of the paragraph, %d on page 2", pagination, lines, len(pages[1]))
		}
	}
}

// A speech broken after a sentence keeps its direction on the next page
// though its rest starts with a Latin word.
func TestHebrewSpeechOverPages(t *testing.T) {
	filler := strings.Repeat("שורה של פעולה שממלאת את העמוד.\n\n", 22)
	speech := "@מרים\nאני הולכת. " + strings.Repeat("OK, אני הולכת הביתה עכשיו. ", 20) + "\n"
	pages := printRTL(t, filler+speech, true)
	if len(pages) != 2 {
		t.Fatalf("%d pages", len(pages))
	}
	more := false
	for n, page := range pages {
		for _, l := range page {
			switch {
			case l.Text == "(MORE)":
				more = true
			case strings.Contains(l.Text, "OK"):
				// dialogue, mirrored: to the right margin of 2.5"
				if end := l.X + width(l.Text); math.Abs(end-6.0) > 0.06 {
					t.Errorf("page %d: %q ends at %.2f\", want 6\"", n+1, l.Text, end)
				}
			}
		}
	}
	if !more {
		t.Errorf("the speech did not break with (MORE): %v", pages)
	}
}

// A direction mark sets a line's direction and prints as nothing.
func TestDirectionMarkPrintsNothing(t *testing.T) {
	page := printRTL(t, "\u200f12.\n\n\u200fשלום.\n", false)[0]
	var got []string
	for _, l := range page {
		got = append(got, l.Text)
		if end := l.X + width(l.Text); math.Abs(end-7.0) > 0.06 {
			t.Errorf("%q ends at %.2f\", want 7\"", l.Text, end)
		}
	}
	if !slices.Equal(got, []string{".12", ".םולש"}) {
		t.Errorf("printed %q", got)
	}
}

// Pointed Hebrew prints its letters with points as their presentation
// forms (the PDF applies no mark positioning): a shuruq as U+FB35, shin
// with dagesh and sin dot as U+FB2D whatever the points' order, in a line
// with Latin in it. pdftotext gives the forms; NFD gives back the text as
// written (a Hebrew line: pdftotext's own reordering of a mixed line
// moves its punctuation, forms or not).
func TestHebrewPresentationForms(t *testing.T) {
	const line = "Dani: וּבְשׁוּבוֹ, ש\u05C2\u05BC! (ok)"
	const hebrew = "שָׁלוֹם וּבְשׁוּבוֹ שׂ\u05BC תּוֹרָה"
	src := "INT. ROOM - DAY\n\n" + line + "\n\n" + hebrew + "\n"
	file := filepath.Join(t.TempDir(), "forms.pdf")
	s := fountain.Parse(rules.DefaultConf().Scenes["he"], strings.NewReader(src))
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, l := range pages[0] {
		if strings.Contains(l.Text, "Dani") {
			got = l.Text
		}
	}
	for _, form := range []rune{0xFB35, 0xFB2A, 0xFB4B, 0xFB2D} {
		if !strings.ContainsRune(got, form) {
			t.Errorf("no %U in %+q", form, got)
		}
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("no pdftotext")
	}
	out, err := exec.Command("pdftotext", file, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	clean := strings.NewReplacer("\u202a", "", "\u202b", "", "\u202c", "").Replace(string(out))
	if !strings.ContainsRune(clean, 0xFB35) {
		t.Errorf("pdftotext gives no presentation form: %+q", clean)
	}
	if !strings.Contains(norm.NFD.String(clean), norm.NFD.String(hebrew)) {
		t.Errorf("pdftotext, NFD, has no %+q:\n%+q", hebrew, clean)
	}
}
