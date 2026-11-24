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
// (fallback.go). Each character is looked for until a font has it or
// every font has been asked: first among those fontconfig ranks for the
// characters (where there is fontconfig: Linux, and macOS with it
// installed), then among all those of the system's and the user's font
// folders. Within each, the font that has the most of the characters
// still wanted is taken first, then the one with the most of the rest,
// and so on (cover). A font is read when it is first asked about, and
// kept; its file is open while it is read from, a few dozen at a time
// (lazyFile). Each character's answer is kept, the "none" too, as it
// comes from asking every font.

var sys = struct {
	mu    sync.Mutex
	files map[string]*sysFile // a font file's path: it read, nil if unreadable
	found map[sysKey]sysGlyph // a character's glyph, the zero one if no font has it
	walk  []string            // the faces of the font folders, once listed
}{files: map[string]*sysFile{}, found: map[sysKey]sysGlyph{}}

// sysFile is a font file read: its collection (one font if not one) and
// the faces of it opened, by index.
type sysFile struct {
	src   *lazyFile
	c     *sfnt.Collection
	faces map[int]*sfnt.Font
}

// lazyFile is a file read from where it is, open while it is read: at
// most maxOpen of them are at a time, the one opened first closed for
// another (opened again when next read). A computer's fonts are hundreds
// of files; each stays read (sys), not open.
type lazyFile struct {
	path string
	fh   *os.File
}

const maxOpen = 64

var lazy struct {
	mu   sync.Mutex
	open []*lazyFile
}

func (l *lazyFile) ReadAt(p []byte, off int64) (int, error) {
	lazy.mu.Lock()
	defer lazy.mu.Unlock()
	if l.fh == nil {
		fh, err := os.Open(l.path)
		if err != nil {
			return 0, err
		}
		l.fh = fh
		lazy.open = append(lazy.open, l)
		if len(lazy.open) > maxOpen {
			lazy.open[0].fh.Close()
			lazy.open[0].fh = nil
			lazy.open = lazy.open[1:]
		}
	}
	return l.fh.ReadAt(p, off)
}

// close closes the file if open.
func (l *lazyFile) close() {
	lazy.mu.Lock()
	defer lazy.mu.Unlock()
	if l.fh != nil {
		l.fh.Close()
		l.fh = nil
		lazy.open = slices.DeleteFunc(lazy.open, func(o *lazyFile) bool { return o == l })
	}
}

type sysKey struct {
	r    rune
	bold bool
}

// fontconfig is fontconfig's ranking of font faces ("path#index") for
// runes, the likeliest first; nil where there is no fontconfig. (A
// variable: the tests replace it.)
var fontconfig = fcMatch

// traceFonts, set by LEXINGTON_FONT_TRACE, writes each font looked in and
// what it gave to standard error.
var traceFonts = os.Getenv("LEXINGTON_FONT_TRACE") != ""

// systemGlyphs are the glyphs on the computer for runes (bold if asked,
// where a bold font has them); a character no font has is not in it.
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
		} else if !slices.Contains(ask, r) {
			ask = append(ask, r)
		}
	}
	if len(ask) == 0 {
		return out
	}
	tried := map[string]bool{}
	look := func(faces []string) {
		var fonts []string
		for _, face := range faces {
			if !tried[face] {
				tried[face] = true
				fonts = append(fonts, face)
			}
		}
		ask = lookIn(fonts, ask, bold, out)
	}
	// fontconfig's ranking, asked again for what is still wanted while it
	// gives some
	for round := 0; round < 4 && len(ask) > 0; round++ {
		before := len(ask)
		look(fontconfig(ask, bold))
		if len(ask) == before {
			break
		}
	}
	if len(ask) > 0 {
		if sys.walk == nil {
			sys.walk = folderFonts(fontDirs(runtime.GOOS, "", homeDir(), os.Getenv))
		}
		if bold { // the bold ones first, then any
			look(slices.DeleteFunc(slices.Clone(sys.walk), func(f string) bool { return !boldName(f) }))
			look(sys.walk)
		} else { // of fonts that have as many, a regular one's
			walk := slices.Clone(sys.walk)
			slices.SortStableFunc(walk, func(a, b string) int { return cmpBool(weighted(a), weighted(b)) })
			look(walk)
		}
	}
	for _, r := range ask {
		sys.found[sysKey{r, bold}] = sysGlyph{}
	}
	return out
}

// lookIn takes glyphs for runes from faces, by cover, into out (and
// sys.found); those none of them has are left.
func lookIn(faces []string, runes []rune, bold bool, out map[rune]sysGlyph) []rune {
	if len(runes) == 0 || len(faces) == 0 {
		return runes
	}
	var buf sfnt.Buffer
	type face struct {
		name string
		f    *sfnt.Font
		bad  map[rune]bool // has a glyph for it, but no outline (a colour emoji's picture)
	}
	var fonts []*face
	var has []func(rune) bool
	for _, name := range faces {
		f := openFont(name)
		if f == nil {
			if traceFonts {
				fmt.Fprintf(os.Stderr, "lexington font: %s unreadable\n", name)
			}
			continue
		}
		fc := &face{name, f, map[rune]bool{}}
		fonts = append(fonts, fc)
		has = append(has, func(r rune) bool {
			g, err := fc.f.GlyphIndex(&buf, r)
			return err == nil && g != 0 && !fc.bad[r]
		})
	}
	for len(runes) > 0 {
		pick := cover(has, runes)
		if len(pick) == 0 {
			break
		}
		left := runes[:0]
		for _, r := range runes {
			i, ok := pick[r]
			if !ok {
				left = append(left, r)
				continue
			}
			g, _ := fonts[i].f.GlyphIndex(&buf, r)
			if !outlined(fonts[i].f, g) {
				fonts[i].bad[r] = true // asked of the others next round
				left = append(left, r)
				continue
			}
			out[r] = sysGlyph{fonts[i].f, g}
			sys.found[sysKey{r, bold}] = out[r]
		}
		runes = left
	}
	if traceFonts {
		n := map[int]int{}
		for _, r := range out {
			for i, fc := range fonts {
				if fc.f == r.f {
					n[i]++
				}
			}
		}
		for i, fc := range fonts {
			if n[i] > 0 {
				fmt.Fprintf(os.Stderr, "lexington font: %s gives %d\n", fc.name, n[i])
			}
		}
		fmt.Fprintf(os.Stderr, "lexington font: %d faces looked in, %d characters left\n", len(fonts), len(runes))
	}
	return runes
}

// cover is which font (an index of fonts, each a glyph test) each of
// runes is taken from: the font that has the most of runes first, then
// the one that has the most of the rest, and so on (the earlier of two
// that have as many), until none has any of those left. A rune no font
// has is not in it.
func cover(fonts []func(rune) bool, runes []rune) map[rune]int {
	out := map[rune]int{}
	// what each font has of runes
	sets := make([][]rune, len(fonts))
	for i, has := range fonts {
		for _, r := range runes {
			if has(r) {
				sets[i] = append(sets[i], r)
			}
		}
	}
	for {
		best, most := -1, 0
		for i, set := range sets {
			n := 0
			for _, r := range set {
				if _, taken := out[r]; !taken {
					n++
				}
			}
			if n > most {
				best, most = i, n
			}
		}
		if best < 0 {
			return out
		}
		for _, r := range sets[best] {
			if _, taken := out[r]; !taken {
				out[r] = best
			}
		}
		sets[best] = nil
	}
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

// splitFace is a face's file and index in it ("path#index"; 0 if none).
func splitFace(face string) (string, int) {
	if i := strings.LastIndex(face, "#"); i > 0 {
		if n, err := strconv.Atoi(face[i+1:]); err == nil {
			return face[:i], n
		}
	}
	return face, 0
}

// openFont is a face's font ("path" or "path#index" in a collection),
// read once; nil if it cannot be (sys.mu held).
func openFont(face string) *sfnt.Font {
	path, index := splitFace(face)
	sf, ok := sys.files[path]
	if !ok {
		src := &lazyFile{path: path}
		if c, err := sfnt.ParseCollectionReaderAt(src); err == nil {
			sf = &sysFile{src: src, c: c, faces: map[int]*sfnt.Font{}}
		}
		src.close()
		sys.files[path] = sf
	}
	if sf == nil || index < 0 || index >= sf.c.NumFonts() {
		return nil
	}
	f, ok := sf.faces[index]
	if !ok {
		f, _ = sf.c.Font(index)
		sf.faces[index] = f
	}
	return f
}

// maxRanked is how many of fontconfig's ranking are looked in at a time.
const maxRanked = 40

// fcMatch is fontconfig's ranking (fc-match) of the faces for runes, at
// most maxRanked of them; nil without fontconfig.
func fcMatch(runes []rune, bold bool) []string {
	fc, err := exec.LookPath("fc-match")
	if err != nil {
		return nil
	}
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
	if err != nil {
		return nil
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" && !slices.Contains(files, l) && fontFile(strings.SplitN(l, "#", 2)[0]) {
			files = append(files, l)
			if len(files) == maxRanked {
				break
			}
		}
	}
	return files
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

// boldName tells whether a face's file is named bold.
func boldName(face string) bool {
	path, _ := splitFace(face)
	return strings.Contains(strings.ToLower(filepath.Base(path)), "bold")
}

// weighted tells whether a face's file is named for a weight not the
// regular one (Noto's "Black", Hiragino's "W6").
func weighted(face string) bool {
	path, _ := splitFace(face)
	name := strings.ToLower(filepath.Base(path))
	for _, w := range []string{"thin", "light", "medium", "semi", "demi", "bold", "black", "heavy", "extra", "ultra",
		"w0", "w1", "w2", "w4", "w5", "w6", "w7", "w8", "w9"} {
		if strings.Contains(name, w) {
			return true
		}
	}
	return false
}

// cmpBool orders false before true.
func cmpBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// fontDirs are the system's and the user's font folders on an operating
// system (GOOS), with root before the system's (the tests' stand-in for
// "/"; "" for the computer's own). On macOS these are the system's fonts
// (Supplemental among them), the fonts macOS downloads as assets
// (PingFang and other CJK ones on recent versions), and the library's
// and the user's.
func fontDirs(goos, root, home string, getenv func(string) string) []string {
	sysDir := func(p string) string { return filepath.Join(root, p) }
	switch goos {
	case "darwin":
		dirs := []string{sysDir("/System/Library/Fonts")}
		for _, assets := range []string{"/System/Library/AssetsV2", "/System/Library/Assets"} {
			m, _ := filepath.Glob(filepath.Join(sysDir(assets), "com_apple_MobileAsset_Font*"))
			dirs = append(dirs, m...)
		}
		dirs = append(dirs, sysDir("/Library/Fonts"), sysDir("/Network/Library/Fonts"))
		if home != "" {
			dirs = append(dirs, filepath.Join(home, "Library/Fonts"))
		}
		return dirs
	case "windows":
		var dirs []string
		if w := getenv("WINDIR"); w != "" {
			dirs = append(dirs, filepath.Join(w, "Fonts"))
		} else {
			dirs = append(dirs, `C:\Windows\Fonts`)
		}
		if l := getenv("LOCALAPPDATA"); l != "" {
			dirs = append(dirs, filepath.Join(l, `Microsoft\Windows\Fonts`))
		}
		return dirs
	default:
		dirs := []string{sysDir("/usr/share/fonts"), sysDir("/usr/local/share/fonts")}
		data := getenv("XDG_DATA_HOME")
		if data == "" && home != "" {
			data = filepath.Join(home, ".local/share")
		}
		if data != "" {
			dirs = append(dirs, filepath.Join(data, "fonts"))
		}
		if home != "" {
			dirs = append(dirs, filepath.Join(home, ".fonts"))
		}
		return dirs
	}
}

// folderFonts are the faces ("path#index") of the font files in dirs and
// their folders, each collection's by index.
func folderFonts(dirs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, d := range dirs {
		filepath.WalkDir(d, func(path string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() || !fontFile(path) || seen[path] {
				return nil
			}
			seen[path] = true
			n := 1
			if ext := strings.ToLower(filepath.Ext(path)); ext == ".ttc" || ext == ".otc" {
				n = 0
				if fh, err := os.Open(path); err == nil {
					if c, err := sfnt.ParseCollectionReaderAt(fh); err == nil {
						n = c.NumFonts()
					}
					fh.Close()
				}
			}
			for i := 0; i < n; i++ {
				out = append(out, fmt.Sprintf("%s#%d", path, i))
			}
			return nil
		})
	}
	return out
}
