package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// TestLockFlags: -lockpages writes the lock, -lock keeps it in the PDF.
func TestLockFlags(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "pages.txt")
	run := func(args ...string) {
		t.Helper()
		o, ok, err := parseFlags(append([]string{"-config", filepath.Join(dir, "none.toml")}, args...))
		if err != nil || !ok {
			t.Fatal(ok, err)
		}
		if err := Convert(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
	in := "../testdata/input/complex_screenplay.fountain"
	run("-i", in, "-to", "pdf", "-lockpages", "-continueds", "-o", lock)
	data, err := os.ReadFile(lock)
	if err != nil {
		t.Fatal(err)
	}
	l := lex.ParsePageLock(string(data))
	if l == nil || !l.Continueds || len(l.Pages) == 0 || !strings.HasPrefix(string(data), "pages: locked, letter, continueds\n  1 ") {
		t.Fatalf("lock:\n%s", data)
	}
	out := filepath.Join(dir, "locked.pdf")
	run("-i", in, "-to", "pdf", "-lock", lock, "-o", out)
	if st, err := os.Stat(out); err != nil || st.Size() == 0 {
		t.Fatal(err)
	}
}
