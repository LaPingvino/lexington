package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Big Fish (fountain.io's FDX sample) through Fountain, HTML and FDX:
// what Final Draft shows in capitals is in capitals, scene numbers stay,
// and its bold and underlined THE END is both, everywhere.
func TestBigFish(t *testing.T) {
	dir := t.TempDir()
	in, err := filepath.Abs("../testdata/fdx/big-fish/Big-Fish.fdx")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range []string{"fountain", "html", "fdx"} {
		o := Options{Input: in, Output: filepath.Join(dir, "bigfish-out."+f), To: f, ConfigFile: filepath.Join(dir, "none.toml")}
		if err := Convert(context.Background(), o); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(o.Output)
		if err != nil {
			t.Fatal(err)
		}
		out[f] = string(data)
	}
	for f, wants := range map[string][]string{
		"fountain": {"\nEDWARD (V.O.)\n", "\nINT.  WILL’S BEDROOM - NIGHT (1973) #2#\n", "\n>_**THE END**_<\n", "\nFLASHCUTS TO:\n"},
		"html":     {">EDWARD (V.O.)<", `<span class="scene-number left">2</span>`, "<b><u>THE END</u></b>"},
		"fdx": {`<Paragraph Type="Scene Heading" Number="2">`, `<Text Style="Bold+Underline">THE END</Text>`,
			`<ElementSettings Type="Scene Heading">`},
	} {
		for _, w := range wants {
			if !strings.Contains(out[f], w) {
				t.Errorf("%s lacks %q", f, w)
			}
		}
	}
	for _, bad := range []string{"@edward", ".int.", "_**THE END**_"} {
		if strings.Contains(out["html"], bad) || (bad != "_**THE END**_" && strings.Contains(out["fountain"], bad)) {
			t.Errorf("prints %q", bad)
		}
	}
}
