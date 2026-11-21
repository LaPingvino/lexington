package pdf

import (
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

func TestUnsupported(t *testing.T) {
	line := func(s string) lex.Screenplay { return lex.Screenplay{{Type: lex.TypeAction, Contents: lex.Content(s)}} }
	if m, shaping := Unsupported(line("Ĉu vi parolas? Tiếng Việt, Русский.")); len(m) != 0 || shaping {
		t.Errorf("Latin, Vietnamese, Cyrillic: missing %q, shaping %v", string(m), shaping)
	}
	// right to left alone needs no shaping: Hebrew prints as written
	if m, shaping := Unsupported(line("‏שָׁלוֹם ׯ")); len(m) != 0 || shaping {
		t.Errorf("Hebrew (in the font since Courier Badi 1.100): missing %q, shaping %v", string(m), shaping)
	}
	if m, shaping := Unsupported(line("مرحبا")); len(m) != 0 || !shaping {
		t.Errorf("Arabic (in the font, its letters join): missing %q, shaping %v", string(m), shaping)
	}
	if _, shaping := Unsupported(line("ܫܠܡܐ")); !shaping {
		t.Errorf("Syriac joins: shaping %v", shaping)
	}
	if _, shaping := Unsupported(line("١٢٣ ،؟")); shaping {
		t.Errorf("Arabic digits and punctuation join nothing: shaping %v", shaping)
	}
	if m, _ := Unsupported(line("你好你")); string(m) != "你好" {
		t.Errorf("Chinese: missing %q", string(m))
	}
}
