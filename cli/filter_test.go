package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fdx"
	"github.com/LaPingvino/lexington/lex"
)

// EDWARD's sides of Big Fish, in each mode.
func TestBigFishSides(t *testing.T) {
	f, err := os.Open("../testdata/fdx/big-fish/Big-Fish.fdx")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := fdx.Parse(f)
	edward := func(l lex.Line) bool { return l.Type == lex.TypeSpeaker && lex.Speaker(l.Contents) == "EDWARD" }
	scenesWith := 0
	for i, l := range s {
		if l.Type != lex.TypeScene {
			continue
		}
		for _, m := range s[i+1:] {
			if m.Type == lex.TypeScene {
				break
			}
			if edward(m) {
				scenesWith++
				break
			}
		}
	}
	count := func(s lex.Screenplay, f func(lex.Line) bool) (n int) {
		for _, l := range s {
			if f(l) {
				n++
			}
		}
		return n
	}
	speeches := count(s, edward)
	scene := func(l lex.Line) bool { return l.Type == lex.TypeScene }
	if scenesWith < 20 || speeches < 100 {
		t.Fatalf("Big Fish: EDWARD in %d scenes, %d speeches", scenesWith, speeches)
	}

	full := lex.Sides(s, "edward", lex.SidesFull, 0)
	if n := count(full, scene); n != scenesWith || count(full, edward) != speeches {
		t.Errorf("full: %d scenes (want %d), %d speeches (want %d)", n, scenesWith, count(full, edward), speeches)
	}
	iso := lex.Sides(s, "edward", lex.SidesIsolated, 0)
	if count(iso, scene) != scenesWith || count(iso, edward) != speeches ||
		count(iso, func(l lex.Line) bool { return l.Type == lex.TypeSpeaker && !edward(l) }) != 0 {
		t.Errorf("isolated: %d scenes, %d speeches, others' speeches", count(iso, scene), count(iso, edward))
	}
	cue := lex.Sides(s, "edward", lex.SidesCue, 4)
	for i, l := range cue {
		if l.Type == lex.TypeSpeaker && !edward(l) {
			// a cue: one line, its last four words, then EDWARD
			if i+3 >= len(cue) || cue[i+1].Type != lex.TypeDialog || len(strings.Fields(cue[i+1].Contents)) > 4 || !edward(cue[i+3]) {
				t.Fatalf("cue at %d: %q", i, cue[i:min(i+4, len(cue))])
			}
		}
	}
	if count(cue, edward) != speeches {
		t.Errorf("cue: %d speeches, want %d", count(cue, edward), speeches)
	}
}

// The command line's filters: a character needs a mode, tags and colors
// go with every writer.
func TestFilterFlags(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "t.fountain")
	script := "Title: T\n\nINT. PUB - NIGHT [[London]] [[red]]\n\nANNA\nHi.\n\nBOB\nBye.\n\nEXT. ROAD - DAY\n\nBOB\nAlone.\n"
	if err := os.WriteFile(in, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := []string{"-config", filepath.Join(dir, "none.toml")}
	if got := Main(context.Background(), append([]string{"-i", in, "-to", "fountain", "-o", filepath.Join(dir, "x.fountain"), "-character", "BOB"}, conf...)); got != 1 {
		t.Errorf("a character without a mode: exit %d", got)
	}
	run := func(args ...string) string {
		out := filepath.Join(dir, "out."+args[0])
		if got := Main(context.Background(), append(append([]string{"-i", in, "-to", args[0], "-o", out}, args[1:]...), conf...)); got != 0 {
			t.Fatalf("%q: exit %d", args, got)
		}
		b, _ := os.ReadFile(out)
		return string(b)
	}
	if out := run("fountain", "-tag", "london"); strings.Contains(out, "ROAD") || !strings.Contains(out, "INT. PUB - NIGHT [[London]] [[red]]") {
		t.Errorf("tag:\n%s", out)
	}
	if out := run("fountain", "-character", "bob", "-sides", "isolated"); strings.Contains(out, "ANNA") || !strings.Contains(out, "EXT. ROAD") {
		t.Errorf("isolated:\n%s", out)
	}
	if out := run("fountain", "-character", "bob", "-sides", "cue", "-cuewords", "1"); !strings.Contains(out, "ANNA\nHi.\n\nBOB\nBye.") {
		t.Errorf("cue:\n%s", out)
	}
	red, _ := lex.ColorFor(lex.ColorText, "red")
	if out := run("html", "-colorstyle", "text", "-tag", "London"); !strings.Contains(out, `style="color: `+lex.HexOf(red)+`"`) || strings.Contains(out, "ROAD") {
		t.Errorf("colors and tag in html")
	}
	if out := run("html"); strings.Contains(out, `style="color:`) {
		t.Errorf("colors by default")
	}
}
