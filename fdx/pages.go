package fdx

import (
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
)

// Final Draft's locked pages, as a real file has them (docs: none;
// EuroMechanical.fdx, written by Final Draft, in
// github.com/michaelforrest/ShootingChecklist):
//
//	<LockedPages>
//	  <LockedPage LevelIndex="0" LockLevel="0" PageNumber="1" Position="0"/>
//	  <LockedPage LevelIndex="1" LockLevel="0" PageNumber="2" Position="964"/>
//	</LockedPages>
//
// Position is where the page starts, counted as DeletedTextLocation's
// Offset is (the characters of the body's text runs, one more for each
// paragraph's end): in that file the four pages fall at the starts of
// paragraphs and within a speech after a sentence (where a (MORE) breaks
// it), as a page would. LockLevel and LevelIndex were 0 and the page's
// index in every one seen (one lock, no A-pages); what Final Draft writes
// for an A-page is not known: Lexington writes LockLevel as the number of
// letters after the page's number (12A: 1) and LevelIndex as the index,
// which is a guess. Reading takes PageNumber and Position only, and leaves
// out a position past the text's end or before the page before it (a file
// edited by another program keeps its old positions).

// FdxLockedPages is the <LockedPages> element.
type FdxLockedPages struct {
	Pages []FdxLockedPage `xml:"LockedPage"`
}

// FdxLockedPage is one locked page.
type FdxLockedPage struct {
	LevelIndex string `xml:"LevelIndex,attr"`
	LockLevel  string `xml:"LockLevel,attr"`
	PageNumber string `xml:"PageNumber,attr"`
	Position   string `xml:"Position,attr"`
}

// place is a paragraph of the body and where its text starts and ends,
// as Positions count.
type place struct {
	p          *FdxParagraph
	start, end int
}

// places are the body's paragraphs in order (a dual dialogue's own), with
// their places.
func places(c *FdxContent) []place {
	var out []place
	off := 0
	var walk func(ps []FdxParagraph)
	walk = func(ps []FdxParagraph) {
		for i := range ps {
			p := &ps[i]
			if p.DualDialogue != nil {
				walk(p.DualDialogue.Paragraphs)
				continue
			}
			start := off
			for _, t := range p.Texts {
				off += len([]rune(t.Content))
			}
			out = append(out, place{p, start, off})
			off++ // the paragraph's end
		}
	}
	walk(c.Paragraphs)
	return out
}

func paragraphText(p *FdxParagraph) string {
	var b strings.Builder
	for _, t := range p.Texts {
		b.WriteString(t.Content)
	}
	return b.String()
}

// runeOfField is the rune where the n-th field of s starts (its length
// for none).
func runeOfField(s string, n int) int {
	k, in, at := 0, false, 0
	for _, r := range s {
		space := unicode.IsSpace(r)
		if !space && !in {
			if k == n {
				return at
			}
			k++
		}
		in = !space
		at++
	}
	return at
}

// fieldOfRune is the field of s that rune at is in (or the next one).
func fieldOfRune(s string, at int) int {
	k, in, i := 0, false, 0
	for _, r := range s {
		space := unicode.IsSpace(r)
		if !space && !in {
			if i >= at {
				return k
			}
			k++
		}
		in = !space
		i++
	}
	return k
}

// letters is the number of letters after a page number's digits.
func letters(label string) int {
	n := 0
	for i := len(label) - 1; i >= 0 && unicode.IsLetter(rune(label[i])); i-- {
		n++
	}
	return n
}

// buildLockedPages are the <LockedPages> of a lock: each page found in
// the screenplay, at its paragraph (seq: each row's paragraph, by its
// number in places' order).
func buildLockedPages(doc *FdxFile, screenplay lex.Screenplay, lock *lex.PageLock, seq map[int]int) *FdxLockedPages {
	if lock == nil || len(lock.Pages) == 0 {
		return nil
	}
	ps := places(&doc.Content)
	out := &FdxLockedPages{}
	for i, at := range lex.LocatePages(screenplay, lock) {
		if at == lex.Missing {
			continue
		}
		n, ok := seq[at.Row]
		if !ok || n >= len(ps) {
			continue
		}
		pos := ps[n].start
		if text := paragraphText(ps[n].p); at.Word > 0 && screenplay[at.Row].Type != lex.TypeScene {
			pos += runeOfField(text, at.Word)
		}
		if i == 0 {
			pos = 0
		}
		label := lock.Pages[i].Label
		out.Pages = append(out.Pages, FdxLockedPage{LevelIndex: strconv.Itoa(len(out.Pages)),
			LockLevel: strconv.Itoa(letters(label)), PageNumber: label, Position: strconv.Itoa(pos)})
	}
	return out
}

// ParseLockedPages reads an FDX's locked pages (nil for none), anchored in
// the screenplay as ParseWithError reads it; a page's paper is not in the
// file (Letter is taken).
func ParseLockedPages(file io.Reader) (*lex.PageLock, error) {
	s, _, doc, rowOf, err := parseFile(file)
	if err != nil {
		return nil, err
	}
	if doc.LockedPages == nil || len(doc.LockedPages.Pages) == 0 {
		return nil, nil
	}
	ps := places(&doc.Content)
	var starts []lex.PagePos
	var labels []string
	last := -1
	for _, lp := range doc.LockedPages.Pages {
		at, err := strconv.Atoi(lp.Position)
		if err != nil || at <= last && len(starts) > 0 || strings.TrimSpace(lp.PageNumber) == "" {
			continue
		}
		for _, pl := range ps {
			if at < pl.start || at > pl.end {
				continue
			}
			row, ok := rowOf[pl.p]
			if !ok {
				break
			}
			word := 0
			if s[row].Type != lex.TypeScene && s[row].Type != lex.TypeDualOpen {
				word = fieldOfRune(paragraphText(pl.p), at-pl.start)
			}
			if len(starts) == 0 {
				row, word = 0, 0 // the first page is the body's start
			}
			starts = append(starts, lex.PagePos{Row: row, Word: word})
			labels = append(labels, strings.TrimSpace(lp.PageNumber))
			last = at
			break
		}
	}
	if len(starts) == 0 {
		return nil, nil
	}
	lock := &lex.PageLock{}
	for i, a := range lex.PageAnchors(s, starts) {
		a.Label = labels[i]
		lock.Pages = append(lock.Pages, a)
	}
	return lock, nil
}
