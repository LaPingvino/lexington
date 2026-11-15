package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/pdfin"
)

// -pagination: on by default (no page ends with a scene heading), none
// as Lexington printed before (one does), a value that is none an error;
// and the same through Options, which Accolade fills in itself.
func TestPaginationFlag(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "t.fountain"), filepath.Join(dir, "t.pdf")
	conf := []string{"-config", filepath.Join(dir, "none.toml")}
	endsWithHeading := func(flags ...string) bool {
		t.Helper()
		found := false
		for pad := 44; pad <= 56; pad++ {
			var b strings.Builder
			b.WriteString("INT. LEAD - DAY\n\n")
			for i := 0; i < pad; i++ {
				fmt.Fprintf(&b, "pad %d.\n", i)
			}
			b.WriteString("\nINT. HOUSE - DAY\n\nIt is quiet.\n\nVery quiet.\n")
			if err := os.WriteFile(in, []byte(b.String()), 0o644); err != nil {
				t.Fatal(err)
			}
			if got := Main(context.Background(), append(append([]string{"-i", in, "-o", out}, conf...), flags...)); got != 0 {
				t.Fatalf("%q: exit %d", flags, got)
			}
			pages, err := pdfin.Pages(out)
			if err != nil {
				t.Fatal(err)
			}
			if p := pages[0]; len(pages) > 1 && p[len(p)-1].Text == "INT. HOUSE - DAY" {
				found = true
			}
		}
		return found
	}
	if endsWithHeading() || endsWithHeading("-pagination", "keep") {
		t.Error("a page ends with a scene heading")
	}
	if !endsWithHeading("-pagination", "none") || !endsWithHeading("-pagination", "widows+more") {
		t.Error("without keep, no page ends with a scene heading")
	}
	if got := Main(context.Background(), append([]string{"-i", in, "-o", out, "-pagination", "tidy"}, conf...)); got != 1 {
		t.Errorf("-pagination tidy: exit %d", got)
	}
	if err := Convert(context.Background(), Options{Input: in, Output: out, ConfigFile: filepath.Join(dir, "none.toml"), Pagination: "off"}); err != nil {
		t.Error(err)
	}
}
