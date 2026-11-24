package pdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"sync"
	"unicode"

	gofont "github.com/go-text/typesetting/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/LaPingvino/lexington/lekhan"
	"github.com/LaPingvino/lexington/lex"
)

// Fallback glyphs. Courier Badi has no CJK, and a font file given may
// lack characters too. The PDF writer does not bundle fonts for them: it
// takes them from the fonts on the computer (sysfont.go; none in the
// browser) and adds them to the font the PDF embeds, each glyph in the
// cells the Lekhan width gives its character (a CJK one two, as wide as
// two spaces of the font), centred and, if wider, narrowed to fit. The
// script is laid out and printed in one font, as it always was: what is
// measured is what is printed, and a Drawing's font (Drawing.Fonts) has
// the glyphs, so its pages paint as the PDF prints them. A character no
// font on the computer has gets an empty glyph of its cells: a blank, not
// a box, where it stands (Unsupported names it).

// fallbackRunes are the characters of a screenplay a font may lack: its
// lines' and anything else printed with them (the title page template,
// the rules' words), a superset of what is printed.
func fallbackRunes(s lex.Screenplay, extra ...any) []rune {
	seen := map[rune]bool{}
	add := func(text string) {
		for _, r := range text {
			if r >= 0x80 {
				seen[r] = true
			}
		}
	}
	for _, l := range s {
		add(string(l.Contents))
	}
	for _, e := range extra {
		add(fmt.Sprintf("%+v", e))
	}
	out := make([]rune, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	slices.Sort(out)
	return out
}

// lacked are those of runes a font (its glyph test; nil: none) has no glyph for and
// that take a cell: what fallback glyphs are wanted for. Only the BMP,
// which gofpdf's cmap reading reaches.
func lacked(has func(rune) bool, runes []rune) []rune {
	var out []rune
	for _, r := range runes {
		if r > 0xFFFF || unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) ||
			unicode.In(r, unicode.Mn, unicode.Me) || r >= 0xD800 && r <= 0xDFFF || r >= 0xE000 && r <= 0xF8FF {
			continue
		}
		if has == nil || !has(r) {
			out = append(out, r)
		}
	}
	return out
}

var (
	fallbackMu sync.Mutex
	// fallbackCache are the fonts made last, by base, style and
	// characters (a script typed into adds a font a character: the oldest
	// go, as each is a font file's size)
	fallbackCache = map[string][]byte{}
	fallbackOrder []string
	// outlines are the glyphs converted, by their source and cells
	outlines   = map[outlineKey]convertedGlyph{}
	glyphTests sync.Map // a font's SHA-256 to its glyph test (a func(rune) bool), safe to share
)

// hasGlyph is a font file's glyph test, made once for a file and safe
// for use by several goroutines.
func hasGlyph(data []byte) func(rune) bool {
	sum := sha256.Sum256(data)
	if f, ok := glyphTests.Load(sum); ok {
		return f.(func(rune) bool)
	}
	var mu sync.Mutex
	test := glyphTest(data)
	f := func(r rune) bool {
		if test == nil {
			return true // unreadable: as it prints
		}
		mu.Lock()
		defer mu.Unlock()
		return test(r)
	}
	g, _ := glyphTests.LoadOrStore(sum, f)
	return g.(func(rune) bool)
}

// withFallback is a TrueType font file with glyphs added for those of
// runes it lacks: each from a font on the computer that has it (bold
// ones for a bold style), an empty one if none has; base itself if it
// lacks none or cannot be added to; in the script's language (a BCP 47
// tag, "" for the system's default), which picks a Han face of its
// region. Made once for a font, runes and language.
func withFallback(base []byte, style string, runes []rune, lang string) []byte {
	want := lacked(hasGlyph(base), runes)
	if len(want) == 0 {
		return base
	}
	sum := sha256.Sum256(base)
	key := fmt.Sprintf("%x/%s/%s/%s", sum[:8], style, lang, string(want))
	fallbackMu.Lock()
	defer fallbackMu.Unlock()
	if out, ok := fallbackCache[key]; ok {
		return out
	}
	bold := style == "B" || style == "BI"
	found := systemGlyphs(want, bold, lang)
	out := mergeGlyphs(base, want, found)
	if out == nil {
		out = base
	}
	fallbackCache[key] = out
	fallbackOrder = append(fallbackOrder, key)
	if len(fallbackOrder) > maxFallbacks {
		delete(fallbackCache, fallbackOrder[0])
		fallbackOrder = fallbackOrder[1:]
	}
	return out
}

// maxFallbacks is how many fonts with fallback glyphs are kept: the four
// styles of two scripts.
const maxFallbacks = 8

type outlineKey struct {
	src       sysGlyph
	upem, adv float64
}

type convertedGlyph struct {
	glyf []byte
	box  glyphBox
}

// outlineOf is outline, made once for a glyph (fallbackMu held).
func outlineOf(s sysGlyph, upem, adv float64) ([]byte, glyphBox) {
	k := outlineKey{s, upem, adv}
	if o, ok := outlines[k]; ok {
		return o.glyf, o.box
	}
	g, b := outline(s.f, s.g, upem, adv)
	outlines[k] = convertedGlyph{g, b}
	return g, b
}

// sysGlyph is where a character's glyph is found: a font and its glyph.
type sysGlyph struct {
	f *sfnt.Font
	g sfnt.GlyphIndex
}

// mergeGlyphs is base (a TrueType file: glyf outlines) with a glyph for
// each of runes added at its end, mapped in its cmap: found's outline,
// converted to quadratic curves, or none. nil if base cannot be read.
func mergeGlyphs(base []byte, runes []rune, found map[rune]sysGlyph) []byte {
	tables := readTables(base)
	head, maxp, hhea, hmtx, loca, glyf, cmap := tables["head"], tables["maxp"], tables["hhea"], tables["hmtx"], tables["loca"], tables["glyf"], tables["cmap"]
	if len(head) < 54 || len(maxp) < 6 || len(hhea) < 36 || glyf == nil || loca == nil || cmap == nil {
		return nil
	}
	be := binary.BigEndian
	upem := float64(be.Uint16(head[18:]))
	long := int16(be.Uint16(head[50:])) == 1
	n := int(be.Uint16(maxp[4:]))
	nhm := int(be.Uint16(hhea[34:]))
	if n == 0 || nhm == 0 || len(hmtx) < 4*nhm+2*(n-nhm) {
		return nil
	}
	offsets := make([]uint32, n+1)
	for i := range offsets {
		if long {
			if 4*i+4 > len(loca) {
				return nil
			}
			offsets[i] = be.Uint32(loca[4*i:])
		} else {
			if 2*i+2 > len(loca) {
				return nil
			}
			offsets[i] = 2 * uint32(be.Uint16(loca[2*i:]))
		}
	}
	if int(offsets[n]) > len(glyf) {
		return nil
	}
	type metric struct {
		adv uint16
		lsb int16
	}
	metrics := make([]metric, n)
	for i := range metrics {
		if i < nhm {
			metrics[i] = metric{be.Uint16(hmtx[4*i:]), int16(be.Uint16(hmtx[4*i+2:]))}
		} else {
			metrics[i] = metric{be.Uint16(hmtx[4*(nhm-1):]), int16(be.Uint16(hmtx[4*nhm+2*(i-nhm):]))}
		}
	}
	// a cell: the space's advance (Courier Badi's 1229 of 2048)
	f, err := sfnt.Parse(base)
	if err != nil {
		return nil
	}
	var buf sfnt.Buffer
	cell := 0.6 * upem
	if g, err := f.GlyphIndex(&buf, ' '); err == nil && g != 0 && int(g) < n {
		cell = float64(metrics[g].adv)
	}
	mapping := map[rune]gofont.GID{}
	if face, err := gofont.ParseTTF(bytes.NewReader(base)); err == nil {
		for it := face.Cmap.Iter(); it.Next(); {
			if r, g := it.Char(); g != 0 {
				mapping[r] = g
			}
		}
	} else {
		return nil
	}
	newGlyf := slices.Clone(glyf[:offsets[n]])
	maxPoints, maxContours := be.Uint16(maxp[6:]), be.Uint16(maxp[8:])
	xMin, yMin, xMax, yMax := int16(be.Uint16(head[36:])), int16(be.Uint16(head[38:])), int16(be.Uint16(head[40:])), int16(be.Uint16(head[42:]))
	advMax := be.Uint16(hhea[10:])
	for _, r := range runes {
		cells := max(1, lekhan.RuneWidth(r))
		adv := cells * int(math.Round(cell))
		for len(newGlyf)%4 != 0 {
			newGlyf = append(newGlyf, 0)
		}
		offsets = append(offsets[:len(offsets)-1], uint32(len(newGlyf)))
		m := metric{adv: uint16(adv)}
		if s, ok := found[r]; ok {
			if g, gb := outlineOf(s, upem, float64(adv)); g != nil {
				newGlyf = append(newGlyf, g...)
				m.lsb = gb.xMin
				maxPoints, maxContours = max(maxPoints, gb.points), max(maxContours, gb.contours)
				xMin, yMin, xMax, yMax = min(xMin, gb.xMin), min(yMin, gb.yMin), max(xMax, gb.xMax), max(yMax, gb.yMax)
			}
		}
		offsets = append(offsets, uint32(len(newGlyf)))
		mapping[r] = gofont.GID(len(metrics))
		metrics = append(metrics, m)
		advMax = max(advMax, m.adv)
	}
	if len(metrics) > 0xFFFF {
		return nil
	}
	for len(newGlyf)%4 != 0 {
		newGlyf = append(newGlyf, 0)
	}
	offsets[len(offsets)-1] = uint32(len(newGlyf))
	total := len(metrics)

	newLoca := make([]byte, 4*len(offsets))
	for i, o := range offsets {
		be.PutUint32(newLoca[4*i:], o)
	}
	newHmtx := make([]byte, 4*total)
	for i, m := range metrics {
		be.PutUint16(newHmtx[4*i:], m.adv)
		be.PutUint16(newHmtx[4*i+2:], uint16(m.lsb))
	}
	head, maxp, hhea = slices.Clone(head), slices.Clone(maxp), slices.Clone(hhea)
	be.PutUint16(head[50:], 1) // long offsets
	be.PutUint16(head[36:], uint16(xMin))
	be.PutUint16(head[38:], uint16(yMin))
	be.PutUint16(head[40:], uint16(xMax))
	be.PutUint16(head[42:], uint16(yMax))
	be.PutUint16(maxp[4:], uint16(total))
	if be.Uint32(maxp) == 0x00010000 && len(maxp) >= 10 {
		be.PutUint16(maxp[6:], maxPoints)
		be.PutUint16(maxp[8:], maxContours)
	}
	be.PutUint16(hhea[10:], advMax)
	be.PutUint16(hhea[34:], uint16(total))
	newCmapTable := newCmap(cmap, mapping)
	if newCmapTable == nil {
		return nil
	}
	replace := map[string][]byte{"head": head, "maxp": maxp, "hhea": hhea, "hmtx": newHmtx, "loca": newLoca,
		"glyf": newGlyf, "cmap": newCmapTable}
	if post := tables["post"]; len(post) >= 32 && be.Uint32(post) == 0x00020000 {
		p := slices.Clone(post[:32]) // version 3: no glyph names (version 2's are of the old glyphs)
		be.PutUint32(p, 0x00030000)
		replace["post"] = p
	}
	return withTables(base, replace, map[string]bool{"hdmx": true, "LTSH": true, "VORG": true})
}

// glyphBox is a glyph's bounds and size in points and contours.
type glyphBox struct {
	xMin, yMin, xMax, yMax int16
	points, contours       uint16
}

// outline is glyph g of f as a TrueType simple glyph in a font of upem
// units an em, adv wide: f's size kept, narrowed to fit adv if wider,
// centred; its cubic curves made quadratic. nil if it has no outline.
func outline(f *sfnt.Font, g sfnt.GlyphIndex, upem, adv float64) ([]byte, glyphBox) {
	var buf sfnt.Buffer
	src := float64(f.UnitsPerEm())
	ppem := fixed.Int26_6(src * 64)
	segs, err := f.LoadGlyph(&buf, g, ppem, nil)
	if err != nil || len(segs) == 0 {
		return nil, glyphBox{}
	}
	a, err := f.GlyphAdvance(&buf, g, ppem, 0)
	if err != nil {
		return nil, glyphBox{}
	}
	k := upem / src
	w := float64(a) / 64 * k
	sx := 1.0
	if w > adv {
		sx = adv / w
	}
	dx := (adv - w*sx) / 2
	type pt struct {
		x, y float64
		on   bool
	}
	at := func(p fixed.Point26_6) pt {
		return pt{float64(p.X)/64*k*sx + dx, -float64(p.Y) / 64 * k, true}
	}
	var contours [][]pt
	var cur []pt
	var last pt
	flush := func() {
		if len(cur) > 1 && cur[len(cur)-1].on && cur[0].on &&
			math.Round(cur[len(cur)-1].x) == math.Round(cur[0].x) && math.Round(cur[len(cur)-1].y) == math.Round(cur[0].y) {
			cur = cur[:len(cur)-1] // closed by returning to its start
		}
		if len(cur) > 1 {
			contours = append(contours, cur)
		}
		cur = nil
	}
	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			flush()
			last = at(s.Args[0])
			cur = append(cur, last)
		case sfnt.SegmentOpLineTo:
			last = at(s.Args[0])
			cur = append(cur, last)
		case sfnt.SegmentOpQuadTo:
			c := at(s.Args[0])
			c.on = false
			last = at(s.Args[1])
			cur = append(cur, c, last)
		case sfnt.SegmentOpCubeTo:
			p0, p1, p2, p3 := last, at(s.Args[0]), at(s.Args[1]), at(s.Args[2])
			// four quadratic pieces, each of a quarter of the cubic
			const pieces = 4
			bez := func(t float64) (float64, float64) {
				u := 1 - t
				return u*u*u*p0.x + 3*u*u*t*p1.x + 3*u*t*t*p2.x + t*t*t*p3.x,
					u*u*u*p0.y + 3*u*u*t*p1.y + 3*u*t*t*p2.y + t*t*t*p3.y
			}
			deriv := func(t float64) (float64, float64) {
				u := 1 - t
				return 3 * (u*u*(p1.x-p0.x) + 2*u*t*(p2.x-p1.x) + t*t*(p3.x-p2.x)),
					3 * (u*u*(p1.y-p0.y) + 2*u*t*(p2.y-p1.y) + t*t*(p3.y-p2.y))
			}
			for i := 0; i < pieces; i++ {
				t0, t1 := float64(i)/pieces, float64(i+1)/pieces
				ax, ay := bez(t0)
				bx, by := bez(t1)
				d0x, d0y := deriv(t0)
				d1x, d1y := deriv(t1)
				h := t1 - t0
				// the quadratic's control: where the tangents at both ends
				// meet, as the average of their two guesses
				cx := ((ax + d0x*h/2) + (bx - d1x*h/2)) / 2
				cy := ((ay + d0y*h/2) + (by - d1y*h/2)) / 2
				cur = append(cur, pt{cx, cy, false}, pt{bx, by, true})
			}
			last = p3
		}
	}
	flush()
	if len(contours) == 0 {
		return nil, glyphBox{}
	}
	var box glyphBox
	first := true
	var ends []uint16
	var flags, xs, ys []byte
	px, py := 0, 0
	count := 0
	for _, c := range contours {
		for _, p := range c {
			x, y := int(math.Round(p.x)), int(math.Round(p.y))
			x, y = max(-32768, min(32767, x)), max(-32768, min(32767, y))
			if first {
				box.xMin, box.xMax, box.yMin, box.yMax = int16(x), int16(x), int16(y), int16(y)
				first = false
			}
			box.xMin, box.xMax = min(box.xMin, int16(x)), max(box.xMax, int16(x))
			box.yMin, box.yMax = min(box.yMin, int16(y)), max(box.yMax, int16(y))
			var fl byte
			if p.on {
				fl = 1
			}
			flags = append(flags, fl)
			xs = binary.BigEndian.AppendUint16(xs, uint16(int16(x-px)))
			ys = binary.BigEndian.AppendUint16(ys, uint16(int16(y-py)))
			px, py = x, y
			count++
		}
		ends = append(ends, uint16(count-1))
	}
	box.points, box.contours = uint16(count), uint16(len(contours))
	out := binary.BigEndian.AppendUint16(nil, uint16(len(contours)))
	for _, v := range []int16{box.xMin, box.yMin, box.xMax, box.yMax} {
		out = binary.BigEndian.AppendUint16(out, uint16(v))
	}
	for _, e := range ends {
		out = binary.BigEndian.AppendUint16(out, e)
	}
	out = binary.BigEndian.AppendUint16(out, 0) // no instructions
	out = append(out, flags...)
	out = append(out, xs...)
	out = append(out, ys...)
	return out, box
}

// readTables are a font file's tables by their tag; nil if it cannot be
// read.
func readTables(data []byte) map[string][]byte {
	if len(data) < 12 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	if len(data) < 12+16*n {
		return nil
	}
	out := map[string][]byte{}
	for i := 0; i < n; i++ {
		rec := data[12+16*i:]
		off, length := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		if uint64(off)+uint64(length) > uint64(len(data)) {
			return nil
		}
		out[string(rec[:4])] = data[off : off+length]
	}
	return out
}
