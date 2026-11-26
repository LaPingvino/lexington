package fdx

import (
	"fmt"
	"html"
	"math/rand"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// The pieces quotes and apostrophes come in, straight and typographic.
var quotePieces = []string{"John", "'s", "’s", "O'Brien", " ", `"`, "“", "”", "word", "it's", " not ", "'", "‘", "’",
	",", ".", "s", "don’t"}

// underline is Final Draft's style name.
const underline = "Underline"

// Final Draft's runs, styled at random around apostrophes and quotes,
// read as Fountain whose text is theirs: no marker turns into text or
// eats a character ("John's" never becomes "John**s"). Where a styled run
// sits between plain ones (no two styles touching) and is no underline
// inside a word, the styles come through too.
func TestQuotesFromFDXGenerated(t *testing.T) {
	styles := []string{"", "Bold", "Italic", underline, "Bold+Italic"}
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 3000; n++ {
		var doc, plain strings.Builder
		var runs []run
		simple := true // styled runs only between plain ones
		prev := ""
		for k := 1 + rng.Intn(5); k > 0; k-- {
			p, s := quotePieces[rng.Intn(len(quotePieces))], styles[rng.Intn(len(styles))]
			if s != "" && (prev != "" || s == underline) {
				simple = false
			}
			prev = s
			fmt.Fprintf(&doc, `<Text Style="%s">%s</Text>`, s, html.EscapeString(p))
			plain.WriteString(p)
			runs = append(runs, run{Span: lex.Span{Text: p, Bold: strings.Contains(s, "Bold"),
				Italic: strings.Contains(s, "Italic"), Underline: s == underline}})
		}
		if strings.TrimSpace(plain.String()) == "" {
			continue
		}
		got := body(t, caps, `<Paragraph Type="Action">`+doc.String()+`</Paragraph>`)
		if len(got) != 1 {
			t.Fatalf("%s: %q", doc.String(), got)
		}
		f := strings.TrimPrefix(got[0], "action=")
		if u := lex.Unstyled(f); strings.TrimSpace(u) != strings.TrimSpace(plain.String()) {
			t.Errorf("text changed: %s -> %q (reads %q)", doc.String(), f, u)
		} else if simple && !sameTrimmed(lex.Emphasis(f), runs) {
			t.Errorf("styles lost: %s -> %q", doc.String(), f)
		}
	}
}

// Fountain with emphasis around apostrophes and quotes goes to FDX and
// back with its text, and where emphasised pieces do not touch (no
// "***a******b***") with its emphasis; those runs, read and written again,
// look the same (spaces show only an underline).
func TestQuotesFromFountainGenerated(t *testing.T) {
	marks := []string{"*", "**", "_", "***"}
	rng := rand.New(rand.NewSource(2))
	for n := 0; n < 600; n++ {
		var b strings.Builder
		touching, marked := false, false
		for k := 1 + rng.Intn(4); k > 0; k-- {
			p := quotePieces[rng.Intn(len(quotePieces))]
			m := strings.TrimSpace(p) == p && rng.Intn(2) == 0
			if m {
				p = marks[rng.Intn(len(marks))] + p
				p += p[:strings.LastIndexAny(p, "*_")+1]
			}
			touching = touching || m && marked
			marked = m
			b.WriteString(p)
		}
		src := strings.TrimSpace("Then " + b.String()) // an action line, whatever it starts with
		if lex.Unstyled(src) != strings.NewReplacer("*", "", "_", "").Replace(src) {
			continue // markers that are text: not this test's business
		}
		fdx, doc := writeFDX(t, "INT. HALL - DAY\n\n"+src+"\n")
		back, err := ParseWithError(strings.NewReader(fdx))
		if err != nil {
			t.Fatal(err)
		}
		l := lines(back, "action")
		if len(l) != 1 {
			t.Fatalf("%q: %q", src, l)
		}
		if lex.Unstyled(l[0]) != lex.Unstyled(src) {
			t.Errorf("%q back from FDX as %q: the text changed", src, l[0])
			continue
		}
		if touching {
			continue
		}
		if !lex.SameStyled(lex.Emphasis(l[0]), lex.Emphasis(src)) {
			t.Errorf("%q back from FDX as %q", src, l[0])
			continue
		}
		_, again := writeFDX(t, "INT. HALL - DAY\n\n"+l[0]+"\n")
		if a, b := looks(doc), looks(again); a != b {
			t.Errorf("%q (back %q): FDX %q, again %q", src, l[0],
				summary(doc.Content.Paragraphs), summary(again.Content.Paragraphs))
		}
	}
}

// looks is how a document's runs print: their characters and styles, of
// a space only its underline.
func looks(doc FdxFile) string {
	var b strings.Builder
	for _, p := range doc.Content.Paragraphs {
		for _, t := range p.Texts {
			for _, r := range t.Content {
				st := t.Style
				if r == ' ' && !strings.Contains(st, underline) {
					st = ""
				} else if r == ' ' {
					st = underline
				}
				fmt.Fprintf(&b, "%c%s|", r, st)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func slicesEqual(a, b []styledRune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameTrimmed is same with the spaces at either end left out (a
// paragraph's text is trimmed).
func sameTrimmed(spans []lex.Span, runs []run) bool {
	var want []lex.Span
	for _, t := range runs {
		want = append(want, t.Span)
	}
	trim := func(r []styledRune) []styledRune {
		for len(r) > 0 && r[0].r == ' ' {
			r = r[1:]
		}
		for len(r) > 0 && r[len(r)-1].r == ' ' {
			r = r[:len(r)-1]
		}
		return r
	}
	return slicesEqual(trim(styledRunes(spans)), trim(styledRunes(want)))
}
