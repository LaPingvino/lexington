package lex

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Locked pages, as a shooting script has them: once it goes out, each
// page keeps its number and its first line. Text that grows past a locked
// page's end goes onto A-pages (12A, 12B …), named as A-scenes are
// (SceneNumberBetween, 10A style: 12A then 12B after it; 12AA between 12A
// and 12B; 12aA between 12 and 12A, the small a Final Draft also writes,
// "4aA" in a user's question on its knowledge base); text that shrinks
// leaves the page short; a page whose text is gone gives its number to
// the page before it, which reads "12-13" (Scripto's "Page 4 becomes
// 4–5"; Final Draft's "7BB-9A", seen as a search snippet of its
// knowledge base, whose pages refuse robots). Pages after the last locked
// one count on (121 after 120), as scene numbers do after the last: no
// source says either way.
//
// A locked page is anchored by words, not by an offset, so that an edit
// anywhere else (in any other program too) does not move it: the scene it
// starts in (by its number), its first words, and the last words of the
// page before it. A page is found again by its first words in its scene
// (where the words before it are before them, if they repeat); else right
// after the words before it (its first line was edited). A page found by
// neither is gone (both sides of its break rewritten): no nearer matching,
// as a script's few words in a row ("he looks at her") repeat too often to
// tell a page by.

// PageLock is a script's locked pages.
type PageLock struct {
	// Paper is the paper the pages were locked on ("letter", "a4", "a5";
	// "" Letter): a lock is a layout, so the PDF writer prints a locked
	// script on it.
	Paper string
	// Continueds is whether the scenes' CONTINUEDs were printed when the
	// pages were locked (they take lines: the writer prints them so).
	Continueds bool
	// Pages are the pages in order, the first the script's first.
	Pages []LockedPage
}

// LockedPage is one locked page.
type LockedPage struct {
	Label  string // its number as printed: "12", "12A"
	Scene  string // the number of the scene its first words are in ("": none)
	Nth    int    // which of the first words' places in that scene (0: the first)
	Before string // the last words before it, normalised (PageWords)
	Words  string // its first words, normalised
}

// anchorWords is how many words an anchor holds on each side.
const anchorWords = 8

// PagePos is a place in a screenplay: a row, and a word in it (counted as
// strings.Fields counts them, in PageText).
type PagePos struct{ Row, Word int }

// PageText is the text of a row that pages are anchored in: its contents
// without notes, a heading's without its number; "" for a row whose text
// is not the script's body (blank lines, notes, sections, synopses, the
// title page's, the boneyard).
func PageText(l Line) string {
	switch l.Type {
	case TypeEmpty, TypeNote, TypeSection, TypeSynopsis, TypeBoneyard, TypeTitlePage, TypeNewPage,
		TypeDualOpen, TypeDualNext, TypeDualClose, TypeMetaSection:
		return ""
	}
	if strings.HasPrefix(l.Type, "_") {
		return "" // a writer's own (a PDF's MORE)
	}
	text := StripNotes(l.Contents)
	if l.Type == TypeScene {
		text, _ = SceneNumber(text)
	}
	return text
}

// PageWord is a word as anchors compare it: lower case, without the
// punctuation and emphasis around it ("" for none left).
func PageWord(w string) string {
	w = strings.ToLower(Unstyled(w))
	return strings.TrimFunc(w, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// streamWord is a word of the body in order.
type streamWord struct {
	at    PagePos
	norm  string
	scene int // the scene it is in (an index of pageStream.scenes; -1 none)
}

type pageStream struct {
	words  []streamWord
	scenes []streamScene
}

type streamScene struct {
	number     string
	start, end int // its words
}

// bodyStart is the first row after the title page.
func bodyStart(s Screenplay) int {
	if len(s) == 0 || s[0].Type != TypeTitlePage {
		return 0
	}
	for i, l := range s {
		if l.Type == TypeNewPage {
			return i + 1
		}
	}
	return len(s)
}

func streamOf(s Screenplay) pageStream {
	var p pageStream
	scene := -1
	for i := bodyStart(s); i < len(s); i++ {
		l := s[i]
		if l.Type == TypeScene {
			if scene >= 0 {
				p.scenes[scene].end = len(p.words)
			}
			_, n := SceneNumber(l.Contents)
			p.scenes = append(p.scenes, streamScene{number: n, start: len(p.words)})
			scene = len(p.scenes) - 1
		}
		for k, f := range strings.Fields(PageText(l)) {
			if w := PageWord(f); w != "" {
				p.words = append(p.words, streamWord{at: PagePos{i, k}, norm: w, scene: scene})
			}
		}
	}
	if scene >= 0 {
		p.scenes[scene].end = len(p.words)
	}
	return p
}

// index is the first word at or after a place (len(words) if none).
func (p pageStream) index(at PagePos) int {
	for k, w := range p.words {
		if w.at.Row > at.Row || (w.at.Row == at.Row && w.at.Word >= at.Word) {
			return k
		}
	}
	return len(p.words)
}

func (p pageStream) norms(from, to int) []string {
	from, to = max(0, from), min(len(p.words), to)
	out := make([]string, 0, max(0, to-from))
	for k := from; k < to; k++ {
		out = append(out, p.words[k].norm)
	}
	return out
}

// at tells whether the words ws are at k.
func (p pageStream) at(k int, ws []string) bool {
	if len(ws) == 0 || k < 0 || k+len(ws) > len(p.words) {
		return false
	}
	for i, w := range ws {
		if p.words[k+i].norm != w {
			return false
		}
	}
	return true
}

// PageAnchor is the anchor of a page that starts at a place (its Label
// left for the caller).
func PageAnchor(s Screenplay, at PagePos) LockedPage {
	p := streamOf(s)
	return p.anchor(p.index(at))
}

func (p pageStream) anchor(k int) LockedPage {
	lp := LockedPage{
		Before: strings.Join(p.norms(k-anchorWords, k), " "),
		Words:  strings.Join(p.norms(k, k+anchorWords), " "),
	}
	if k < len(p.words) && p.words[k].scene >= 0 {
		sc := p.scenes[p.words[k].scene]
		lp.Scene = sc.number
		ws := strings.Fields(lp.Words)
		for j := sc.start; j < k; j++ {
			if lp.Scene != "" && p.at(j, ws) {
				lp.Nth++
			}
		}
	}
	return lp
}

// PageAnchors are the anchors of pages starting at the places given.
func PageAnchors(s Screenplay, starts []PagePos) []LockedPage {
	p := streamOf(s)
	out := make([]LockedPage, len(starts))
	for i, at := range starts {
		out[i] = p.anchor(p.index(at))
	}
	return out
}

// Missing is the place of a locked page that is not found.
var Missing = PagePos{-1, -1}

// LocatePages finds each locked page's first word in a screenplay: in
// the order of the pages, each after the one before; Missing for a page
// not found. The first page is the body's start.
func LocatePages(s Screenplay, l *PageLock) []PagePos {
	if l == nil {
		return nil
	}
	p := streamOf(s)
	out := make([]PagePos, len(l.Pages))
	prev := -1
	for i, lp := range l.Pages {
		k, sure := 0, 2
		if i > 0 {
			k, sure = p.locate(lp, prev)
		}
		if k >= 0 && sure < 2 && i+1 < len(l.Pages) {
			// found past the next page's own place: its text is gone
			if next, nextSure := p.locate(l.Pages[i+1], prev); next >= 0 && nextSure > 0 && next <= k {
				k = -1
			}
		}
		if k < 0 || k >= len(p.words) {
			out[i] = Missing
			continue
		}
		out[i], prev = p.words[k].at, k
	}
	return out
}

// region is where a page's words are looked for: its scene's words after
// prev (or everything after it, for no scene); ok false when its scene is
// not in the script.
func (p pageStream) region(lp LockedPage, prev int) (from, to int, ok bool) {
	from, to = prev+1, len(p.words)
	if lp.Scene == "" {
		return from, to, true
	}
	for _, sc := range p.scenes {
		if sc.number == lp.Scene && sc.end > prev+1 {
			return max(from, sc.start), sc.end, true
		}
	}
	return 0, 0, false
}

// locate is where a page starts, -1 if not found, with how sure: 2 its
// words and those before it, 1 its words, 0 the words before it only.
func (p pageStream) locate(lp LockedPage, prev int) (int, int) {
	from, to, ok := p.region(lp, prev)
	if !ok { // its scene is gone: its words wherever they are now
		lp.Scene, lp.Nth = "", 0
		k, sure := p.locate(lp, prev)
		return k, min(sure, 1)
	}
	words, before := strings.Fields(lp.Words), strings.Fields(lp.Before)
	// its first words: the Nth in its scene with the words before it
	// before them; else the only place with them before; else the Nth;
	// else the first after prev with them before, or without
	if len(words) > 0 {
		start := from
		if lp.Scene != "" {
			for _, sc := range p.scenes {
				if sc.number == lp.Scene && sc.end > prev+1 {
					start = sc.start
					break
				}
			}
		}
		nth, first, firstBefore, befores, n := -1, -1, -1, 0, 0
		for k := start; k < to; k++ {
			if !p.at(k, words) {
				continue
			}
			if k > prev {
				hasBefore := len(before) > 0 && p.at(k-len(before), before)
				if n == lp.Nth {
					if hasBefore {
						return k, 2
					}
					nth = k
				}
				if hasBefore {
					befores++
					if firstBefore < 0 {
						firstBefore = k
					}
				}
				if first < 0 {
					first = k
				}
			}
			n++
		}
		switch {
		case befores == 1:
			return firstBefore, 2
		case nth >= 0:
			return nth, 1
		case firstBefore >= 0:
			return firstBefore, 1
		case first >= 0:
			return first, 1
		}
	}
	// right after the words before it (its first line edited)
	if len(before) > 0 {
		lo := max(prev+1-len(before), 0)
		if lp.Scene != "" {
			lo = max(lo, from-len(before)) // they may end the scene before
		}
		for k := lo; k+len(before) < to; k++ { // (its start in its scene)
			if p.at(k, before) && k+len(before) > prev {
				return k + len(before), 0
			}
		}
	}
	return -1, 0
}

// pageLine is a locked page as a line of text: label, scene (- none) and
// which of its first words' places (*n, after the first), then the words
// before it and its first: "12A 14*2: so he left / she opens the door".
var pageLine = regexp.MustCompile(`^\s*(\S+)\s+(\S+?)(?:\*(\d+))?\s*:\s*(.*?)\s*/\s*(.*?)\s*$`)

// String is the page as a line (ParseLockedPage reads it).
func (lp LockedPage) String() string {
	scene := lp.Scene
	if scene == "" {
		scene = "-"
	}
	if lp.Nth > 0 {
		scene += "*" + strconv.Itoa(lp.Nth)
	}
	return strings.TrimSpace(lp.Label + " " + scene + ": " + lp.Before + " / " + lp.Words)
}

// ParseLockedPage reads a page as String writes it.
func ParseLockedPage(s string) (LockedPage, bool) {
	m := pageLine.FindStringSubmatch(s)
	if m == nil {
		return LockedPage{}, false
	}
	lp := LockedPage{Label: m[1], Scene: m[2], Before: m[4], Words: m[5]}
	if lp.Scene == "-" {
		lp.Scene = ""
	}
	if m[3] != "" {
		lp.Nth, _ = strconv.Atoi(m[3])
	}
	return lp, true
}

// PageLabelBetween is the label of a new page between the pages labelled
// prev and next ("" none after it), not one of taken: as A-scenes are
// numbered (10A style).
func PageLabelBetween(prev, next string, taken map[string]bool) string {
	return SceneNumberBetween(prev, next, taken, NumberAfter)
}

// pagesKey is the key of a lock's line in its text form.
const pagesKey = "pages"

// Text is the lock as text: a line "pages: locked, letter, continueds"
// (the paper; continueds only when printed), then each page on a line of
// its own, indented (LockedPage.String). ParsePageLock reads it, also from
// among other lines (Accolade keeps it in its production block).
func (l PageLock) Text() string {
	paper := l.Paper
	if paper == "" {
		paper = "letter"
	}
	head := pagesKey + ": locked, " + strings.ToLower(paper)
	if l.Continueds {
		head += ", continueds"
	}
	lines := []string{head}
	for _, p := range l.Pages {
		lines = append(lines, "  "+p.String())
	}
	return strings.Join(lines, "\n")
}

// ParsePageLock reads a lock as Text writes it from a text: its "pages:"
// line (any case; "unlocked" or no "locked" in it: none) and the indented
// lines right after it. nil when there is none.
func ParsePageLock(text string) *PageLock {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), pagesKey) || strings.HasPrefix(line, " ") {
			continue
		}
		v := strings.ToLower(value)
		if !strings.Contains(v, "locked") || strings.Contains(v, "unlocked") {
			return nil
		}
		l := &PageLock{Paper: "letter", Continueds: strings.Contains(v, "continued")}
		for _, p := range []string{"a4", "a5", "letter"} {
			if strings.Contains(v, p) {
				l.Paper = p
				break
			}
		}
		for _, pl := range lines[i+1:] {
			if pl == "" || (pl[0] != ' ' && pl[0] != '\t') {
				break
			}
			if lp, ok := ParseLockedPage(pl); ok {
				l.Pages = append(l.Pages, lp)
			}
		}
		return l
	}
	return nil
}

// PageLockLines tells which lines of a text ParsePageLock reads (from, to:
// the "pages:" line and the indented ones after it); ok false for none.
func PageLockLines(text string) (from, to int, ok bool) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		key, _, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(key), pagesKey) || strings.HasPrefix(line, " ") {
			continue
		}
		j := i + 1
		for j < len(lines) && lines[j] != "" && (lines[j][0] == ' ' || lines[j][0] == '\t') {
			j++
		}
		return i, j, true
	}
	return 0, 0, false
}
