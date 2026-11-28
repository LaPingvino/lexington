package fdx

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
)

// significant drops what FDX has no place for: blank lines (Final Draft
// spaces blocks itself) and Fountain's outline elements. Title page
// fields are reduced to their role, sorted: Final Draft title pages are
// plain paragraphs, so field names and order are not kept.
func significant(sp lex.Screenplay) []string {
	var title, body []string
	inTitle := len(sp) > 0 && sp[0].Type == lex.TypeTitlePage
	for _, l := range sp {
		if inTitle {
			switch {
			case l.Type == lex.TypeNewPage:
				inTitle = false
			case l.Contents != "":
				title = append(title, titleRole(l.Type)+"="+l.Contents)
			}
			continue
		}
		switch l.Type {
		case lex.TypeEmpty, "section", "synopse", "note":
			continue
		}
		body = append(body, string(l.Type)+"="+l.Contents)
	}
	sort.Strings(title)
	return append(title, body...)
}

func TestFountainFDXRoundTrip(t *testing.T) {
	files, err := filepath.Glob("../testdata/input/*.fountain")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no test scripts found")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			original := fountain.Parse(scenes, bytes.NewReader(src))

			var fdxDoc bytes.Buffer
			if werr := (&FDXWriter{}).Write(&fdxDoc, original); werr != nil {
				t.Fatal(werr)
			}
			back, err := ParseWithError(&fdxDoc)
			if err != nil {
				t.Fatal(err)
			}

			want, got := significant(original), significant(back)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				for i := 0; i < len(want) || i < len(got); i++ {
					var w, g string
					if i < len(want) {
						w = want[i]
					}
					if i < len(got) {
						g = got[i]
					}
					if w != g {
						t.Errorf("first difference at element %d:\n  fountain: %q\n  via FDX:  %q", i, w, g)
						break
					}
				}
			}
		})
	}
}

// fdxThere is Fountain text through FDX and back to Fountain text,
// read again: what an FDX export brings back, by both of the readers
// (ParseWithError, and ParseRevisions as Accolade's import has it).
func fdxThere(t *testing.T, src string) (doc string, back []lex.Screenplay) {
	t.Helper()
	var b bytes.Buffer
	if err := (&FDXWriter{}).Write(&b, fountain.Parse(scenes, strings.NewReader(src))); err != nil {
		t.Fatal(err)
	}
	doc = b.String()
	read := []func() (lex.Screenplay, error){
		func() (lex.Screenplay, error) { return ParseWithError(strings.NewReader(doc)) },
		func() (lex.Screenplay, error) {
			s, _, err := ParseRevisions(strings.NewReader(doc))
			return s, err
		},
	}
	for _, r := range read {
		s, err := r()
		if err != nil {
			t.Fatal(err)
		}
		var text bytes.Buffer
		if err := (&fountain.FountainWriter{SceneConfig: scenes}).Write(&text, s); err != nil {
			t.Fatal(err)
		}
		back = append(back, fountain.Parse(scenes, &text))
	}
	return doc, back
}

// A note is Final Draft's ScriptNote on a paragraph, never printed
// text; a centred line an Action centred; NAME ^ and the speech before
// it one DualDialogue: and so they come back to Fountain.
func TestNotesCentredDualThroughFDX(t *testing.T) {
	const note = "A note for the writer"
	cases := map[string]string{
		"own line, before centred": "INT. KITCHEN - DAY\n\nMaria waits.\n\n[[" + note + "]]\n\n> THE END OF THE BEGINNING <\n",
		"inline":                   "INT. KITCHEN - DAY\n\nMaria waits. [[" + note + "]] She sits.\n\n> THE END OF THE BEGINNING <\n",
		"at the end":               "INT. KITCHEN - DAY\n\n> THE END OF THE BEGINNING <\n\nMaria waits.\n\n[[" + note + "]]\n",
		"before dual dialogue":     "INT. KITCHEN - DAY\n\n> THE END OF THE BEGINNING <\n\n[[" + note + "]]\n\nMARIA\nThe one on the table.\n\nJONAS ^\nOh. That one.\n",
		"after dual dialogue":      "INT. KITCHEN - DAY\n\nMARIA\nThe one on the table.\n\nJONAS ^\nOh. That one.\n\n[[" + note + "]]\n\n> THE END OF THE BEGINNING <\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			doc, backs := fdxThere(t, src)
			for _, want := range []string{"<ScriptNote", `<Paragraph Type="Action" Alignment="Center">`} {
				if !strings.Contains(doc, want) {
					t.Errorf("FDX lacks %s:\n%s", want, doc)
				}
			}
			if strings.Contains(src, "^") && !strings.Contains(doc, "<DualDialogue>") {
				t.Errorf("FDX lacks <DualDialogue>:\n%s", doc)
			}
			for _, back := range backs {
				noted, centred := false, false
				var dual []string
				inDual := false
				for _, l := range back {
					if strings.Contains(lex.StripNotes(l.Contents), note) {
						t.Errorf("the note is printed, as %s: %q", l.Type, l.Contents)
					}
					for _, n := range lex.Notes(l.Contents) {
						noted = noted || strings.TrimSpace(n) == note
					}
					switch {
					case l.Type == lex.TypeCenter && strings.TrimSpace(lex.StripNotes(l.Contents)) == "THE END OF THE BEGINNING":
						centred = true
					case l.Type == lex.TypeDualOpen:
						inDual = true
					case l.Type == lex.TypeDualClose:
						inDual = false
					case inDual && l.Type == lex.TypeSpeaker:
						dual = append(dual, strings.TrimSpace(lex.StripNotes(l.Contents)))
					}
				}
				if !noted {
					t.Errorf("the note is lost: %v", back)
				}
				if !centred {
					t.Errorf("the centred line is not centred: %v", back)
				}
				if strings.Contains(src, "^") && strings.Join(dual, ",") != "MARIA,JONAS" {
					t.Errorf("dual dialogue's speakers %v, want MARIA, JONAS: %v", dual, back)
				}
			}
		})
	}
}
