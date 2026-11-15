package lex

import (
	"math"
	"testing"
)

// cvd are the simulations of color vision deficiency at full severity
// (Machado, Oliveira and Fernandes 2009), on linear RGB; nil is none.
var cvd = map[string]*[3][3]float64{
	"normal": nil,
	"protan": {{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}},
	"deutan": {{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}},
	"tritan": {{1.255528, -0.076749, -0.178779}, {-0.078411, 0.930809, 0.147602}, {0.004733, 0.691367, 0.303900}},
}

func linear(v uint8) float64 {
	x := float64(v) / 255
	if x <= 0.04045 {
		return x / 12.92
	}
	return math.Pow((x+0.055)/1.055, 2.4)
}

// lab is a color in CIELAB (D65) as seen in a vision.
func lab(c [3]uint8, m *[3][3]float64) [3]float64 {
	l := [3]float64{linear(c[0]), linear(c[1]), linear(c[2])}
	if m != nil {
		var s [3]float64
		for i := range s {
			s[i] = math.Min(1, math.Max(0, m[i][0]*l[0]+m[i][1]*l[1]+m[i][2]*l[2]))
		}
		l = s
	}
	x := (0.4124*l[0] + 0.3576*l[1] + 0.1805*l[2]) / 0.95047
	y := 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
	z := (0.0193*l[0] + 0.1192*l[1] + 0.9505*l[2]) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116
	}
	return [3]float64{116*f(y) - 16, 500 * (f(x) - f(y)), 200 * (f(y) - f(z))}
}

// apart is how far apart two colors' bars are in the vision where they
// are closest (ΔE*76), and in lightness.
func apart(a, b string) (de, dl float64) {
	ca, _ := ColorFor(ColorLine, a)
	cb, _ := ColorFor(ColorLine, b)
	de = math.Inf(1)
	for _, m := range cvd {
		la, lb := lab(ca, m), lab(cb, m)
		de = math.Min(de, math.Hypot(math.Hypot(la[0]-lb[0], la[1]-lb[1]), la[2]-lb[2]))
	}
	return de, math.Abs(lab(ca, nil)[0] - lab(cb, nil)[0])
}

// confusable is the ΔE under which two colors count as confusable.
const confusable = 20

// No two colors that are confusable (in some vision) share a border
// style in any format, and their patterns differ in shape; the colors
// that share an ODF style are far apart in every vision and in
// lightness.
func TestPatternsApart(t *testing.T) {
	colors := []string{"red", "blue", "green", "pink", "magenta", "gray", "purple", "cyan", "teal", "yellow", "orange", "brown"}
	pairs := 0
	for i, a := range colors {
		for _, b := range colors[i+1:] {
			pa, pb := BarPattern(a), BarPattern(b)
			if pa.Label == pb.Label || pa.Name == pb.Name {
				t.Errorf("%s and %s: the same pattern %s", a, b, pa.Label)
			}
			de, dl := apart(a, b)
			if de < confusable {
				pairs++
				if pa.ODF == pb.ODF || pa.Differ(pb) == 0 {
					t.Errorf("%s and %s (ΔE %.1f): ODF %s/%s, patterns %s/%s alike", a, b, de, pa.ODF, pb.ODF, pa.Label, pb.Label)
				}
				// (pink and cyan, ΔE 9.5, differ in rhythm only: dashed and
				// dash-dot-dot; no assignment found gave every pair both)
				if de < 12 && pa.Differ(pb) < 2 && !(a == "pink" && b == "cyan") {
					t.Errorf("%s and %s (ΔE %.1f), most confusable: patterns %s/%s differ in one way only", a, b, de, pa.Label, pb.Label)
				}
			}
			if pa.ODF == pb.ODF && (de < 25 || dl < 7) {
				t.Errorf("%s and %s share ODF's %s: ΔE %.1f, ΔL* %.1f", a, b, pa.ODF, de, dl)
			}
		}
	}
	if pairs < 15 { // the bars are close in lightness: many pairs are
		t.Errorf("only %d confusable pairs: the simulation is off", pairs)
	}
}
