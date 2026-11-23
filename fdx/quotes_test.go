package fdx

import (
	"strings"
	"testing"
)

// Apostrophes and quotes beside emphasis, both ways. A Highland Pro user
// reports "John's" coming back as "John**s" between Highland and Final
// Draft. Here Fountain → FDX gives the runs below, FDX → Fountain gives
// the Fountain back, and the same with typographic quotes (’ “ ”).
func TestQuotesBesideEmphasis(t *testing.T) {
	for _, c := range []struct{ fountain, fdx string }{
		{"John's coat.", "Action:John's coat."},
		{"*John*'s coat.", "Action:[Italic]John's coat."},
		{"It's _not_ mine.", "Action:It's [Underline]not mine."},
		{`O'Brien's **"word"**.`, `Action:O'Brien's [Bold]"word".`},
		{"**John**'s, *it*'s.", "Action:[Bold]John's, [Italic]it's."},
		{"'*quoted*'", "Action:'[Italic]quoted'"},
		{"John’s coat.", "Action:John’s coat."},
		{"*John*’s coat.", "Action:[Italic]John’s coat."},
		{"O’Brien’s **“word”**.", "Action:O’Brien’s [Bold]“word”."},
		{"“*Really*,” she says.", "Action:“[Italic]Really,” she says."},
	} {
		_, doc := writeFDX(t, "INT. HALL - DAY\n\n"+c.fountain+"\n")
		got := summary(doc.Content.Paragraphs)
		if len(got) != 2 || got[1] != c.fdx {
			t.Errorf("%q to FDX: %q, want %q", c.fountain, got, c.fdx)
			continue
		}
		fdx, _ := writeFDX(t, "INT. HALL - DAY\n\n"+c.fountain+"\n")
		back, err := ParseWithError(strings.NewReader(fdx))
		if err != nil {
			t.Fatal(err)
		}
		if l := lines(back, "action"); len(l) != 1 || l[0] != c.fountain {
			t.Errorf("%q back from FDX: %q", c.fountain, l)
		}
	}
}

// Final Draft's runs with an apostrophe or quote next to a styled one read
// as Fountain that writes the same runs again; markers that are text
// stay text.
func TestQuotesBesideEmphasisFromFDX(t *testing.T) {
	check(t, body(t, caps, `
<Paragraph Type="Action"><Text Style="Italic">John</Text><Text>'s coat.</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Bold">John</Text><Text>’s coat.</Text></Paragraph>
<Paragraph Type="Action"><Text>It's </Text><Text Style="Underline">not</Text><Text> mine.</Text></Paragraph>
<Paragraph Type="Action"><Text>O’Brien’s </Text><Text Style="Bold">“word”</Text><Text>.</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Bold">"word"</Text><Text>'s</Text></Paragraph>
<Paragraph Type="Action"><Text>John's *star* and snake_case_name.</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Italic">John's</Text></Paragraph>`),
		"action=*John*'s coat.",
		"action=**John**’s coat.",
		"action=It's _not_ mine.",
		"action=O’Brien’s **“word”**.",
		`action=**"word"**'s`,
		`action=John's \*star\* and snake_case_name.`,
		"action=*John's*")
	// and to FDX again: the same runs
	for _, c := range []struct{ fountain, fdx string }{
		{"*John*'s coat.", "Action:[Italic]John's coat."},
		{`**"word"**'s`, `Action:[Bold]"word"'s`},
		{`John's \*star\* and snake_case_name.`, "Action:John's *star* and snake_case_name."},
		{"*John's*", "Action:[Italic]John's"},
	} {
		_, doc := writeFDX(t, "INT. HALL - DAY\n\n"+c.fountain+"\n")
		if got := summary(doc.Content.Paragraphs); len(got) != 2 || got[1] != c.fdx {
			t.Errorf("%q to FDX: %q, want %q", c.fountain, got, c.fdx)
		}
	}
}
