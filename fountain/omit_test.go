package fountain

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

func TestOmitRestore(t *testing.T) {
	scene := "INT. HOUSE - DAY #12# [[red]]\n\nAnna waits. /* a boneyard of its own */\n\nANNA\nWell?\n\n"
	omitted, ok := OmitScene(scene)
	if !ok {
		t.Fatal("not omitted")
	}
	want := ".OMITTED #12#\n\n/* Omitted 12\nINT. HOUSE - DAY #12# [[red]]\n\nAnna waits. /* a boneyard of its own * /\n\nANNA\nWell?\n*/\n\n"
	if omitted != want {
		t.Errorf("omitted:\n%q\nwant\n%q", omitted, want)
	}
	if !IsOmitted(omitted) {
		t.Error("IsOmitted")
	}
	back, ok := RestoreScene(omitted)
	if !ok || back != scene {
		t.Errorf("restored %v:\n%q", ok, back)
	}
	// the last scene, no new line at the end
	if o, _ := OmitScene("EXT. END #3#\n\nBye."); o != ".OMITTED #3#\n\n/* Omitted 3\nEXT. END #3#\n\nBye.\n*/" {
		t.Errorf("last: %q", o)
	} else if b, _ := RestoreScene(o); b != "EXT. END #3#\n\nBye." {
		t.Errorf("last back: %q", b)
	}
	for _, s := range []string{"INT. HOUSE - DAY\n\nNo number.\n", omitted} {
		if _, ok := OmitScene(s); ok {
			t.Errorf("omitted %q", s)
		}
	}
	if _, ok := RestoreScene(".OMITTED #12#\n\n"); ok {
		t.Error("restored without its block")
	}
	if _, ok := RestoreScene("INT. HOUSE - DAY #12#\n"); ok {
		t.Error("restored a scene")
	}
}

// Other Fountain readers see OMITTED with its number; the old scene is
// boneyard, not printed.
func TestOmittedParses(t *testing.T) {
	o, _ := OmitScene("INT. HOUSE - DAY #12#\n\nAnna waits.\n\n")
	s := Parse([]string{"INT", "EXT"}, strings.NewReader("INT. A - DAY #11#\n\nHi.\n\n"+o+"INT. B - DAY #13#\n\nHo.\n"))
	var scenes []string
	for _, l := range s {
		switch l.Type {
		case lex.TypeScene:
			scenes = append(scenes, l.Contents)
		case lex.TypeAction:
			if strings.Contains(l.Contents, "Anna") {
				t.Errorf("the omitted scene printed: %q", l.Contents)
			}
		}
	}
	if strings.Join(scenes, "|") != "INT. A - DAY #11#|OMITTED #12#|INT. B - DAY #13#" {
		t.Errorf("scenes %q", scenes)
	}
}

// A merged run of omitted scenes splits back into one OMITTED heading a
// block, each restorable as it was.
func TestSplitOmitted(t *testing.T) {
	a, _ := OmitScene("INT. A - DAY #12#\n\nOne.\n\n")
	b, _ := OmitScene("INT. B - DAY #13#\n\nTwo.\n\n")
	// as Accolade merges them: the second heading and its blank line go
	merged := ".OMITTED #12-13#\n\n" + strings.TrimPrefix(a, ".OMITTED #12#\n\n") + strings.TrimPrefix(b, ".OMITTED #13#\n\n") + "EXT. C - DAY #14#\n"
	split, ok := SplitOmitted(merged)
	if !ok || split != a+b+"EXT. C - DAY #14#\n" {
		t.Fatalf("split %v:\n%q\nwant\n%q", ok, split, a+b+"EXT. C - DAY #14#\n")
	}
	if n, _ := lex.OmittedNumber(".OMITTED #12-13#"); n != "12-13" {
		t.Errorf("range number %q", n)
	}
	for _, s := range []string{a, "INT. A - DAY #12-13#\n", ".OMITTED #I-1-A#\n\n/* Omitted I\n*/\n", ".OMITTED #14-12#\n"} {
		if _, ok := SplitOmitted(s); ok {
			t.Errorf("split %q", s)
		}
	}
	// the ends kept where their block is gone; the middle has no name
	if got, ok := SplitOmitted(".OMITTED #12-14#\n\nNo block.\n"); !ok || got != ".OMITTED #12#\n\n.OMITTED #14#\n\nNo block.\n" {
		t.Errorf("no blocks: %v %q", ok, got)
	}
	if got, _ := SplitOmitted(".OMITTED #12-14#\n\n" + strings.TrimPrefix(b, ".OMITTED #13#\n\n")); got != ".OMITTED #12#\n\n"+strings.TrimSuffix(b, "\n\n")+"\n\n.OMITTED #14#\n\n" {
		t.Errorf("one block:\n%q", got)
	}
	if _, ok := RestoreScene(merged); ok {
		t.Error("restored a merged run whole")
	}
}

// A v1.13.0 file's merged run reads as its scenes: Lexington prints and
// writes each number (and merges them again only when asked).
func TestOldOmittedRunParses(t *testing.T) {
	a, _ := OmitScene("INT. A - DAY #12#\n\nOne.\n\n")
	b, _ := OmitScene("INT. B - DAY #13#\n\nTwo.\n\n")
	text := "INT. X - DAY #11#\n\nHi.\n\n.OMITTED #12-13#\n\n" + strings.TrimPrefix(a, ".OMITTED #12#\n\n") +
		strings.TrimPrefix(b, ".OMITTED #13#\n\n") + "EXT. C - DAY #14#\n"
	var scenes []string
	for _, l := range ParseLines([]string{"INT", "EXT"}, strings.NewReader(text)) {
		if l.Type == lex.TypeScene {
			scenes = append(scenes, fmt.Sprintf("%s@%d", l.Contents, l.Source))
		}
	}
	if got := strings.Join(scenes, "|"); got != "INT. X - DAY #11#@1|OMITTED #12#@5|OMITTED #13#@13|EXT. C - DAY #14#@19" {
		t.Errorf("scenes %q", got)
	}
}
