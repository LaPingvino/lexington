package fountain

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// rowType is the type of the row of s whose contents start with text.
func rowType(s lex.Screenplay, text string) lex.ElementType {
	for _, l := range s {
		if strings.HasPrefix(strings.ToUpper(l.Contents), strings.ToUpper(text)) {
			return l.Type
		}
	}
	return ""
}

// The languages named more than once.
const esperanto, german, hebrew = "eo", "de", "he"

// dotCases are, per language, sentences that start with one of its
// scene words, each with a heading by that word with its dot; and a
// heading by one of them without it, as scripts before 1.68 had them.
var dotCases = map[string]struct {
	sentences [][2]string
	old       string
}{
	esperanto: {[][2]string{
		{"Ene de la domo estas mallume.", "ENE. KUIREJO - TAGO"},
		{"Ek de nun ŝi kuras.", "EK. ĜARDENO - NOKTO"},
		{"Ekst la fenestro pluvas.", "EKST. STRATO - TAGO"},
	}, "ENE KUIREJO - TAGO"},
	german: {[][2]string{
		{"Innen ist es still.", "INNEN. KÜCHE - TAG"},
		{"Aussen hängt ein Schild.", "AUSSEN. GARTEN - NACHT"},
		{"Etabl ist kein Wort.", "ETABL. STADT - TAG"},
	}, "INNEN KÜCHE - TAG"},
	"nl": {[][2]string{
		{"Open de deur en kijk.", "OPEN. PLEIN - DAG"},
		{"Bin je daar?", "BIN. KEUKEN - DAG"},
		{"Bu zegt het kind.", "BU. STRAAT - NACHT"},
	}, "BIN KEUKEN - DAG"},
	"tok": {[][2]string{
		{"insa tomo la jan li lape.", "INSA. TOMO"},
		{"kon li tawa.", "KON. MA"},
	}, "INSA TOMO"},
	hebrew: {[][2]string{
		{"פנים חיוורות מביטות בחלון.", "פנים. משרד - יום"},
		{"חוץ מזה, הכול שקט.", "חוץ. גינה - לילה"},
	}, "פנים משרד - יום"},
}

// A scene word that is an ordinary word starts a heading only with its
// dot: a sentence that starts with it is action.
func TestSceneWordNeedsItsDot(t *testing.T) {
	conf := rules.DefaultConf()
	for code, cases := range dotCases {
		for _, c := range cases.sentences {
			sentence, heading := c[0], c[1]
			lang := conf.Language(code)
			text := "Erste.\n\n" + sentence + "\n\n" + heading + "\n\nLast.\n"
			s := ParseIn(lang, strings.NewReader(text))
			if got := rowType(s, sentence); got != lex.TypeAction {
				t.Errorf("%s %q: %s, want action", code, sentence, got)
			}
			if got := rowType(s, heading); got != lex.TypeScene {
				t.Errorf("%s %q: %s, want a heading", code, heading, got)
			}
			// and written again, the sentence needs no mark to stay action
			var b bytes.Buffer
			w := &FountainWriter{SceneConfig: lang.Scenes, Language: &lang}
			if err := w.Write(&b, s); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(b.String(), "\n"+sentence+"\n") {
				t.Errorf("%s written:\n%s", code, b.String())
			}
		}
	}
}

// A heading by a word without its dot, as scripts before 1.68 had them:
// ParseLoose reads it as a heading still, KeepLoose forces it with ".",
// and a word that always had its dot (EN.) needs it there too.
func TestDotlessHeadingsOfOldScripts(t *testing.T) {
	conf := rules.DefaultConf()
	for code, cases := range dotCases {
		c := struct{ lang, heading string }{code, cases.old}
		text := "Erste.\n\n" + c.heading + "\n\nLast.\n"
		if got := rowType(ParseIn(conf.Language(c.lang), strings.NewReader(text)), c.heading); got != lex.TypeAction {
			t.Errorf("%s %q: %s by the spec, want action", c.lang, c.heading, got)
		}
		if got := rowType(ParseLoose(conf.Scenes(c.lang), strings.NewReader(text)), c.heading); got != lex.TypeScene {
			t.Errorf("%s %q: %s loose, want a heading", c.lang, c.heading, got)
		}
		kept, places := KeepLooseIn(text, conf.Language(c.lang))
		if places == 0 || !strings.Contains(kept, "\n."+c.heading+"\n") {
			t.Errorf("%s: kept (%d places):\n%s", c.lang, places, kept)
		}
		if got := rowType(ParseIn(conf.Language(c.lang), strings.NewReader(kept)), c.heading); got != lex.TypeScene {
			t.Errorf("%s %q kept: %s, want a heading", c.lang, c.heading, got)
		}
	}
	// EN. had its dot before: "En la domo" was action, and is
	text := "Unu.\n\nEn la domo estas mallume.\n\nFino.\n"
	if got := rowType(ParseLoose(conf.Scenes(esperanto), strings.NewReader(text)), "En la domo"); got != lex.TypeAction {
		t.Errorf("En la domo, loose: %s", got)
	}
	if kept, places := KeepLooseIn(text, conf.Language(esperanto)); places != 0 {
		t.Errorf("En la domo kept (%d places):\n%s", places, kept)
	}
}

// The writer reads a scene word in any case, as the parser does: action
// in mixed case that starts with one gets its "!".
func TestWriterSceneWordInAnyCase(t *testing.T) {
	for _, line := range []string{"Ext. the pipe bursts.", "int. is short for interior.", "Est. 1887, the sign says."} {
		s := lex.Screenplay{
			{Type: lex.TypeAction, Contents: "First."},
			{Type: lex.TypeEmpty},
			{Type: lex.TypeAction, Contents: line},
		}
		var b bytes.Buffer
		if err := (&FountainWriter{SceneConfig: Scene}).Write(&b, s); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(b.String(), "\n!"+line) {
			t.Errorf("%q written without its mark:\n%s", line, b.String())
		}
		if got := rowType(Parse(Scene, strings.NewReader(b.String())), line); got != lex.TypeAction {
			t.Errorf("%q read back as %s", line, got)
		}
	}
}
