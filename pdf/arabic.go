package pdf

import (
	"bytes"
	"strings"
	"sync"
	"unicode"

	"github.com/go-text/typesetting/di"
	gofont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Arabic is shaped through the font's own OpenType features (Courier
// Badi's GSUB: joining forms, lam-alef), by go-text's port of HarfBuzz,
// and only Arabic runs are. The PDF writer (gofpdf) prints characters,
// not glyphs, so each glyph the shaper gives is printed as the character
// the font maps to it: an Arabic letter's contextual form is its
// presentation form (U+FB50–U+FDFF, U+FE70–U+FEFF), lam-alef its
// ligature. This happens in logical order, as composeHebrew does, before
// the line is wrapped, reordered (bidi.go) and drawn, so a Recorder
// records the shaped text and a page view draws what the PDF prints.
//
// A glyph the font maps no character to (the contextual forms of letters
// beyond Arabic's, Persian's and Urdu's, which Courier Badi has but only
// through GSUB) cannot be printed so: such a letter stays as written, its
// isolated form (Unsupported warns of it). The font's GPOS (mark to base,
// mark to mark: a fatha on a hamza, over a shadda) places the marks:
// markOffsets, printed apart by bidiPrint, for Hebrew's points too.

// fontShaper shapes a font's Arabic runs (here) and Devanagari ones (devanagari.go).
type fontShaper struct {
	mu    sync.Mutex
	face  *gofont.Face
	chars map[gofont.GID]rune // the character each glyph prints from
	sh    shaping.HarfbuzzShaper
	codes *coded // every glyph's character, in the PDF's copy of the font
}

var shapers sync.Map // a font file's bytes (as a string key of its address) to its *fontShaper

// newFontShaper is the shaper of a font file, nil if it cannot be read.
func newFontShaper(data []byte) *fontShaper {
	face, err := gofont.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		return nil
	}
	var buf sfnt.Buffer
	s := &fontShaper{face: face, chars: map[gofont.GID]rune{}, codes: glyphCoded(data)}
	for _, r := range shapeTargets {
		for c := r[0]; c <= r[1]; c++ {
			if g, err := f.GlyphIndex(&buf, c); err == nil && g != 0 {
				if _, ok := s.chars[gofont.GID(g)]; !ok {
					s.chars[gofont.GID(g)] = c
				}
			}
		}
	}
	return s
}

// arabicLanguage is the language Arabic runs are shaped in: none, so the
// font's default language system applies, as the text may be Arabic,
// Persian, Urdu or another language written in Arabic letters.
var arabicLanguage language.Language

// shapeTargets are the characters a shaped glyph can print as: Arabic and
// its presentation forms, the spaces and signs a run holds.
var shapeTargets = [][2]rune{{0x20, 0x7E}, {0xA0, 0xFF}, {0x0600, 0x06FF}, {0x0750, 0x077F}, {0x08A0, 0x08FF},
	{0x200C, 0x200F}, {0xFB50, 0xFDFF}, {0xFE70, 0xFEFF}}

// arabicRune tells whether a character is shaped with the Arabic run
// around it: an Arabic letter or mark, or a joiner.
func arabicRune(r rune) bool {
	return unicode.Is(unicode.Arabic, r) && r != 0x060C && r != 0x061B && r != 0x061F &&
		!(r >= 0x0660 && r <= 0x0669) && !(r >= 0x06F0 && r <= 0x06F9) || r == 0x200C || r == 0x200D
}

// arabicRunEnd is where the Arabic run starting at rs[i] ends. The vowel
// marks (fatha, shadda, sukun…) are of no script of their own
// (Inherited): the run goes on over them, or a vowelled word would be
// shaped letter by letter, unjoined.
func arabicRunEnd(rs []rune, i int) int {
	j := i
	for j < len(rs) && (arabicRune(rs[j]) || unicode.Is(unicode.Mn, rs[j])) {
		j++
	}
	return j
}

// shape is text with its Arabic runs shaped; unshaped is how many of its
// letters could not be (no character for their form).
func (s *fontShaper) shape(text string) (out string, unshaped int) {
	rs := []rune(text)
	var b []rune
	for i := 0; i < len(rs); {
		if !arabicRune(rs[i]) {
			b = append(b, rs[i])
			i++
			continue
		}
		j := arabicRunEnd(rs, i)
		run, n := s.shapeRun(rs[i:j])
		b, unshaped = append(b, run...), unshaped+n
		i = j
	}
	return string(b), unshaped
}

// shapeRun is one Arabic run shaped, in logical order.
func (s *fontShaper) shapeRun(run []rune) ([]rune, int) {
	s.mu.Lock()
	out := s.sh.Shape(shaping.Input{Text: run, RunStart: 0, RunEnd: len(run), Direction: di.DirectionRTL,
		Face: s.face, Size: fixed.I(12), Script: language.Arabic, Language: arabicLanguage})
	s.mu.Unlock()
	// the glyphs come right to left (visual order): by cluster, in
	// logical order, each cluster's glyphs as they come within it
	type cluster struct {
		start int
		gids  []gofont.GID
	}
	var cs []cluster
	for k := len(out.Glyphs) - 1; k >= 0; k-- {
		g := out.Glyphs[k]
		if n := len(cs); n > 0 && cs[n-1].start == g.ClusterIndex {
			cs[n-1].gids = append(cs[n-1].gids, g.GlyphID)
			continue
		}
		cs = append(cs, cluster{start: g.ClusterIndex, gids: []gofont.GID{g.GlyphID}})
	}
	var b []rune
	unshaped := 0
	for k, c := range cs {
		end := len(run)
		if k+1 < len(cs) {
			end = cs[k+1].start
		}
		if c.start >= end || c.start < 0 || end > len(run) {
			return run, 0 // not in logical order: leave it as written
		}
		if strings.Trim(string(run[c.start:end]), "\u200c\u200d") == "" {
			continue // a joiner: it sets the forms, prints nothing
		}
		var chars []rune
		ok := true
		for _, g := range c.gids {
			r, has := s.chars[g]
			if !has {
				ok = false
				break
			}
			chars = append(chars, r)
		}
		if !ok {
			unshaped++
			chars = run[c.start:end]
		}
		// marks after their letter, as in logical order
		b = append(b, chars...)
	}
	return b, unshaped
}

// markOffsets are where the font's GPOS puts the marks on a base
// character: cluster is the base as printed (a presentation form) and its
// marks in logical order; each mark's offset from where it would stand
// unplaced (at the base's origin, as a mark has no advance), in ems, y
// up. ok is false when the shaper does not give a glyph a character (a
// ligature of marks), and the marks then keep their default places.
func (s *fontShaper) markOffsets(cluster []rune) (dx, dy []float64, ok bool) {
	if len(cluster) < 2 {
		return nil, nil, false
	}
	script, lang := language.Arabic, arabicLanguage
	if b := cluster[0]; b >= 0x0590 && b <= 0x05FF || b >= 0xFB1D && b <= 0xFB4F {
		script, lang = language.Hebrew, language.NewLanguage("he")
	}
	upem := int(s.face.Upem())
	s.mu.Lock()
	out := s.sh.Shape(shaping.Input{Text: cluster, RunStart: 0, RunEnd: len(cluster), Direction: di.DirectionRTL,
		Face: s.face, Size: fixed.I(upem), Script: script, Language: lang})
	s.mu.Unlock()
	// right to left: the glyphs come as the cluster reversed, the base last
	if len(out.Glyphs) != len(cluster) {
		return nil, nil, false
	}
	type at struct{ x, y fixed.Int26_6 }
	pos := make([]at, len(out.Glyphs))
	pen := fixed.Int26_6(0)
	for i, g := range out.Glyphs {
		pos[i] = at{pen + g.XOffset, g.YOffset}
		pen += g.Advance
	}
	n := len(out.Glyphs)
	base := pos[n-1]
	em := float64(upem) * 64
	for k := 1; k < len(cluster); k++ {
		m := pos[n-1-k]
		dx = append(dx, float64(m.x-base.x)/em)
		dy = append(dy, float64(m.y-base.y)/em)
	}
	return dx, dy, true
}

// shaperFor is the shaper of a font's file data, made once.
func shaperFor(name string, data []byte) *fontShaper {
	if s, ok := shapers.Load(name); ok {
		return s.(*fontShaper)
	}
	s := newFontShaper(data)
	shapers.Store(name, s)
	return s
}

// hasArabic tells whether text has a character shape works on.
func hasArabic(text string) bool {
	for _, r := range text {
		if r >= 0x0600 && arabicRune(r) && r != 0x200C && r != 0x200D {
			return true
		}
	}
	return false
}
