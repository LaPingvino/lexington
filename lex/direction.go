package lex

import (
	"strings"

	"golang.org/x/text/unicode/bidi"
)

// A line's direction can be set with a mark at its start, which other
// apps read the same way: right to left (RLM) or left to right (LRM).
const (
	LRM = '‎'
	RLM = '‏'
)

// OwnDirection is the direction a line has of its own: by its first
// direction mark, or else its first strong letter (Hebrew, Arabic… read
// right to left, Latin, Cyrillic… left to right). ok is false for a line
// with neither ("…", "2.", a bare number): that one takes its language's
// direction (rules.TOMLConf.RTL), the last resort only.
func OwnDirection(line string) (rtl, ok bool) {
	for _, r := range line {
		switch r {
		case RLM:
			return true, true
		case LRM:
			return false, true
		}
		p, _ := bidi.LookupRune(r)
		switch p.Class() {
		case bidi.R, bidi.AL:
			return true, true
		case bidi.L:
			return false, true
		}
	}
	return false, false
}

// LineRTL tells whether a line reads right to left: by its own direction,
// or, having none, by its language's (rtl).
func LineRTL(line string, rtl bool) bool {
	if r, ok := OwnDirection(line); ok {
		return r
	}
	return rtl
}

// TitleDirection is the direction a script's title page gives it
// ("Direction: rtl", as Hekaya writes it): "rtl", "ltr" or "".
func TitleDirection(s Screenplay) string {
	for _, l := range s {
		if strings.EqualFold(l.Type, "Direction") {
			switch d := strings.ToLower(strings.TrimSpace(string(l.Contents))); d {
			case "rtl", "ltr":
				return d
			}
		}
		if l.Type == TypeScene {
			break
		}
	}
	return ""
}
