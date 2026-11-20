package lex

import "testing"

func TestOwnDirection(t *testing.T) {
	for _, c := range []struct {
		line    string
		rtl, ok bool
	}{
		{"שלום", true, true},
		{"مرحبا", true, true},
		{"Hello", false, true},
		{"2. שלום", true, true},
		{"(INT) בית", false, true}, // the first strong letter decides
		{"‏INT. HOUSE", true, true},
		{"‎שלום", false, true},
		{"…", false, false},
		{"2.", false, false},
		{"", false, false},
	} {
		rtl, ok := OwnDirection(c.line)
		if rtl != c.rtl || ok != c.ok {
			t.Errorf("OwnDirection(%q) = %v, %v; want %v, %v", c.line, rtl, ok, c.rtl, c.ok)
		}
		if got, want := LineRTL(c.line, true), c.rtl || !c.ok; got != want {
			t.Errorf("LineRTL(%q, true) = %v", c.line, got)
		}
		if got := LineRTL(c.line, false); got != c.rtl {
			t.Errorf("LineRTL(%q, false) = %v", c.line, got)
		}
	}
}
