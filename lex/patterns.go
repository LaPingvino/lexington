package lex

import (
	"hash/fnv"
	"strings"
)

// A Pattern is how a color bar is drawn when colors have patterns (for
// those who tell colors apart badly): stripes along the bar, each across
// part of its width, solid or dashed. The patterns differ in their
// rhythm along the bar (Along: solid, dash, dot, or dashes with dots) and
// in their number of stripes across it (Across: one, two, three), so that
// they differ in shape, not only in tone: in grayscale, and to anyone.
type Pattern struct {
	Label   string   // what it looks like
	Name    string   // Word's border for it (w:val), all twelve different
	ODF     string   // ODF's border style for it (fo:border), shared by two
	Across  int      // stripes across the bar
	Along   string   // solid, dash, dot or mix
	Stripes []Stripe // the bar's stripes
}

// Stripe is a stripe of a bar: from From to To across its width (0 to 1),
// dashed with Dash (lengths in points along the bar, on, off, on, ...;
// none for solid), from Offset points on.
type Stripe struct {
	From, To float64
	Dash     []float64
	Offset   float64
}

// Differ is how much two patterns differ in shape: 0 (alike), 1 (in the
// number of stripes or in rhythm) or 2 (in both).
func (p Pattern) Differ(o Pattern) int {
	n := 0
	if p.Across != o.Across {
		n++
	}
	if p.Along != o.Along {
		n++
	}
	return n
}

// patterns are the bar's twelve patterns. Word has a border for each;
// ODF has six that show apart at a bar's width (solid, dashed, dotted,
// double, groove, ridge: inset and outset look solid), so two patterns
// alike in shape share each.
var patterns = []Pattern{
	{"solid", "single", "solid", 1, "solid", []Stripe{{0, 1, nil, 0}}},
	{"dashed", "dashed", "dashed", 1, "dash", []Stripe{{0, 1, []float64{6, 3}, 0}}},
	{"dotted", "dotted", "dotted", 1, "dot", []Stripe{{0, 1, []float64{1.5, 2.5}, 0}}},
	{"long dash", "dashSmallGap", "dashed", 1, "dash", []Stripe{{0, 1, []float64{14, 4}, 0}}},
	{"dash-dot", "dotDash", "ridge", 1, "mix", []Stripe{{0, 1, []float64{8, 3, 2, 3}, 0}}},
	{"dash-dot-dot", "dotDotDash", "ridge", 1, "mix", []Stripe{{0, 1, []float64{8, 3, 2, 3, 2, 3}, 0}}},
	{"double", "double", "double", 2, "solid", []Stripe{{0, 0.35, nil, 0}, {0.65, 1, nil, 0}}},
	{"double dashed", "thinThickSmallGap", "groove", 2, "dash", []Stripe{{0, 0.35, []float64{6, 3}, 0}, {0.65, 1, []float64{6, 3}, 0}}},
	{"triple", "triple", "double", 3, "solid", []Stripe{{0, 0.2, nil, 0}, {0.4, 0.6, nil, 0}, {0.8, 1, nil, 0}}},
	{"thick-thin", "thickThinMediumGap", "solid", 2, "solid", []Stripe{{0, 0.55, nil, 0}, {0.8, 1, nil, 0}}},
	{"checker", "wave", "groove", 2, "dash", []Stripe{{0, 0.5, []float64{4, 4}, 0}, {0.5, 1, []float64{4, 4}, 4}}},
	{"ladder", "doubleWave", "dotted", 3, "dot", []Stripe{{0, 0.25, nil, 0}, {0.75, 1, nil, 0}, {0.25, 0.75, []float64{1.5, 4}, 0}}},
}

// colorPatterns give each of Beat's twelve colors its pattern (an index
// in patterns), chosen so that colors easily taken for each other never
// look alike. Their bars (ColorFor's line colors, all at least 3:1 on
// white, so between L* 47 and 62) were compared as people with protan,
// deutan or tritan vision see them (Machado, Oliveira and Fernandes 2009,
// at full severity) and as others do; two colors are confusable when in
// one of those they are less than 20 apart in CIELAB (ΔE*76). A search
// over the assignments put, in this order of importance:
//   - no two confusable colors on the same ODF style: those that share
//     one are 27.8 ΔE or more apart in every vision, and as far apart in
//     lightness as the bars allow (one dark, one light: ΔL* 7.6 or more):
//     red/pink, blue/yellow, green/purple, magenta/orange, gray/cyan,
//     teal/brown;
//   - confusable colors on patterns that differ in shape (Differ): by
//     stripes and rhythm both (2) for the most confusable pairs
//     (yellow/orange, magenta/gray, cyan/teal, green/teal, pink/teal,
//     blue/purple, red/green, ...), at least by one of them for all.
//
// TestPatternsApart checks it.
var colorPatterns = map[string]int{
	"red": 3, "blue": 10, "green": 9, "pink": 1, "magenta": 6, "gray": 4,
	"purple": 0, "cyan": 5, "teal": 11, "yellow": 7, "orange": 8, "brown": 2,
}

// BarPattern is the pattern of a color's bar: one of twelve, Beat's
// twelve colors each their own (colorPatterns); other colors (other
// names, #rrggbb) one of them by a hash of the color, the same each time.
func BarPattern(color string) Pattern {
	color = strings.ToLower(strings.TrimSpace(color))
	if color == "grey" {
		color = "gray"
	}
	if i, ok := colorPatterns[color]; ok {
		return patterns[i]
	}
	h := fnv.New32a()
	h.Write([]byte(color))
	return patterns[h.Sum32()%uint32(len(patterns))]
}
