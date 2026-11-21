package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
)

// -strip writes the Fountain input without what it names, as it is
// (byte for byte where nothing went), or converts what is left.
func TestStripFlag(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "s.fountain")
	text := "Title: Rain\n\nINT. A - DAY #1#\n\nAnna waits. [[from: \"She waited.\"]] [[slow]]\n\n/* Notes\ncall\n*/\n"
	if err := os.WriteFile(in, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(want int, args ...string) string {
		t.Helper()
		out := filepath.Join(dir, "out")
		os.Remove(out)
		args = append([]string{"-i", in, "-config", filepath.Join(dir, "none.toml")}, args...)
		if got := Main(context.Background(), args); got != want {
			t.Fatalf("%q: exit %d, want %d", args, got, want)
		}
		b, _ := os.ReadFile(out)
		return string(b)
	}
	out := filepath.Join(dir, "out")
	if got := run(0, "-strip", "clean", "-to", "fountain", "-o", out); got != "Title: Rain\n\nINT. A - DAY #1#\n\nAnna waits. [[slow]]\n" {
		t.Errorf("clean: %q", got)
	}
	if got := run(0, "-strip", "clean", "-o", out); got != "Title: Rain\n\nINT. A - DAY #1#\n\nAnna waits. [[slow]]\n" {
		t.Errorf("no -to, a name without an extension: Fountain as is, got %q", got)
	}
	if got := run(0, "-strip", "writer,notes,titlepage", "-to", "fountain", "-o", out); got != "INT. A - DAY\n\nAnna waits.\n" {
		t.Errorf("writer,notes,titlepage: %q", got)
	}
	if got := run(0, "-strip", "notes", "-to", "html", "-o", out); strings.Contains(got, "slow") || strings.Contains(got, "call") || !strings.Contains(got, "Anna waits.") {
		t.Errorf("html: %s", got)
	}
	run(1, "-strip", "nothing", "-to", "fountain", "-o", out)
	run(1, "-strip", "clean", "-to", "fountain", "-o", in) // never over the input
	if b, _ := os.ReadFile(in); string(b) != text {
		t.Error("the input changed")
	}
	// nothing to take: the same bytes
	plain := "INT. A - DAY\r\n\r\nAnna waits.  \r\n"
	os.WriteFile(in, []byte(plain), 0o644)
	if got := run(0, "-strip", "clean", "-o", out+".fountain"); got != "" {
		t.Errorf("wrote %q to out", got)
	}
	if b, _ := os.ReadFile(out + ".fountain"); string(b) != plain {
		t.Errorf("unchanged: %q", b)
	}
	if fountain.StripList() == "" {
		t.Error("no keys")
	}
}
