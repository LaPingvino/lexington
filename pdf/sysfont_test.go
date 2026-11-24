//go:build !js

package pdf

import (
	"encoding/binary"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/font"
)

// resetSys forgets the fonts read and the characters found, for a test
// and after it.
func resetSys(t *testing.T) {
	t.Helper()
	reset := func() {
		sys.mu.Lock()
		for _, sf := range sys.files {
			if sf != nil {
				sf.src.close()
			}
		}
		sys.files, sys.found, sys.walk, sys.regional = map[string]*sysFile{}, map[sysKey]sysGlyph{}, nil, map[[2]string]string{}
		sys.mu.Unlock()
		fallbackMu.Lock()
		fallbackCache, fallbackOrder, outlines = map[string][]byte{}, nil, map[outlineKey]convertedGlyph{}
		fallbackMu.Unlock()
	}
	reset()
	saved := systemFallback
	t.Cleanup(func() { systemFallback = saved; reset() })
}

// The cover takes the font with the most of the characters first, then
// the one with the most of the rest: not the first font that has some.
func TestCover(t *testing.T) {
	set := func(s string) func(rune) bool { return func(r rune) bool { return strings.ContainsRune(s, r) } }
	fonts := []func(rune) bool{
		set("ab"),     // 0: has some, first
		set("abcdef"), // 1: has the most
		set("gh"),     // 2: the rest
		set("ghi"),    // 3: more of the rest
		set(""),       // 4: none
	}
	got := cover(fonts, []rune("abcdefghiz"))
	want := map[rune]int{'a': 1, 'b': 1, 'c': 1, 'd': 1, 'e': 1, 'f': 1, 'g': 3, 'h': 3, 'i': 3}
	if !maps.Equal(got, want) {
		t.Errorf("cover %v, want %v", got, want)
	}
	// of two that have as many, the earlier
	if got := cover([]func(rune) bool{set("x"), set("x")}, []rune("x")); got['x'] != 0 {
		t.Errorf("x from %d, want the earlier font", got['x'])
	}
	if got := cover(nil, []rune("x")); len(got) != 0 {
		t.Errorf("no fonts: %v", got)
	}
}

// ttcOf is a font collection of one font file.
func ttcOf(ttf []byte) []byte {
	const shift = 16                                      // the collection's header: tag, version, count, an offset
	out := binary.BigEndian.AppendUint32(nil, 0x74746366) // 'ttcf'
	out = binary.BigEndian.AppendUint32(out, 0x00010000)
	out = binary.BigEndian.AppendUint32(out, 1)
	out = binary.BigEndian.AppendUint32(out, shift)
	f := append([]byte(nil), ttf...)
	n := int(binary.BigEndian.Uint16(f[4:]))
	for i := 0; i < n; i++ {
		rec := f[12+16*i:]
		binary.BigEndian.PutUint32(rec[8:], binary.BigEndian.Uint32(rec[8:])+shift)
	}
	return append(out, f...)
}

// The font folders of macOS, laid out in a temporary folder: the
// system's (Supplemental among them), the downloaded font assets
// (PingFang's place on recent versions), the library's and the user's;
// collections by face, names in Japanese (decomposed, as macOS keeps
// them) too, and nothing of the assets that are not fonts.
func TestFontDirsMacOS(t *testing.T) {
	resetSys(t)
	root, home := t.TempDir(), t.TempDir()
	ttf := font.GetFont(font.CourierBadiName, "")
	ttc := ttcOf(ttf)
	files := map[string][]byte{
		root + "/System/Library/Fonts/Hiragino Sans GB.ttc":                                                      ttc,
		root + "/System/Library/Fonts/\u30d2\u30e9\u30ad\u3099\u30ce\u89d2\u30b3\u3099\u30b7\u30c3\u30af W3.ttc": ttc,
		root + "/System/Library/Fonts/Supplemental/Songti.ttc":                                                   ttc,
		root + "/System/Library/AssetsV2/com_apple_MobileAsset_Font8/0a1b2c.asset/AssetData/PingFang.ttc":        ttc,
		root + "/System/Library/AssetsV2/com_apple_MobileAsset_Font8/0a1b2c.asset/Info.plist":                    []byte("<plist/>"),
		root + "/System/Library/AssetsV2/com_apple_MobileAsset_MacSoftwareUpdate/x.asset/AssetData/NotAFont.ttf": ttf,
		root + "/Library/Fonts/Arial Unicode.ttf":                                                                ttf,
		home + "/Library/Fonts/Mine.otf":                                                                         ttf,
	}
	native := map[string][]byte{}
	for path, data := range files {
		native[filepath.FromSlash(path)] = data
	}
	files = native
	for path, data := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	faces := folderFonts(fontDirs("darwin", root, home, func(string) string { return "" }))
	var want []string
	for path := range files {
		if fontFile(path) && !strings.Contains(path, "SoftwareUpdate") {
			want = append(want, path+"#0")
		}
	}
	slices.Sort(want)
	got := slices.Sorted(slices.Values(faces))
	if !slices.Equal(got, want) {
		t.Errorf("faces:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// a glyph is read out of a collection the walk found
	systemFallback = func([]rune, bool, fallbackLang) map[rune][]string { return nil }
	sys.walk = faces
	if g := systemGlyphs([]rune("Ĉ"), false, ""); g['Ĉ'].f == nil {
		t.Error("no Ĉ from the collections of the folders")
	}
}

// Windows' folders: the system's and the user's.
func TestFontDirsWindows(t *testing.T) {
	env := map[string]string{"WINDIR": `D:\Win`, "LOCALAPPDATA": `C:\Users\j\AppData\Local`}
	dirs := fontDirs("windows", "", "", func(k string) string { return env[k] })
	if len(dirs) != 2 || !strings.HasPrefix(dirs[0], `D:\Win`) || !strings.HasSuffix(dirs[0], "Fonts") ||
		!strings.Contains(dirs[1], `Microsoft\Windows\Fonts`) {
		t.Errorf("windows font folders %q", dirs)
	}
}

// The characters of the Chinese and Japanese samples, those Courier Badi
// lacks.
const zhJa = "一上下不丛个举么也了人什们你好名夫子年日月老（）。、東京症候群パリ"

// needCJKFolders skips a test when no font in the folders has Chinese and
// Japanese.
func needCJKFolders(t *testing.T) {
	t.Helper()
	systemFallback = func([]rune, bool, fallbackLang) map[rune][]string { return nil }
	if len(systemGlyphs([]rune(zhJa), false, "")) < len([]rune(zhJa)) {
		t.Skip("no font in this computer's font folders has Chinese and Japanese")
	}
	resetSys(t)
}

// Without fontconfig the folders are walked, and every character is
// found in some font of them: CFF outlines and collections too (Noto
// CJK's on Linux).
func TestSystemGlyphsWithoutFontconfig(t *testing.T) {
	resetSys(t)
	needCJKFolders(t)
	systemFallback = func([]rune, bool, fallbackLang) map[rune][]string { return nil }
	got := systemGlyphs([]rune(zhJa), false, "")
	for _, r := range zhJa {
		if got[r].f == nil {
			t.Errorf("no font for %c", r)
		}
	}
	if b := systemGlyphs([]rune("一東"), true, ""); len(b) != 2 {
		t.Errorf("bold: %d of 2", len(b))
	}
}

// Fonts fontconfig ranks that lack the characters do not end the
// search: the folders are asked too. And a character is "none" only once
// every font is asked: a narrow search before does not leave the wider
// one without it (the Chinese sample's characters, missing from the
// Japanese one's PDF).
func TestRankedFontsLackingThem(t *testing.T) {
	resetSys(t)
	needCJKFolders(t)
	courier := filepath.Join(t.TempDir(), "Courier.ttf")
	if err := os.WriteFile(courier, font.GetFont(font.CourierBadiName, ""), 0o644); err != nil {
		t.Fatal(err)
	}
	systemFallback = func(r []rune, _ bool, _ fallbackLang) map[rune][]string { return ranked(r, []string{courier + "#0"}) }
	zh := systemGlyphs([]rune("一上下不丛个举么"), false, "")
	ja := systemGlyphs([]rune("上夫子年日月老（）パリ"), false, "")
	for _, r := range "一上下不丛个举么" {
		if zh[r].f == nil {
			t.Errorf("zh: no font for %c", r)
		}
	}
	for _, r := range "上夫子年日月老（）パリ" {
		if ja[r].f == nil {
			t.Errorf("ja: no font for %c", r)
		}
	}
	if m := systemHas([]rune("ܐܠ" + "上")); !m['上'] {
		t.Error("上 missing beside characters no font has")
	}
}

// Of fonts that have as many of the characters, a regular weight's is
// taken: its name is no weight's.
func TestWeighted(t *testing.T) {
	for face, want := range map[string]bool{
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc#2": false,
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Black.ttc#2":   true,
		"/System/Library/Fonts/Hiragino Sans GB.ttc#0":        false,
		"/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc#0":            false,
		"/System/Library/Fonts/ヒラギノ角ゴシック W0.ttc#0":            true,
		`C:\Windows\Fonts\msyhbd.ttc#0`:                       false,
		"/Library/Fonts/Arial Unicode.ttf#0":                  false,
	} {
		if got := weighted(face); got != want {
			t.Errorf("weighted(%q) = %v", face, got)
		}
	}
}
