package pdf

import (
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

func TestUnsupported(t *testing.T) {
	line := func(s string) lex.Screenplay { return lex.Screenplay{{Type: lex.TypeAction, Contents: lex.Content(s)}} }
	if m, rtl := Unsupported(line("Ĉu vi parolas? Tiếng Việt, Русский.")); len(m) != 0 || rtl {
		t.Errorf("Latin, Vietnamese, Cyrillic: missing %q, rtl %v", string(m), rtl)
	}
	if m, rtl := Unsupported(line("‏שלום")); len(m) != 4 || !rtl {
		t.Errorf("Hebrew: missing %q, rtl %v", string(m), rtl)
	}
	if m, rtl := Unsupported(line("مرحبا")); len(m) != 0 || !rtl {
		t.Errorf("Arabic (in the font): missing %q, rtl %v", string(m), rtl)
	}
	if m, _ := Unsupported(line("你好你")); string(m) != "你好" {
		t.Errorf("Chinese: missing %q", string(m))
	}
}
