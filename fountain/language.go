package fountain

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// A Dialect is a language's rules (rules.Language) ready to read lines
// with: what makes a line a scene heading, a transition, a character's
// cue. The parser reads with one, and so can an editor that tells
// elements apart while the writer types (Accolade), so that both read a
// line the same. It holds no state of a script: a script's names are
// its Cast.
type Dialect struct {
	Lang     rules.Language
	headings []*regexp.Regexp
	ext      map[string]bool
	trans    map[string]bool
	maxWord  int // the longest scene word, in bytes
}

// maxShort is how long (in bytes) a line read as a heading by a pattern,
// a transition or a cue can be: a paragraph is none of them, and is not
// read through for them on every key.
const maxShort = 240

var dialects sync.Map // a language's rules, as text, to its *Dialect

// NewDialect is the dialect of a language's rules.
func NewDialect(l rules.Language) *Dialect {
	key := dialectKey(l)
	if d, ok := dialects.Load(key); ok {
		return d.(*Dialect)
	}
	d := &Dialect{Lang: l, ext: map[string]bool{}, trans: map[string]bool{}}
	for _, p := range l.Headings {
		if re := headingPattern(p, l); re != nil {
			d.headings = append(d.headings, re)
		}
	}
	for _, e := range l.Extensions {
		d.ext[squash(e)] = true
	}
	for _, w := range l.Scenes {
		d.maxWord = max(d.maxWord, len(strings.ToUpper(w)))
	}
	for _, t := range l.Transitions {
		d.trans[squash(t)] = true
	}
	dialects.Store(key, d)
	return d
}

func dialectKey(l rules.Language) string {
	var b strings.Builder
	for _, part := range [][]string{l.Scenes, l.Headings, l.Settings, l.Times, l.Cues, l.Extensions, l.TransitionEnds, l.Transitions} {
		b.WriteString(strings.Join(part, "\x1f"))
		b.WriteByte(0)
	}
	b.WriteString(l.TransitionMarks + "\x00" + l.Shape.Ends)
	b.WriteString(string(rune(l.Shape.Words+1)) + string(rune(l.Shape.Chars+1)) + string(rune(l.Repeat+1)))
	// the title page keys too: two languages alike but for them (Esperanto
	// and toki pona) are two dialects
	for _, k := range slices.Sorted(maps.Keys(l.TitleKeys)) {
		b.WriteString("\x00" + k + "\x1f" + l.TitleKeys[k])
	}
	b.WriteString("\x00" + l.Direction + "\x00" + l.NewHeading)
	return b.String()
}

// squash is a word to compare: in capitals, its spaces one, dots kept.
func squash(s string) string { return strings.ToUpper(strings.Join(strings.Fields(s), " ")) }

// numberSlot is what a scene number in a heading pattern is: digits
// (Western, Arabic, Persian, Chinese), a letter after them or before
// them (15B, A15: a locked script's scenes put in between).
const numberSlot = `[A-Za-z]?[0-9٠-٩۰-۹零〇一二三四五六七八九十百]+[A-Za-z]?`

// headingPattern is a heading pattern as a regular expression over a
// line in capitals, nil for one that cannot be read: the pattern, or it
// with slots left out (rules.PatternVariants: "场景2 厨房 内", no time).
func headingPattern(p string, l rules.Language) *regexp.Regexp {
	var alts []string
	for _, v := range rules.PatternVariants(p) {
		alts = append(alts, headingBody(v, l))
	}
	re, err := regexp.Compile(`^(?:` + strings.Join(alts, "|") + `)\s*$`)
	if err != nil {
		return nil
	}
	return re
}

// headingBody is one pattern as a regular expression's body.
func headingBody(p string, l rules.Language) string {
	words := func(ws ...[]string) string {
		var alts []string
		for _, list := range ws {
			for _, w := range list {
				if w = strings.TrimSuffix(strings.TrimSpace(w), "."); w != "" {
					alts = append(alts, regexp.QuoteMeta(squash(w)))
				}
			}
		}
		if len(alts) == 0 {
			return `(?:\x00)` // matches nothing
		}
		return `(?:` + strings.Join(alts, "|") + `)\.?`
	}
	for _, sep := range []string{"-", "–", "—", "/"} { // the separator takes its spaces
		p = strings.ReplaceAll(strings.ReplaceAll(p, " "+sep, sep), sep+" ", sep)
	}
	var b strings.Builder
	for p != "" {
		switch {
		case strings.HasPrefix(p, "{n}"):
			b.WriteString(`(?P<n>` + numberSlot + `)`)
			p = p[3:]
		case strings.HasPrefix(p, "{io}"):
			b.WriteString(`(?P<io>` + words(l.Settings, l.Scenes) + `)`)
			p = p[4:]
		case strings.HasPrefix(p, "{time}"):
			b.WriteString(`(?P<time>` + words(l.Times) + `)`)
			p = p[6:]
		case strings.HasPrefix(p, "{place}"):
			b.WriteString(`(?P<place>\S.*?)`)
			p = p[7:]
		case strings.HasPrefix(p, "{any}"):
			b.WriteString(`(?P<any>.*?)`)
			p = p[5:]
		default:
			r := []rune(p)[0]
			switch {
			case unicode.IsSpace(r):
				b.WriteString(`\s+`)
			case r == '-' || r == '–' || r == '—':
				b.WriteString(`\s*[-–—]\s*`)
			case r == '/':
				b.WriteString(`\s*/\s*`)
			default:
				b.WriteString(regexp.QuoteMeta(string(unicode.ToUpper(r))))
			}
			p = p[len(string(r)):]
		}
	}
	return b.String()
}

// HasPatterns tells whether the language has heading patterns.
func (d *Dialect) HasPatterns() bool { return len(d.headings) > 0 }

// sceneNumberAtEnd is a scene number written at a heading's end (#12#).
var sceneNumberAtEnd = regexp.MustCompile(`\s*#[^#\s]+#\s*$`)

// Heading tells whether a line (forced or not: a leading . is no word)
// is a scene heading by the language's words or patterns.
func (d *Dialect) Heading(row string) bool {
	row = strings.TrimSpace(row)
	// a scene word is at the start: only so much of the line is read
	head := row
	if len(head) > d.maxWord+8 {
		head = head[:d.maxWord+8]
	}
	upper := strings.ToUpper(head)
	for _, w := range d.Lang.Scenes {
		if StartsScene(upper, w) {
			return true
		}
	}
	if len(d.headings) == 0 || len(row) > maxShort {
		return false // a paragraph is no heading
	}
	upper = strings.ToUpper(row)
	bare := squash(sceneNumberAtEnd.ReplaceAllString(upper, ""))
	for _, re := range d.headings {
		if re.MatchString(bare) {
			return true
		}
	}
	return false
}

// HeadingParts are a scene heading's parts by the slot they are in
// (rules.Slots: "io", "place", "time", "n", "any"), in capitals, and
// whether the line is a heading (Heading). A heading by a scene word
// has the word as its io, the rest up to its last dash as its place and
// what follows the dash as its time (INT. KITCHEN - DAY); one by a
// pattern has the slots of the pattern that read it. A #12# at the end
// is its n where it has none of its own.
func (d *Dialect) HeadingParts(row string) (map[string]string, bool) {
	if !d.Heading(row) {
		return nil, false
	}
	upper := squash(strings.TrimPrefix(strings.TrimSpace(row), "."))
	out := map[string]string{}
	if m := sceneNumberAtEnd.FindString(upper); m != "" {
		upper = strings.TrimSpace(strings.TrimSuffix(upper, m))
		out[rules.SlotNumber] = strings.Trim(strings.TrimSpace(m), "#")
	}
	word := ""
	for _, w := range d.Lang.Scenes {
		if StartsScene(upper, w) && len(w) > len(word) {
			word = w
		}
	}
	if word != "" {
		w := strings.ToUpper(strings.TrimSuffix(word, "."))
		rest := strings.TrimPrefix(upper[len(w):], ".")
		out[rules.SlotIO] = strings.TrimSpace(upper[:len(upper)-len(rest)])
		rest = strings.TrimSpace(rest)
		cut, at := -1, ""
		for _, dash := range []string{" - ", " – ", " — ", " -- "} {
			if i := strings.LastIndex(rest, dash); i > cut {
				cut, at = i, dash
			}
		}
		if cut >= 0 {
			out[rules.SlotPlace], out[rules.SlotTime] = strings.TrimSpace(rest[:cut]), strings.TrimSpace(rest[cut+len(at):])
		} else if place, time := bracketedTime(rest, d.Lang.Times); time != "" {
			out[rules.SlotPlace], out[rules.SlotTime] = place, time // ○駅（朝）
		} else if rest != "" {
			out[rules.SlotPlace] = rest
		}
		return out, true
	}
	for _, re := range d.headings {
		m := re.FindStringSubmatch(upper)
		if m == nil {
			continue
		}
		for i, name := range re.SubexpNames() {
			if v := strings.TrimSpace(m[i]); name != "" && v != "" {
				out[name] = v
			}
		}
		return out, true
	}
	return out, true
}

// bracketedTime is a heading's rest split at a time word in brackets at
// its end ("駅（朝）": "駅", "朝"), "" for none.
func bracketedTime(rest string, times []string) (place, time string) {
	place, in := splitExtension(rest)
	for _, t := range times {
		if in != "" && squash(t) == in {
			return place, in
		}
	}
	return rest, ""
}

// Transition tells whether a line is a transition by the language's
// words, not forced: no small letters, and it ends in one of its
// TransitionEnds or is one of its Transitions (its TransitionMarks
// around it left out). Fountain's blank lines around it are the
// reader's to see.
func (d *Dialect) Transition(row string) bool {
	bare := strings.TrimSpace(row)
	if bare == "" || len(bare) > maxShort || bare != strings.ToUpper(bare) || strings.HasPrefix(bare, ">") {
		return false
	}
	for _, e := range d.Lang.TransitionEnds {
		if e != "" && strings.HasSuffix(bare, strings.ToUpper(e)) && strings.TrimSpace(bare) != strings.TrimSpace(e) {
			return true
		}
	}
	if len(d.trans) == 0 {
		return false
	}
	trimmed := strings.TrimFunc(bare, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(d.Lang.TransitionMarks, r)
	})
	return d.trans[squash(trimmed)] || d.trans[squash(trimmed+":")]
}

// CueName is a cue's name as cues are compared: its extension, dual
// dialogue's ^ and @ left out, in capitals, its spaces as one, Arabic
// without its vowel marks and with one alef (as Hekaya's registry
// compares names). "" for none.
func CueName(cue string) string {
	s := strings.TrimSpace(cue)
	s = strings.TrimPrefix(s, "@")
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "^"))
	name, _ := splitExtension(s)
	var b strings.Builder
	for _, r := range squash(name) {
		switch {
		case r >= 0x0610 && r <= 0x061A, r >= 0x064B && r <= 0x065F, r == 0x0670:
			continue // tashkeel
		case r == 'أ', r == 'إ', r == 'آ', r == 'ٱ':
			r = 'ا'
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// splitExtension is a cue's name and what is in the brackets at its end
// ("ANNA (V.O.)": "ANNA", "V.O."; "张三（画外音）": "张三", "画外音"), "" for
// none.
func splitExtension(s string) (string, string) {
	s = strings.TrimSpace(s)
	for _, br := range [][2]string{{"(", ")"}, {"（", "）"}, {"[", "]"}, {"【", "】"}} {
		if strings.HasSuffix(s, br[1]) {
			if i := strings.LastIndex(s, br[0]); i > 0 {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+len(br[0]) : len(s)-len(br[1])])
			}
		}
	}
	return s, ""
}

// shaped tells whether a line is shaped like a name by the shape rule.
func (d *Dialect) shaped(row string) bool {
	name, _ := splitExtension(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(row), "^")))
	if name == "" || strings.ContainsAny(name, d.Lang.Shape.Ends) || strings.ContainsAny(name[:1], ".!@~>#=[(/*") {
		return false
	}
	if n := d.Lang.Shape.Words; n > 0 && len(strings.Fields(name)) > n {
		return false
	}
	if n := d.Lang.Shape.Chars; n > 0 {
		c := 0
		for _, r := range name {
			if !unicode.IsSpace(r) {
				c++
			}
		}
		if c > n {
			return false
		}
	}
	return strings.IndexFunc(name, unicode.IsLetter) >= 0
}

// paren tells whether a line is a parenthetical: in brackets, round or
// full width.
func paren(row string) bool {
	t := strings.TrimSpace(row)
	return len(t) > 1 && (strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")") ||
		strings.HasPrefix(t, "（") && strings.HasSuffix(t, "）"))
}

// A Cast are the names a script's cues are read by: its character list
// (the names in its /* Characters */ block, ListedNames) and the names
// read as cues so far, for the list rule; for the repeat rule, how often
// each name passes shape in the whole script.
type Cast struct {
	listed map[string]bool
	shaped map[string]int
}

// CastOf is the cast of a script's text in a dialect.
func (d *Dialect) CastOf(text string) *Cast {
	return d.castOf(strings.Split(text, "\n"), ListedNames(text))
}

// CastOfLines is CastOf of a text as its lines: no rule that needs the
// cast, no work; the Characters block found from the end.
func (d *Dialect) CastOfLines(lines []string) *Cast {
	if !d.Lang.Has(rules.CueList) && !d.Lang.Has(rules.CueRepeat) {
		return nil
	}
	return d.castOf(lines, listedInLines(lines))
}

// listedInLines are ListedNames of a text as its lines: the last
// "/* Characters" block, read from the end (Accolade's end blocks).
func listedInLines(lines []string) []string {
	open := EndBlockMarks(EndCharacters).Open
	for i := len(lines) - 1; i >= 0; i-- {
		if lines[i] != open {
			continue
		}
		var out []string
		for _, line := range lines[i+1:] {
			if strings.HasPrefix(strings.TrimSpace(line), "*/") {
				break
			}
			if line == "" || line[0] == ' ' || line[0] == '\t' {
				continue
			}
			name, _, _ := strings.Cut(line, ":")
			if name = strings.TrimSpace(name); name != "" {
				out = append(out, name)
			}
		}
		return out
	}
	return nil
}

func (d *Dialect) castOf(rows []string, listed []string) *Cast {
	c := &Cast{listed: map[string]bool{}, shaped: map[string]int{}}
	for _, n := range listed {
		c.Add(n)
	}
	if d.Lang.Has(rules.CueRepeat) {
		for i := range rows[:bodyLines(rows)] {
			if k := d.shapedKey(rows, i); k != "" {
				c.shaped[k]++
			}
		}
	}
	return c
}

// Add puts a name in the cast's list (the parser: every name it reads
// as a cue).
func (c *Cast) Add(name string) {
	if c == nil {
		return
	}
	if k := CueName(name); k != "" {
		c.listed[k] = true
	}
}

// Listed tells whether a name is in the cast's list.
func (c *Cast) Listed(name string) bool { return c != nil && c.listed[CueName(name)] }

// Cue tells whether a line after a blank line, with next right under it
// (not blank), is a character's cue by the dialect's rules, with the
// script's cast (nil: none). The @ of a forced cue is the reader's to
// see, as are the blank lines.
func (d *Dialect) Cue(row, next string, c *Cast) bool {
	t := strings.TrimSpace(row)
	if t == "" || len(t) > maxShort || strings.TrimSpace(next) == "" {
		return false
	}
	bare := strings.TrimSpace(strings.TrimSuffix(t, "^"))
	name, ext := splitExtension(bare)
	if name == "" || strings.ContainsAny(name[:1], ".!~>#=[(/*") {
		return false
	}
	l := d.Lang
	if l.Has(rules.CueCaps) && AllCaps(strings.Split(bare, "(")[0]) {
		return true
	}
	if l.Has(rules.CueList) && c.Listed(name) {
		return true
	}
	if l.Has(rules.CueExtension) && ext != "" {
		for _, part := range strings.FieldsFunc(ext, func(r rune) bool { return strings.ContainsRune(",;/、，；", r) }) {
			if d.ext[squash(part)] {
				return true
			}
		}
	}
	if l.Has(rules.CueParenthetical) && paren(next) && d.shaped(t) {
		return true
	}
	if l.Has(rules.CueShape) && d.shaped(t) {
		return true
	}
	if l.Has(rules.CueRepeat) && c != nil && l.Repeat > 0 && d.shaped(t) && c.shaped[CueName(t)] >= l.Repeat {
		return true
	}
	return false
}

// TitleKey is a title page key as the standard key it is in the
// language (العنوان: Title), or as written.
func (d *Dialect) TitleKey(key string) string {
	k := strings.TrimSpace(key)
	for from, to := range d.Lang.TitleKeys {
		if strings.EqualFold(squash(from), squash(k)) {
			return to
		}
	}
	return k
}

// ListedNames are the names in a script's character list: its
// /* Characters */ end block, each entry's name (a line at the margin,
// up to its colon), as written.
func ListedNames(text string) []string {
	inside, found := "", false
	blocks, _ := EndBlocks(text)
	for _, b := range blocks {
		if b.Name == EndCharacters {
			inside, found = EndBlockMarks(EndCharacters).Unescape(b.Inside), true
		}
	}
	if !found {
		return nil
	}
	var out []string
	for _, line := range strings.Split(inside, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		name, _, _ := strings.Cut(line, ":")
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// A CastCache keeps a text's Cast between edits (an editor's, per key):
// the names each line gives the repeat rule, their counts, and the
// character list, updated from the lines that changed (and their
// neighbours) rather than read again over the whole text.
type CastCache struct {
	mu     sync.Mutex
	d      *Dialect
	lines  []string
	keys   []string // per line: the name it passes shape with, or ""
	body   int      // the lines before the first end block
	counts map[string]int
	listed []string
}

// NewCastCache is an empty cache for texts in a dialect.
func (d *Dialect) NewCastCache() *CastCache { return &CastCache{d: d} }

// Dialect is the dialect the cache reads with.
func (c *CastCache) Dialect() *Dialect { return c.d }

// Cast is the cast of a text as its lines: CastOfLines, kept up to date
// from the last text's.
func (c *CastCache) Cast(lines []string) *Cast {
	d := c.d
	if !d.Lang.Has(rules.CueList) && !d.Lang.Has(rules.CueRepeat) {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	body := bodyLines(lines)
	p, n, o := 0, len(lines), len(c.lines)
	for p < n && p < o && lines[p] == c.lines[p] {
		p++
	}
	s := 0
	for s < n-p && s < o-p && lines[n-1-s] == c.lines[o-1-s] {
		s++
	}
	full := c.keys == nil || body != c.body+(n-o) && body != c.body || o == 0
	repeat := d.Lang.Has(rules.CueRepeat)
	if full {
		c.keys = make([]string, n)
		c.counts = map[string]int{}
		if repeat {
			for i := range lines {
				c.keys[i] = d.shapedKey(lines, i)
				if i < body && c.keys[i] != "" {
					c.counts[c.keys[i]]++
				}
			}
		}
		c.listed = listedInLines(lines)
	} else {
		// the changed lines and one on each side (a blank line around
		// a name is part of its shape)
		from, oldTo, newTo := max(p-1, 0), min(o-s+1, o), min(n-s+1, n)
		if repeat {
			for i := from; i < oldTo; i++ {
				if i < c.body && c.keys[i] != "" {
					c.counts[c.keys[i]]--
				}
			}
		}
		keys := make([]string, 0, n)
		keys = append(keys, c.keys[:from]...)
		for i := from; i < newTo; i++ {
			k := ""
			if repeat {
				k = d.shapedKey(lines, i)
			}
			keys = append(keys, k)
			if i < body && k != "" {
				c.counts[k]++
			}
		}
		keys = append(keys, c.keys[oldTo:]...)
		c.keys = keys
		if newTo >= body-1 || oldTo >= c.body-1 { // the end blocks changed
			c.listed = listedInLines(lines)
		}
	}
	c.lines, c.body = slices.Clone(lines), body
	out := &Cast{listed: map[string]bool{}, shaped: map[string]int{}}
	for _, name := range c.listed {
		out.Add(name)
	}
	for k, v := range c.counts {
		if v > 0 {
			out.shaped[k] = v
		}
	}
	return out
}

// bodyLines is how many of a text's lines come before its first end block.
func bodyLines(lines []string) int {
	for i, r := range lines {
		if strings.HasPrefix(r, "/* ") && slices.Contains(append(EndBlockNames, EndNotes), strings.TrimPrefix(r, "/* ")) {
			return i
		}
	}
	return len(lines)
}

// shapedKey is the name line i passes shape with in its place (a blank
// line before it, text after it), "" for none.
func (d *Dialect) shapedKey(rows []string, i int) string {
	t := strings.TrimSpace(lex.StripNotes(rows[i]))
	if t == "" || len(t) > maxShort || i > 0 && strings.TrimSpace(rows[i-1]) != "" || i+1 >= len(rows) || strings.TrimSpace(rows[i+1]) == "" {
		return ""
	}
	if d.shaped(t) {
		return CueName(t)
	}
	return ""
}
