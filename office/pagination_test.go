package office

import (
	"regexp"

	"github.com/LaPingvino/lexington/lex"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/rules"
)

// With pagination (the default), Word keeps headings, names and
// parentheticals with what follows (through the blank paragraph after
// them), the paragraph before a transition with it, the lines of a
// speech together, a dual dialogue row whole, and has widow control;
// without, none of it.
func TestDOCXPagination(t *testing.T) {
	for _, on := range []bool{true, false} {
		set := rules.Default
		if !on {
			set = set.WithPagination(rules.Pagination{})
		}
		p, _ := partsOf(t, &DOCXWriter{Elements: set}, withParen(t))
		styles, doc := p["word/styles.xml"], p["word/document.xml"]
		has := map[string]bool{
			"widow control": strings.Contains(styles, `<w:pPrDefault><w:pPr><w:widowControl/>`),
			"heading keeps": regexp.MustCompile(`w:styleId="SceneHeading">.*?<w:pPr><w:keepNext/><w:keepLines/>`).MatchString(styles),
			"name keeps":    regexp.MustCompile(`w:styleId="Character">.*?<w:pPr><w:keepNext/>`).MatchString(styles),
			"paren keeps":   regexp.MustCompile(`w:styleId="Parenthetical">.*?<w:pPr><w:keepNext/>`).MatchString(styles),
			"speech whole":  regexp.MustCompile(`w:styleId="Dialogue">.*?<w:pPr><w:keepLines/>`).MatchString(styles),
			// the blank paragraph after a heading
			"blank keeps": regexp.MustCompile(`<w:pStyle w:val="SceneHeading"/>.*?</w:p><w:p><w:pPr><w:keepNext/></w:pPr></w:p>`).MatchString(doc),
			// the paragraph and blank before SMASH CUT TO:
			"before transition": beforeTransition(doc),
			"dual whole":        strings.Contains(doc, `<w:tr><w:trPr><w:cantSplit/></w:trPr>`),
		}
		for what, got := range has {
			if got != on {
				t.Errorf("pagination %v: %s %v", on, what, got)
			}
		}
		if !on && !strings.Contains(styles, `<w:widowControl w:val="0"/>`) {
			t.Error("pagination off: widow control not off")
		}
	}
}

// And Writer the same, in ODF's words.
func TestODTPagination(t *testing.T) {
	for _, on := range []bool{true, false} {
		set := rules.Default
		if !on {
			set = set.WithPagination(rules.Pagination{})
		}
		p, _ := partsOf(t, &ODTWriter{Elements: set}, withParen(t))
		styles, content := p["styles.xml"], p["content.xml"]
		has := map[string]bool{
			"widows":        strings.Contains(styles, `fo:widows="2" fo:orphans="2"`),
			"heading keeps": regexp.MustCompile(`style:name="SceneHeading"[^>]*><style:paragraph-properties[^>]*fo:keep-with-next="always" fo:keep-together="always"`).MatchString(styles),
			"name keeps":    regexp.MustCompile(`style:name="Character"[^>]*><style:paragraph-properties[^>]*fo:keep-with-next="always"`).MatchString(styles),
			"paren keeps":   regexp.MustCompile(`style:name="Parenthetical"[^>]*><style:paragraph-properties[^>]*fo:keep-with-next="always"`).MatchString(styles),
			"blank keeps":   regexp.MustCompile(`style:parent-style-name="Standard"><style:paragraph-properties fo:keep-with-next="always">`).MatchString(content),
			"dual whole":    strings.Contains(content, `<style:table-row-properties fo:keep-together="always"/>`),
		}
		for what, got := range has {
			if got != on {
				t.Errorf("pagination %v: %s %v", on, what, got)
			}
		}
		if !on && !strings.Contains(styles, `fo:widows="0" fo:orphans="0"`) {
			t.Error("pagination off: widows not off")
		}
	}
}

// beforeTransition reports whether the two paragraphs before the first
// transition (its blank line, and the line before) keep with the next.
func beforeTransition(doc string) bool {
	ps := strings.Split(doc, "<w:p>")
	for k, para := range ps {
		if strings.HasPrefix(para, `<w:pPr><w:pStyle w:val="Transition"/>`) && k >= 2 {
			return strings.Contains(ps[k-1], "<w:keepNext/>") && strings.Contains(ps[k-2], "<w:keepNext/>")
		}
	}
	return false
}

// withParen is the example script with a speech with a parenthetical.
func withParen(t *testing.T) lex.Screenplay {
	return append(script(t), lex.Line{Type: lex.TypeEmpty}, lex.Line{Type: lex.TypeSpeaker, Contents: "ANNA"},
		lex.Line{Type: lex.TypeParen, Contents: "(softly)"}, lex.Line{Type: lex.TypeDialog, Contents: "There."})
}
