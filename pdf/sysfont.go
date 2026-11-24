//go:build !js

package pdf

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// The fonts on the computer, for the characters the PDF's font lacks
// (fallback.go): on Linux (and wherever fontconfig is) those fontconfig
// ranks for them; on macOS and Windows those of the system's and the
// user's font folders, the ones known to have CJK first. A font is read
// when it is first asked about, and kept; so is each character's answer.

var sys = struct {
	mu    sync.Mutex
	fonts map[string]*sfnt.Font // a file's path and its index in a collection: the font, nil if unreadable
	found map[sysKey]sysGlyph   // a character's glyph, the zero one if no font has it
	walk  []string              // the font files of the font folders, once listed (no fontconfig)
}{fonts: map[string]*sfnt.Font{}, found: map[sysKey]sysGlyph{}}

type sysKey struct {
	r    rune
	bold bool
}

// systemGlyphs are the glyphs on the computer for runes: each from the
// first font that has it (bold if asked, where there is one); a
// character none has is not in it.
func systemGlyphs(runes []rune, bold bool) map[rune]sysGlyph {
	sys.mu.Lock()
	defer sys.mu.Unlock()
	out := map[rune]sysGlyph{}
	var ask []rune
	for _, r := range runes {
		if g, ok := sys.found[sysKey{r, bold}]; ok {
			if g.f != nil {
				out[r] = g
			}
		} else {
			ask = append(ask, r)
		}
	}
	if len(ask) == 0 {
		return out
	}
	var buf sfnt.Buffer
	for _, file := range candidates(ask, bold) {
		if len(ask) == 0 {
			break
		}
		f := openFont(file)
		if f == nil {
			continue
		}
		left := ask[:0]
		for _, r := range ask {
			if g, err := f.GlyphIndex(&buf, r); err == nil && g != 0 && outlined(f, g) {
				out[r] = sysGlyph{f, g}
				sys.found[sysKey{r, bold}] = out[r]
			} else {
				left = append(left, r)
			}
		}
		ask = left
	}
	for _, r := range ask {
		sys.found[sysKey{r, bold}] = sysGlyph{}
	}
	return out
}

// outlined tells whether a glyph is drawn by an outline (not a colour
// emoji's picture).
func outlined(f *sfnt.Font, g sfnt.GlyphIndex) bool {
	var buf sfnt.Buffer
	_, err := f.LoadGlyph(&buf, g, 64*64, nil)
	return err == nil
}

// systemHas tells whether a font on the computer has a glyph for r.
func systemHas(runes []rune) map[rune]bool {
	found := systemGlyphs(runes, false)
	out := map[rune]bool{}
	for r := range found {
		out[r] = true
	}
	return out
}

// openFont is the font of a file ("path" or "path#index" in a
// collection), read once; nil if it cannot be.
func openFont(file string) *sfnt.Font {
	if f, ok := sys.fonts[file]; ok {
		return f
	}
	path, index := file, 0
	if i := strings.LastIndex(file, "#"); i > 0 {
		if n, err := strconv.Atoi(file[i+1:]); err == nil {
			path, index = file[:i], n
		}
	}
	var f *sfnt.Font
	if fh, err := os.Open(path); err == nil { // (kept open: the font reads its glyphs from it)
		if c, err := sfnt.ParseCollectionReaderAt(fh); err == nil && index < c.NumFonts() {
			f, _ = c.Font(index)
		}
		if f == nil {
			fh.Close()
		}
	}
	sys.fonts[file] = f
	return f
}

// maxCandidates is how many fonts are looked in for a character, at most.
const maxCandidates = 40

// candidates are the font files to look in for runes, the likeliest
// first.
func candidates(runes []rune, bold bool) []string {
	if fc, err := exec.LookPath("fc-match"); err == nil {
		var charset strings.Builder
		for i, r := range runes {
			if i > 0 {
				charset.WriteByte(' ')
			}
			fmt.Fprintf(&charset, "%x", r)
		}
		pattern := "monospace:charset=" + charset.String()
		if bold {
			pattern += ":weight=bold"
		}
		out, err := exec.Command(fc, "-s", "--format=%{file}#%{index}\n", pattern).Output()
		if err == nil {
			var files []string
			for _, l := range strings.Split(string(out), "\n") {
				if l = strings.TrimSpace(l); l != "" && !slices.Contains(files, l) && fontFile(strings.SplitN(l, "#", 2)[0]) {
					files = append(files, l)
				}
			}
			if len(files) > maxCandidates {
				files = files[:maxCandidates]
			}
			if len(files) > 0 {
				return files
			}
		}
	}
	if sys.walk == nil {
		sys.walk = folderFonts()
		if len(sys.walk) > 8*maxCandidates { // (a character no font has would open them all)
			sys.walk = sys.walk[:8*maxCandidates]
		}
	}
	if !bold {
		return sys.walk
	}
	// the bold ones first
	var b, rest []string
	for _, f := range sys.walk {
		if strings.Contains(strings.ToLower(filepath.Base(f)), "bold") {
			b = append(b, f)
		} else {
			rest = append(rest, f)
		}
	}
	return append(b, rest...)
}

// fontFile tells whether a file is of a font kind a glyph can be taken
// from (TrueType, OpenType, a collection of them).
func fontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

// knownCJK are the font files of macOS and Windows known to have CJK,
// looked in first.
var knownCJK = []string{
	"PingFang", "Hiragino", "AppleSDGothicNeo", "STHeiti", "Songti", "AppleGothic", "Arial Unicode",
	"msgothic", "YuGoth", "meiryo", "msyh", "simsun", "simhei", "malgun", "gulim", "msjh", "mingliu",
	"NotoSansCJK", "NotoSerifCJK", "SourceHan", "DroidSansFallback", "wqy",
}

// folderFonts are the font files of the system's and the user's font
// folders: each collection's fonts by index, those known to have CJK
// first.
func folderFonts() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		dirs = []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library/Fonts")}
	case "windows":
		dirs = []string{filepath.Join(os.Getenv("WINDIR"), "Fonts"), filepath.Join(os.Getenv("LOCALAPPDATA"), `Microsoft\Windows\Fonts`)}
	default:
		dirs = []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts")}
	}
	var known, rest []string
	for _, d := range dirs {
		if d == "" || d == "Fonts" {
			continue
		}
		filepath.WalkDir(d, func(path string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() || !fontFile(path) {
				return nil
			}
			files := []string{path}
			if ext := strings.ToLower(filepath.Ext(path)); ext == ".ttc" || ext == ".otc" {
				files = files[:0]
				if fh, err := os.Open(path); err == nil {
					if c, err := sfnt.ParseCollectionReaderAt(fh); err == nil {
						for i := 0; i < c.NumFonts(); i++ {
							files = append(files, fmt.Sprintf("%s#%d", path, i))
						}
					}
					fh.Close()
				}
			} else {
				files[0] += "#0"
			}
			base := strings.ToLower(filepath.Base(path))
			isKnown := slices.ContainsFunc(knownCJK, func(k string) bool { return strings.Contains(base, strings.ToLower(k)) })
			if isKnown {
				known = append(known, files...)
			} else {
				rest = append(rest, files...)
			}
			return nil
		})
	}
	return append(known, rest...)
}
