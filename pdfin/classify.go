package pdfin

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

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
//
//nolint:gocyclo,funlen // long-standing; its scene pattern became a parameter
func classify(pages [][]line, width float64, scene *regexp.Regexp) lex.Screenplay {
	var out lex.Screenplay
	start := 0
	if title, ok := titlePage(pages[0], width, scene); ok {
		out = append(out, title...)
		start = 1
	}
	running := runningLines(pages[start:])
	var body []line
	for _, p := range pages[start:] {
		body = append(body, cleanPage(p, running, scene)...)
	}
	if len(body) == 0 {
		return out
	}
	margin := actionMargin(body)
	step := lineStep(body)
	lay := layoutOf(body, margin, step, scene)
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
			// a blank line between a name and its speech (some scripts set
			// one) is not the speech's end: elementOf read the name as a cue
			// for the speech after it
			afterCue := prevType == lex.TypeSpeaker ||
				prevType == lex.TypeParen && len(out) >= 2 && out[len(out)-2].Type == lex.TypeSpeaker
			cueGap := afterCue && l.Page == prev.Page && gap <= 2.6*step
			if !cueGap && ((l.Page == prev.Page && gap > 1.5*step) || (l.Page != prev.Page && blankAcrossPages(prevType, l, margin))) {
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
		var next, after *line // the next line, and the next after one blank line
		if i+1 < len(body) && body[i+1].Page == l.Page && body[i+1].Y-l.Y <= 1.5*step {
			next = &body[i+1]
		} else if i+1 < len(body) && body[i+1].Page == l.Page && body[i+1].Y-l.Y <= 2.6*step {
			after = &body[i+1]
		}
		t, text := elementOf(l, next, after, margin, width, prevType, scene)
		if lay.flush {
			t, text = lay.element(l, next, prevType, margin, scene, t, text)
		}
		// a name glued to its parenthetical: THE WOMAN(very calmly)
		if t == lex.TypeSpeaker {
			if m := gluedCue.FindStringSubmatch(text); m != nil && !contd.MatchString(text) && !extension.MatchString(m[2]) {
				out = append(out, lex.Line{Type: lex.TypeSpeaker, Contents: strings.TrimSpace(m[1])}, lex.Line{Type: lex.TypeParen, Contents: "(" + m[2] + ")"})
				prevType, prev = lex.TypeSpeaker, &body[i] // the speech after it as after a name
				continue
			}
		}
		if resume && t == lex.TypeDialog && len(out) > 0 && out[len(out)-1].Type == lex.TypeDialog {
			out[len(out)-1].Contents += " " + text
			resume, prevType, prev = false, t, &body[i]
			continue
		}
		resume = false
		// a scene number on the line above its heading: "3." / "EXT. …"
		if t == lex.TypeScene && prevType == lex.TypeAction && prev != nil && l.Y-prev.Y <= 1.5*step && len(out) > 0 &&
			sceneNumber.MatchString(out[len(out)-1].Contents) && !strings.HasSuffix(text, "#") {
			text += " #" + strings.TrimSuffix(out[len(out)-1].Contents, ".") + "#"
			out = out[:len(out)-1]
		}
		// a speech or paragraph wrapped over several lines is one element
		if (t == lex.TypeDialog || t == lex.TypeAction || t == lex.TypeParen) && t == prevType && prev != nil &&
			l.Y-prev.Y <= 1.5*step && l.Page == prev.Page && math.Abs(l.x()-prev.x()) < 0.35 && len(out) > 0 &&
			!(t == lex.TypeParen && strings.HasSuffix(out[len(out)-1].Contents, ")")) && !hardBreak(t, *prev, l) {
			out[len(out)-1].Contents = joinWrapped(out[len(out)-1].Contents, text)
			prev = &body[i]
			continue
		}
		out = append(out, lex.Line{Type: t, Contents: text})
		prevType, prev = t, &body[i]
	}
	return dehyphenate(out)
}

// hardBreak: a line of action much shorter than the one after it, which
// goes on at the same indent without a gap, ends where the printed line
// was broken on purpose (a name above its speech in a book's layout, a
// list): the break is kept, not the line joined to the next.
func hardBreak(t lex.ElementType, prev, l line) bool {
	if t != lex.TypeAction {
		return false
	}
	p, n := prev.end()-prev.x(), l.end()-l.x()
	return n > 2.5 && p < 0.55*n && !strings.HasSuffix(prev.text(), "-")
}

// wrapMark stands where a word broken at a line's end ("sol-" / "diers")
// was joined, until dehyphenate sees the whole script.
const wrapMark = "\x00"

// joinWrapped joins a wrapped line to the text before it: with a space,
// or a word broken with a hyphen marked for dehyphenate.
func joinWrapped(before, text string) string {
	if r, _ := utf8.DecodeRuneInString(text); unicode.IsLower(r) && brokenEnd.MatchString(before) {
		return before + wrapMark + text
	}
	return before + " " + text
}

// brokenEnd is a word broken with a hyphen at a line's end.
var brokenEnd = regexp.MustCompile(`\p{L}-$`)

// dehyphenate joins the words broken at line ends where the script has
// the whole word elsewhere ("sol-" "diers" and "soldiers"), keeps the
// compounds whole ("smoke-filled", with both words in the script); the
// rest are left as printed, with a space ("sol- diers"), never guessed.
func dehyphenate(s lex.Screenplay) lex.Screenplay {
	marked := false
	words := map[string]bool{}
	for _, l := range s {
		if strings.Contains(l.Contents, wrapMark) {
			marked = true
		}
		for _, w := range wordRe.FindAllString(strings.ReplaceAll(l.Contents, wrapMark, " "), -1) {
			words[strings.ToLower(w)] = true
		}
	}
	if !marked {
		return s
	}
	for i, l := range s {
		if !strings.Contains(l.Contents, wrapMark) {
			continue
		}
		parts := strings.Split(l.Contents, wrapMark)
		var b strings.Builder
		b.WriteString(parts[0])
		for _, p := range parts[1:] {
			head := wordRe.FindString(b.String()[strings.LastIndexAny(b.String(), " \t")+1:])
			tail := wordRe.FindString(p)
			if whole := strings.ToLower(strings.TrimSuffix(head, "-") + tail); head != "" && tail != "" && words[whole] {
				b.WriteString("\x01") // the hyphen goes
			} else if head != "" && tail != "" && (words[strings.ToLower(head+tail)] ||
				words[strings.ToLower(strings.TrimSuffix(head, "-"))] && words[strings.ToLower(tail)]) {
				// a compound: written whole elsewhere, or both its words are
				// the script's ("smoke-" "filled"); the hyphen stays
			} else {
				b.WriteString(" ")
			}
			b.WriteString(p)
		}
		s[i].Contents = strings.ReplaceAll(b.String(), "-\x01", "")
	}
	return s
}

// wordRe is a word: letters, with hyphens inside or at its end.
var wordRe = regexp.MustCompile(`\p{L}+(?:-\p{L}*)*`)

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

// layout is a script's own indents, where they are not a screenplay's
// usual ones: names at the margin (with a colon or not), each with its
// speech on the next line a little in, the action further in (a stage
// play's layout, and some programs' "alternative" screenplay one).
type layout struct {
	flush          bool    // names at the margin
	speech, action float64 // the speech's and the action's indents from the margin
}

// layoutOf finds a script's layout: names at the margin when many lines
// at it are names with a speech under them at one indent, and few names
// stand where a screenplay's do.
func layoutOf(ls []line, margin, step float64, scene *regexp.Regexp) layout {
	speech := map[float64]int{}
	flushNames, usual := 0, 0
	for i := 0; i+1 < len(ls); i++ {
		l, n := ls[i], ls[i+1]
		if n.Page != l.Page || n.Y-l.Y > 1.5*step || !l.upper() || n.upper() {
			continue
		}
		d := l.x() - margin
		switch {
		case d >= nameFrom && d < nameTo:
			usual++
		case d < 0.15 && nameLike(cueName(l.text())) && !scene.MatchString(l.text()):
			if in := n.x() - margin; in >= 0.1 && in < 1.0 {
				speech[math.Round(in*20)/20]++
				flushNames++
			}
		}
	}
	if flushNames < 5 || usual*3 > flushNames {
		return layout{}
	}
	lay := layout{flush: true}
	best := 0
	for in, c := range speech {
		if c > best || c == best && in < lay.speech {
			lay.speech, best = in, c
		}
	}
	// the action: the most common indent of the lines that are neither
	// at the margin nor at the speech's
	other := map[float64]int{}
	for _, l := range ls {
		if d := l.x() - margin; d >= 0.1 && math.Abs(d-lay.speech) > 0.1 && d < 2 {
			other[math.Round(d*20)/20]++
		}
	}
	lay.action = -1
	best = 0
	for in, c := range other {
		if c > best {
			lay.action, best = in, c
		}
	}
	return lay
}

// element is a line's type and text in a layout with names at the
// margin: a name over its speech, the speech's lines, the action at its
// indent; else as elementOf read it (t, text).
func (lay layout) element(l line, next *line, prev lex.ElementType, margin float64, scene *regexp.Regexp,
	t lex.ElementType, text string) (lex.ElementType, string) {
	d := l.x() - margin
	inSpeech := prev == lex.TypeSpeaker || prev == lex.TypeParen || prev == lex.TypeDialog
	atSpeech := func(n *line) bool { return n != nil && math.Abs(n.x()-margin-lay.speech) < 0.15 }
	switch {
	case d < 0.15 && l.upper() && nameLike(cueName(l.text())) && !scene.MatchString(l.text()) && atSpeech(next) &&
		(!next.upper() || parenLine(next.text())) && t != lex.TypeTrans:
		// (a name in bold, which elementOf takes for a heading, too)
		return lex.TypeSpeaker, contd.ReplaceAllString(cueName(l.text()), "")
	case t == lex.TypeScene || t == lex.TypeTrans:
		return t, text
	case inSpeech && atSpeech(&l) && strings.HasPrefix(l.text(), "("):
		return lex.TypeParen, l.text()
	case inSpeech && prev == lex.TypeParen && atSpeech(&l) && !strings.HasSuffix(l.text(), ")") && t == lex.TypeParen:
		return lex.TypeParen, l.text()
	case inSpeech && atSpeech(&l):
		return lex.TypeDialog, l.text()
	case lay.action >= 0 && math.Abs(d-lay.action) < 0.1 && (t == lex.TypeCenter || t == lex.TypeDialog || t == lex.TypeParen):
		return lex.TypeAction, l.text() // the action's indent, not a centred line
	}
	return t, text
}

// cueName is a name at the margin without the colon a stage play's
// names have ("EDWARD:").
func cueName(text string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ":"))
}

// elementOf is the type and the text of a line, by its indent.
//
//nolint:gocyclo // long-standing; its scene pattern became a parameter
func elementOf(l line, next, after *line, margin, width float64, prev lex.ElementType,
	scene *regexp.Regexp) (lex.ElementType, string) {
	text := l.text()
	d := l.x() - margin
	right := width - 1.0 // the right margin of the text
	inSpeech := prev == lex.TypeSpeaker || prev == lex.TypeParen || prev == lex.TypeDialog
	// the line under a name: its speech (or a parenthetical), at the
	// speech's indent
	speechUnder := func(n *line) bool {
		return n != nil && (n.x()-margin >= dialogueFrom && n.x()-margin < nameFrom ||
			// a short speech centred under its name
			n.x()-margin >= dialogueFrom && centred(*n, margin, right) && (!n.upper() || parenLine(n.text())) ||
			// a parenthetical under the name, wherever it stands
			n.x()-margin < nameTo && parenLine(n.text()))
	}
	switch {
	case d < dialogueFrom && sceneWord.MatchString(text):
		// "Scene 12 - int. kitchen, day -": the number kept as the scene's
		m := sceneWord.FindStringSubmatch(text)
		return lex.TypeScene, strings.ToUpper(strings.Trim(m[3], " -–—:")) + " #" + m[2] + "#"
	case prev == lex.TypeSpeaker && d < nameTo && parenLine(text):
		return lex.TypeParen, text // under its name, centred or not
	case endMark.MatchString(text):
		// the script's last line, numbered as a scene's or not
		return lex.TypeCenter, endMark.FindStringSubmatch(text)[1]
	case fadeOut.MatchString(text):
		return lex.TypeTrans, strings.TrimSpace(strings.Split(text, "#")[0])
	case l.numbered && (d < 0.3 || strings.HasPrefix(text, "#")),
		d < 0.3 && l.upper() && (scene.MatchString(text) || (l.bold() && len(text) < 60)),
		// its scene word and the next word in capitals, the rest not
		// ("EXT – STREET in OLD WARSAW – DAY")
		d < 0.3 && scene.MatchString(text) && upperStart(text, 2),
		// a scene number before a heading's words, wherever they stand
		// (some scripts indent them both)
		l.numberAt > 0 && scene.MatchString(text):
		return lex.TypeScene, ocrComma.ReplaceAllString(text, "$1.")
	case l.upper() && d > 2.5 && (transition.MatchString(text) || l.end() > right-0.4):
		return lex.TypeTrans, text
	case d >= nameFrom && d < nameTo && l.upper() && !inSpeechEnd(text) && speechUnder(next):
		// a name, with its speech on the next line
		return lex.TypeSpeaker, contd.ReplaceAllString(text, "")
	case d >= nameFrom && d < nameTo && l.upper() && !inSpeechEnd(text) && len([]rune(text)) <= 40 &&
		speechUnder(after) && (!after.upper() || strings.HasPrefix(after.text(), "(")):
		// a name with a blank line before its speech, as some scripts set them
		return lex.TypeSpeaker, contd.ReplaceAllString(text, "")
	case !inSpeech && d >= dialogueFrom && d < nameTo && nameLike(text) && next != nil &&
		math.Abs(next.x()-l.x()) < 0.35 && !next.upper() && !strings.HasPrefix(next.text(), "(") &&
		!startsDash(next.text()):
		// (not a shot's words going on: "A RED FLAG" / "—waving")
		// a name at its speech's indent (a typescript's), glued to its
		// parenthetical or not
		return lex.TypeSpeaker, contd.ReplaceAllString(text, "")
	case strings.HasPrefix(text, "(") && d >= dialogueFrom && d < nameFrom && (inSpeech || prev == ""):
		return lex.TypeParen, text
	case prev == lex.TypeParen && d >= parenFrom && d < nameFrom && inSpeech:
		return lex.TypeParen, text // a parenthetical's second line
	case d >= dialogueFrom && d < nameFrom && inSpeech:
		return lex.TypeDialog, text
	case inSpeech && d >= dialogueFrom && centred(l, margin, right) && !l.upper():
		return lex.TypeDialog, text // a short speech centred under its name
	case d > 0.5 && centred(l, margin, right):
		return lex.TypeCenter, text
	}
	return lex.TypeAction, text
}

var (
	// sceneWord is a heading numbered with a word: "Scene 12 - int. day -"
	sceneWord = regexp.MustCompile(`^(?i)(scene|sc\.)\s*(\d+[A-Za-z]?)\s*[-–—:.]\s*(\S.*)$`)
	// gluedCue is a name with its parenthetical: "THE WOMAN(very calmly)"
	gluedCue = regexp.MustCompile(`^([^()]*\S)\s*\(([^()]+)\)$`)
	// extension is a parenthetical that goes with the name: (V.O.)
	extension = regexp.MustCompile(`^(?i)(v\.?\s?o\.?|o\.?\s?s\.?|o\.?\s?c\.?|cont['’]?d\.?|cont\.?|off|filtered)$`)
)

// startsDash: the text starts with a dash (a shot's words going on).
func startsDash(text string) bool {
	r, _ := utf8.DecodeRuneInString(text)
	return r == '-' || r == '—' || r == '–'
}

// parenLine: a whole line in parentheses.
func parenLine(text string) bool {
	return strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") && strings.Count(text, "(") == 1
}

// upperStart: the text's first n words are in capitals.
func upperStart(text string, n int) bool {
	ws := strings.Fields(text)
	if len(ws) < n {
		return false
	}
	for _, w := range ws[:n] {
		if w != strings.ToUpper(w) {
			return false
		}
	}
	return true
}

// nameLike: a name in capitals, short, before a parenthetical or not.
func nameLike(text string) bool {
	name := text
	if m := gluedCue.FindStringSubmatch(text); m != nil {
		name = m[1]
	}
	name = strings.TrimSpace(name)
	return name != "" && name == strings.ToUpper(name) && hasLetter(name) && len([]rune(name)) <= 30 &&
		len(strings.Fields(name)) <= 4 && !inSpeechEnd(name) && !strings.ContainsAny(name[len(name)-1:], ",;:-") &&
		!strings.HasSuffix(name, "—") && !strings.HasSuffix(name, "–")
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
	bin := func(x float64) int { return counts[math.Round(x*10)/10] }
	for _, x := range xs {
		// enough lines that stray marks in the margin (an OCR layer's
		// noise, line numbers) are not taken for it; a scan's margin
		// wanders, over two tenths (0.7 and 0.8)
		if counts[x] >= max(2, len(ls)/7) {
			return x
		}
		if up := math.Round((x+0.1)*10) / 10; counts[x]+bin(up) >= max(2, len(ls)/7) && counts[x] >= max(2, len(ls)/30) {
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
	// (common: more than a few, which a scan's stray marks between the
	// lines make)
	if half := (best + 1) / 2; near(half) >= max(2, n/10) && half >= 9 {
		best = half
	}
	return float64(best) / 100
}

// cleanPage leaves out what is not the script: page numbers, (MORE),
// CONTINUED; scene numbers in the margins go to the end of their heading.
//
//nolint:gocyclo // long-standing; its scene pattern became a parameter
func cleanPage(ls []line, running map[string]bool, scene *regexp.Regexp) []line {
	var out []line
	for i, l := range ls {
		t := l.text()
		if running[runningKey(l)] {
			continue
		}
		// a stray mark alone on a line (a scan's dust: "•", "'")
		if len([]rune(t)) <= 2 && !strings.ContainsFunc(t, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) {
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
		// a scene number before a heading's words in one run: 12. INT. HOUSE
		if m := numberedHeading.FindStringSubmatch(l.Runs[0].Text); m != nil && scene.MatchString(m[2]) {
			first := l.Runs[0]
			l.Runs = append([]run{{X: first.X, End: first.X, Text: m[1]}, {X: first.X, End: first.End, Text: m[2]}}, l.Runs[1:]...)
		}
		// a scene number left (and right) of a heading
		if len(l.Runs) >= 2 && sceneNumber.MatchString(l.Runs[0].Text) {
			num := strings.TrimSuffix(l.Runs[0].Text, ".")
			rest := l.Runs[1:]
			if n := len(rest); n >= 1 && strings.TrimSuffix(rest[n-1].Text, ".") == num {
				l.numbered = true
				if n == 1 { // the numbers only: a heading without words
					l.Runs = []run{{X: rest[0].X, End: rest[0].X, Text: "#" + num + "#"}}
					out = append(out, l)
					continue
				}
				rest = rest[:n-1]
			}
			if t := joinRuns(rest); scene.MatchString(t) || t == strings.ToUpper(t) {
				l.numberAt = l.Runs[0].X
				l.Runs = append(append([]run(nil), rest...), run{X: rest[len(rest)-1].End, End: rest[len(rest)-1].End, Text: "#" + num + "#"})
			}
		}
		// or only right of it
		if n := len(l.Runs); n >= 2 && sceneNumber.MatchString(l.Runs[n-1].Text) && l.Runs[n-1].X > 6.0 &&
			!strings.HasSuffix(l.Runs[n-2].Text, "#") && scene.MatchString(joinRuns(l.Runs[:n-1])) {
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
//
//nolint:gocyclo // long-standing; its scene pattern became a parameter
func titlePage(ls []line, width float64, scene *regexp.Regexp) (lex.Screenplay, bool) {
	if len(ls) == 0 || len(ls) > 30 {
		return nil, false
	}
	for _, l := range ls {
		for _, r := range l.Runs { // a heading, with or without a scene number
			if scene.MatchString(r.Text) {
				return nil, false
			}
		}
	}
	out := lex.Screenplay{{Type: lex.TypeTitlePage}}
	// the centred lines in the upper half are the title, the credit and
	// the author, in that order; the rest are details. With a credit line
	// ("Written by", "Written and directed by"), all above it is the
	// title, a line of its own each ("STAR WARS" / "EPISODE IV" / ...).
	fields := []string{"Title", "Credit", "Author"}
	stage, meta := 0, false
	credit := hasCreditLine(ls, width)
	for _, l := range ls {
		t := l.text()
		if pageNumber.MatchString(t) {
			continue
		}
		isCentred := math.Abs((l.x()+l.end())/2-width/2) < 0.6
		if isCentred && l.Y < 7 && !meta && !sourceLine.MatchString(t) {
			switch {
			case credit && stage == 1 && !creditLine.MatchString(t):
				out = append(out, lex.Line{Type: "Title", Contents: t}) // the title goes on
			case stage < len(fields):
				// no credit line ("Written by")? then the second is the author
				if stage == 1 && !creditLine.MatchString(t) && len(ls) > 0 && !hasCredit(ls, width) {
					stage = 2
				}
				out = append(out, lex.Line{Type: fields[stage], Contents: t})
				stage++
			default:
				out[len(out)-1].Contents += " " + t
			}
			continue
		}
		meta = true
		key := "Contact"
		switch {
		case sourceLine.MatchString(t):
			key = "Source"
		case dateLike.MatchString(t):
			key = "Draft date"
		}
		out = append(out, lex.Line{Type: key, Contents: t})
	}
	if stage == 0 {
		return nil, false
	}
	return append(out, lex.Line{Type: lex.TypeNewPage}), true
}

// hasCreditLine: whether one of the title page's centred lines in its
// upper part is a credit line ("Written by").
func hasCreditLine(ls []line, width float64) bool {
	for _, l := range ls {
		if math.Abs((l.x()+l.end())/2-width/2) < 0.6 && l.Y < 7 && creditLine.MatchString(l.text()) {
			return true
		}
	}
	return false
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
