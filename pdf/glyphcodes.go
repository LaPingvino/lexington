package pdf

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"slices"
	"sort"
	"sync"
	"unsafe"

	gofont "github.com/go-text/typesetting/font"
	"golang.org/x/image/font/sfnt"
)

// Glyph codes. gofpdf prints characters: it subsets a font by the
// characters printed and maps each to its glyph through the font's cmap.
// A shaped glyph that no character maps to (a Devanagari conjunct, a half
// form, a word's end form) it cannot print. So the font the PDF embeds is
// the font file with a private-use character (U+E000 on; past U+F8FF, a
// CJK ideograph the font lacks) given to each such glyph in its cmap: printing that character prints the glyph. The
// codes are only shown, never read: the text is printed beside them, as
// written, invisible (devanagari.go), and they are wrapped in an empty
// ActualText.

// coded is a font file with a character for each of its glyphs.
type coded struct {
	data []byte              // the file with the private-use codes in its cmap
	code map[gofont.GID]rune // each glyph's character: its own, or the private-use one
	ign  map[gofont.GID]bool // the glyphs of the joiners (ZWJ, ZWNJ): they print as nothing
}

var (
	codedByAddr sync.Map // a file's first byte's address and length to its *coded
	codedByHash sync.Map // a file's SHA-256 to its *coded
)

// glyphCoded is the font file data with its glyphs coded, made once for
// a file (by its address, then its contents); data itself if it cannot
// be read or needs none.
func glyphCoded(data []byte) *coded {
	if len(data) == 0 {
		return &coded{data: data}
	}
	type addr struct {
		p *byte
		n int
	}
	key := addr{unsafe.SliceData(data), len(data)}
	if c, ok := codedByAddr.Load(key); ok {
		return c.(*coded)
	}
	sum := sha256.Sum256(data)
	if c, ok := codedByHash.Load(sum); ok {
		return c.(*coded) // (a file read again: its address is not kept, nor the bytes with it)
	}
	c := makeCoded(data)
	if prev, loaded := codedByHash.LoadOrStore(sum, c); loaded {
		c = prev.(*coded)
	}
	codedByAddr.Store(key, c)
	// the coded file is its own identity too (a Recorder's drawing keeps
	// it, and a page made again adds it again)
	codedByAddr.Store(addr{unsafe.SliceData(c.data), len(c.data)}, c)
	return c
}

func makeCoded(data []byte) *coded {
	c := &coded{data: data, code: map[gofont.GID]rune{}, ign: map[gofont.GID]bool{}}
	face, err := gofont.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return c
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		return c
	}
	mapping := map[rune]gofont.GID{}
	for it := face.Cmap.Iter(); it.Next(); {
		r, g := it.Char()
		if g == 0 {
			continue
		}
		mapping[r] = g
		if old, ok := c.code[g]; !ok || r < old {
			c.code[g] = r
		}
	}
	for _, r := range []rune{0x200C, 0x200D} {
		if g, ok := mapping[r]; ok {
			c.ign[g] = true
		}
	}
	// the private-use area first; a font with more glyphs than it has
	// codes (Courier Badi's Devanagari, drawn at many widths) goes on in
	// the CJK ideographs, which gofpdf's format 4 cmap still reaches: they
	// are only shown, the text is read from the ActualText
	ranges := [][2]rune{{0xE000, 0xF8FF}, {0x3400, 0x4DBF}, {0x4E00, 0x9FFF}}
	k, next := 0, ranges[0][0]
	added := false
	for g := gofont.GID(1); int(g) < f.NumGlyphs(); g++ {
		if _, ok := c.code[g]; ok {
			continue
		}
		for {
			if next > ranges[k][1] {
				if k++; k == len(ranges) {
					break
				}
				next = ranges[k][0]
			}
			if _, used := mapping[next]; !used {
				break
			}
			next++
		}
		if k == len(ranges) {
			break
		}
		mapping[next], c.code[g] = g, next
		next++
		added = true
	}
	if !added {
		return c
	}
	if out := withCmap(data, mapping); out != nil {
		c.data = out
	} else { // unchanged: the glyphs without a character stay so
		for r, g := range mapping {
			if (r >= 0xE000 && r <= 0xF8FF || r >= 0x3400 && r <= 0x9FFF) && c.code[g] == r {
				delete(c.code, g)
			}
		}
	}
	return c
}

// withCmap is a font file with its Unicode cmap subtables (format 4 and
// 12) replaced by ones of mapping (its other subtables kept); nil if the
// file cannot be rewritten.
func withCmap(data []byte, mapping map[rune]gofont.GID) []byte {
	type table struct {
		tag  string
		data []byte
	}
	if len(data) < 12 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	if len(data) < 12+16*n {
		return nil
	}
	var tables []table
	cmapAt := -1
	for i := 0; i < n; i++ {
		rec := data[12+16*i:]
		off, length := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		if uint64(off)+uint64(length) > uint64(len(data)) {
			return nil
		}
		tag := string(rec[:4])
		if tag == "cmap" {
			cmapAt = i
		}
		tables = append(tables, table{tag, data[off : off+length]})
	}
	if cmapAt < 0 {
		return nil
	}
	cmap := newCmap(tables[cmapAt].data, mapping)
	if cmap == nil {
		return nil
	}
	tables[cmapAt].data = cmap
	// the file: the header and directory, then the tables, each on four bytes
	out := slices.Clone(data[:12+16*n])
	headAt := -1
	for i, t := range tables {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		rec := out[12+16*i:]
		b := t.data
		if t.tag == "head" && len(b) >= 12 {
			b = slices.Clone(b)
			binary.BigEndian.PutUint32(b[8:], 0) // checkSumAdjustment, set below
			headAt = len(out)
		}
		binary.BigEndian.PutUint32(rec[4:], checksum(b))
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(b)))
		out = append(out, b...)
	}
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	if headAt >= 0 {
		binary.BigEndian.PutUint32(out[headAt+8:], 0xB1B0AFBA-checksum(out))
	}
	return out
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}

// newCmap is a cmap table with its Unicode subtables made of mapping: a
// format 4 (the BMP) for platforms 0/3 and Windows BMP, a format 12 (all)
// for 0/4 and 3/10; the old table's other subtables (Macintosh, variation
// sequences) kept. nil if it cannot be.
func newCmap(old []byte, mapping map[rune]gofont.GID) []byte {
	if len(old) < 4 {
		return nil
	}
	type sub struct {
		platform, encoding uint16
		data               []byte
	}
	var subs []sub
	n := int(binary.BigEndian.Uint16(old[2:]))
	for i := 0; i < n && 4+8*i+8 <= len(old); i++ {
		rec := old[4+8*i:]
		p, e, off := binary.BigEndian.Uint16(rec), binary.BigEndian.Uint16(rec[2:]), binary.BigEndian.Uint32(rec[4:])
		if int(off)+4 > len(old) {
			return nil
		}
		format := binary.BigEndian.Uint16(old[off:])
		if (p == 0 || p == 3) && (format == 4 || format == 12) {
			continue // made anew
		}
		var length uint32
		switch format {
		case 0, 2, 4, 6:
			length = uint32(binary.BigEndian.Uint16(old[off+2:]))
		default:
			if int(off)+8 > len(old) {
				return nil
			}
			length = binary.BigEndian.Uint32(old[off+4:])
			if format == 14 {
				length = binary.BigEndian.Uint32(old[off+2:])
			}
		}
		if uint64(off)+uint64(length) > uint64(len(old)) {
			return nil
		}
		subs = append(subs, sub{p, e, old[off : off+length]})
	}
	runes := make([]rune, 0, len(mapping))
	for r := range mapping {
		runes = append(runes, r)
	}
	slices.Sort(runes)
	f4, f12 := format4(runes, mapping), format12(runes, mapping)
	if f4 == nil {
		return nil
	}
	subs = append(subs, sub{0, 3, f4}, sub{0, 4, f12}, sub{3, 1, f4}, sub{3, 10, f12})
	sort.SliceStable(subs, func(i, j int) bool {
		if subs[i].platform != subs[j].platform {
			return subs[i].platform < subs[j].platform
		}
		return subs[i].encoding < subs[j].encoding
	})
	out := make([]byte, 4+8*len(subs))
	binary.BigEndian.PutUint16(out[2:], uint16(len(subs)))
	at := map[*byte]uint32{} // a subtable shared by two records is written once
	for i, s := range subs {
		rec := out[4+8*i:]
		binary.BigEndian.PutUint16(rec, s.platform)
		binary.BigEndian.PutUint16(rec[2:], s.encoding)
		off, ok := at[unsafe.SliceData(s.data)]
		if !ok {
			for len(out)%4 != 0 {
				out = append(out, 0)
			}
			off = uint32(len(out))
			at[unsafe.SliceData(s.data)] = off
			out = append(out, s.data...)
		}
		binary.BigEndian.PutUint32(rec[4:], off)
	}
	return out
}

// format4 is a format 4 subtable of the BMP characters of runes (sorted):
// a segment for each stretch of consecutive characters, by delta when
// their glyphs are consecutive too; nil if it is larger than the format
// allows.
func format4(runes []rune, mapping map[rune]gofont.GID) []byte {
	type seg struct {
		start, end rune
		delta      bool
	}
	var segs []seg
	for _, r := range runes {
		if r > 0xFFFE {
			break
		}
		if k := len(segs) - 1; k >= 0 && segs[k].end+1 == r {
			segs[k].delta = segs[k].delta && mapping[r] == mapping[r-1]+1
			segs[k].end = r
			continue
		}
		segs = append(segs, seg{r, r, true})
	}
	segs = append(segs, seg{0xFFFF, 0xFFFF, true})
	nseg := len(segs)
	var glyphs []uint16
	idRangeOffset := make([]uint16, nseg)
	idDelta := make([]uint16, nseg)
	for i, s := range segs {
		switch {
		case s.start == 0xFFFF:
			idDelta[i] = 1
		case s.delta:
			idDelta[i] = uint16(int(mapping[s.start]) - int(s.start))
		default:
			// from this segment's idRangeOffset word to its glyphs
			idRangeOffset[i] = uint16(2 * (nseg - i + len(glyphs)))
			for r := s.start; r <= s.end; r++ {
				glyphs = append(glyphs, uint16(mapping[r]))
			}
		}
	}
	length := 16 + 8*nseg + 2*len(glyphs)
	if length > 0xFFFF {
		return nil
	}
	b := make([]byte, 0, length)
	u16 := func(v uint16) { b = binary.BigEndian.AppendUint16(b, v) }
	search, selector := 1, 0
	for search*2 <= nseg {
		search *= 2
		selector++
	}
	u16(4)
	u16(uint16(length))
	u16(0) // language
	u16(uint16(2 * nseg))
	u16(uint16(2 * search))
	u16(uint16(selector))
	u16(uint16(2*nseg - 2*search))
	for _, s := range segs {
		u16(uint16(s.end))
	}
	u16(0) // reservedPad
	for _, s := range segs {
		u16(uint16(s.start))
	}
	for _, d := range idDelta {
		u16(d)
	}
	for _, o := range idRangeOffset {
		u16(o)
	}
	for _, g := range glyphs {
		u16(g)
	}
	return b
}

// format12 is a format 12 subtable of runes (sorted): a group for each
// stretch of consecutive characters with consecutive glyphs.
func format12(runes []rune, mapping map[rune]gofont.GID) []byte {
	type group struct {
		start, end rune
		gid        uint32
	}
	var groups []group
	for _, r := range runes {
		g := uint32(mapping[r])
		if k := len(groups) - 1; k >= 0 && groups[k].end+1 == r && groups[k].gid+uint32(r-groups[k].start) == g {
			groups[k].end = r
			continue
		}
		groups = append(groups, group{r, r, g})
	}
	b := make([]byte, 0, 16+12*len(groups))
	b = binary.BigEndian.AppendUint16(b, 12)
	b = binary.BigEndian.AppendUint16(b, 0)
	b = binary.BigEndian.AppendUint32(b, uint32(16+12*len(groups)))
	b = binary.BigEndian.AppendUint32(b, 0) // language
	b = binary.BigEndian.AppendUint32(b, uint32(len(groups)))
	for _, g := range groups {
		b = binary.BigEndian.AppendUint32(b, uint32(g.start))
		b = binary.BigEndian.AppendUint32(b, uint32(g.end))
		b = binary.BigEndian.AppendUint32(b, g.gid)
	}
	return b
}
