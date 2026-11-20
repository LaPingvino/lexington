package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// shootBreakdown is shoot.fountain's breakdown: a revolver in the kitchen
// twice, a taxi, CARL tagged silent (and, as he is only in the omitted
// scene's boneyard, without a number of his own until tagged), the
// neighbour's dog and a category of one's own.
func shootBreakdown() Breakdown {
	return Breakdown{
		Categories: []lex.TagCategory{{Name: "Puppets", Color: "#123456"}},
		Elements: map[int][]Element{
			1: {{"Props", "revolver"}, {"Cast Members", "Carl"}, {"Props", "Revolver"}, {"Wardrobe", "raincoat"}},
			2: {{"Vehicles", "taxi"}, {"Animals", "dog"}, {"Puppets", "dog puppet"}, {"Background Actors", "neighbours"}},
			4: {{"Props", "revolver"}, {"Cast", "ANNA"}},
			5: {{"Animals", "dog"}, {"Cast Members", "carl"}},
		},
	}
}

func TestSheets(t *testing.T) {
	r := shoot(t)
	b := shootBreakdown()
	sheets := r.Sheets(b)
	if len(sheets) != 4 { // the omitted scene has none
		t.Fatalf("%d sheets", len(sheets))
	}
	s := sheets[0]
	if s.ID() != "1" || s.Synopsis != "Anna and Tom wait out the rain." || strings.Join(s.Cast, ",") != "1 ANNA,2 TOM,3 CARL" {
		t.Errorf("sheet 1: %+v", s)
	}
	if len(s.Groups) != 2 || s.Groups[0].Category.Name != "Props" || strings.Join(s.Groups[0].Names, ",") != "revolver" ||
		s.Groups[1].Category.Name != "Wardrobe" {
		t.Errorf("sheet 1's groups: %+v", s.Groups)
	}
	if g := sheets[1].Groups; len(g) != 4 || g[3].Category.Name != "Puppets" || g[0].Category.Name != "Background Actors" {
		t.Errorf("sheet 1A's groups: %+v", g)
	}
	if c := sheets[3].Cast; strings.Join(c, ",") != "3 CARL" {
		t.Errorf("sheet 4's cast %v", c)
	}
}

// The breakdown and element CSVs are stable (PRODUCTION.md §13); -update
// rewrites them.
func TestBreakdownCSVGolden(t *testing.T) {
	r := shoot(t)
	b := shootBreakdown()
	for kind, tab := range map[string]Table{KindSheets: b.SheetTable(r.Sheets(b)), KindElements: r.ElementTable(b)} {
		var buf bytes.Buffer
		if err := tab.WriteCSV(&buf); err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("testdata", kind+".csv")
		if *update {
			if err := os.WriteFile(golden, buf.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(buf.Bytes(), want) {
			t.Errorf("%s.csv:\n%s\nwant\n%s", kind, buf.String(), want)
		}
	}
}

func TestSheetsPDF(t *testing.T) {
	r := shoot(t)
	b := shootBreakdown()
	var buf bytes.Buffer
	if err := WriteSheetsPDF(&buf, "The Shed", r.Sheets(b), "a4"); err != nil {
		t.Fatal(err)
	}
	pdf := buf.String()
	if !strings.HasPrefix(pdf, "%PDF-") || strings.Count(pdf, "/Type /Page\n") != 4 {
		t.Errorf("%d pages", strings.Count(pdf, "/Type /Page\n"))
	}
	if p := os.Getenv("SHEET_PDF"); p != "" {
		_ = os.WriteFile(p, buf.Bytes(), 0o644)
	}
}

func TestKey(t *testing.T) {
	for in, want := range map[string]string{"Makeup/Hair": "makeup_hair", "Background Actors": "background_actors",
		"Notes": "notes", " Dog & Cat ": "dog_cat"} {
		if got := Key(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
