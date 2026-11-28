package trelby

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
)

// sample is a Trelby screenplay as Trelby writes one (format 3, written
// here by hand after its fileformat.txt): its settings, which are passed
// over, a title page, words wrapped over lines, every element type.
const sample = "\ufeff#Version 3\n" +
	"#Begin-Auto-Completion \nScene/Enabled:True\n#End-Auto-Completion \n" +
	"#Begin-Config \nFontSize:12\n#End-Config \n" +
	"#Title-String 0.000000,80.000000,32,cb,Times,,THE KOALA INCIDENT\n" +
	"#Title-String 0.000000,100.000000,12,c,Courier,,by\\n\\nAnonymous Person\n" +
	"#Title-String 200.000000,240.000000,12,r,Courier,,Draft 2\\nMay\n" +
	"#Title-String 15.000000,240.000000,12,,Courier,,Agent: C:\\\\Nobody\n" +
	"#Header-String 1,0,r,,${PAGE}.\n#Header-Empty-Lines 1\n#Start-Script \n" +
	"..Act one\n" +
	".@Act one\n" +
	".\\int. warehouse - night\n" +
	">.JIM and JASON, two *men* in their\n" +
	"&.thir\n" +
	"..ties, stand.\n" +
	".%Do we need more?\n" +
	".%= They meet.\n" +
	".%## The shooting\n" +
	"._jim\n" +
	".((looks at Jason)\n" +
	"|:What are you doing\n" +
	">:with that gun --\n" +
	".:Hey!\n" +
	".=Insert - the gun\n" +
	"./dissolve to:\n"

var sampleRows = []string{
	"titlepage: ",
	"Title: **THE KOALA INCIDENT**",
	"Credit: by",
	"Author: Anonymous Person",
	`Contact: Agent: C:\Nobody`,
	"Draft date: Draft 2",
	"Draft date: May",
	"newpage: ",
	"action: Act one",
	"section: # ACT ONE",
	"scene: INT. WAREHOUSE - NIGHT",
	"action: JIM and JASON, two *men* in their thirties, stand.",
	"note: [[Do we need more?]]",
	"synopse: They meet.",
	"section: ## The shooting",
	"speaker: JIM",
	"paren: (looks at Jason)",
	"dialog: What are you doing",
	"dialog: with that gun -- Hey!",
	"action: INSERT - THE GUN",
	"trans: DISSOLVE TO:",
}

func parse(t *testing.T, src string) lex.Screenplay {
	t.Helper()
	s, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func write(t *testing.T, s lex.Screenplay) string {
	t.Helper()
	var b bytes.Buffer
	if err := (&TrelbyWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRead(t *testing.T) {
	formattest.Same(t, "sample", sampleRows, formattest.Rows(parse(t, sample), nil))
}

// TestRoundTrip: read, written and read back is the same script.
func TestRoundTrip(t *testing.T) {
	first := parse(t, sample)
	out := write(t, first)
	formattest.Same(t, "again", formattest.Rows(first, nil), formattest.Rows(parse(t, out), nil))
	if !strings.HasPrefix(out, "\ufeff#Version 3\n") || !strings.Contains(out, "\n#Start-Script \n") {
		t.Errorf("no BOM, #Version 3 or #Start-Script line:\n%s", out)
	}
	checkLoadable(t, out)
}

// checkLoadable checks what Trelby's loader asks of a file: every line of
// the script two characters at least, its type the same up to an
// element's last line, the last line ending an element.
func checkLoadable(t *testing.T, out string) {
	t.Helper()
	_, script, _ := strings.Cut(out, "#Start-Script \n")
	lines := strings.Split(strings.TrimSuffix(script, "\n"), "\n")
	var prev byte
	for i, l := range lines {
		if len(l) < 2 {
			t.Fatalf("script line %d too short: %q", i+1, l)
		}
		if prev != 0 && l[1] != prev {
			t.Fatalf("script line %d changes type inside an element: %q", i+1, l)
		}
		prev = l[1]
		if l[0] == endLast {
			prev = 0
		}
	}
	if prev != 0 {
		t.Fatalf("the last line does not end an element: %q", lines[len(lines)-1])
	}
}

// TestFountain: Fountain in, Trelby out, read back: what Trelby holds is
// kept (its text with its emphasis, dual dialogue; the title page's
// fields).
func TestFountain(t *testing.T) {
	for name, src := range formattest.Inputs(t) {
		t.Run(name, func(t *testing.T) {
			orig := formattest.Read(src)
			out := write(t, orig)
			checkLoadable(t, out)
			back := parse(t, out)
			formattest.Same(t, name, trelbyRows(orig), trelbyRows(back))
		})
	}
}

// trelbyRows are a script's rows as Trelby keeps them: no numbers, notes
// or outline, lyrics as dialogue, the title
// page's fields plain and in any order.
func trelbyRows(s lex.Screenplay) []string {
	var title, body []string
	inTitle := len(s) > 0 && s[0].Type == lex.TypeTitlePage
	for _, l := range s {
		if inTitle {
			if l.Type == lex.TypeNewPage {
				inTitle = false
			} else if l.Contents != "" {
				title = append(title, string(l.Type)+": "+lex.Unstyled(l.Contents))
			}
			continue
		}
		t := l.Type
		switch t {
		case lex.TypeEmpty, lex.TypeNewPage, lex.TypeNote, lex.TypeSection, lex.TypeSynopsis, lex.TypeBoneyard:
			continue
		case lex.TypeDualOpen, lex.TypeDualNext, lex.TypeDualClose:
			body = append(body, string(t))
			continue
		case lex.TypeLyrics:
			t = lex.TypeDialog
		}
		text := l.Contents
		if l.Type == lex.TypeScene {
			text, _ = lex.SceneNumber(text)
		}
		text = strings.TrimSpace(lex.StripNotes(text))
		if text == "" {
			continue
		}
		body = append(body, string(t)+": "+text)
	}
	sort.Strings(title)
	return append(title, body...)
}

func TestOutlineAsNotes(t *testing.T) {
	back := strings.Join(formattest.Rows(parse(t, write(t, formattest.Read(formattest.Script+"\n## Scene group\n\n= Its synopsis.\n"))), nil), "\n")
	for _, w := range []string{"section: # ACT ONE", "synopse: The opening.", "synopse: A synopsis.",
		"note: [[a note]]", "note: [[a note of its own]]", "section: ## Scene group", "synopse: Its synopsis.",
		"Title: _**BRICK & STEEL**_", "Source: Based on a story"} {
		if !strings.Contains(back, w) {
			t.Errorf("no %q in\n%s", w, back)
		}
	}
}

func TestEmpty(t *testing.T) {
	out := write(t, nil)
	checkLoadable(t, out)
	if _, err := Parse(strings.NewReader(out)); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(strings.NewReader("INT. HALL")); err == nil {
		t.Error("no error for a file that is not Trelby's")
	}
}
