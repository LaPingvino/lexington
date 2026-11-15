package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// Printed, the page breaks as the pagination says: headings, names and
// parentheticals (and the blank lines after them) kept with what follows,
// what is before a transition with it, speeches and dual dialogue
// whole, two lines of a paragraph on each side of a break; without it,
// none of that.
func TestHTMLPagination(t *testing.T) {
	s := lex.Screenplay{{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"}, {Type: lex.TypeEmpty},
		{Type: lex.TypeAction, Contents: "A line."}}
	rulesOn := []string{
		".scene-heading + .empty, .speaker + .empty, .parenthetical + .empty",
		".dialogue:has(+ .parenthetical), :has(+ .transition), :has(+ .empty + .transition) { break-after: avoid;",
		".dialogue, .transition, .dual-dialogue { break-inside: avoid;",
		".lyrics { orphans: 2; widows: 2; }",
	}
	for _, p := range []rules.Pagination{rules.AllPagination, {}} {
		var b bytes.Buffer
		if err := (&HTMLWriter{Elements: rules.Default.WithPagination(p)}).Write(&b, s); err != nil {
			t.Fatal(err)
		}
		for _, r := range rulesOn {
			if strings.Contains(b.String(), r) != p.Keep && !strings.Contains(r, "orphans") ||
				strings.Contains(r, "orphans") && strings.Contains(b.String(), r) != p.Widows {
				t.Errorf("pagination %s: %q is %v", p, r, !p.Keep)
			}
		}
		if !p.Widows && !strings.Contains(b.String(), "orphans: 1; widows: 1;") {
			t.Errorf("pagination %s: widows not off", p)
		}
	}
}
