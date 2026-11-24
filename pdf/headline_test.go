package pdf

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"github.com/LaPingvino/lexington/font"
)

// headlineY is a height through Courier Badi's Devanagari headline (its
// band: 856 to 1007, in font units).
const headlineY = 930

// minJoin is how far, at least, the headline pieces of a word overlap
// where they join, in font units (of 2048): pieces that only meet edge to
// edge, or nearly, can show a hairline where a rasteriser rounds each
// glyph on its own; Courier Badi 1.108 runs each end 20 past its edge.
const minJoin = 30

// A word's headline is drawn whole: the font's calt chooses the pads with
// a headline (.h) between its clusters, as HarfBuzz does, and every piece
// of it (a cluster's, a pad's) overlaps the next by minJoin at least.
func TestDevanagariHeadlineJoins(t *testing.T) {
	words := []string{"मगरमच्छ", "जामुन", "रहता", "हिन्दी", "पहुँचता", "दीदी"} // (not क्ष, ध, थ, श: their tops are open by design, as the donor draws them)
	for _, style := range []string{"", "B", "I", "BI"} {
		data := font.GetFont(font.CourierBadiName, style)
		sh := newFontShaper(data)
		f, err := sfnt.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		upem := int(sh.face.Upem())
		for _, w := range words {
			run := []rune(w)
			out := sh.sh.Shape(shaping.Input{Text: run, RunStart: 0, RunEnd: len(run), Direction: di.DirectionLTR,
				Face: sh.face, Size: fixed.I(upem), Script: language.Devanagari})
			var names []string
			var pieces [][2]int // the headline's ink runs, each glyph's, in font units
			pen := 0
			var buf sfnt.Buffer
			for _, g := range out.Glyphs {
				name, _ := f.GlyphName(&buf, sfnt.GlyphIndex(g.GlyphID))
				names = append(names, name)
				x := pen + int(g.XOffset>>6)
				pen += int(g.Advance >> 6)
				segs, err := f.LoadGlyph(&buf, sfnt.GlyphIndex(g.GlyphID), fixed.I(upem), nil)
				if err != nil {
					t.Fatal(err)
				}
				for _, p := range inkRuns(segs, headlineY) {
					pieces = append(pieces, [2]int{p[0] + x, p[1] + x})
				}
			}
			// between two clusters, the pads with the headline
			pads := 0
			for _, n := range names {
				if strings.HasPrefix(n, "lead.") || strings.HasPrefix(n, "trail") {
					if strings.HasSuffix(n, ".h") {
						pads++
					}
				}
			}
			if pads == 0 {
				t.Errorf("%s %q: no headline pads: %v", style, w, names)
			}
			if gap := headlineGap(pieces, minJoin); gap != "" {
				t.Errorf("%s %q: headline %s (glyphs %v)", style, w, gap, names)
			}
		}
	}
}

// inkRuns are where a glyph's outline (sfnt's, y down, in font units)
// has ink at height y (y up), as [from, to) runs, to a unit.
func inkRuns(segs sfnt.Segments, y int) [][2]int {
	const scale = 1 // font units a pixel
	b := segs.Bounds()
	x0, x1 := b.Min.X.Floor(), b.Max.X.Ceil()
	if x1 <= x0 || -y < b.Min.Y.Floor() || -y > b.Max.Y.Ceil() {
		return nil
	}
	w := (x1-x0)/scale + 2
	r := vector.NewRasterizer(w, 1)
	at := func(p fixed.Point26_6) (float32, float32) {
		return (float32(p.X)/64 - float32(x0)) / scale, float32(p.Y)/64 + float32(y) + 0.5
	}
	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			r.MoveTo(at(s.Args[0]))
		case sfnt.SegmentOpLineTo:
			r.LineTo(at(s.Args[0]))
		case sfnt.SegmentOpQuadTo:
			ax, ay := at(s.Args[0])
			bx, by := at(s.Args[1])
			r.QuadTo(ax, ay, bx, by)
		case sfnt.SegmentOpCubeTo:
			ax, ay := at(s.Args[0])
			bx, by := at(s.Args[1])
			cx, cy := at(s.Args[2])
			r.CubeTo(ax, ay, bx, by, cx, cy)
		}
	}
	r.ClosePath()
	img := image.NewAlpha(image.Rect(0, 0, w, 1))
	r.Draw(img, img.Bounds(), image.Opaque, image.Point{})
	var out [][2]int
	for i := 0; i < w; i++ {
		if img.AlphaAt(i, 0).A < 128 {
			continue
		}
		j := i
		for j < w && img.AlphaAt(j, 0).A >= 128 {
			j++
		}
		out = append(out, [2]int{x0 + i*scale, x0 + j*scale})
		i = j
	}
	return out
}

// headlineGap tells where the pieces, each cut by join/2 at both ends, no
// longer make one run from the first to the last: where two pieces
// overlap by less than join. "" if they do.
func headlineGap(pieces [][2]int, join int) string {
	if len(pieces) == 0 {
		return "missing"
	}
	slices.SortFunc(pieces, func(a, b [2]int) int { return a[0] - b[0] })
	first, last := pieces[0][0], pieces[0][1]
	for _, p := range pieces {
		last = max(last, p[1])
	}
	reach := first
	for _, p := range pieces {
		from, to := p[0]+join/2, p[1]-join/2
		if p[0] == first {
			from = first
		}
		if p[1] == last {
			to = last
		}
		if from > reach {
			return fmt.Sprintf("broken at %d..%d", reach, from)
		}
		reach = max(reach, to)
	}
	return ""
}
