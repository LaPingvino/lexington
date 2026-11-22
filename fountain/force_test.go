package fountain

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/LaPingvino/lexington/rules"
)

// forceSample is a script in lang using each of its scene words, and
// every other language's words it does not know, as action and as a
// character.
func forceSample(conf rules.TOMLConf, lang string) string {
	heading := func(w, place string) string {
		last, _ := utf8.DecodeLastRuneInString(w)
		switch {
		case last == '.':
			return w + " " + place
		case unicode.IsLetter(last) || unicode.IsDigit(last):
			return w + ". " + place
		}
		return w + place
	}
	var b strings.Builder
	b.WriteString("Title: A Test\nContact:\n    Someone\n    Street 1\n\n")
	for _, w := range conf.Scenes(lang) {
		b.WriteString(heading(w, "PLACE - DAY") + "\n\nSomething happens.\n\n")
	}
	for other, ol := range conf.Languages {
		words := ol.Scenes
		if other == lang {
			continue
		}
		for _, w := range words {
			b.WriteString(heading(w, "kitchen is written on the wall.") + "\n\n")
			b.WriteString(strings.ToUpper(heading(w, "BOB")) + "\nHello.\n\n")
		}
	}
	b.WriteString("[[a note]] " + heading(conf.Scenes(lang)[0], "GARDEN") + "\n\n/* Notes\n" +
		heading(conf.Scenes(lang)[0], "in the boneyard") + "\n*/\n\n>THE END<\n")
	return b.String()
}

func elementList(text string, scenes []string) string {
	var out []string
	for _, l := range Parse(scenes, strings.NewReader(text)) {
		out = append(out, l.Type)
	}
	return strings.Join(out, " ")
}

// For every pair of configured languages: the forced text reads as the
// same elements with the other's words, and Unforce gives the text back.
func TestForceEveryLanguage(t *testing.T) {
	conf := rules.DefaultConf()
	for from := range conf.Languages {
		for to := range conf.Languages {
			text := forceSample(conf, from)
			f := Force(text, conf.Scenes(from), conf.Scenes(to))
			if a, b := elementList(text, conf.Scenes(from)), elementList(f, conf.Scenes(to)); a != b {
				t.Errorf("%s→%s reads otherwise:\n%s\n%s\n%s", from, to, a, b, f)
			}
			if back := Unforce(f, conf.Scenes(from), conf.Scenes(to)); back != text {
				t.Errorf("%s→%s→%s:\n%s\n---\n%s", from, to, from, text, back)
			}
			if from == to && f != text {
				t.Errorf("%s forced for itself", from)
			}
		}
	}
}

// The marks are Fountain's and only where needed; a mark the other words
// need, or one the writer put that they need too, stays on the way back.
func TestForceMarks(t *testing.T) {
	conf := rules.DefaultConf()
	eo, en, de := conf.Scenes("eo"), conf.Scenes("en"), conf.Scenes("de")
	text := "EN. DOMO - TAGO\n\nINT. is English for inside.\n\nINT. BOB\nSaluton.\n\n!EN. needed in Esperanto too\n\n.INT. ON PURPOSE\n\nen. domo - nokto\n"
	want := ".EN. DOMO - TAGO\n\n!INT. is English for inside.\n\n@INT. BOB\nSaluton.\n\n!EN. needed in Esperanto too\n\n.INT. ON PURPOSE\n\n.en. domo - nokto\n"
	if got := Force(text, eo, en); got != want {
		t.Fatalf("forced:\n%s", got)
	}
	if back := Unforce(want, eo, en); back != text {
		t.Fatalf("back:\n%s", back)
	}
	// a heading English reads as a character makes the next line dialogue:
	// only the heading is marked
	if got := Force("EN. DOMO - TAGO\nBram kuiras.\n", eo, en); got != ".EN. DOMO - TAGO\nBram kuiras.\n" {
		t.Errorf("a run: %q", got)
	}
	// another app's text: "!INT. KITCHEN" over a name is action only with its mark
	if back := Unforce("!INT. KITCHEN\nBOB\n", eo, en); back != "!INT. KITCHEN\nBOB\n" {
		t.Errorf("a needed mark went: %q", back)
	}
	// a German script's dotted English heading: English needs no dot, so it stays
	if back := Unforce(".INT. HOUSE\n", de, en); back != ".INT. HOUSE\n" {
		t.Errorf("a writer's dot went: %q", back)
	}
}
