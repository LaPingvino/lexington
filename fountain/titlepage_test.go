package fountain

import (
	"strings"
	"testing"
)

var testScenes = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

// types renders the parse of src as "type=contents" lines, skipping empties.
func types(src string) string {
	var out []string
	for _, l := range Parse(testScenes, strings.NewReader(src)) {
		if l.Type != "empty" {
			out = append(out, string(l.Type)+"="+l.Contents)
		}
	}
	return strings.Join(out, "\n")
}

func TestTitlePageFollowedByFadeIn(t *testing.T) {
	got := types("Title: The Barn\nAuthor: Jane Smith\n\nFADE IN:\n\nINT. BARN - DAY\n")
	want := "titlepage=\nTitle=The Barn\nAuthor=Jane Smith\nnewpage=\naction=FADE IN:\nscene=INT. BARN - DAY"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestScriptOpeningWithFadeIn(t *testing.T) {
	got := types("FADE IN:\n\nINT. BARN - DAY\n")
	want := "action=FADE IN:\nscene=INT. BARN - DAY"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestScriptOpeningWithSceneHeadingContainingColon(t *testing.T) {
	got := types("INT. BARN: LOFT - DAY\n\nHay everywhere.\n")
	want := "scene=INT. BARN: LOFT - DAY\naction=Hay everywhere."
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLeadingBlankLinesDoNotMakeATitlePage(t *testing.T) {
	got := types("\n\n\nINT. BARN - DAY\n")
	if got != "scene=INT. BARN - DAY" {
		t.Errorf("got:\n%s", got)
	}
}

func TestTitlePageMultiLineValues(t *testing.T) {
	src := "Title:\n    The Barn\n    A Story\nAuthor: Jane Smith\nContact:\n" +
		"    Jane Smith\n\t1 Writer's Lane\nDraft date: 1 May\n\nINT. BARN - DAY\n"
	got := types(src)
	want := strings.Join([]string{
		"titlepage=", "Title=The Barn", "Title=A Story", "Author=Jane Smith",
		"metasection=", "Contact=Jane Smith", "Contact=1 Writer's Lane", "Draft date=1 May",
		"newpage=", "scene=INT. BARN - DAY",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTitlePageFieldsSeparatedByBlankLines(t *testing.T) {
	// single blank lines between fields are tolerated
	got := types("Title: The Barn\n\nAuthor: Jane Smith\n\n\nINT. BARN - DAY\n")
	want := "titlepage=\nTitle=The Barn\nAuthor=Jane Smith\nnewpage=\nscene=INT. BARN - DAY"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestIndentedDialogueElements(t *testing.T) {
	// leading spaces on character, parenthetical and dialogue are ignored
	src := "INT. BARN - DAY\n\n" + strings.Repeat(" ", 37) + "JOHN\n" + strings.Repeat(" ", 31) + "(beat)\n" +
		"" + strings.Repeat(" ", 25) + "It's coming.\n"
	got := types(src)
	want := "scene=INT. BARN - DAY\nspeaker=JOHN\nparen=(beat)\ndialog=It's coming."
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestIndentedForcedTransition(t *testing.T) {
	got := types("INT. BARN - DAY\n\nRain.\n\n" + strings.Repeat(" ", 60) + "> FADE OUT.\n" +
		"\n" + strings.Repeat(" ", 20) + "> THE END <\n")
	want := "scene=INT. BARN - DAY\naction=Rain.\ntrans=FADE OUT.\ncenter=THE END"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
