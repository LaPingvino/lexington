package pdf

import "testing"

// A code far into a bfrange maps by its whole distance (Identity ranges
// such as <0000> <FFFF> <0000> carry Cyrillic and other scripts).
func TestAddOffset(t *testing.T) {
	for _, c := range []struct {
		dst  string
		off  uint32
		want string
	}{
		{"\\x00\\x00", 0x0418, "\\x04\\x18"}, // И
		{"\\x00\\xff", 1, "\\x01\\x00"},      // carry
		{"\\x00\\x41", 2, "\\x00\\x43"},
	} {
		if got := addOffset(c.dst, c.off); got != c.want {
			t.Errorf("%q + %#x = %q, want %q", c.dst, c.off, got, c.want)
		}
	}
	if codeValue("\\x04\\x18") != 0x0418 {
		t.Error("codeValue")
	}
}
