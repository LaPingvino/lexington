package pdf

import (
	"strings"
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
	if m, shaping := Unsupported(line("مرحبا، لا إله إلا الله")); len(m) != 0 || shaping {
		t.Errorf("Arabic (shaped since 1.27): missing %q, shaping %v", string(m), shaping)
	}
	// a letter whose joined form the font has only as a glyph
	if _, shaping := Unsupported(line("بڠب")); !shaping {
		t.Errorf("an extended Arabic letter's medial form: shaping %v", shaping)
	}
	if _, shaping := Unsupported(line("ܫܠܡܐ")); !shaping {
		t.Errorf("Syriac joins: shaping %v", shaping)
	}
	if _, shaping := Unsupported(line("١٢٣ ،؟")); shaping {
		t.Errorf("Arabic digits and punctuation join nothing: shaping %v", shaping)
	}
	// Chinese: Courier Badi has none; a font on the computer may
	want := "你好"
	if len(systemHas([]rune("你好"))) == 2 {
		want = ""
	}
	if m, _ := Unsupported(line("你好你")); string(m) != want {
		t.Errorf("Chinese: missing %q, want %q", string(m), want)
	}
	if s := Unshaped(line("ܫܠܡܐ ߒߞߏ")); strings.Join(s, ",") != "Nko,Syriac" {
		t.Errorf("Syriac and N'Ko: unshaped %q", s)
	}
}
