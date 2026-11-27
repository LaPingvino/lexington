package trelby

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// styledSample is a Trelby screenplay with Fountain's markers in its
// text, as Trelby's Fountain import keeps them: emphasis over a wrapped
// line, in a heading Trelby capitalises, plain asterisks escaped or
// unclosed, dual dialogue, a caret with no speech before it.
const styledSample = "\ufeff#Version 3\n#Start-Script \n" +
	".\\int. *dark* room - night\n" +
	">.She is **very\n" +
	"&.ang\n" +
	"..ry** and _alone_. He dialed *69, then \\*9\\*.\n" +
	"._bob\n" +
	".:Now!\n" +
	"._mary ^\n" +
	".:*No*.\n" +
	"._carl ^\n" +
	".:Me too.\n"

var styledRows = []string{
	"scene: INT. *DARK* ROOM - NIGHT",
	`action: She is **very angry** and _alone_. He dialed *69, then \*9\*.`,
	"dualspeaker_open: ",
	"speaker: BOB",
	"dialog: Now!",
	"dualspeaker_next: ",
	"speaker: MARY",
	"dialog: *No*.",
	"dualspeaker_close: ",
	"speaker: CARL", // a third speaker's caret pairs with nothing
	"dialog: Me too.",
}

func TestReadEmphasisAndDual(t *testing.T) {
	s := parse(t, styledSample)
	formattest.Same(t, "styled", styledRows, formattest.Rows(s, nil))
	var spans []lex.Span
	for _, l := range s {
		if l.Type == lex.TypeAction {
			spans = lex.Emphasis(l.Contents)
		}
	}
	if len(spans) < 2 || spans[1].Text != "very angry" || !spans[1].Bold {
		t.Errorf("the bold over the wrapped line is not one span: %#v", spans)
	}
}

// TestStyledRoundTrip: Fountain's emphasis and dual dialogue, written to
// Trelby and read back, are as they were; and the Trelby file written
// again is the same file.
func TestStyledRoundTrip(t *testing.T) {
	src := "INT. ROOM - DAY\n\nShe is **very** *angry*, _alone_ and ***late***; a plain \\* and *69.\n\n" +
		"BOB\nNow!\n\nMARY ^\n(quietly)\n*No*.\n\nCARL\nOkay.\n"
	orig := formattest.Read(src)
	out := write(t, orig)
	checkLoadable(t, out)
	for _, w := range []string{
		"..She is **very** *angry*, _alone_ and ***late***; a plain * and *69.\n", // the backslash needless
		"._MARY ^\n",
		"._BOB\n",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("no %q in\n%s", w, out)
		}
	}
	back := parse(t, out)
	formattest.Same(t, "back", spanRows(orig), spanRows(back))
	if again := write(t, back); again != out {
		t.Errorf("written again, not the same:\n%s\nvs\n%s", out, again)
	}
}

// TestConfigStyles: an element type Trelby's config exports bold,
// italic or underlined reads as styled throughout, but for what its text
// has already; notes and act breaks stay plain.
func TestConfigStyles(t *testing.T) {
	src := "\ufeff#Version 3\n#Begin-Config \n" +
		"Element/Scene/Export/Bold:True\n" +
		"Element/Scene/Screen/Italic:True\n" +
		"Element/Transition/Export/Underlined:True\n" +
		"Element/Transition/Export/Italic:True\n" +
		"Element/Note/Export/Italic:True\n" +
		"#End-Config \n#Start-Script \n" +
		".\\int. hall - day\n" +
		".\\**ext. yard - day**\n" +
		".%a note\n" +
		"./cut to:\n"
	formattest.Same(t, "config", []string{
		"scene: **INT. HALL - DAY**",
		"scene: **EXT. YARD - DAY**",
		"note: [[a note]]",
		"trans: _*CUT TO:*_",
	}, formattest.Rows(parse(t, src), nil))
}

// TestTitleFlags: a title page value styled as a whole keeps its style
// in the title string's flags; styled in part, it is plain.
func TestTitleFlags(t *testing.T) {
	s := formattest.Read("Title: _**BIG**_\nCredit: *by*\nAuthor: Ann\nDraft date: a **new** draft\n\nINT. HALL - DAY\n")
	out := write(t, s)
	for _, w := range []string{",24,cbu,Courier,,BIG\n", ",12,r,Courier,,a new draft\n"} {
		if !strings.Contains(out, w) {
			t.Errorf("no %q in\n%s", w, out)
		}
	}
	back := strings.Join(formattest.Rows(parse(t, out), nil), "\n")
	for _, w := range []string{"Title: _**BIG**_", "Contact: a new draft"} { // alone on the right: read as the contact
		if !strings.Contains(back, w) {
			t.Errorf("no %q in\n%s", w, back)
		}
	}
}

// TestStyledCredit: a credit and its authors styled alike go out as one
// styled title string and read back as credit and authors.
func TestStyledCredit(t *testing.T) {
	back := strings.Join(formattest.Rows(parse(t, write(t, formattest.Read(
		"Title: X\nCredit: **Written by**\nAuthor: **Ann**\n\nINT. A - DAY\n"))), nil), "\n")
	for _, w := range []string{"Title: X", "Credit: **Written by**", "Author: **Ann**"} {
		if !strings.Contains(back, w) {
			t.Errorf("no %q in\n%s", w, back)
		}
	}
}

// TestNoteInDual: a note in the first speech of dual dialogue goes after
// the second, so the caret still pairs them.
func TestNoteInDual(t *testing.T) {
	out := write(t, formattest.Read("INT. A - DAY\n\nBOB\nHi. [[check]]\n\nMARY ^\nHo.\n\nThey go.\n"))
	if !strings.Contains(out, "._BOB\n.:Hi.\n._MARY ^\n.:Ho.\n.%check\n") {
		t.Errorf("the note is not after the dual dialogue:\n%s", out)
	}
	rows := strings.Join(formattest.Rows(parse(t, out), nil), "\n")
	if !strings.Contains(rows, "dualspeaker_next") {
		t.Errorf("the pair is lost:\n%s", rows)
	}
}

// TestNeedlessEscapes: plain asterisks and underscores that read as text
// anyway go out without backslashes; those that would not, with them.
func TestNeedlessEscapes(t *testing.T) {
	s := lex.Screenplay{{Type: lex.TypeAction, Contents: lex.EscapeEmphasis("snake_case 2 * 3, *not italics*")}}
	out := write(t, s)
	if !strings.Contains(out, "..snake_case 2 * 3, *not italics\\*\n") {
		t.Errorf("escapes:\n%s", out)
	}
	formattest.Same(t, "back", spanRows(s), spanRows(parse(t, out)))
}

// spanRows are a script's rows with their emphasis as spans, however its
// plain asterisks are escaped.
func spanRows(s lex.Screenplay) []string {
	var out []string
	for _, l := range s {
		if l.Type != lex.TypeEmpty {
			out = append(out, fmt.Sprintf("%s: %v", l.Type, lex.Emphasis(l.Contents)))
		}
	}
	return out
}
