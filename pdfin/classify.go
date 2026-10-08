package pdfin

import (
	"math"
	"sort"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// The indents of the screenplay's elements, in inches from the action's
// margin, with some room on both sides: Final Draft, Highland, Fade In
// and Lexington put dialogue at 1", parentheticals at 1.5-1.6", names at
// 2-2.2".
const (
	dialogueFrom = 0.6
	parenFrom    = 1.3
	nameFrom     = 1.8
	nameTo       = 3.2
)

// classify turns the pages' lines into a screenplay.
func classify(pages [][]line, width float64) lex.Screenplay {
	var out lex.Screenplay
	start := 0
	if title, ok := titlePage(pages[0], width); ok {
		out = append(out, title...)
		start = 1
	}
	running := runningLines(pages[start:])
	var body []line
	for _, p := range pages[start:] {
		body = append(body, cleanPage(p, running)...)
	}
	if len(body) == 0 {
		return out
	}
	margin := actionMargin(body)
	step := lineStep(body)
	bottom := 0.0 // how low the text goes on a full page
	for _, l := range body {
		bottom = math.Max(bottom, l.Y)
	}

	var prevType lex.ElementType
	var prev *line
	resume := false // the speech goes on after NAME (CONT'D) on a new page
	for i := 0; i < len(body); i++ {
		l := body[i]
		// a speech continued on the next page: NAME (CONT'D) after (MORE)
		if prev != nil && l.Page != prev.Page && (prevType == lex.TypeDialog || prevType == lex.TypeParen) &&
			strings.Contains(strings.ToUpper(l.text()), "CONT") && l.x()-margin >= nameFrom && l.x()-margin < nameTo {
			if name := speakerBefore(out); name != "" && name == baseName(contd.ReplaceAllString(l.text(), "")) {
				prev, resume = &body[i], true
				continue
			}
		}
		// a blank line where the gap is more than a line (not across pages:
		// there the elements' own spacing is lost, so judge by type)
		// a page that ends well before the others: a forced page break
		if prev != nil && l.Page != prev.Page && prev.Y < bottom-2.5 && bottom > 7 {
			out = append(out, lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeNewPage})
			prevType, prev = lex.TypeEmpty, nil
		}
		if prev != nil {
			gap := l.Y - prev.Y
			if (l.Page == prev.Page && gap > 1.5*step) || (l.Page != prev.Page && blankAcrossPages(prevType, l, margin)) {
				out = append(out, lex.Line{Type: lex.TypeEmpty})
				prevType = lex.TypeEmpty
			}
		}
		// dual dialogue: two names side by side
		if j, dual := dualBlock(body, i, margin, width); dual != nil {
			out = append(out, dual...)
			prevType, prev, i = lex.TypeDialog, &body[j-1], j-1
			continue
		}
		var next *line
		if i+1 < len(body) && body[i+1].Page == l.Page && body[i+1].Y-l.Y <= 1.5*step {
			next = &body[i+1]
		}
		t, text := elementOf(l, next, margin, width, prevType)
		if resume && t == lex.TypeDialog && len(out) > 0 && out[len(out)-1].Type == lex.TypeDialog {
			out[len(out)-1].Contents += " " + text
			resume, prevType, prev = false, t, &body[i]
			continue
		}
		resume = false
		// a speech or paragraph wrapped over several lines is one element
		if (t == lex.TypeDialog || t == lex.TypeAction || t == lex.TypeParen) && t == prevType && prev != nil &&
			l.Y-prev.Y <= 1.5*step && l.Page == prev.Page && math.Abs(l.x()-prev.x()) < 0.35 && len(out) > 0 &&
			!(t == lex.TypeParen && strings.HasSuffix(out[len(out)-1].Contents, ")")) {
			out[len(out)-1].Contents += " " + text
			prev = &body[i]
			continue
		}
		out = append(out, lex.Line{Type: t, Contents: text})
		prevType, prev = t, &body[i]
	}
	return out
}

func pageOf(l *line) int {
	if l == nil {
		return 0
	}
	return l.Page
}

// blankAcrossPages: whether a blank line separates the last element of a
// page from the first of the next.
func blankAcrossPages(prev lex.ElementType, l line, margin float64) bool {
	d := l.x() - margin
	if prev == lex.TypeDialog || prev == lex.TypeParen || prev == lex.TypeSpeaker {
		// a speech goes on, unless the new page starts something else
		return d < dialogueFrom-0.1 || (d >= nameFrom && d < nameTo)
	}
	return true
}

// speakerBefore is the name of the last speaker in out.
func speakerBefore(out lex.Screenplay) string {
	for i := len(out) - 1; i >= 0; i-- {
		switch out[i].Type {
		case lex.TypeSpeaker:
			return baseName(out[i].Contents)
		case lex.TypeEmpty, lex.TypeScene, lex.TypeAction:
			return ""
		}
	}
	return ""
}

// baseName is a character's name without (V.O.), (CONT'D) and the like.
func baseName(s string) string {
	if i := strings.Index(s, "("); i > 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// elementOf is the type and the text of a line, by its indent.
func elementOf(l line, next *line, margin, width float64, prev lex.ElementType) (lex.ElementType, string) {
	text := l.text()
	d := l.x() - margin
	right := width - 1.0 // the right margin of the text
	inSpeech := prev == lex.TypeSpeaker || prev == lex.TypeParen || prev == lex.TypeDialog
	switch {
	case d < 0.3 && l.upper() && (scenePrefix.MatchString(text) || (l.bold() && len(text) < 60)):
		return lex.TypeScene, text
	case l.upper() && d > 2.5 && (transition.MatchString(text) || l.end() > right-0.4):
		return lex.TypeTrans, text
	case d >= nameFrom && d < nameTo && l.upper() && !inSpeechEnd(text) && next != nil &&
		next.x()-margin >= dialogueFrom && next.x()-margin < nameFrom:
		// a name, with its speech on the next line
		return lex.TypeSpeaker, contd.ReplaceAllString(text, "")
	case strings.HasPrefix(text, "(") && d >= dialogueFrom && d < nameFrom && (inSpeech || prev == ""):
		return lex.TypeParen, text
	case prev == lex.TypeParen && d >= parenFrom && d < nameFrom && inSpeech:
		return lex.TypeParen, text // a parenthetical's second line
	case d >= dialogueFrom && d < nameFrom && inSpeech:
		return lex.TypeDialog, text
	case d > 0.5 && centred(l, margin, right):
		return lex.TypeCenter, text
	}
	return lex.TypeAction, text
}

// inSpeechEnd: a line that looks like a name but ends a sentence ("NO!").
func inSpeechEnd(text string) bool {
	return strings.HasSuffix(text, "!") || strings.HasSuffix(text, "?") || strings.HasSuffix(text, ".")
}

func centred(l line, left, right float64) bool {
	return math.Abs((l.x()-left)-(right-l.end())) < 0.3
}

// actionMargin is the leftmost indent that many lines have: the action's.
func actionMargin(ls []line) float64 {
	counts := map[float64]int{}
	for _, l := range ls {
		counts[math.Round(l.x()*10)/10]++
	}
	var xs []float64
	for x := range counts {
		xs = append(xs, x)
	}
	sort.Float64s(xs)
	for _, x := range xs {
		if counts[x] >= max(2, len(ls)/20) {
			return x
		}
	}
	return xs[0]
}

// lineStep is the usual distance between lines (12 pt for Courier 12):
// the most common gap, or half of it if that is common too (then the
// most common gap is a blank line).
func lineStep(ls []line) float64 {
	counts := map[int]int{} // gaps in hundredths of an inch
	for i := 1; i < len(ls); i++ {
		if g := ls[i].Y - ls[i-1].Y; ls[i].Page == ls[i-1].Page && g > 0.08 && g < 0.6 {
			counts[int(math.Round(g*100))]++
		}
	}
	near := func(g int) int { return counts[g-1] + counts[g] + counts[g+1] }
	best, n := 0, 0
	for g := range counts {
		if c := near(g); c > n || (c == n && g < best) {
			best, n = g, c
		}
	}
	if best == 0 {
		return 1.0 / 6
	}
	if half := (best + 1) / 2; near(half) > 0 && near(half)*4 >= n && half >= 9 {
		best = half
	}
	return float64(best) / 100
}

// cleanPage leaves out what is not the script: page numbers, (MORE),
// CONTINUED; scene numbers in the margins go to the end of their heading.
func cleanPage(ls []line, running map[string]bool) []line {
	var out []line
	for i, l := range ls {
		t := l.text()
		if running[runningKey(l)] {
			continue
		}
		top := i < 2 && l.Y < 0.9
		bottom := i >= len(ls)-2 && l.Y > 10.0
		switch {
		case (top || bottom) && (pageNumber.MatchString(t) || continued.MatchString(strings.ToUpper(t))):
			continue
		case more.MatchString(t), continued.MatchString(strings.ToUpper(t)):
			continue
		}
		// a scene number left (and right) of a heading
		if len(l.Runs) >= 2 && sceneNumber.MatchString(l.Runs[0].Text) {
			num := strings.TrimSuffix(l.Runs[0].Text, ".")
			rest := l.Runs[1:]
			if n := len(rest); n >= 2 && strings.TrimSuffix(rest[n-1].Text, ".") == num {
				rest = rest[:n-1]
			}
			if t := joinRuns(rest); scenePrefix.MatchString(t) || t == strings.ToUpper(t) {
				l.Runs = append(append([]run(nil), rest...), run{X: rest[len(rest)-1].End, End: rest[len(rest)-1].End, Text: "#" + num + "#"})
			}
		}
		// or only right of it
		if n := len(l.Runs); n >= 2 && sceneNumber.MatchString(l.Runs[n-1].Text) && l.Runs[n-1].X > 6.0 &&
			!strings.HasSuffix(l.Runs[n-2].Text, "#") && scenePrefix.MatchString(joinRuns(l.Runs[:n-1])) {
			num := strings.TrimSuffix(l.Runs[n-1].Text, ".")
			l.Runs = append(append([]run(nil), l.Runs[:n-1]...), run{X: l.Runs[n-2].End, End: l.Runs[n-2].End, Text: "#" + num + "#"})
		}
		out = append(out, l)
	}
	return out
}

// dualBlock reads dual dialogue starting at line i: two upper-case names
// on one line, then lines with a run in each column. It returns where the
// block ends and its lines, or nil.
func dualBlock(ls []line, i int, margin, width float64) (int, lex.Screenplay) {
	l := ls[i]
	mid := width / 2
	name := func(t string) bool { return t == strings.ToUpper(t) && hasLetter(t) && !inSpeechEnd(t) }
	if len(l.Runs) != 2 || l.Runs[0].X > mid || l.Runs[1].X < mid || !name(l.Runs[0].Text) || !name(l.Runs[1].Text) {
		return 0, nil
	}
	var cols [2][]string
	step := lineStep(ls)
	j := i
	for ; j < len(ls); j++ {
		if j > i && (ls[j].Page != ls[j-1].Page || ls[j].Y-ls[j-1].Y > 1.5*step) {
			break
		}
		for _, r := range ls[j].Runs {
			c := 0
			if r.X >= mid {
				c = 1
			}
			cols[c] = append(cols[c], r.Text)
		}
	}
	out := lex.Screenplay{{Type: lex.TypeDualOpen}}
	for c, col := range cols {
		if c == 1 {
			out = append(out, lex.Line{Type: lex.TypeDualNext})
		}
		for k, t := range col {
			switch {
			case k == 0:
				out = append(out, lex.Line{Type: lex.TypeSpeaker, Contents: contd.ReplaceAllString(t, "")})
			case strings.HasPrefix(t, "("):
				out = append(out, lex.Line{Type: lex.TypeParen, Contents: t})
			case len(out) > 0 && out[len(out)-1].Type == lex.TypeDialog:
				out[len(out)-1].Contents += " " + t
			default:
				out = append(out, lex.Line{Type: lex.TypeDialog, Contents: t})
			}
		}
	}
	return j, append(out, lex.Line{Type: lex.TypeDualClose})
}

// titlePage reads a first page without scenes or speeches as the title
// page: the title, the credit ("Written by") and the author centred in
// the upper part, the rest (contact, draft) as details.
func titlePage(ls []line, width float64) (lex.Screenplay, bool) {
	if len(ls) == 0 || len(ls) > 30 {
		return nil, false
	}
	for _, l := range ls {
		for _, r := range l.Runs { // a heading, with or without a scene number
			if scenePrefix.MatchString(r.Text) {
				return nil, false
			}
		}
	}
	out := lex.Screenplay{{Type: lex.TypeTitlePage}}
	// the centred lines in the upper half are the title, the credit and
	// the author, in that order; the rest are details
	fields := []string{"Title", "Credit", "Author"}
	stage, meta := 0, false
	for _, l := range ls {
		t := l.text()
		if pageNumber.MatchString(t) {
			continue
		}
		isCentred := math.Abs((l.x()+l.end())/2-width/2) < 0.6
		if isCentred && l.Y < 7 && !meta {
			if stage < len(fields) {
				// no credit line ("Written by")? then the second is the author
				if stage == 1 && !creditLine.MatchString(t) && len(ls) > 0 && !hasCredit(ls, width) {
					stage = 2
				}
				out = append(out, lex.Line{Type: fields[stage], Contents: t})
				stage++
			} else {
				out[len(out)-1].Contents += " " + t
			}
			continue
		}
		if !meta {
			out = append(out, lex.Line{Type: "metasection"})
			meta = true
		}
		key := "Contact"
		if dateLike.MatchString(t) {
			key = "Draft date"
		}
		out = append(out, lex.Line{Type: key, Contents: t})
	}
	if stage == 0 {
		return nil, false
	}
	return append(out, lex.Line{Type: lex.TypeNewPage}), true
}

// hasCredit: whether the title page's centred lines are three or more
// (title, credit, author) or one of them is a credit line.
func hasCredit(ls []line, width float64) bool {
	n := 0
	for _, l := range ls {
		if math.Abs((l.x()+l.end())/2-width/2) < 0.6 && l.Y < 7 {
			n++
			if creditLine.MatchString(l.text()) {
				return true
			}
		}
	}
	return n >= 3
}

// runningKey is a line near the top or bottom of the page without its
// numbers, to find running headers and footers ("BLUE DRAFT 4.28.11 3.").
func runningKey(l line) string {
	if l.Y > 1.0 && l.Y < 10.0 {
		return ""
	}
	k := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' || r == ' ' || r == '.' {
			return -1
		}
		return r
	}, strings.ToUpper(l.text()))
	if k == "" {
		return ""
	}
	return k
}

// runningLines are the running headers and footers: lines at the top or
// bottom that come back on a third of the pages or more.
func runningLines(pages [][]line) map[string]bool {
	counts := map[string]int{}
	for _, p := range pages {
		seen := map[string]bool{}
		for _, l := range p {
			if k := runningKey(l); k != "" && !seen[k] {
				seen[k] = true
				counts[k]++
			}
		}
	}
	out := map[string]bool{}
	for k, n := range counts {
		if n >= 3 && n*3 >= len(pages) {
			out[k] = true
		}
	}
	return out
}
