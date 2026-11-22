package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/report"
	"github.com/LaPingvino/lexington/rules"
)

func TestReportFlag(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "cast.csv")
	o, ok, err := parseFlags([]string{"-config", filepath.Join(dir, "none.toml"), "-i", "../report/testdata/shoot.fountain",
		"-report", "scenes", "-o", out})
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if err := Convert(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "order,number,heading,") || !strings.Contains(string(data), "1A,EXT. GARDEN - NIGHT") {
		t.Errorf("scenes.csv:\n%s", data)
	}
	o.Report = "props"
	if err := Convert(context.Background(), o); err == nil {
		t.Error("no error for -report props")
	}
}

// A4 holds more lines than Letter: the report measures the paper asked.
func TestReportPaper(t *testing.T) {
	s := fountain.Parse(rules.DefaultConf().Scenes("en"), strings.NewReader(strings.Repeat("INT. A - DAY\n\n"+strings.Repeat("Line.\n\n", 20), 3)))
	letter, err := Report(s, Options{ConfigFile: filepath.Join(t.TempDir(), "none.toml")}, report.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a4, err := Report(s, Options{ConfigFile: filepath.Join(t.TempDir(), "none.toml"), Page: "a4"}, report.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if a4.Length() >= letter.Length() {
		t.Errorf("A4 %s, Letter %s", a4.Length(), letter.Length())
	}
}
