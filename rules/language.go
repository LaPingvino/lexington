package rules

import (
	"maps"
	"slices"
	"strings"
	"sync"
)

// A Language is how scripts in one language are written in Fountain:
// what its scene headings start with, what makes a line a character's
// cue or a transition, its title page's keys, the way its text runs.
// They are named rules chosen in the configuration, per language; the
// parser (the fountain package) has no code for any one language.
// Every language in the configuration carries its whole definition
// (DefaultConf, Dump): a field a configuration file leaves out is the
// default configuration's for that language (English's for a language
// the default has none of), so a file written before a field existed
// reads as it did.
//
//	[Languages.zh]
//	Scenes = ["内景", "外景", "内景/外景", "外景/内景"]
//	Cues = ["forced", "list", "parenthetical", "extension", "repeat"]
//	Extensions = ["画外音", "画外", "OS", "VO"]
//	Shape = {Words = 0, Chars = 6, Ends = "。！？…，"}
//	Repeat = 2
//	TransitionEnds = []
//	Transitions = ["切至：", "淡出", "淡入"]
//
// The cue rules (Cues), each a way a line after a blank line, with text
// right under it, is read as a character's cue:
//
//	forced         @ before the name (always, named or not)
//	caps           the name in capitals: it has capital letters and no
//	               small ones (Fountain's; scripts without capitals,
//	               Hebrew, Arabic, Chinese, never are)
//	list           the name is in the script's character list: the
//	               names in its /* Characters */ block, and every name
//	               read as a cue before (Hekaya's Character Registry:
//	               the first @سمير makes a later سمير a cue)
//	parenthetical  a parenthetical follows it directly ("(quietly)")
//	extension      it ends in one of the language's Extensions in
//	               brackets: "张三（画外音）", "ANNA (V.O.)"
//	shape          it is shaped like a name: at most Shape.Words words
//	               and Shape.Chars characters (0: any number), none of
//	               the language's sentence ends (Shape.Ends)
//	repeat         it passes shape at least Repeat times in the script
//
// A name is compared without its extension and dual dialogue's ^, in
// capitals, its spaces as one, Arabic without its vowel marks and with
// one alef (أ إ آ ٱ as ا), as Hekaya's registry compares them.
//
// Transitions: a line with blank lines around it, with no small letters,
// that ends in one of TransitionEnds (" TO:") or is one of Transitions
// ("FADE OUT.", "قطع"), the TransitionMarks ("-" for "- قطع -", ":")
// around it left out. > forces one anywhere.
type Language struct {
	// Scenes are the words a scene heading starts with ("INT", "EXT"),
	// followed by a dot or a space (or anything, for a word that ends in
	// a sign: ○).
	Scenes []string `toml:"Scenes"`
	// Headings are scene heading patterns for headings that do not start
	// with a scene word ("场景{n} {place} {time} {io}", "{n}. {place}
	// {io}/{time}"): their slots {n} a scene number, {io} one of Settings
	// or Scenes, {time} one of Times, {place} and {any} any text; a space
	// is any spaces, a dash any dash with or without spaces around it, a
	// slash with or without them. Read after a blank line, as a scene
	// word is.
	Headings []string `toml:"Headings"`
	// Settings are its words for inside and outside within a heading
	// (内, INNEN, INT), Times for the time of day (日, TAG, JOUR).
	Settings []string `toml:"Settings"`
	Times    []string `toml:"Times"`
	// Matchup are its scene heading words by role (Roles: inside,
	// outside, both, both the other way, establishing), "" for one it
	// has none for: what translates to what (matchups.go).
	Matchup []string `toml:"Matchup"`
	// Direction is the way its text runs, "rtl" or "ltr": what a line
	// with no direction of its own takes (lex.OwnDirection).
	Direction string `toml:"Direction"`
	// Cues are the cue rules, by name (see above).
	Cues []string `toml:"Cues"`
	// Extensions are the language's words for a cue's extensions
	// (V.O., O.S., CONT'D), as the extension rule reads them.
	Extensions []string `toml:"Extensions"`
	// Shape is what the shape rule takes for a name.
	Shape Shape `toml:"Shape"`
	// Repeat is how often a name must pass shape for the repeat rule.
	Repeat int `toml:"Repeat"`
	// TransitionEnds end a transition (" TO:"); Transitions are
	// transitions as a whole line; TransitionMarks may stand around one
	// of those ("- قطع -").
	TransitionEnds  []string `toml:"TransitionEnds"`
	Transitions     []string `toml:"Transitions"`
	TransitionMarks string   `toml:"TransitionMarks"`
	// TitleKeys are the language's title page keys and the standard key
	// each one is (العنوان is Title, سيناريو Author). The standard keys
	// themselves (Title, Credit, Author, Source, Draft date, Contact,
	// Direction…) are known in every language.
	TitleKeys map[string]string `toml:"TitleKeys"`
}

// Shape is what a name looks like to the shape rule.
type Shape struct {
	Words int    `toml:"Words"` // at most this many words (0: any)
	Chars int    `toml:"Chars"` // at most this many characters, spaces not counted (0: any)
	Ends  string `toml:"Ends"`  // none of these characters (sentence ends)
}

// The cue rules' names.
const (
	CueForced        = "forced"
	CueCaps          = "caps"
	CueList          = "list"
	CueParenthetical = "parenthetical"
	CueExtension     = "extension"
	CueShape         = "shape"
	CueRepeat        = "repeat"
)

// CueRules are the cue rules' names, in the order they are described.
var CueRules = []string{CueForced, CueCaps, CueList, CueParenthetical, CueExtension, CueShape, CueRepeat}

// Has tells whether the language reads cues by the rule named.
func (l Language) Has(rule string) bool {
	return rule == CueForced || slices.Contains(l.Cues, rule)
}

// RTL tells whether the language's text runs right to left.
func (l Language) RTL() bool { return strings.EqualFold(strings.TrimSpace(l.Direction), "rtl") }

// clone is a copy of l that shares nothing with it.
func (l Language) clone() Language {
	l.Scenes = slices.Clone(l.Scenes)
	l.Headings = slices.Clone(l.Headings)
	l.Settings = slices.Clone(l.Settings)
	l.Times = slices.Clone(l.Times)
	l.Matchup = slices.Clone(l.Matchup)
	l.Cues = slices.Clone(l.Cues)
	l.Extensions = slices.Clone(l.Extensions)
	l.TransitionEnds = slices.Clone(l.TransitionEnds)
	l.Transitions = slices.Clone(l.Transitions)
	l.TitleKeys = maps.Clone(l.TitleKeys)
	return l
}

// over is l with the fields set in o (not nil, not zero) taking the
// place of l's.
func (l Language) over(o Language) Language {
	l = l.clone()
	if o.Scenes != nil {
		l.Scenes = slices.Clone(o.Scenes)
	}
	if o.Headings != nil {
		l.Headings = slices.Clone(o.Headings)
	}
	if o.Settings != nil {
		l.Settings = slices.Clone(o.Settings)
	}
	if o.Times != nil {
		l.Times = slices.Clone(o.Times)
	}
	if o.Matchup != nil {
		l.Matchup = slices.Clone(o.Matchup)
	}
	if o.Direction != "" {
		l.Direction = o.Direction
	}
	if o.Cues != nil {
		l.Cues = slices.Clone(o.Cues)
	}
	if o.Extensions != nil {
		l.Extensions = slices.Clone(o.Extensions)
	}
	if o.Shape != (Shape{}) {
		l.Shape = o.Shape
	}
	if o.Repeat != 0 {
		l.Repeat = o.Repeat
	}
	if o.TransitionEnds != nil {
		l.TransitionEnds = slices.Clone(o.TransitionEnds)
	}
	if o.Transitions != nil {
		l.Transitions = slices.Clone(o.Transitions)
	}
	if o.TransitionMarks != "" {
		l.TransitionMarks = o.TransitionMarks
	}
	if o.TitleKeys != nil {
		l.TitleKeys = maps.Clone(o.TitleKeys)
	}
	return l
}

// Words is a language read as Fountain reads English, with other scene
// heading words: cues in capitals, transitions ending in " TO:". It is
// what the parser's functions that take only scene words read with.
func Words(scenes []string) Language {
	l := english()
	l.Scenes = slices.Clone(scenes)
	l.Matchup = nil
	return l
}

// Hekaya is the fixed language of Hekaya, Fountain in Arabic
// (github.com/michaelkmalak/hekaya, spec v0.1): what a .hekaya file is
// read with, whatever the configuration says of Arabic (which starts as
// the same, but is the writer's to change). Where Hekaya's spec is open,
// Lexington's choices are in docs/LANGUAGES.md.
const Hekaya = "hekaya"

// DefaultExtensions are the file extensions the default configuration
// knows, each with its language: Hekaya's own .hekaya.
func DefaultExtensions() map[string]string { return map[string]string{"hekaya": Hekaya} }

// FileExtensions are the extensions c knows (the default's, c's over
// them), without their dot, each with its language.
func (c TOMLConf) FileExtensions() map[string]string {
	out := DefaultExtensions()
	for k, v := range c.Extensions {
		out[strings.ToLower(strings.TrimPrefix(k, "."))] = v
	}
	return out
}

// ExtensionLanguage is the language of a file named name by its
// extension ("film.hekaya": hekaya), "" and false for an extension no
// language has (.fountain, .txt: the name's language part says, if any).
func (c TOMLConf) ExtensionLanguage(name string) (string, bool) {
	i := strings.LastIndex(name, ".")
	if i < 0 || strings.ContainsAny(name[i:], `/\`) {
		return "", false
	}
	l, ok := c.FileExtensions()[strings.ToLower(name[i+1:])]
	return l, ok && l != ""
}

// ExtensionOf is the extension (with its dot) c gives a file in lang of
// its own, "" for none: ".hekaya" for Hekaya.
func (c TOMLConf) ExtensionOf(lang string) string {
	var exts []string
	for e, l := range c.FileExtensions() {
		if l == lang {
			exts = append(exts, e)
		}
	}
	if len(exts) == 0 {
		return ""
	}
	slices.Sort(exts)
	return "." + exts[0]
}

// Language is lang as c reads it, in full: c's definition over the
// default configuration's (English's rules, with no scene words, for a
// language the default does not have). Hekaya is always Hekaya's own.
func (c TOMLConf) Language(lang string) Language {
	if lang == Hekaya {
		return hekaya()
	}
	if k, ok := c.lookup(lang, true); ok {
		lang = k // hi-Deva-IN as hi-Deva, de-CH as de
	}
	base, ok := defaultLanguage(lang)
	if !ok {
		base = custom()
	}
	if l, ok := c.Languages[lang]; ok {
		return base.over(l)
	}
	if lang == "" {
		if l, ok := c.Languages[c.StandardLanguage()]; ok {
			return english().over(l)
		}
	}
	return base
}

// Knows tells whether c has a language: one with scene words of its
// own (Hekaya always), lang's tag or one it falls back to (Lookup).
func (c TOMLConf) Knows(lang string) bool {
	if lang == Hekaya {
		return true
	}
	_, ok := c.lookup(lang, false)
	return ok
}

// Lookup is the language of c (or of the default configuration) that a
// BCP 47 tag reads as: the tag itself, in any case and with _ for -
// (hi-deva, hi_Deva: hi-Deva), else the tag without its last subtag, and
// so on (hi-Deva-IN, hi-Deva, hi; de-CH, de). False for none.
func (c TOMLConf) Lookup(lang string) (string, bool) {
	if lang == Hekaya {
		return Hekaya, true
	}
	return c.lookup(lang, true)
}

// lookup is Lookup in c's languages, and the default's too if defaults.
func (c TOMLConf) lookup(lang string, withDefaults bool) (string, bool) {
	if _, ok := c.Languages[lang]; ok {
		return lang, true // asked often: the exact key first
	}
	for _, t := range fallbacks(lang) {
		if _, ok := c.Languages[t]; ok {
			return t, true
		}
		for _, k := range slices.Sorted(maps.Keys(c.Languages)) {
			if strings.EqualFold(k, t) {
				return k, true // [Languages.hi-deva] written so
			}
		}
		if withDefaults {
			if _, ok := defaults()[t]; ok {
				return t, true
			}
		}
	}
	return "", false
}

// fallbacks are the tags a language tag reads as, in order: itself as
// BCP 47 writes it, then without its last subtag, down to the language
// (hi-Deva-IN, hi-Deva, hi). A name that is no tag ("my lang") is only
// itself.
func fallbacks(lang string) []string {
	t := Tag(lang)
	if t == "" {
		if lang == "" {
			return nil
		}
		return []string{lang}
	}
	var out []string
	for {
		out = append(out, t)
		i := strings.LastIndex(t, "-")
		if i < 0 {
			return out
		}
		t = t[:i]
	}
}

// Tag is a language tag as BCP 47 writes it: the language in small
// letters, a script (four letters) with a capital, a region (two
// letters) in capitals, the rest in small letters, joined by -
// ("HI_deva_in": "hi-Deva-IN"). "" for what is no tag: empty, a subtag
// empty or longer than eight, or of other than letters and digits, or a
// language that is not letters.
func Tag(lang string) string {
	parts := strings.FieldsFunc(lang, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 || strings.Count(lang, "-")+strings.Count(lang, "_") != len(parts)-1 {
		return ""
	}
	for i, p := range parts {
		if len(p) > 8 {
			return ""
		}
		letters := true
		for _, r := range p {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			case r >= '0' && r <= '9':
				letters = false
			default:
				return ""
			}
		}
		switch {
		case i == 0:
			if !letters {
				return ""
			}
			parts[i] = strings.ToLower(p)
		case letters && len(p) == 4:
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		case letters && len(p) == 2:
			parts[i] = strings.ToUpper(p)
		default:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "-")
}

// LanguageNames are the languages c has, in order (Hekaya, fixed, not
// among them).
func (c TOMLConf) LanguageNames() []string {
	return slices.Sorted(maps.Keys(c.Languages))
}

// Scenes is lang's scene heading words (Language(lang).Scenes), nil for
// a language c does not have (by Lookup's fallback: hi-Deva-IN is
// hi-Deva's).
func (c TOMLConf) Scenes(lang string) []string {
	if lang == Hekaya {
		return hekaya().Scenes
	}
	k, ok := c.lookup(lang, false)
	if !ok {
		return nil
	}
	if l := c.Languages[k]; l.Scenes != nil {
		return l.Scenes // asked often (per key): the words alone, not the whole language
	}
	return c.Language(k).Scenes
}

// normalized is c with the tables of configuration files from before
// Languages ([Scenes], [Matchups], [Directions]) moved into Languages,
// each language in full.
func (c TOMLConf) normalized() TOMLConf {
	if c.LegacyScenes == nil && c.LegacyMatchups == nil && c.LegacyDirections == nil {
		return c
	}
	out := c
	out.Languages = maps.Clone(c.Languages)
	if out.Languages == nil {
		out.Languages = map[string]Language{}
	}
	names := slices.Concat(slices.Collect(maps.Keys(c.LegacyScenes)), slices.Collect(maps.Keys(c.LegacyMatchups)),
		slices.Collect(maps.Keys(c.LegacyDirections)))
	for _, n := range names {
		l, ok := out.Languages[n]
		if !ok {
			l = TOMLConf{}.Language(n)
		}
		if w, ok := c.LegacyScenes[n]; ok {
			l.Scenes = slices.Clone(w)
		}
		if m, ok := c.LegacyMatchups[n]; ok {
			l.Matchup = slices.Clone(m)
		}
		if d, ok := c.LegacyDirections[n]; ok {
			l.Direction = d
		}
		out.Languages[n] = l
	}
	out.LegacyScenes, out.LegacyMatchups, out.LegacyDirections = nil, nil, nil
	return out
}

// defaults are the default languages, made once (defaultLanguage).
var defaults = sync.OnceValue(defaultLanguages)

// defaultLanguage is the default configuration's lang, a copy.
func defaultLanguage(lang string) (Language, bool) {
	l, ok := defaults()[lang]
	if !ok {
		return Language{}, false
	}
	return l.clone(), true
}
