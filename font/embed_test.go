package font

import (
	"bytes"
	"testing"
)

func TestEachStyleHasItsOwnFont(t *testing.T) {
	seen := map[string]string{}
	for _, style := range []string{"", "I", "B", "BI"} {
		data := GetFont(CourierPrimeName, style)
		if !bytes.HasPrefix(data, []byte{0, 1, 0, 0}) {
			t.Fatalf("style %q: not a TrueType font", style)
		}
		if prev, ok := seen[string(data[:4096])]; ok {
			t.Errorf("style %q uses the same font as style %q", style, prev)
		}
		seen[string(data[:4096])] = style
	}
	if !bytes.Equal(GetFont(CourierBadiName, "ib"), CourierBadiBoldItalic) {
		t.Error(`"ib" should give bold italic`)
	}
}
