package fountain

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// A heading's own number. A language's heading pattern may have a scene
// number in it ({n}: "场景{n} {place} {time} {io}", "{n}. {io} {any}"),
// so the number is part of the heading's words ("场景12 厨房 日 内",
// "12. פנים. משרד – יום"), as those scripts print it. Fountain's own
// number (#12# at the heading's end) is the other place a scene's number
// can be; a heading with both keeps them equal (WithHeadingNumber writes
// both). HeadingNumber reads the number in the words; WithHeadingNumber
// writes it there: in place of the one there is, put in where the
// heading is written without one (by the pattern it is that pattern
// without its number), or left out with the text that goes with it
// (场景, "12. "), every other character of the line as it was typed.
//
// A number is read in the digits it is written in (12, ١٢, ۱۲, 十二) and
// given as Western digits ("12"); one written in another's digits is
// written in the same digits again (١٢ → ١٣, 十二 → 十三).

// numberForm is a form a heading may be written in by one of the
// language's patterns with a number: the pattern itself, or it with
// slots left out (rules.PatternVariants), the number among them or not.
type numberForm struct {
	pattern []rules.PatternPart // the language's pattern
	form    []rules.PatternPart // this form of it
	re      *regexp.Regexp      // the form over a squashed line (squash)
	hasN    bool                // the form has the number
}

// numberForms are the forms of the language's patterns with a number,
// the one it writes a new heading by first (HeadingForm), each pattern's
// forms in their order; the pattern without its number is one even where
// it is no heading by itself, as a heading by a scene word written so
// ("פנים. משרד – יום") takes a number by it ("12. פנים. משרד – יום").
func numberForms(l rules.Language) []numberForm {
	pats := slices.Clone(l.Headings)
	if f := l.HeadingForm(); slices.Contains(pats, f) {
		pats = append([]string{f}, slices.DeleteFunc(pats, func(p string) bool { return p == f })...)
	}
	var out []numberForm
	for _, p := range pats {
		slots := rules.PatternSlots(p)
		if !slices.Contains(slots, rules.SlotNumber) {
			continue
		}
		forms := rules.PatternVariants(p)
		without := map[string]string{}
		for _, s := range slots {
			if s != rules.SlotNumber {
				without[s] = "{" + s + "}"
			}
		}
		if f := rules.FillPattern(p, without); !slices.Contains(forms, f) {
			forms = append(forms, f)
		}
		for _, f := range forms {
			re, err := regexp.Compile(`^(?:` + headingBody(f, l) + `)\s*$`)
			if err != nil {
				continue
			}
			out = append(out, numberForm{pattern: rules.ParsePattern(p), form: rules.ParsePattern(f), re: re,
				hasN: slices.Contains(rules.PatternSlots(f), rules.SlotNumber)})
		}
	}
	return out
}

// rowEnd is what may follow a heading's words: Fountain's number and
// notes, in either order.
var rowEnd = regexp.MustCompile(`(?:\s*(?:\[\[.*?\]\]|#[^#\s]+#))+\s*$`)

// splitRow takes a heading line apart: what comes before its words
// (spaces, the dot that forces a heading), its words, what follows them
// (#12#, [[notes]]).
func splitRow(row string) (lead, body, tail string) {
	body = row
	if m := rowEnd.FindStringIndex(body); m != nil {
		body, tail = body[:m[0]], body[m[0]:]
	}
	trimmed := strings.TrimLeftFunc(body, unicode.IsSpace)
	if strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, "..") {
		trimmed = trimmed[1:]
	}
	lead, body = body[:len(body)-len(trimmed)], trimmed
	trimmed = strings.TrimRightFunc(body, unicode.IsSpace)
	return lead, trimmed, body[len(trimmed):] + tail
}

// squashed is squash(s), and where each of its bytes came from in s:
// at[k] is the byte of s its k-th byte was written for (a space for the
// run of spaces it stands for), at[len] the end of s's last word.
func squashed(s string) (string, []int) {
	var b strings.Builder
	var at []int
	space, end := -1, 0
	for i, r := range s {
		if unicode.IsSpace(r) {
			if space < 0 && b.Len() > 0 {
				space = i
			}
			continue
		}
		if space >= 0 {
			b.WriteByte(' ')
			at = append(at, space)
			space = -1
		}
		u := string(unicode.ToUpper(r))
		b.WriteString(u)
		for range len(u) {
			at = append(at, i)
		}
		end = i + len(string(r))
	}
	return b.String(), append(at, end)
}

// reading is a heading's words read by a form: the slots' spans in the
// words (bytes), in the form's order.
type reading struct {
	f     numberForm
	spans [][2]int
}

// readNumbered is the first form reading body that has a number (n) or
// not, ok false for none.
func (d *Dialect) readNumbered(body string, n bool) (reading, bool) {
	d.formsOnce.Do(func() { d.forms = numberForms(d.Lang) })
	up, at := squashed(body)
	for _, f := range d.forms {
		if f.hasN != n {
			continue
		}
		m := f.re.FindStringSubmatchIndex(up)
		if m == nil {
			continue
		}
		r := reading{f: f}
		for i, name := range f.re.SubexpNames() {
			if name == "" {
				continue
			}
			s, e := m[2*i], m[2*i+1]
			if s < 0 {
				s, e = 0, 0
			}
			r.spans = append(r.spans, [2]int{at[s], at[e]})
		}
		return r, true
	}
	return reading{}, false
}

// own are the form's parts with the heading's own text between its
// slots, and the slots' values as written.
func (r reading) own(body string) ([]rules.PatternPart, map[string]string) {
	parts := slices.Clone(r.f.form)
	values := map[string]string{}
	k, from := 0, 0
	for i, pt := range parts {
		if pt.Slot == "" {
			continue
		}
		if k >= len(r.spans) {
			break
		}
		sp := r.spans[k]
		if i > 0 && parts[i-1].Slot == "" {
			parts[i-1].Text = body[from:sp[0]]
		}
		values[pt.Slot] = body[sp[0]:sp[1]]
		from = sp[1]
		k++
	}
	if n := len(parts); n > 0 && parts[n-1].Slot == "" {
		parts[n-1].Text = body[from:]
	}
	return parts, values
}

// HeadingNumber is the number in a heading's words (Western digits:
// "12" for ١٢ or 十二), and whether its words have one: by a pattern with
// {n}, not Fountain's #12# at its end (lex.SceneNumber reads that).
func (d *Dialect) HeadingNumber(row string) (string, bool) {
	if !d.Heading(row) {
		return "", false
	}
	_, body, _ := splitRow(row)
	r, ok := d.readNumbered(body, true)
	if !ok {
		return "", false
	}
	_, values := r.own(body)
	return westernNumber(values[rules.SlotNumber]), true
}

// TakesNumber tells whether a heading's words have a number or can have
// one: a pattern of the language's with {n} writes it so.
func (d *Dialect) TakesNumber(row string) bool {
	if !d.Heading(row) {
		return false
	}
	_, body, _ := splitRow(row)
	if _, ok := d.readNumbered(body, true); ok {
		return true
	}
	_, ok := d.readNumbered(body, false)
	return ok
}

// WithHeadingNumber is a heading with the number n in its words: in
// place of the one there ("" leaves it out, with its label: 场景12 厨房
// 日 内 is 厨房 日 内), else put in by the pattern its words are written
// by without one, the rest of the line as it was; a Fountain #12# at
// its end becomes #n# too (taken out for ""). ok is false, and the line
// is as it was, where the words cannot have a number (no pattern with
// {n} writes them) or have none to leave out.
func (d *Dialect) WithHeadingNumber(row, n string) (string, bool) {
	if !d.Heading(row) {
		return row, false
	}
	lead, body, tail := splitRow(row)
	n = strings.TrimSpace(n)
	finish := func(body string) (string, bool) {
		out := lead + body + tail
		if _, had := lex.SceneNumber(out); had != "" {
			out = lex.WithSceneNumber(out, n)
		}
		return out, true
	}
	if r, ok := d.readNumbered(body, true); ok {
		parts, values := r.own(body)
		if n != "" {
			k := slices.IndexFunc(parts, func(p rules.PatternPart) bool { return p.Slot == rules.SlotNumber })
			sp := r.spans[slotIndex(parts, k)]
			return finish(body[:sp[0]] + inDigitsOf(values[rules.SlotNumber], n) + body[sp[1]:])
		}
		delete(values, rules.SlotNumber)
		return finish(rules.FillParts(parts, values))
	}
	if n == "" {
		return row, false
	}
	r, ok := d.readNumbered(body, false)
	if !ok {
		return row, false
	}
	formParts, values := r.own(body)
	// the pattern's parts, with the heading's own text where its form
	// has the same text part (FillKept: the parts the form kept)
	placeholders := map[string]string{}
	for k := range values {
		placeholders[k] = "{" + k + "}"
	}
	kept := rules.FillKept(r.f.pattern, placeholders)
	parts := slices.Clone(r.f.pattern)
	var in []int // the pattern's parts the form has, in order
	for i := range parts {
		if kept[i] {
			in = append(in, i)
		}
	}
	j := 0 // the form's part
	for k, i := range in {
		if j >= len(formParts) {
			break
		}
		if parts[i].Slot != "" {
			j++
			continue
		}
		if k > 0 && parts[in[k-1]].Slot == "" { // two texts the form has as one: the first has it
			parts[i].Text = ""
		} else if formParts[j].Slot == "" {
			parts[i].Text = formParts[j].Text
		}
		if k+1 == len(in) || parts[in[k+1]].Slot != "" {
			j++
		}
	}
	values[rules.SlotNumber] = n
	return finish(rules.FillParts(parts, values))
}

// slotIndex is which slot part k is among parts' slots.
func slotIndex(parts []rules.PatternPart, k int) int {
	n := 0
	for _, p := range parts[:k] {
		if p.Slot != "" {
			n++
		}
	}
	return n
}

// The digits a number may be written in besides Western ones.
const (
	arabicIndic = "٠١٢٣٤٥٦٧٨٩"
	persian     = "۰۱۲۳۴۵۶۷۸۹"
	chinese     = "〇一二三四五六七八九"
)

// westernNumber is a number with Western digits: ١٢ and 十二 are 12, a
// letter beside it kept (十二A is 12A).
func westernNumber(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case strings.ContainsRune(arabicIndic, r):
			b.WriteByte(byte('0' + slices.Index([]rune(arabicIndic), r)))
		case strings.ContainsRune(persian, r):
			b.WriteByte(byte('0' + slices.Index([]rune(persian), r)))
		case isChinese(r):
			j := i
			for j < len(rs) && isChinese(rs[j]) {
				j++
			}
			if v, ok := chineseValue(string(rs[i:j])); ok {
				b.WriteString(strconv.Itoa(v))
			} else {
				b.WriteString(string(rs[i:j]))
			}
			i = j - 1
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isChinese(r rune) bool { return strings.ContainsRune(chinese+"零十百", r) }

// chineseValue is a number in Chinese numerals: 十二, 一百零五, or digit
// by digit (一〇五).
func chineseValue(s string) (int, bool) {
	digit := func(r rune) int {
		if r == '零' {
			return 0
		}
		return slices.Index([]rune(chinese), r)
	}
	if !strings.ContainsAny(s, "十百") {
		v := 0
		for _, r := range s {
			v = v*10 + digit(r)
		}
		return v, s != ""
	}
	total, cur := 0, -1
	for _, r := range s {
		switch r {
		case '百', '十':
			unit := 100
			if r == '十' {
				unit = 10
			}
			if cur < 0 {
				cur = 1 // 十二: one ten
			}
			total += cur * unit
			cur = -1
		default:
			cur = digit(r)
		}
	}
	if cur > 0 {
		total += cur
	}
	return total, true
}

// chineseNumber is v in Chinese numerals as a heading writes them: 十二,
// 二十, 一百零五; Western digits from 1000 on.
func chineseNumber(v int) string {
	d := []rune(chinese)
	switch {
	case v < 10:
		return string(d[v])
	case v < 20:
		return "十" + map[bool]string{true: "", false: string(d[v%10])}[v%10 == 0]
	case v < 100:
		s := string(d[v/10]) + "十"
		if v%10 != 0 {
			s += string(d[v%10])
		}
		return s
	case v < 1000:
		s := string(d[v/100]) + "百"
		switch r := v % 100; {
		case r == 0:
		case r < 10:
			s += "零" + string(d[r])
		default:
			s += string(d[r/10]) + "十"
			if r%10 != 0 {
				s += string(d[r%10])
			}
		}
		return s
	}
	return strconv.Itoa(v)
}

// inDigitsOf is the number n written in the digits old is written in.
func inDigitsOf(old, n string) string {
	system := ""
	for _, r := range old {
		switch {
		case strings.ContainsRune(arabicIndic, r):
			system = arabicIndic
		case strings.ContainsRune(persian, r):
			system = persian
		case isChinese(r):
			system = chinese
		}
	}
	if system == "" {
		return n
	}
	var b strings.Builder
	rs := []rune(n)
	for i := 0; i < len(rs); i++ {
		if rs[i] < '0' || rs[i] > '9' {
			b.WriteRune(rs[i])
			continue
		}
		j := i
		for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
			j++
		}
		if system == chinese {
			v, _ := strconv.Atoi(string(rs[i:j]))
			b.WriteString(chineseNumber(v))
		} else {
			for _, r := range rs[i:j] {
				b.WriteRune([]rune(system)[r-'0'])
			}
		}
		i = j - 1
	}
	return b.String()
}
