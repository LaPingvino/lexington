package fountain

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// Strip is a script without what one hands on without: a clean Fountain
// file, for other apps and other people. It always drops Accolade's data
// — its end blocks (EndBlocks), the language line after them
// (LanguageLine) and its links to the source story ([[from: …]],
// IsSourceLink) — and, as the options say, more of what is the writer's
// own (StripOptions).
//
// What is not stripped stays as it is, byte for byte (a text with
// nothing to strip comes back as it was given). A line that held only
// what is stripped goes with it, and a blank line with it where two
// would be left; a line that would read as another element without what
// went (a cue after a boneyard line, a scene heading after a section)
// gets its force mark, as KeepLoose gives them, so the script reads as
// it did. scenes are the scene words the text is read with.
func Strip(text string, scenes []string, o StripOptions) string {
	crlf := strings.Contains(text, "\r\n")
	in := text
	if crlf {
		in = strings.ReplaceAll(text, "\r\n", "\n")
	}
	out := strip(in, scenes, o)
	if out == in {
		return text
	}
	if crlf {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	return out
}

// StripOptions are what Strip drops besides Accolade's data.
type StripOptions struct {
	Notes        bool // every note, [[…]]
	Boneyard     bool // every boneyard, /* … */ (an omitted scene's kept text too)
	SceneNumbers bool // the scene numbers, #12#
	Omitted      bool // the OMITTED headings, with the scene kept under each
	TitlePage    bool // the title page
	Outline      bool // the writer's outline: sections (# Act One) and synopses (= …)
}

// StripOption is one of StripOptions, by the key the command line's
// -strip names it with.
type StripOption struct {
	Key  string
	Help string
	set  func(*StripOptions) *bool
}

// Of is the option's field in o.
func (s StripOption) Of(o *StripOptions) *bool { return s.set(o) }

// StripKeys are the options, in the order they are listed.
var StripKeys = []StripOption{
	{"notes", "every note [[…]]", func(o *StripOptions) *bool { return &o.Notes }},
	{"boneyard", "every boneyard /* … */, an omitted scene's kept text too", func(o *StripOptions) *bool { return &o.Boneyard }},
	{"numbers", "the scene numbers #12#", func(o *StripOptions) *bool { return &o.SceneNumbers }},
	{"omitted", "the OMITTED headings, with the scene kept under each", func(o *StripOptions) *bool { return &o.Omitted }},
	{"titlepage", "the title page", func(o *StripOptions) *bool { return &o.TitlePage }},
	{"outline", "the writer's outline: sections # and synopses =", func(o *StripOptions) *bool { return &o.Outline }},
}

// StripPreset is a set of options with a name.
type StripPreset struct {
	Key, Name, Help string
	Options         StripOptions
}

// StripPresets are the sets one asks for most: Accolade's data only; a
// reading copy, without the notes and the outline; and the writer's
// version, the shooting script's numbers and OMITTED headings taken off
// (the writer's draft from before the lock, as it is now).
var StripPresets = []StripPreset{
	{"clean", "Clean (Accolade's data only)", "Accolade's data only", StripOptions{}},
	{"reading", "Reading copy (no notes, no outline)", "notes and outline", StripOptions{Notes: true, Outline: true}},
	{"writer", "Writer's version (no scene numbers, no OMITTED)", "scene numbers and OMITTED", StripOptions{SceneNumbers: true, Omitted: true}},
}

// ParseStrip reads the command line's -strip: a comma list of options
// and presets, each adding to the others ("writer,notes"); "clean" (or
// "") is Accolade's data only.
func ParseStrip(list string) (StripOptions, error) {
	var o StripOptions
	for _, k := range strings.Split(list, ",") {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" {
			continue
		}
		if i := slices.IndexFunc(StripPresets, func(p StripPreset) bool { return p.Key == k }); i >= 0 {
			o = o.With(StripPresets[i].Options)
			continue
		}
		i := slices.IndexFunc(StripKeys, func(s StripOption) bool { return s.Key == k })
		if i < 0 {
			return o, fmt.Errorf("-strip: no option %q (%s)", k, StripList())
		}
		*StripKeys[i].Of(&o) = true
	}
	return o, nil
}

// With is o with p's options on too.
func (o StripOptions) With(p StripOptions) StripOptions {
	for _, s := range StripKeys {
		if *s.Of(&p) {
			*s.Of(&o) = true
		}
	}
	return o
}

// String is o as -strip takes it: its options' keys, or "clean".
func (o StripOptions) String() string {
	var keys []string
	for _, s := range StripKeys {
		if *s.Of(&o) {
			keys = append(keys, s.Key)
		}
	}
	if len(keys) == 0 {
		return "clean"
	}
	return strings.Join(keys, ",")
}

// StripList is the keys -strip takes, presets first.
func StripList() string {
	var keys []string
	for _, p := range StripPresets {
		keys = append(keys, p.Key)
	}
	for _, s := range StripKeys {
		keys = append(keys, s.Key)
	}
	return strings.Join(keys, ", ")
}

// sline is a line of the text being stripped, and the line of the text
// it came from (from 1).
type sline struct {
	text   string
	origin int
}

func strip(text string, scenes []string, o StripOptions) string {
	// Accolade's end blocks and the language line after them
	if at, _ := LanguageLine(text); at >= 0 {
		text = text[:at]
	}
	if _, body := EndBlocks(text); body < len(text) {
		nl := strings.HasSuffix(text, "\n")
		text = text[:body]
		if text != "" && nl {
			text += "\n"
		}
	}
	final := strings.HasSuffix(text, "\n")
	body := strings.TrimSuffix(text, "\n")
	if body == "" {
		return text
	}
	want := elementsOf(body, scenes, false)
	var ls []sline
	for i, l := range strings.Split(body, "\n") {
		ls = append(ls, sline{l, i + 1})
	}
	if o.Omitted {
		ls = stripOmitted(ls, scenes)
	}
	ls = stripSpans(ls, o)
	if o.SceneNumbers {
		ls = stripNumbers(ls, scenes)
	}
	if o.Outline {
		ls = stripOutline(ls, scenes)
	}
	if o.TitlePage {
		ls = stripTitlePage(ls, scenes)
	}
	ls = keepReading(ls, want, scenes)
	out := joinS(ls)
	if final && out != "" {
		out += "\n"
	}
	return out
}

func joinS(ls []sline) string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.text
	}
	return strings.Join(out, "\n")
}

// blankLine is a line that parts paragraphs: empty, or white space but
// not the two spaces that keep a speech going.
func blankLine(l string) bool {
	return strings.TrimSpace(l) == "" && !onlySpaces(l)
}

// dropLines is ls without the lines drop says; where a run of them went
// from between two blank lines (or the start of the text and a blank
// line, or a blank line and the end), one of those goes too.
func dropLines(ls []sline, drop []bool) []sline {
	out := make([]sline, 0, len(ls))
	for i := 0; i < len(ls); {
		if !drop[i] {
			out = append(out, ls[i])
			i++
			continue
		}
		j := i
		for j < len(ls) && drop[j] {
			j++
		}
		if len(out) == 0 || blankLine(out[len(out)-1].text) {
			switch {
			case j < len(ls) && blankLine(ls[j].text):
				j++
			case j == len(ls) && len(out) > 0:
				out = out[:len(out)-1]
			}
		}
		i = j
	}
	return out
}

// typesOf are the elements ls are read as, by index.
func typesOf(ls []sline, scenes []string) []string {
	got := elementsOf(joinS(ls), scenes, false)
	out := make([]string, len(ls))
	for i := range ls {
		out[i] = got[i+1]
	}
	return out
}

// stripOmitted drops each OMITTED heading, and the boneyard right under
// it that keeps the scene (OmitScene's).
func stripOmitted(ls []sline, scenes []string) []sline {
	types := typesOf(ls, scenes)
	drop := make([]bool, len(ls))
	for i, l := range ls {
		if types[i] != lex.TypeScene {
			continue
		}
		if _, ok := lex.OmittedNumber(l.text); !ok {
			continue
		}
		drop[i] = true
		j := i + 1
		for j < len(ls) && strings.TrimSpace(ls[j].text) == "" {
			j++
		}
		if j == len(ls) || !strings.HasPrefix(strings.TrimSpace(ls[j].text), omittedOpen) {
			continue
		}
		end := j
		for end < len(ls) && strings.TrimSpace(ls[end].text) != "*/" {
			end++
		}
		if end == len(ls) { // never closed: not the kept scene
			continue
		}
		for k := i + 1; k <= end; k++ {
			drop[k] = true
		}
	}
	return dropLines(ls, drop)
}

// span is a stretch of text to take out, in bytes.
type span struct{ start, end int }

// stripSpans takes out the notes and boneyards o says, and the source
// links always; a line left with nothing goes.
func stripSpans(ls []sline, o StripOptions) []sline {
	text := joinS(ls)
	spans := spansOf(text, o)
	if len(spans) == 0 {
		return ls
	}
	starts := make([]int, len(ls)) // each line's first byte
	for i, at := 1, 0; i < len(ls); i++ {
		at += len(ls[i-1].text) + 1
		starts[i] = at
	}
	originAt := func(b int) int { // the origin of the line byte b is in
		return ls[sort.SearchInts(starts, b+1)-1].origin
	}
	var out []sline
	var touched []bool
	cur := strings.Builder{}
	curOrigin := ls[0].origin
	curTouched := false
	write := func(a, b int) {
		for a < b {
			nl := strings.IndexByte(text[a:b], '\n')
			if nl < 0 {
				cur.WriteString(text[a:b])
				return
			}
			cur.WriteString(text[a : a+nl])
			out, touched = append(out, sline{cur.String(), curOrigin}), append(touched, curTouched)
			cur.Reset()
			a += nl + 1
			curOrigin, curTouched = originAt(a), false
		}
	}
	last := 0
	for _, s := range spans {
		write(last, s.start)
		curTouched = true
		last = s.end
	}
	write(last, len(text))
	out, touched = append(out, sline{cur.String(), curOrigin}), append(touched, curTouched)
	drop := make([]bool, len(out))
	for i, l := range out {
		drop[i] = touched[i] && strings.TrimSpace(l.text) == ""
	}
	return dropLines(out, drop)
}

// spansOf are the notes and boneyards of text to take out, in order. A
// boneyard is what Parse reads as one: from a line that starts with "/*"
// to the end of the line that closes it with "*/" (to the end of the
// text if none does); a "/*" within a line is the line's text. A note
// in a boneyard is the boneyard's, but for a source link: those go
// wherever they are. A note does not go on over a blank line (as Parse
// reads one).
func spansOf(text string, o StripOptions) []span {
	bones := boneyards(text)
	in := func(at int) bool {
		for _, b := range bones {
			if at >= b.start && at < b.end {
				return true
			}
		}
		return false
	}
	var out []span
	if o.Boneyard {
		for _, b := range bones {
			out = append(out, widen(text, b.start, b.end))
		}
	}
	for i := 0; i < len(text); {
		n := strings.Index(text[i:], "[[")
		if n < 0 {
			break
		}
		start := i + n
		end := noteEnd(text, start)
		if end < 0 {
			i = start + 2
			continue
		}
		i = end
		bone := in(start)
		if bone && o.Boneyard {
			continue // goes with it
		}
		if (o.Notes && !bone) || IsSourceLink(text[start+2:end-2]) {
			out = append(out, widen(text, start, end))
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].start < out[b].start })
	// a note over a boneyard's edge: the first wins
	kept := out[:0]
	for _, s := range out {
		if len(kept) > 0 && s.start < kept[len(kept)-1].end {
			continue
		}
		kept = append(kept, s)
	}
	return kept
}

// boneyards are the boneyards of text, as Parse reads them: each from
// its "/*" to the end of the line that closes it.
func boneyards(text string) []span {
	var out []span
	open := -1
	for at := 0; at < len(text); {
		end := strings.IndexByte(text[at:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += at
		}
		t := strings.TrimSpace(text[at:end])
		switch {
		case open < 0 && strings.HasPrefix(t, "/*"):
			open = at + strings.Index(text[at:end], "/*")
			if strings.Contains(t[2:], "*/") {
				out, open = append(out, span{open, end}), -1
			}
		case open >= 0 && strings.Contains(t, "*/"):
			out, open = append(out, span{open, end}), -1
		}
		at = end + 1
	}
	if open >= 0 {
		out = append(out, span{open, len(text)})
	}
	return out
}

// noteEnd is where the note opened at start ends (past its "]]"), or -1
// if a blank line comes first or it is never closed.
func noteEnd(text string, start int) int {
	e := strings.Index(text[start+2:], "]]")
	if e < 0 {
		return -1
	}
	end := start + 2 + e + 2
	for _, l := range strings.Split(text[start:end], "\n")[1:] {
		if strings.TrimSpace(l) == "" && !strings.HasPrefix(l, "  ") {
			return -1
		}
	}
	return end
}

// widen is the span with the white space before it on its line, and, if
// nothing is before it on its line, the white space after it: as
// lex.StripNotes takes a note out, nothing left dangling.
func widen(text string, start, end int) span {
	for start > 0 && (text[start-1] == ' ' || text[start-1] == '\t') {
		start--
	}
	if start == 0 || text[start-1] == '\n' {
		for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
			end++
		}
	}
	return span{start, end}
}

// stripNumbers takes the scene numbers off the headings.
func stripNumbers(ls []sline, scenes []string) []sline {
	types := typesOf(ls, scenes)
	for i, l := range ls {
		if types[i] != lex.TypeScene {
			continue
		}
		if _, n := lex.SceneNumber(l.text); n != "" {
			ls[i].text = lex.WithSceneNumber(l.text, "")
		}
	}
	return ls
}

// stripOutline drops the sections and synopses.
func stripOutline(ls []sline, scenes []string) []sline {
	types := typesOf(ls, scenes)
	drop := make([]bool, len(ls))
	for i := range ls {
		drop[i] = types[i] == lex.TypeSection || types[i] == lex.TypeSynopsis
	}
	return dropLines(ls, drop)
}

// stripTitlePage drops the title page, and the blank lines after it.
func stripTitlePage(ls []sline, scenes []string) []sline {
	rows := ParseLines(scenes, strings.NewReader(joinS(ls)))
	if len(rows) == 0 || rows[0].Type != lex.TypeTitlePage {
		return ls
	}
	end := len(ls) // the first line after it (from 0)
	if i := slices.IndexFunc(rows, func(r lex.Line) bool { return r.Type == lex.TypeNewPage }); i >= 0 {
		end = min(rows[i].Source-1, len(ls))
	}
	rest := ls[end:]
	for len(rest) > 0 && strings.TrimSpace(rest[0].text) == "" {
		rest = rest[1:]
	}
	return rest
}

// keepReading gives the lines that now read as another element than they
// did (want, by their origin) their force mark, where Fountain has one:
// the first line of each run of them, then all is read again, until
// nothing changes (as KeepLoose).
func keepReading(ls []sline, want map[int]string, scenes []string) []sline {
	marked := make([]bool, len(ls))
	for range len(ls) + 1 {
		types := typesOf(ls, scenes)
		changed, inRun := false, false
		for i, l := range ls {
			w, ok := want[l.origin]
			if !ok || w == types[i] {
				inRun = false
				continue
			}
			if inRun {
				continue
			}
			inRun = true
			m, has := keepMarks[w]
			if !has || marked[i] || forced(strings.TrimSpace(l.text)) {
				continue
			}
			ls[i].text, marked[i], changed = string(m)+strings.TrimLeft(l.text, " \t"), true, true
		}
		if !changed {
			break
		}
	}
	return ls
}
