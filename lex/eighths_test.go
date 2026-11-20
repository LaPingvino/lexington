package lex

import "testing"

func TestEighths(t *testing.T) {
	for _, c := range []struct {
		pages float64
		want  Eighths
		text  string
	}{
		{1, 8, "1"},              // a page is 8/8
		{0.5, 4, "4/8"},          // half a page: 4/8, never 1/2
		{0.01, 1, "1/8"},         // no scene is shorter than an eighth
		{0.0624, 1, "1/8"},       // just under half an eighth: still one
		{3.0 / 16, 2, "2/8"},     // one and a half eighths: up
		{0.18, 1, "1/8"},         // under one and a half: down
		{1 + 3.0/8, 11, "1 3/8"}, // pages and eighths
		{2.97, 24, "3"},          // to the nearest: 23.76 eighths
		{0, 0, "0"},              // nothing is nothing
	} {
		got := EighthsOf(c.pages)
		if got != c.want || got.String() != c.text {
			t.Errorf("EighthsOf(%v) = %d (%s), want %d (%s)", c.pages, got, got, c.want, c.text)
		}
	}
}
