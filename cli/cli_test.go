package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
)

const script = "Title: The Barn\n\nINT. BARN - DAY\n\nRain falls.\n\nANNA\nIt's coming.\n"

func TestConvert(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Formats {
		if f.Needs != "" {
			continue
		}
		out := filepath.Join(dir, "barn."+f.Key)
		if err := Convert(context.Background(), Options{Input: in, Output: out, ConfigFile: filepath.Join(dir, "none.toml")}); err != nil {
			t.Errorf("%s: %v", f.Key, err)
			continue
		}
		if data, err := os.ReadFile(out); err != nil || len(data) == 0 {
			t.Errorf("%s: %d bytes, %v", f.Key, len(data), err)
		}
	}
}

func TestWriteParsedScreenplay(t *testing.T) {
	out := filepath.Join(t.TempDir(), "barn.html")
	s := fountain.Parse(nil, strings.NewReader(script))
	if err := Write(context.Background(), s, Options{To: "html", Output: out, ConfigFile: "none.toml"}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(out); !strings.Contains(string(data), "It&#39;s coming.") && !strings.Contains(string(data), "It's coming.") {
		t.Errorf("html lacks the dialogue:\n%s", data)
	}
}

func TestMainExitCodes(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	os.WriteFile(in, []byte(script), 0o644)
	for args, want := range map[string]int{
		"-i " + in + " -o " + filepath.Join(dir, "x.html"):              0,
		"-i " + in + " -to nonsense -o " + filepath.Join(dir, "x.none"): 1,
		"-i " + filepath.Join(dir, "missing.fountain"):                  1,
		"-presets":      0,
		"-no-such-flag": 2,
	} {
		if got := Main(context.Background(), append(strings.Fields(args), "-config", filepath.Join(dir, "none.toml"))); got != want {
			t.Errorf("%s: exit %d, want %d", args, got, want)
		}
	}
}

// The formats made with other programs, where those are installed (CI's
// integration job installs them).
func TestExternalFormats(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "barn.fountain")
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range Formats {
		if f.Needs == "" {
			continue
		}
		if !f.Available() {
			t.Logf("%s: %s is not installed", f.Key, f.Needs)
			continue
		}
		out := filepath.Join(dir, "barn-"+f.Key+"."+f.Extension())
		if err := Convert(context.Background(), Options{Input: in, Output: out, To: f.Key, ConfigFile: filepath.Join(dir, "none.toml")}); err != nil {
			t.Errorf("%s: %v", f.Key, err)
			continue
		}
		if data, err := os.ReadFile(out); err != nil || len(data) == 0 {
			t.Errorf("%s: %d bytes, %v", f.Key, len(data), err)
		}
	}
}
