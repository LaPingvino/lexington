package pdfin

import (
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Prose: a PDF read as running text (a story, an article, a book), not as
// a screenplay. Lines wrapped by the page are joined into paragraphs, a
// word broken over two lines is joined again, headings are told by their
// size (or a bold line of their own), lists by their bullets or numbers;
// page numbers and running headers and footers are left out.

// Block is a paragraph, a heading or a list item of prose.
type Block struct {
	// Heading is the heading's level, 1 the largest; 0 for a paragraph.
	Heading int
	// List is a list item's mark as printed ("•", "-", "3.", "b)"); ""
	// for others.
	List string
	Text string
}

// Prose is a text read from a PDF, in its blocks.
type Prose []Block

// ReadProseFile reads the PDF file at path as prose.
func ReadProseFile(path string, opts Options) (Prose, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return ReadProse(f, st.Size(), opts)
}

// ReadProse reads a PDF as prose, OCRing scanned pages if opts has an OCR
// (opts.Scenes is not used).
func ReadProse(r io.ReaderAt, size int64, opts Options) (Prose, error) {
	pages, _, err := readPages(r, size, opts)
	if err != nil {
		return nil, err
	}
	return proseOf(pages), nil
}

var (
	// a page number as printed: 12, - 12 -, [12], Page 12, 12 of 30, xii
	prosePageNumber = regexp.MustCompile(`(?i)^(page\s+)?[-–—\[(]?\s*(\d{1,4}|[ivxlcdm]{1,7})\s*[-–—\])]?(\s*(of|/)\s*\d{1,4})?$`)
	bullet          = regexp.MustCompile(`^([•◦▪▫‣⁃●○■□–\-*·]|\d{1,3}[.)]|[a-zA-Z][.)])\s+`)
)

// proseOf puts the pages' lines together into prose.
func proseOf(pages [][]line) Prose {
	body := proseBody(pages)
	if len(body) == 0 {
		return nil
	}
	margin := actionMargin(body)
	right := textRight(body)
	step := lineStep(body)
	text := bodySize(body)
	levels := headingSizes(body, text)
	width := right - margin
	level := func(l line) int {
		s := lineSize(l)
		if text > 0 && s >= text*1.15 {
			for i, h := range levels {
				if s >= h*0.9 {
					return i + 1
				}
			}
		}
		// a bold line of its own, short, not a sentence: a heading under
		// the sized ones
		t := l.text()
		if allBold(l) && l.end()-l.x() < 0.75*width && utf8.RuneCountInString(t) < 80 &&
			!strings.ContainsAny(t[len(t)-1:], ".,;") && (text == 0 || s <= 0 || math.Abs(s-text) < 0.6 || s > text) {
			return min(len(levels)+1, 6)
		}
		return 0
	}

	var out Prose
	var cur *Block
	var prev *line
	prevLevel := 0
	for i := range body {
		l := body[i]
		t := l.text()
		lv := level(l)
		mark := ""
		if m := bullet.FindStringSubmatch(t); m != nil && lv == 0 && !(len(m[1]) <= 2 && unicode.IsLetter(rune(m[1][0])) && prev != nil && !endsParagraph(*prev, right, width)) {
			mark = m[1]
			t = strings.TrimSpace(t[len(m[0]):])
		}
		newBlock := cur == nil || lv != prevLevel || mark != ""
		if !newBlock && prev != nil {
			samePage := l.Page == prev.Page
			gap := l.Y - prev.Y
			switch {
			case samePage && gap > 1.25*step: // space between paragraphs
				newBlock = true
			case lv > 0: // a heading on more than one line goes on
			case cur.List != "" && l.x() > margin+0.1: // a list item's next line, indented
			case l.x() > prev.x()+0.15 && prev.x() < margin+0.1: // an indented first line
				newBlock = true
			case l.x() < prev.x()-0.15 && cur.List != "": // back at the margin after a list
				newBlock = true
			case endsParagraph(*prev, right, width):
				newBlock = true
			}
		}
		if t == "" {
			continue
		}
		if newBlock {
			out = append(out, Block{Heading: lv, List: mark, Text: t})
			cur = &out[len(out)-1]
		} else {
			cur.Text = joinLine(cur.Text, t)
		}
		prev, prevLevel = &body[i], lv
	}
	return out
}

// endsParagraph: a line that stops well before the right edge, as a
// paragraph's last line does.
func endsParagraph(l line, right, width float64) bool {
	return l.end() < right-0.12*width
}

// joinLine adds a wrapped line to the text before it: a word broken with
// a hyphen at the end of the line is joined again.
func joinLine(text, next string) string {
	if next == "" {
		return text
	}
	r, _ := utf8.DecodeRuneInString(next)
	if strings.HasSuffix(text, "­") { // a soft hyphen: always a broken word
		return strings.TrimSuffix(text, "­") + next
	}
	if n := len(text); n >= 2 && text[n-1] == '-' && unicode.IsLower(r) {
		before, _ := utf8.DecodeLastRuneInString(text[:n-1])
		if unicode.IsLetter(before) {
			return text[:n-1] + next
		}
	}
	return text + " " + next
}

// proseBody is the pages' lines without page numbers and running
// headers and footers: a line at the top or bottom of a page that comes
// back (its numbers aside) on a third of the pages or more.
func proseBody(pages [][]line) []line {
	edge := func(p []line, i int) bool { return i < 2 || i >= len(p)-2 }
	counts := map[string]int{}
	for _, p := range pages {
		seen := map[string]bool{}
		for i, l := range p {
			if k := proseRunningKey(l); edge(p, i) && k != "" && !seen[k] {
				seen[k] = true
				counts[k]++
			}
		}
	}
	running := func(l line) bool {
		n := counts[proseRunningKey(l)]
		return len(pages) >= 2 && n >= 2 && n*3 >= len(pages)
	}
	var all []line
	for _, p := range pages {
		all = append(all, p...)
	}
	text := bodySize(all)
	// a number in large or bold type is a chapter's (3, IV), not the page's
	heading := func(l line) bool { return allBold(l) || (text > 0 && lineSize(l) >= text*1.15) }
	var out []line
	for _, p := range pages {
		for i, l := range p {
			t := strings.TrimSpace(l.text())
			if edge(p, i) && ((prosePageNumber.MatchString(t) && !heading(l)) || running(l)) {
				continue
			}
			out = append(out, l)
		}
	}
	return out
}

// proseRunningKey is a line's text without its numbers, upper case, and
// about where it is on the page (a title on the first page is not the
// running header with its words above the others).
func proseRunningKey(l line) string {
	k := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || unicode.IsSpace(r) || r == '.' {
			return -1
		}
		return unicode.ToUpper(r)
	}, l.text())
	if k == "" {
		return ""
	}
	return fmt.Sprintf("%s@%d", k, int(math.Round(l.Y*4)))
}

// textRight is where most full lines end: the text's right edge.
func textRight(ls []line) float64 {
	ends := make([]float64, 0, len(ls))
	for _, l := range ls {
		ends = append(ends, l.end())
	}
	sort.Float64s(ends)
	return ends[len(ends)*9/10]
}

// lineSize is a line's font size: its largest run's.
func lineSize(l line) float64 {
	s := 0.0
	for _, r := range l.Runs {
		s = math.Max(s, r.Size)
	}
	return s
}

func allBold(l line) bool {
	for _, r := range l.Runs {
		if !r.Bold {
			return false
		}
	}
	return true
}

// bodySize is the size most of the text is in (by its letters), 0 if the
// PDF does not say.
func bodySize(ls []line) float64 {
	counts := map[float64]int{}
	for _, l := range ls {
		if s := lineSize(l); s > 0 {
			counts[math.Round(s*2)/2] += utf8.RuneCountInString(l.text())
		}
	}
	best, n := 0.0, 0
	for s, c := range counts {
		if c > n || (c == n && s < best) {
			best, n = s, c
		}
	}
	return best
}

// headingSizes are the sizes larger than the text's, the largest first:
// the headings' levels. Sizes within a tenth of each other are one level
// (OCR reads a size a little differently on every line).
func headingSizes(ls []line, text float64) []float64 {
	if text == 0 {
		return nil
	}
	var sizes []float64
	for _, l := range ls {
		if s := lineSize(l); s >= text*1.15 && utf8.RuneCountInString(l.text()) < 200 {
			sizes = append(sizes, s)
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(sizes)))
	var levels []float64
	for _, s := range sizes {
		if n := len(levels); n == 0 || s < levels[n-1]*0.9 {
			levels = append(levels, s)
		}
	}
	return levels
}

// Markdown is the prose as Markdown: headings with #, list items with
// their marks (bullets as -), a blank line between blocks.
func (p Prose) Markdown() string {
	var b strings.Builder
	for i, bl := range p {
		if i > 0 {
			b.WriteString("\n")
			if !(bl.List != "" && p[i-1].List != "") {
				b.WriteString("\n")
			}
		}
		text := escapeMarkdown(bl.Text)
		switch {
		case bl.Heading > 0:
			b.WriteString(strings.Repeat("#", min(bl.Heading, 6)) + " " + text)
		case bl.List != "":
			mark := bl.List
			if r, _ := utf8.DecodeRuneInString(mark); !unicode.IsDigit(r) {
				mark = "-"
			} else {
				mark = strings.TrimRight(mark, ".)") + "."
			}
			b.WriteString(mark + " " + text)
		default:
			b.WriteString(escapeBlockStart(text))
		}
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

// blockStart is what would make a paragraph something else in Markdown
// at its start: a heading's #, a quote's >, a list's mark.
var blockStart = regexp.MustCompile(`^(#|>|[-+*] |\d+[.)] )`)

// escapeBlockStart escapes such a start: \# \> \- and 1\. for a number.
func escapeBlockStart(s string) string {
	m := blockStart.FindString(s)
	switch {
	case m == "":
		return s
	case m[0] >= '0' && m[0] <= '9':
		n := len(m) - 2
		return s[:n] + `\` + s[n:]
	}
	return `\` + s
}

// escapeMarkdown escapes the characters that are emphasis or code in
// Markdown.
func escapeMarkdown(s string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`").Replace(s)
}
