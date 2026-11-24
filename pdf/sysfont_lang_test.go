//go:build !js

package pdf

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/LaPingvino/lexington/font"
	"golang.org/x/image/font/sfnt"
)

// A language as each system's fallback names it: Chinese by its script
// and region, Japanese and Korean with their regions for Windows, Hekaya
// as Arabic, and what is no tag as none.
func TestLangFor(t *testing.T) {
	for in, want := range map[string]fallbackLang{
		"ja":         {tag: "ja", fc: "ja", apple: "ja", win: "ja-JP", regions: []string{"JP", "J"}},
		"ja-JP":      {tag: "ja-JP", fc: "ja", apple: "ja-JP", win: "ja-JP", regions: []string{"JP", "J"}},
		"ko":         {tag: "ko", fc: "ko", apple: "ko", win: "ko-KR", regions: []string{"KR", "K"}},
		"zh":         {tag: "zh", fc: "zh-cn", apple: "zh-Hans", win: "zh-CN", regions: []string{"SC", "CN", "GB"}},
		"zh-CN":      {tag: "zh-CN", fc: "zh-cn", apple: "zh-Hans", win: "zh-CN", regions: []string{"SC", "CN", "GB"}},
		"zh-hans":    {tag: "zh-Hans", fc: "zh-cn", apple: "zh-Hans", win: "zh-CN", regions: []string{"SC", "CN", "GB"}},
		"zh-Hant":    {tag: "zh-Hant", fc: "zh-tw", apple: "zh-Hant", win: "zh-TW", regions: []string{"TC", "TW", "HK", "HC"}},
		"zh_TW":      {tag: "zh-TW", fc: "zh-tw", apple: "zh-Hant", win: "zh-TW", regions: []string{"TC", "TW", "HK", "HC"}},
		"zh-Hant-HK": {tag: "zh-Hant-HK", fc: "zh-hk", apple: "zh-Hant-HK", win: "zh-HK", regions: []string{"HK", "HC", "TC", "TW"}},
		"en":         {tag: "en", fc: "en", apple: "en", win: "en"},
		"hi-Deva-IN": {tag: "hi-Deva-IN", fc: "hi", apple: "hi-Deva-IN", win: "hi-Deva-IN"},
		"hekaya":     {tag: "ar", fc: "ar", apple: "ar", win: "ar"},
		"":           {},
		"my lang":    {},
	} {
		got := langFor(in)
		if got.tag != want.tag || got.fc != want.fc || got.apple != want.apple || got.win != want.win || !slices.Equal(got.regions, want.regions) {
			t.Errorf("langFor(%q) = %+v, want %+v", in, got, want)
		}
	}
}

// fontconfig's pattern: monospaced, the language's, the characters', bold
// if asked; no language for none.
func TestFcPattern(t *testing.T) {
	if got := fcPattern([]rune("直к"), false, langFor("ja")); got != "monospace:lang=ja:charset=76f4 43a" {
		t.Errorf("ja: %q", got)
	}
	if got := fcPattern([]rune("直"), true, langFor("zh-Hant")); got != "monospace:lang=zh-tw:charset=76f4:weight=bold" {
		t.Errorf("zh-Hant bold: %q", got)
	}
	if got := fcPattern([]rune("直"), false, langFor("")); got != "monospace:charset=76f4" {
		t.Errorf("none: %q", got)
	}
	out := "/f/a.ttc#1\n/f/a.ttc#1\n/f/b.pfb#0\n\n /f/c.otf#0 \n"
	if got := fcFaces(out); !slices.Equal(got, []string{"/f/a.ttc#1", "/f/c.otf#0"}) {
		t.Errorf("fcFaces: %q", got)
	}
}

// Of a collection's faces, the one of the language's region: Noto's
// Sans for Sans and Mono for Mono, its weight's, the likeliest region
// first; a face of the region, of no region, or with no sibling of it,
// stays.
func TestRegionalSibling(t *testing.T) {
	noto := []string{"Noto Sans CJK JP", "Noto Sans CJK KR", "Noto Sans CJK SC", "Noto Sans CJK TC", "Noto Sans CJK HK",
		"Noto Sans Mono CJK JP", "Noto Sans Mono CJK KR", "Noto Sans Mono CJK SC", "Noto Sans Mono CJK TC", "Noto Sans Mono CJK HK"}
	for _, c := range []struct {
		lang      string
		face, out int
	}{
		{"ja", 1, 0}, {"zh", 1, 2}, {"zh-Hant", 1, 3}, {"zh-HK", 0, 4}, {"ko", 2, 1},
		{"ja", 6, 5}, {"zh", 6, 7}, {"ja", 0, 0}, {"en", 1, 1}, {"", 1, 1},
	} {
		if got := regionalSibling(noto, c.face, langFor(c.lang)); got != c.out {
			t.Errorf("%s from %s: %s, want %s", c.lang, noto[c.face], noto[got], noto[c.out])
		}
	}
	pingfang := []string{"PingFang SC", "PingFang TC", "PingFang HK"}
	if got := regionalSibling(pingfang, 0, langFor("zh-TW")); got != 1 {
		t.Errorf("PingFang zh-TW: %d", got)
	}
	if got := regionalSibling(pingfang, 0, langFor("ja")); got != 0 { // no Japanese PingFang
		t.Errorf("PingFang ja: %d", got)
	}
	weights := []string{"Noto Sans CJK KR Medium", "Noto Sans CJK JP Bold", "Noto Sans CJK JP Medium"}
	if got := regionalSibling(weights, 0, langFor("ja")); got != 2 {
		t.Errorf("by weight: %d", got)
	}
	if key, r := regionOf("Source Han Sans K"); key != "Source Han Sans _" || r != "K" {
		t.Errorf("regionOf: %q %q", key, r)
	}
	if key, _ := regionOf("Microsoft YaHei"); key != "" {
		t.Errorf("YaHei has a region: %q", key)
	}
}

// DirectWrite's answer, run by run, read as each character's face: a run
// of several characters, one of none, a character outside the BMP (two
// UTF-16 units), an empty run taken as one unit, a run past the end cut.
func TestMapRuns(t *testing.T) {
	runes := []rune("直接𠀋é")
	var asked []int
	got := mapRuns(runes, func(text []uint16, pos, n int) (int, string) {
		if len(text) != 5 || pos+n != 5 {
			t.Fatalf("text %d long, from %d %d", len(text), pos, n)
		}
		asked = append(asked, pos)
		switch pos {
		case 0:
			return 2, "/f/jp.ttc#0" // 直接
		case 2:
			return 2, "/f/ext.ttf#0" // 𠀋
		}
		return 0, "" // é: nothing mapped, taken as one unit
	})
	want := map[rune][]string{'直': {"/f/jp.ttc#0"}, '接': {"/f/jp.ttc#0"}, '𠀋': {"/f/ext.ttf#0"}}
	if !maps.EqualFunc(got, want, slices.Equal) || !slices.Equal(asked, []int{0, 2, 4}) {
		t.Errorf("mapRuns %v (asked at %v), want %v", got, asked, want)
	}
	cut := mapRuns([]rune("ab"), func(_ []uint16, pos, n int) (int, string) { return 9, "/f/x.ttf#0" })
	if len(cut) != 2 || cut['b'][0] != "/f/x.ttf#0" {
		t.Errorf("a run past the end: %v", cut)
	}
	if mapRuns(nil, nil) != nil {
		t.Error("no runes: a map")
	}
}

// CoreText's answer, character by character: each one's UTF-16 asked,
// the font's file and PostScript name read as a face, LastResort as none.
func TestEachRune(t *testing.T) {
	got := eachRune([]rune("直𠀋☃x"), func(u []uint16) (string, string, bool) {
		switch string(utf16.Decode(u)) {
		case "直":
			return "/S/Hiragino.ttc", "HiraginoSans-W3", true
		case "𠀋":
			return "/S/Ext.ttf", "Ext", true
		case "☃":
			return "/S/LastResort.otf", "LastResort", true
		}
		return "", "", false
	}, func(path, ps string) string {
		if ps == "HiraginoSans-W3" {
			return path + "#2"
		}
		return path + "#0"
	})
	want := map[rune][]string{'直': {"/S/Hiragino.ttc#2"}, '𠀋': {"/S/Ext.ttf#0"}}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Errorf("eachRune %v, want %v", got, want)
	}
}

// What the IDWriteTextAnalysisSource answers DirectWrite: the text from a
// position, before it, and the locale's run.
func TestTextSource(t *testing.T) {
	s := &textSource{text: []uint16{1, 2, 3}}
	for _, c := range []struct{ pos, at, atN, bef, befN, run int }{
		{0, 0, 3, -1, 0, 3}, {1, 1, 2, 0, 1, 2}, {3, -1, 0, 0, 3, 0}, {4, -1, 0, -1, 0, 0},
	} {
		i, n := s.at(c.pos)
		j, m := s.before(c.pos)
		if i != c.at || n != c.atN || j != c.bef || m != c.befN || s.localeRun(c.pos) != c.run {
			t.Errorf("at %d: %d %d, before %d %d, run %d", c.pos, i, n, j, m, s.localeRun(c.pos))
		}
	}
}

// writeFont writes Courier Badi to a temporary file: a face that has
// Latin and no CJK.
func writeFont(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Courier.ttf")
	if err := os.WriteFile(path, font.GetFont(font.CourierBadiName, ""), 0o644); err != nil {
		t.Fatal(err)
	}
	return path + "#0"
}

// notoCJK is the Noto Sans CJK collection, or the test skips.
func notoCJK(t *testing.T) string {
	t.Helper()
	for _, p := range []string{"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no Noto Sans CJK collection")
	return ""
}

// The system's ranking is taken in its order, each character from the
// first face that has it: not the face that has the most of them (the
// folders' cover), which would take é from the CJK face too.
func TestRankedInOrder(t *testing.T) {
	resetSys(t)
	courier, cjk := writeFont(t), notoCJK(t)+"#0"
	systemFallback = func(r []rune, _ bool, _ fallbackLang) map[rune][]string { return ranked(r, []string{courier, cjk}) }
	got := systemGlyphs([]rune("é直接"), false, "")
	sys.mu.Lock()
	c, n := openFont(courier), openFont(cjk)
	sys.mu.Unlock()
	if got['é'].f != c || got['直'].f != n || got['接'].f != n {
		t.Errorf("é from %v, 直 from %v: want Courier, CJK", got['é'].f == c, got['直'].f == n)
	}
}

// familyOfGlyph is the family name of the face a glyph was taken from.
func familyOfGlyph(t *testing.T, g sysGlyph) string {
	t.Helper()
	if g.f == nil {
		return ""
	}
	name, _ := g.f.Name(nil, sfnt.NameIDFamily)
	return name
}

// needJPSC skips a test without both Noto Sans CJK JP and SC installed.
func needJPSC(t *testing.T) {
	t.Helper()
	fams := map[string]bool{}
	for _, f := range []string{"Noto Sans CJK JP", "Noto Sans CJK SC"} {
		out, err := exec.Command("fc-list", f+":", "family").Output()
		fams[f] = err == nil && len(bytes.TrimSpace(out)) > 0
	}
	if !fams["Noto Sans CJK JP"] || !fams["Noto Sans CJK SC"] {
		t.Skip("Noto Sans CJK JP and SC are not both installed (or no fontconfig)")
	}
}

// On this computer, through the system's own fallback (fontconfig on
// Linux): a Japanese script's Han characters from a Japanese face, a
// Chinese one's from a Chinese face, Traditional Chinese's and Korean's
// from theirs, in one process (the answers kept by language), and the
// PDF's fonts made of them differ.
func TestLanguagePicksHanFace(t *testing.T) {
	needJPSC(t)
	resetSys(t)
	const han = "直骨"
	for lang, want := range map[string]string{"ja": "JP", "zh": "SC", "zh-Hant": "TC", "ko": "KR", "ja-JP": "JP", "zh-CN": "SC"} {
		got := systemGlyphs([]rune(han), false, lang)
		for _, r := range han {
			fam := familyOfGlyph(t, got[r])
			if _, region := regionOf(fam); region != want {
				t.Errorf("%s: %c from %q, want a %s face", lang, r, fam, want)
			}
		}
	}
	if b := systemGlyphs([]rune(han), true, "ja"); !strings.HasSuffix(familyOfGlyph(t, b['直']), "JP") {
		t.Errorf("ja bold: 直 from %q", familyOfGlyph(t, b['直']))
	}
	base := font.GetFont("CourierPrime", "")
	ja, zh := withFallback(base, "", []rune(han), "ja"), withFallback(base, "", []rune(han), "zh")
	if bytes.Equal(ja, zh) || bytes.Equal(ja, base) {
		t.Error("the Japanese and the Chinese script's fonts are the same")
	}
}

// Without the system's fallback the folders' walk takes the language's
// region's face too, of faces that have as many of the characters.
func TestWalkPrefersRegion(t *testing.T) {
	needJPSC(t)
	resetSys(t)
	systemFallback = func([]rune, bool, fallbackLang) map[rune][]string { return nil }
	for lang, want := range map[string]string{"ja": "JP", "zh": "SC"} {
		got := systemGlyphs([]rune("直"), false, lang)
		if _, region := regionOf(familyOfGlyph(t, got['直'])); region != want {
			t.Errorf("%s: 直 from %q, want a %s face", lang, familyOfGlyph(t, got['直']), want)
		}
	}
}
