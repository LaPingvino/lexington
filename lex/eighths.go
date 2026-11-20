package lex

import (
	"fmt"
	"math"
)

// Eighths is a length of script in eighths of a page, the measure a
// production breaks a script down and schedules it in: a page is eight
// eighths, and a length is written as whole pages and eighths, never
// reduced: 4/8 (not 1/2), 1 3/8, 2 (two whole pages).
type Eighths int

// EighthsOf is a scene's length in pages as eighths: to the nearest
// eighth (a half eighth up), and at least one, as no scene is shorter
// than an eighth of a page (Scriptation: "Every scene should be at least
// 1/8", https://help.scriptation.com/en/article/how-do-i-add-page-counts-to-my-script-3jt60r/).
// A length of nothing (or less) is none.
func EighthsOf(pages float64) Eighths {
	if pages <= 0 {
		return 0
	}
	n := Eighths(math.Floor(pages*8 + 0.5 + 1e-9))
	return max(n, 1)
}

// Pages are the whole pages in the length.
func (e Eighths) Pages() int { return int(e) / 8 }

// Rest is the eighths after the whole pages.
func (e Eighths) Rest() int { return int(e) % 8 }

// String is the length as productions write it: "3/8", "1", "1 3/8"
// ("0" for none).
func (e Eighths) String() string {
	switch {
	case e <= 0:
		return "0"
	case e.Rest() == 0:
		return fmt.Sprint(e.Pages())
	case e.Pages() == 0:
		return fmt.Sprintf("%d/8", e.Rest())
	}
	return fmt.Sprintf("%d %d/8", e.Pages(), e.Rest())
}
