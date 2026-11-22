package report

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

var update = flag.Bool("update", false, "rewrite the golden CSV files")

func shoot(t *testing.T) Report {
	t.Helper()
	data, err := os.ReadFile("testdata/shoot.fountain")
	if err != nil {
		t.Fatal(err)
	}
	s := fountain.Parse(rules.DefaultConf().Scenes("en"), bytes.NewReader(data))
	return Make(s, Options{Aliases: map[string]string{"Annie": "ANNA"}})
}

func TestScenes(t *testing.T) {
	r := shoot(t)
	type row struct {
		id, place, location, time string
		page                      int
		eighths                   lex.Eighths
		chars                     []string
		omitted                   bool
	}
	want := []row{
		{"1", "INT", "KOVAČ HOUSE - KITCHEN", "DAY", 1, 2, []string{"ANNA", "TOM"}, false},
		{"1A", "EXT", "GARDEN", "NIGHT", 1, 1, []string{"TOM", "ANNA"}, false}, // ANNIE is ANNA
		{"2", "", "", "", 1, 0, nil, true},                                     // CARL is in the boneyard
		{"3", "INT", "KOVAČ HOUSE - KITCHEN", "NIGHT", 1, 1, []string{"ANNA", "TOM"}, false},
		{"4", "EXT", "GARDEN", "DAY", 1, 1, nil, false},
	}
	if len(r.Scenes) != len(want) {
		t.Fatalf("%d scenes: %+v", len(r.Scenes), r.Scenes)
	}
	for i, s := range r.Scenes {
		w := want[i]
		got := row{s.ID(), s.Place, s.Location, s.Time, s.Page, s.Eighths, s.Characters, s.Omitted}
		if !reflect.DeepEqual(got, w) {
			t.Errorf("scene %d: %+v, want %+v", i+1, got, w)
		}
	}
	if r.Scenes[2].Heading != "OMITTED" || r.Scenes[1].Heading != "EXT. GARDEN - NIGHT" {
		t.Errorf("headings %q, %q", r.Scenes[2].Heading, r.Scenes[1].Heading)
	}
}

func TestLocations(t *testing.T) {
	r := shoot(t)
	want := []Location{
		{Name: "KOVAČ HOUSE - KITCHEN", Places: []string{"INT"}, Times: []string{"DAY", "NIGHT"}, Scenes: []string{"1", "3"}, Eighths: 3},
		{Name: "GARDEN", Places: []string{"EXT"}, Times: []string{"NIGHT", "DAY"}, Scenes: []string{"1A", "4"}, Eighths: 2},
	}
	if !reflect.DeepEqual(r.Locations, want) {
		t.Errorf("locations\n%+v\nwant\n%+v", r.Locations, want)
	}
}

func TestCharacters(t *testing.T) {
	r := shoot(t)
	want := []Character{
		{Name: "ANNA", Scenes: []string{"1", "1A", "3"}, First: "1", Last: "3", FirstPage: 1, LastPage: 1,
			Speeches: 4, Words: 12, Eighths: 4},
		{Name: "TOM", Scenes: []string{"1", "1A", "3"}, First: "1", Last: "3", FirstPage: 1, LastPage: 1,
			Speeches: 3, Words: 10, Eighths: 4},
	}
	if !reflect.DeepEqual(r.Characters, want) {
		t.Errorf("characters\n%+v\nwant\n%+v", r.Characters, want)
	}
	cast := r.Cast()
	if len(cast) != 2 || cast[0].Name != "ANNA" || cast[0].Number != 1 || cast[1].Name != "TOM" || cast[1].Number != 2 {
		t.Errorf("cast %+v", cast)
	}
	// totals are the sum of their scenes' rounded lengths
	var sum lex.Eighths
	for _, s := range r.Scenes {
		if slicesContains(s.Characters, "TOM") {
			sum += s.Eighths
		}
	}
	if sum != r.Characters[1].Eighths {
		t.Errorf("TOM's %s, his scenes' %s", r.Characters[1].Eighths, sum)
	}
}

func slicesContains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// The cast is by size: scenes, then length, speeches, words; a tie to
// who speaks first.
func TestCastOrder(t *testing.T) {
	r := Report{Characters: []Character{
		{Name: "ZED", Scenes: []string{"1"}, Speeches: 1},
		{Name: "BEA", Scenes: []string{"1", "2"}, Eighths: 3},
		{Name: "AL", Scenes: []string{"1", "2"}, Eighths: 5},
		{Name: "CY", Scenes: []string{"1"}, Speeches: 1},
	}}
	var got []string
	for _, c := range r.Cast() {
		got = append(got, c.Name)
	}
	if strings.Join(got, " ") != "AL BEA ZED CY" {
		t.Errorf("cast %v", got)
	}
}

// The CSV layouts are stable: the golden files are what other programs
// read (docs in Accolade's PRODUCTION.md). Run with -update to rewrite.
func TestCSVGolden(t *testing.T) {
	r := shoot(t)
	for _, kind := range Kinds {
		tab, err := r.Table(kind)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := tab.WriteCSV(&b); err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join("testdata", kind+".csv")
		if *update {
			if err := os.WriteFile(golden, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b.Bytes(), want) {
			t.Errorf("%s.csv:\n%s\nwant\n%s", kind, b.String(), want)
		}
	}
	if _, err := r.Table("props"); err == nil {
		t.Error("no error for an unknown report")
	}
}

func TestHeading(t *testing.T) {
	starts := rules.DefaultConf().Scenes("en")
	for _, c := range []struct{ in, place, location, time, number string }{
		{"INT. HOUSE - DAY #12A#", "INT", "HOUSE", "DAY", "12A"},
		{"INT./EXT. CAR - MOVING - NIGHT", "INT./EXT", "CAR - MOVING", "NIGHT", ""},
		{"I/E BUS – DAWN [[note]]", "I/E", "BUS", "DAWN", ""},
		{".THE VOID", "", "THE VOID", "", ""},
		{"ext. beach", "EXT", "BEACH", "", ""},
		{"INTERIOR DESIGN STUDIO - DAY", "", "INTERIOR DESIGN STUDIO", "DAY", ""},
	} {
		s := heading(c.in, starts)
		if s.Place != c.place || s.Location != c.location || s.Time != c.time || s.Number != c.number {
			t.Errorf("%q: %q %q %q %q", c.in, s.Place, s.Location, s.Time, s.Number)
		}
	}
}

func TestHTML(t *testing.T) {
	tab, _ := shoot(t).Table(KindScenes)
	var b bytes.Buffer
	if err := WriteHTML(&b, "The Shed <scenes>", "5 scenes", tab); err != nil {
		t.Fatal(err)
	}
	page := b.String()
	for _, want := range []string{"The Shed &lt;scenes&gt;", "<th>Location</th>", "<td>OMITTED</td>", "KOVAČ HOUSE"} {
		if !strings.Contains(page, want) {
			t.Errorf("no %q in\n%s", want, page)
		}
	}
	if strings.Contains(page, "<th>Heading</th>") {
		t.Error("the CSV's data columns are printed")
	}
}
