package lex

import (
	"hash/fnv"
	"strings"
)

// A Pattern is how a color bar is drawn when colors have patterns (for
// those who tell colors apart badly): stripes along the bar, each across
// part of its width, solid or dashed. The patterns differ in their
// rhythm along the bar (solid, dashes long and short, dots, dashes with
// dots) and in their number of stripes across it (one, two, three, a
// broad and a narrow one, a ladder), so that they differ in shape, not
// only in tone: in grayscale, and to anyone.
type Pattern struct {
	Name    string   // as Word names its nearest border (w:val)
	ODF     string   // as ODF (CSS) names its nearest border style
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

// patterns are the bar's twelve patterns, one for each of Beat's colors
// that NoteColor knows (in the order of patternOf).
var patterns = []Pattern{
	{"single", "solid", []Stripe{{0, 1, nil, 0}}},
	{"dashed", "dashed", []Stripe{{0, 1, []float64{6, 3}, 0}}},
	{"dotted", "dotted", []Stripe{{0, 1, []float64{1.5, 2.5}, 0}}},
	{"dashSmallGap", "dashed", []Stripe{{0, 1, []float64{14, 4}, 0}}},
	{"dotDash", "dashed", []Stripe{{0, 1, []float64{8, 3, 2, 3}, 0}}},
	{"dotDotDash", "dotted", []Stripe{{0, 1, []float64{8, 3, 2, 3, 2, 3}, 0}}},
	{"double", "double", []Stripe{{0, 0.35, nil, 0}, {0.65, 1, nil, 0}}},
	{"thinThickSmallGap", "double", []Stripe{{0, 0.35, []float64{6, 3}, 0}, {0.65, 1, []float64{6, 3}, 0}}},
	{"triple", "double", []Stripe{{0, 0.2, nil, 0}, {0.4, 0.6, nil, 0}, {0.8, 1, nil, 0}}},
	{"thickThinMediumGap", "ridge", []Stripe{{0, 0.55, nil, 0}, {0.8, 1, nil, 0}}},
	{"wave", "groove", []Stripe{{0, 0.5, []float64{4, 4}, 0}, {0.5, 1, []float64{4, 4}, 4}}},
	{"doubleWave", "outset", []Stripe{{0, 0.25, nil, 0}, {0.75, 1, nil, 0}, {0.25, 0.75, []float64{1.5, 4}, 0}}},
}

// patternOf are the colors by their pattern's index.
var patternOf = []string{"red", "blue", "green", "pink", "magenta", "gray", "purple", "cyan", "teal", "yellow",
	"orange", "brown"}

// BarPattern is the pattern of a color's bar: one of twelve, Beat's
// twelve colors each their own; other colors (other names, #rrggbb) one
// of them by a hash of the color, the same each time.
func BarPattern(color string) Pattern {
	color = strings.ToLower(strings.TrimSpace(color))
	if color == "grey" {
		color = "gray"
	}
	for i, c := range patternOf {
		if c == color {
			return patterns[i]
		}
	}
	h := fnv.New32a()
	h.Write([]byte(color))
	return patterns[h.Sum32()%uint32(len(patterns))]
}
