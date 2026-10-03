package fdx

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
)

// significant drops what FDX has no place for: blank lines (Final Draft
// spaces blocks itself) and Fountain's outline elements. Title page
// fields are reduced to their role, sorted: Final Draft title pages are
// plain paragraphs, so field names and order are not kept.
func significant(sp lex.Screenplay) []string {
	var title, body []string
	inTitle := len(sp) > 0 && sp[0].Type == lex.TypeTitlePage
	for _, l := range sp {
		if inTitle {
			switch {
			case l.Type == lex.TypeNewPage:
				inTitle = false
			case l.Contents != "":
				title = append(title, titleRole(l.Type)+"="+l.Contents)
			}
			continue
		}
		switch l.Type {
		case lex.TypeEmpty, "section", "synopse", "note":
			continue
		}
		body = append(body, string(l.Type)+"="+l.Contents)
	}
	sort.Strings(title)
	return append(title, body...)
}

func TestFountainFDXRoundTrip(t *testing.T) {
	files, err := filepath.Glob("../testdata/input/*.fountain")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test scripts found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			original := fountain.Parse(scenes, bytes.NewReader(src))

			var fdxDoc bytes.Buffer
			if werr := (&FDXWriter{}).Write(&fdxDoc, original); werr != nil {
				t.Fatal(werr)
			}
			back, err := ParseWithError(&fdxDoc)
			if err != nil {
				t.Fatal(err)
			}

			want, got := significant(original), significant(back)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				for i := 0; i < len(want) || i < len(got); i++ {
					var w, g string
					if i < len(want) {
						w = want[i]
					}
					if i < len(got) {
						g = got[i]
					}
					if w != g {
						t.Errorf("first difference at element %d:\n  fountain: %q\n  via FDX:  %q", i, w, g)
						break
					}
				}
			}
		})
	}
}
