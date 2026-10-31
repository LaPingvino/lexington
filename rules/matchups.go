package rules

import "strings"

// Roles are what a scene heading's first word says about the place, in
// the order of a language's Matchups.
const (
	Inside = iota
	Outside
	Both     // INT./EXT.
	BothBack // EXT./INT.
	Establishing
	Roles
)

// norm is a heading word to compare: upper case, without dots and spaces.
func norm(s string) string {
	return strings.ToUpper(strings.NewReplacer(".", "", " ", "").Replace(s))
}

// Matchup are a language's words by role: its Matchups, or for a language
// without them its Scenes' first word with a slash split in two (INT./EXT.
// into INT. and EXT.), as the statistics read them.
func (c TOMLConf) Matchup(lang string) []string {
	if m := c.Matchups[lang]; len(m) > 0 {
		out := make([]string, Roles)
		copy(out, m)
		return out
	}
	out := make([]string, Roles)
	dot := ""
	for _, s := range c.Scenes[lang] {
		if strings.Contains(s, ".") {
			dot = "."
		}
	}
	for _, s := range c.Scenes[lang] {
		if i, o, ok := strings.Cut(s, "/"); ok {
			i, o = strings.Trim(i, ". "), strings.Trim(o, ". ")
			out[Inside], out[Outside] = i+dot, o+dot
			out[Both], out[BothBack] = i+dot+"/"+o+dot, o+dot+"/"+i+dot
			break
		}
	}
	return out
}

// RoleOf is the role of a scene heading's first word in lang (INT., I/E,
// BI), or false if it has none there: the word itself first, then words
// it shortens or that shorten it (I/E for INT./EXT.).
func (c TOMLConf) RoleOf(lang, word string) (int, bool) {
	m, w := c.Matchup(lang), norm(word)
	if w == "" {
		return 0, false
	}
	for r, x := range m {
		if x != "" && norm(x) == w {
			return r, true
		}
	}
	like := func(a, b string) bool {
		return a != "" && b != "" && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a))
	}
	if a, b, ok := strings.Cut(w, "/"); ok {
		in, out := norm(m[Inside]), norm(m[Outside])
		switch {
		case like(a, in) && like(b, out):
			return Both, true
		case like(a, out) && like(b, in):
			return BothBack, true
		}
		return 0, false
	}
	for _, r := range []int{Inside, Outside} {
		if like(w, norm(m[r])) {
			return r, true
		}
	}
	return 0, false
}

// Translate is a scene heading word of language from in language to
// (INT. to BIN.), or false when to has no word for its role.
func (c TOMLConf) Translate(word, from, to string) (string, bool) {
	r, ok := c.RoleOf(from, word)
	if !ok {
		return "", false
	}
	t := c.Matchup(to)[r]
	return t, t != ""
}
