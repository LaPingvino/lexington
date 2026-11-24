//go:build !js

package pdf

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/LaPingvino/lexington/rules"
)

// The script's language, as each system's fallback asks for it, and what
// the systems' answers are read by. These are not of any one system, so
// that the tests on one computer test them all (sysfont_fc.go,
// sysfont_darwin.go, sysfont_windows.go use them).

// fallbackLang is a script's language as the systems' font fallbacks name
// it. Han characters are one in Unicode but drawn by each region's own
// forms: a Japanese script wants a Japanese face of them, a Chinese one a
// Chinese face, which is what the language picks.
type fallbackLang struct {
	tag     string   // BCP 47 ("ja", "zh-Hant-TW"); "" for none: the system's default
	fc      string   // fontconfig's ("ja", "zh-cn", "zh-tw", "zh-hk")
	apple   string   // CoreText's ("ja", "zh-Hans", "zh-Hant")
	win     string   // DirectWrite's locale ("ja-JP", "zh-CN", "zh-TW")
	regions []string // the words a Han face's family name has for it ("JP"; "SC", "CN", "GB"), the likeliest first
}

// hanRegions are the region words of Han faces' family names, by the
// language they are for: Noto Sans CJK JP, Source Han Sans K, PingFang
// TC, Hiragino Sans GB.
var hanRegions = map[string][]string{
	"ja":      {"JP", "J"},
	"ko":      {"KR", "K"},
	"zh-Hans": {"SC", "CN", "GB"},
	"zh-Hant": {"TC", "TW", "HK", "HC"},
	"zh-HK":   {"HK", "HC", "TC", "TW"},
}

// allRegions are all of hanRegions' words.
var allRegions = func() map[string]bool {
	out := map[string]bool{}
	for _, ws := range hanRegions {
		for _, w := range ws {
			out[w] = true
		}
	}
	return out
}()

// langFor is a language (a BCP 47 tag, Lexington's "hekaya", "" for
// none) as the fallbacks name it. A name that is no tag is none, and so
// is one whose language is not of two or three letters (ISO 639: the
// "draft" of script.draft.fountain is no language).
func langFor(lang string) fallbackLang {
	if strings.EqualFold(strings.TrimSpace(lang), rules.Hekaya) {
		lang = "ar"
	}
	tag := rules.Tag(strings.TrimSpace(lang))
	if tag == "" {
		return fallbackLang{}
	}
	parts := strings.Split(tag, "-")
	base, script, region := parts[0], "", ""
	if len(base) < 2 || len(base) > 3 {
		return fallbackLang{}
	}
	for _, p := range parts[1:] {
		switch {
		case len(p) == 4 && script == "":
			script = p
		case (len(p) == 2 || len(p) == 3) && region == "":
			region = p
		}
	}
	l := fallbackLang{tag: tag, fc: base, apple: tag, win: tag}
	han := ""
	switch base {
	case "ja":
		han, l.win = "ja", "ja-JP"
	case "ko":
		han, l.win = "ko", "ko-KR"
	case "yue":
		han = "zh-HK"
	case "zh":
		switch {
		case region == "HK" || region == "MO":
			han = "zh-HK"
		case script == "Hant" || region == "TW":
			han = "zh-Hant"
		case script == "Hans" || region == "" || region == "CN" || region == "SG":
			han = "zh-Hans"
		default: // another region of Chinese: as Simplified
			han = "zh-Hans"
		}
	}
	switch han {
	case "zh-Hans":
		l.fc, l.apple, l.win = "zh-cn", "zh-Hans", "zh-CN"
	case "zh-Hant":
		l.fc, l.apple, l.win = "zh-tw", "zh-Hant", "zh-TW"
	case "zh-HK":
		l.fc, l.apple, l.win = "zh-hk", "zh-Hant-HK", "zh-HK"
	}
	l.regions = hanRegions[han]
	return l
}

// regionOf is a face's family name without its Han region word, and that
// word ("Noto Sans CJK KR Medium": "Noto Sans CJK _ Medium", "KR"); ""
// for a family with none.
func regionOf(family string) (key, region string) {
	words := strings.Fields(family)
	for i := len(words) - 1; i > 0; i-- {
		if allRegions[words[i]] {
			region = words[i]
			words[i] = "_"
			return strings.Join(words, " "), region
		}
	}
	return "", ""
}

// regionalSibling is which of a collection's faces (their family names,
// by index) is face's design for the language's region: the one of the
// likeliest region word of face's family (face itself if it is that);
// face if it has no region or no face of the language's is beside it. fontconfig takes a collection's faces for
// any of its languages alike, and its ranking (on a system with no
// preference for the language) gives Noto Sans CJK KR for Japanese: the
// file is the one fontconfig chose, its face the language's.
func regionalSibling(families []string, face int, l fallbackLang) int {
	if len(l.regions) == 0 || face < 0 || face >= len(families) {
		return face
	}
	key, _ := regionOf(families[face])
	if key == "" {
		return face
	}
	for _, want := range l.regions {
		for i, fam := range families {
			if k, r := regionOf(fam); k == key && r == want {
				return i
			}
		}
	}
	return face
}

// regionRank is how likely a family name's region is for the language:
// the index of its word in l.regions, len(l.regions) for a family of
// none of them.
func regionRank(family string, l fallbackLang) int {
	_, r := regionOf(family)
	if i := slices.Index(l.regions, r); r != "" && i >= 0 {
		return i
	}
	return len(l.regions)
}

// fcPattern is the fontconfig pattern fc-match ranks the faces for runes
// by: a monospaced one, of the language if any, bold if asked.
func fcPattern(runes []rune, bold bool, l fallbackLang) string {
	var charset strings.Builder
	for i, r := range runes {
		if i > 0 {
			charset.WriteByte(' ')
		}
		fmt.Fprintf(&charset, "%x", r)
	}
	pattern := "monospace"
	if l.fc != "" {
		pattern += ":lang=" + l.fc
	}
	pattern += ":charset=" + charset.String()
	if bold {
		pattern += ":weight=bold"
	}
	return pattern
}

// fcFaces are the faces of fc-match's output ("path#index" lines), at
// most maxRanked, each once, only those of font files glyphs can be taken
// from.
func fcFaces(out string) []string {
	var files []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" && !slices.Contains(files, l) && fontFile(strings.SplitN(l, "#", 2)[0]) {
			files = append(files, l)
			if len(files) == maxRanked {
				break
			}
		}
	}
	return files
}

// ranked is the same ranking of faces for each of runes (fontconfig's).
func ranked(runes []rune, faces []string) map[rune][]string {
	if len(faces) == 0 {
		return nil
	}
	out := make(map[rune][]string, len(runes))
	for _, r := range runes {
		out[r] = faces
	}
	return out
}

// mapRuns is a system's choice of face for each of runes, read run by
// run, as DirectWrite's MapCharacters answers: given where in the text
// (UTF-16) it starts and how much is left, mapAt tells how long a run
// from there is and the face ("path#index", "" for none) it is in. A
// run that comes back empty is taken as one unit, so the reading ends.
func mapRuns(runes []rune, mapAt func(text []uint16, pos, n int) (int, string)) map[rune][]string {
	if len(runes) == 0 {
		return nil
	}
	text := utf16.Encode(runes)
	// which rune each code unit belongs to
	of := make([]rune, 0, len(text))
	for _, r := range runes {
		for range utf16.RuneLen(r) {
			of = append(of, r)
		}
	}
	out := map[rune][]string{}
	for pos := 0; pos < len(text); {
		n, face := mapAt(text, pos, len(text)-pos)
		if n <= 0 {
			n = 1
		}
		n = min(n, len(text)-pos)
		if face != "" {
			for _, r := range of[pos : pos+n] {
				if !slices.Contains(out[r], face) {
					out[r] = append(out[r], face)
				}
			}
		}
		pos += n
	}
	return out
}

// eachRune is a system's choice of face for each of runes, asked one
// character at a time, as CoreText's CTFontCreateForStringWithLanguage
// is: fontFor tells the file and PostScript name of the font for a
// character's UTF-16, faceOf which face of the file that name is
// ("path#index"; "" for none). LastResort, the font of boxes macOS
// draws for what no font has, is none.
func eachRune(runes []rune, fontFor func([]uint16) (path, ps string, ok bool), faceOf func(path, ps string) string) map[rune][]string {
	out := map[rune][]string{}
	for _, r := range runes {
		path, ps, ok := fontFor(utf16.Encode([]rune{r}))
		if !ok || path == "" || strings.HasPrefix(ps, "LastResort") {
			continue
		}
		if face := faceOf(path, ps); face != "" {
			out[r] = []string{face}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// textSource is the text a DirectWrite analysis is asked of: what its
// IDWriteTextAnalysisSource answers (sysfont_windows.go), by these.
type textSource struct {
	text   []uint16
	locale []uint16 // the language's locale name, NUL-ended
}

// at is the text from pos to its end: its start's index and length; a
// pos at or past the end is none (-1, 0).
func (s *textSource) at(pos int) (int, int) {
	if pos < 0 || pos >= len(s.text) {
		return -1, 0
	}
	return pos, len(s.text) - pos
}

// before is the text before pos: from its start, pos long; none (-1, 0)
// at the start or past the end.
func (s *textSource) before(pos int) (int, int) {
	if pos <= 0 || pos > len(s.text) {
		return -1, 0
	}
	return 0, pos
}

// localeRun is how long the run of the language from pos is: to the end.
func (s *textSource) localeRun(pos int) int {
	return max(len(s.text)-pos, 0)
}
