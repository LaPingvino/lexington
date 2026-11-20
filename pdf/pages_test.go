package pdf_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fdx"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
)

// lockedSource is a script of some fifteen pages: scenes of long action
// (broken within), speeches long enough for (MORE), dual dialogue and
// emphasis. Each paragraph starts with its own token (S3P2: scene 3's
// second).
func lockedSource(scenes int) string {
	var b strings.Builder
	b.WriteString("Title: Locked\nAuthor: Someone\n\n")
	for k := 1; k <= scenes; k++ {
		fmt.Fprintf(&b, "INT. ROOM %d - DAY #%d#\n\n", k, k)
		for j := 1; j <= 3; j++ {
			id := fmt.Sprintf("S%dP%d", k, j)
			fmt.Fprintf(&b, "%s Anna walks along the wall of room %d and counts %sa the windows, one by one, "+
				"as if each of %sb them could tell her *something* new about the _house_ %sc and about the "+
				"people who lived here %sd before her, long ago, when the garden %se was still green.\n\n",
				id, k, id, id, id, id, id)
		}
		fmt.Fprintf(&b, "ANNA\nS%[1]dD1 This is room %[1]d. I remember the light %[1]da. I remember the smell of the "+
			"floor %[1]db. We used to run here. Mother would call us %[1]dc from the kitchen. Nobody came. "+
			"Then the rain %[1]dd started and we stayed.\n\n", k)
		if k%4 == 0 {
			fmt.Fprintf(&b, "BEN\nS%dD2 Shall we go?\n\nANNA ^\nS%dD3 Not yet.\n\n", k, k)
		}
	}
	return b.String()
}

func parseLocked(src string) lex.Screenplay {
	return fountain.ParseLines(fountain.Scene, strings.NewReader(src))
}

func labelsOf(pages []pdf.Page) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.Label
	}
	return out
}

func lockLabels(l lex.PageLock) []string {
	out := make([]string, len(l.Pages))
	for i, p := range l.Pages {
		out[i] = p.Label
	}
	return out
}

// startOf is the text in src that page i of the lock starts in (at or
// within it): its paragraph, a heading, or a speech's cue with its line.
func startOf(t *testing.T, src string, lock lex.PageLock, i int) string {
	t.Helper()
	rows := lex.Printable(parseLocked(src), nil)
	at := lex.LocatePages(rows, &lock)[i]
	if at == lex.Missing {
		t.Fatalf("page %d not found", i)
	}
	r := rows[at.Row]
	switch r.Type {
	case lex.TypeScene:
		h, _ := lex.SceneNumber(r.Contents)
		return h
	case lex.TypeSpeaker:
		return r.Contents + "\n" + rows[at.Row+1].Contents[:4]
	}
	tok := strings.Fields(r.Contents)[0]
	if !strings.HasPrefix(r.Contents, "S") || !tokenForm.MatchString(strings.ToLower(tok)) {
		t.Fatalf("page %d starts in %q", i, r.Contents)
	}
	if r.Type == lex.TypeDialog { // its cue with it
		for k := at.Row - 1; k >= 0; k-- {
			if rows[k].Type == lex.TypeSpeaker {
				return rows[k].Contents + "\n" + tok
			}
		}
	}
	return tok
}

// TestLockKeepsPages: locked, the same script prints the same pages
// (no A-pages), and locking it again gives the same lock: in every
// pagination, on A4, with CONTINUEDs.
func TestLockKeepsPages(t *testing.T) {
	src := lockedSource(30)
	s := parseLocked(src)
	for _, c := range []struct {
		name       string
		pagination rules.Pagination
		paper      string
		continueds bool
	}{
		{"all", rules.AllPagination, "", false},
		{"all-continueds", rules.AllPagination, "", true},
		{"none", rules.Pagination{}, "", false},
		{"keep", rules.Pagination{Keep: true}, "", true},
		{"keep+widows", rules.Pagination{Keep: true, Widows: true}, "a4", false},
		{"a4-continueds", rules.AllPagination, "a4", true},
		{"a5", rules.AllPagination, "a5", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := &pdf.PDFWriter{Elements: rules.Default.WithPagination(c.pagination), Page: c.paper, Continueds: c.continueds}
			unlocked := labelsOf(w.Pages(s))
			lock := w.LockPages(s)
			if len(lock.Pages) < 10 {
				t.Fatalf("only %d pages", len(lock.Pages))
			}
			if got := lockLabels(lock); !slices.Equal(got, unlocked) {
				t.Fatalf("lock's pages %v, printed %v", got, unlocked)
			}
			w.Lock = &lock
			pages := w.Pages(s)
			if got := labelsOf(pages); !slices.Equal(got, unlocked) {
				t.Fatalf("locked: %v, want %v", got, unlocked)
			}
			again := w.LockPages(s)
			if !slices.Equal(again.Pages, lock.Pages) {
				t.Fatalf("locking again:\n%v\nfirst:\n%v", again.Pages, lock.Pages)
			}
		})
	}
}

// lockOf locks a script's pages as the default writer prints them.
func lockOf(t *testing.T, s lex.Screenplay, continueds bool) lex.PageLock {
	t.Helper()
	return (&pdf.PDFWriter{Continueds: continueds}).LockPages(s)
}

func lockedPages(s lex.Screenplay, lock lex.PageLock, revs *lex.Revisions) []pdf.Page {
	return (&pdf.PDFWriter{Lock: &lock, Revisions: revs}).Pages(s)
}

// insertBefore puts text in before the paragraph starting with token.
func insertBefore(t *testing.T, src, token, text string) string {
	t.Helper()
	at := strings.Index(src, token)
	if at < 0 {
		t.Fatalf("no %s", token)
	}
	return src[:at] + text + src[at:]
}

// extras are n new paragraphs, each with its own token (tag1, tag2 …).
func extras(tag string, n int) string {
	var b strings.Builder
	for k := 1; k <= n; k++ {
		id := fmt.Sprintf("%s%d", tag, k)
		fmt.Fprintf(&b, "%s Something new happens here, %sa and it goes on for a while: %sb the windows rattle, "+
			"%sc the door opens by itself %sd and a cold wind comes in %se from the garden, %sf where nothing "+
			"has grown %sg for years.\n\n", id, id, id, id, id, id, id, id)
	}
	return b.String()
}

const extra = "Something new happens here, and it goes on for a while: the windows rattle, the door opens " +
	"by itself and a cold wind comes in from the garden, where nothing has grown for years.\n\n"

func TestLockedPageGrows(t *testing.T) {
	src := lockedSource(14)
	lock := lockOf(t, parseLocked(src), false)
	want := lockLabels(lock)
	// the page starting near S5P2 ("6", say) grows: the paragraph before
	// that page's start
	i := 4
	grown := insertBefore(t, src, startOf(t, src, lock, 4), extras("XA", 2))
	got := labelsOf(lockedPages(parseLocked(grown), lock, nil))
	a := lock.Pages[i-1].Label + "A"
	exp := slices.Insert(slices.Clone(want), i, a)
	if !slices.Equal(got, exp) {
		t.Fatalf("grown: %v\nwant %v", got, exp)
	}
	// more: two A-pages
	grown = insertBefore(t, src, startOf(t, src, lock, 4), extras("XB", 12))
	got = labelsOf(lockedPages(parseLocked(grown), lock, nil))
	b := lock.Pages[i-1].Label + "B"
	exp = slices.Insert(slices.Clone(want), i, a, b)
	if !slices.Equal(got, exp) {
		t.Fatalf("grown more: %v\nwant %v", got, exp)
	}
	// on several pages: each its own
	j := 6
	grown = insertBefore(t, insertBefore(t, src, startOf(t, src, lock, 6), extras("XC", 2)), startOf(t, src, lock, 4), extras("XA", 2))
	got = labelsOf(lockedPages(parseLocked(grown), lock, nil))
	exp = slices.Insert(slices.Clone(want), j, lock.Pages[j-1].Label+"A")
	exp = slices.Insert(exp, i, a)
	if !slices.Equal(got, exp) {
		t.Fatalf("grown on two pages: %v\nwant %v", got, exp)
	}
	// at the very boundary: before the next page's first words
	k := 4
	grown = insertBefore(t, src, startOf(t, src, lock, 4), extras("XG", 1))
	pages := lockedPages(parseLocked(grown), lock, nil)
	if got := labelsOf(pages); got[k] != lock.Pages[k-1].Label+"A" || got[k+1] != lock.Pages[k].Label {
		t.Fatalf("at the boundary: %v", got)
	}
}

func TestLockedPageShrinks(t *testing.T) {
	src := lockedSource(14)
	lock := lockOf(t, parseLocked(src), false)
	want := lockLabels(lock)
	// a paragraph out of the middle of a page: the page is short, the
	// others as they were
	cut := strings.Replace(src, "S5P1 ", "S5P1 Short. ", 1)
	at := strings.Index(cut, "S5P1 Short. ")
	end := strings.Index(cut[at:], "\n\n")
	cut = cut[:at] + "S5P1 Short." + cut[at+end:]
	got := labelsOf(lockedPages(parseLocked(cut), lock, nil))
	if !slices.Equal(got, want) {
		t.Fatalf("shrunk: %v\nwant %v", got, want)
	}
	// a whole page's text gone: the page before it has both numbers
	rows := lex.Printable(parseLocked(src), nil)
	pos := lex.LocatePages(rows, &lock)
	p := 4
	a, b := pos[p], pos[p+1]
	var gone lex.Screenplay
	gone = append(gone, rows[:a.Row]...)
	if a.Word > 0 {
		r := rows[a.Row]
		r.Contents = strings.Join(strings.Fields(r.Contents)[:a.Word], " ")
		gone = append(gone, r)
	}
	if b.Word > 0 {
		r := rows[b.Row]
		r.Contents = strings.Join(strings.Fields(r.Contents)[b.Word:], " ")
		if rows[b.Row].Type == lex.TypeDialog { // its cue
			gone = append(gone, lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeSpeaker, Contents: "ANNA"})
		}
		gone = append(gone, r)
		gone = append(gone, rows[b.Row+1:]...)
	} else {
		gone = append(gone, rows[b.Row:]...)
	}
	got = labelsOf(lockedPages(gone, lock, nil))
	exp := slices.Clone(want)
	exp[p-1] += "-" + want[p]
	exp = slices.Delete(exp, p, p+1)
	if !slices.Equal(got, exp) {
		t.Fatalf("page %s's text gone: %v\nwant %v", want[p], got, exp)
	}
}

var tokenForm = regexp.MustCompile(`^s\d+[pd]\d+$`)

// TestLockedFirstLineEdited: a locked page whose first line changes is
// found by the words before it.
func TestLockedFirstLineEdited(t *testing.T) {
	src := lockedSource(14)
	lock := lockOf(t, parseLocked(src), false)
	want := lockLabels(lock)
	for p := 1; p < len(lock.Pages); p++ {
		lp := lock.Pages[p]
		head := strings.Index(src, "INT. ROOM "+lp.Scene+" - DAY")
		f := strings.Fields(lp.Words)
		if lp.Nth > 0 || head < 0 || len(f) < 3 || tokenForm.MatchString(f[0]) || f[0] == "int" {
			continue
		}
		// its first three words, written otherwise
		first := strings.Index(strings.ToLower(src[head:]), strings.Join(f[:3], " "))
		if first < 0 {
			continue
		}
		at := head + first
		edited := src[:at] + "Quite differently put" + src[at+len(strings.Join(f[:3], " ")):]
		got := labelsOf(lockedPages(parseLocked(edited), lock, nil))
		if !slices.Equal(got, want) {
			t.Fatalf("page %s's first words edited: %v\nwant %v", lp.Label, got, want)
		}
		return
	}
	t.Fatal("no page to edit")
}

// TestLockAgain: locked again with an A-page, the A-page keeps its number;
// what grows then between them is 12aA (Final Draft's small a), after the
// A-page 12B.
func TestLockAgain(t *testing.T) {
	src := lockedSource(14)
	lock := lockOf(t, parseLocked(src), false)
	i := 4
	base := lock.Pages[i-1].Label
	grown := insertBefore(t, src, startOf(t, src, lock, 4), extras("XA", 2))
	w := &pdf.PDFWriter{Lock: &lock}
	again := w.LockPages(parseLocked(grown))
	if got := lockLabels(again); !slices.Contains(got, base+"A") || len(got) != len(lock.Pages)+1 {
		t.Fatalf("locked again: %v", got)
	}
	// the page base grows again, before its A-page
	more := insertBefore(t, grown, "XA1 ", extras("XD", 5))
	got := labelsOf(lockedPages(parseLocked(more), again, nil))
	if k := slices.Index(got, base); k < 0 || got[k+1] != base+"aA" || got[k+2] != base+"A" {
		t.Fatalf("grown again: %v", got)
	}
	// the A-page grows: after it, B
	more = insertBefore(t, grown, startOf(t, src, lock, 4), extras("XE", 14))
	got = labelsOf(lockedPages(parseLocked(more), again, nil))
	if k := slices.Index(got, base+"A"); k < 0 || got[k+1] != base+"B" {
		t.Fatalf("A-page grown: %v", got)
	}
}

// TestLockedRevisedPages: the pages a revision changed, as locked: the
// page with the change and its A-page, numbered so, and nothing else.
func TestLockedRevisedPages(t *testing.T) {
	src := lockedSource(14)
	lock := lockOf(t, parseLocked(src), false)
	i := 4
	base := lock.Pages[i-1].Label
	grown := insertBefore(t, src, startOf(t, src, lock, 4), "NEWLINE "+extras("XA", 2))
	s := parseLocked(grown)
	for k := range s {
		if strings.HasPrefix(s[k].Contents, "NEWLINE") {
			s[k].Revision = "Blue"
		}
	}
	pages := lockedPages(s, lock, testSets)
	var revised []string
	for _, p := range pages {
		if p.Revision != "" {
			revised = append(revised, p.Label+":"+p.Revision)
		}
	}
	if want := []string{base + ":Blue", base + "A:Blue"}; !slices.Equal(revised, want) {
		t.Fatalf("revised pages %v, want %v", revised, want)
	}
	// revised pages only: those two, numbered and with their header
	file := filepath.Join(t.TempDir(), "revised.pdf")
	r := *testSets
	r.PagesOnly = true
	if err := (&pdf.PDFWriter{OutputFile: file, Lock: &lock, Revisions: &r}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	printed, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(printed) != 2 {
		t.Fatalf("%d pages printed", len(printed))
	}
	for k, want := range []string{base + ".", base + "A."} {
		if top := printed[k][0].Text; !strings.HasSuffix(top, want) || !strings.Contains(top, "Blue Revision") {
			t.Errorf("page %d's top %q, want the header and %s", k, top, want)
		}
	}
	// a deletion at the foot of a page marks the next page's first line:
	// the short page is revised too
	j := 6
	s = parseLocked(src)
	pos := lex.LocatePages(lex.Printable(s, nil), &lock)
	first := lex.Printable(s, nil)[pos[j].Row]
	for k := range s {
		if s[k].Contents == first.Contents && pos[j].Word == 0 {
			s[k].Revision = "Blue"
		}
	}
	revised = nil
	for _, p := range lockedPages(s, lock, testSets) {
		if p.Revision != "" {
			revised = append(revised, p.Label)
		}
	}
	if pos[j].Word == 0 && !slices.Equal(revised, []string{lock.Pages[j-1].Label, lock.Pages[j].Label}) {
		t.Fatalf("a deletion's pages %v", revised)
	}
}

// TestContinueds: a scene broken over pages has (CONTINUED) at the foot
// and CONTINUED: at the top, with its number; a page that starts a scene
// neither.
func TestContinueds(t *testing.T) {
	s := parseLocked(lockedSource(6))
	file := filepath.Join(t.TempDir(), "contd.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Continueds: true}).Write(nil, s); err != nil {
		t.Fatal(err)
	}
	pages, err := pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	tops, foots := 0, 0
	for n, page := range pages[1:] { // after the title page
		text := ""
		for _, l := range page {
			text += l.Text + "\n"
		}
		hasTop := strings.Contains(text, "CONTINUED:")
		hasFoot := strings.Contains(text, "(CONTINUED)")
		if hasTop {
			tops++
			first := page[0].Text
			if n > 0 && !strings.Contains(first, ".") { // the page number's line first
				first = page[1].Text
			}
			if !strings.Contains(text, "CONTINUED:") {
				t.Errorf("page %d: %q", n+1, first)
			}
		}
		if hasFoot {
			foots++
		}
		if n+1 < len(pages)-1 {
			next := pages[n+2]
			nextText := ""
			for _, l := range next {
				nextText += l.Text + "\n"
			}
			if hasFoot != strings.Contains(nextText, "CONTINUED:") {
				t.Errorf("page %d: foot %v, next page's top %v", n+1, hasFoot, !hasFoot)
			}
		}
	}
	if tops == 0 || tops != foots {
		t.Fatalf("%d CONTINUED: and %d (CONTINUED)", tops, foots)
	}
	// "CONTINUED: (2)" where a scene goes on a third page
	long := parseLocked("INT. HALL - DAY #1#\n\n" + extras("XF", 40))
	if err := (&pdf.PDFWriter{OutputFile: file, Continueds: true}).Write(nil, long); err != nil {
		t.Fatal(err)
	}
	pages, err = pdfin.Pages(file)
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, page := range pages {
		for _, l := range page {
			all.WriteString(l.Text + "\n")
		}
	}
	if !strings.Contains(all.String(), "CONTINUED: (2)") {
		t.Fatalf("no CONTINUED: (2) in\n%s", all.String())
	}
}

func TestLockedPageLines(t *testing.T) {
	for _, lp := range []lex.LockedPage{
		{Label: "12A", Scene: "14", Nth: 2, Before: "so he left", Words: "she opens the door"},
		{Label: "1", Words: "int house day"},
	} {
		back, ok := lex.ParseLockedPage(lp.String())
		if !ok || back != lp {
			t.Errorf("%q read back as %+v", lp.String(), back)
		}
	}
}

// TestLockKeepsBigFish: a whole feature (Big Fish, through the FDX
// reader) prints the same pages locked, with and without CONTINUEDs.
func TestLockKeepsBigFish(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "testdata", "fdx", "big-fish", "Big-Fish.fdx"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s, err := fdx.ParseWithError(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, continueds := range []bool{false, true} {
		w := &pdf.PDFWriter{Continueds: continueds}
		lock := w.LockPages(s)
		if len(lock.Pages) < 100 {
			t.Fatalf("%d pages", len(lock.Pages))
		}
		w.Lock = &lock
		got := labelsOf(w.Pages(s))
		if want := lockLabels(lock); !slices.Equal(got, want) {
			for i := range min(len(got), len(want)) {
				if got[i] != want[i] {
					t.Fatalf("continueds %v: page %d is %s, want %s (%d pages, want %d)", continueds, i, got[i], want[i], len(got), len(want))
				}
			}
			t.Fatalf("continueds %v: %d pages, want %d", continueds, len(got), len(want))
		}
	}
}

// TestLockedShot writes a locked script with an A-page and CONTINUEDs to
// LOCKED_SHOT (a folder), to look at: page 12 grown in Blue onto 12A.
func TestLockedShot(t *testing.T) {
	dir := os.Getenv("LOCKED_SHOT")
	if dir == "" {
		t.Skip("LOCKED_SHOT not set")
	}
	src := lockedSource(26)
	lock := lockOf(t, parseLocked(src), true)
	i := slices.IndexFunc(lock.Pages, func(p lex.LockedPage) bool { return p.Label == "13" })
	grown := insertBefore(t, src, startOf(t, src, lock, i), "NEWLINE "+extras("XA", 3))
	s := parseLocked(grown)
	for k := range s {
		if strings.HasPrefix(s[k].Contents, "NEWLINE") || strings.HasPrefix(s[k].Contents, "XA") {
			s[k].Revision = "Blue"
		}
	}
	for _, only := range []bool{false, true} {
		r := *testSets
		r.Sets = r.Sets[:2]
		r.PagesOnly = only
		name := "locked.pdf"
		if only {
			name = "locked-revised.pdf"
		}
		if err := (&pdf.PDFWriter{OutputFile: filepath.Join(dir, name), Lock: &lock, Revisions: &r}).Write(nil, s); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(labelsOf(lockedPages(s, lock, testSets)))
}
