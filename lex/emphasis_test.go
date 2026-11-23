package lex

import (
	"fmt"
	"strings"
	"testing"
)

// show writes spans with their styles as tags: <b>, <i>, <u>.
func show(spans []Span) string {
	var b strings.Builder
	for _, s := range spans {
		var tags []string
		if s.Bold {
			tags = append(tags, "b")
		}
		if s.Italic {
			tags = append(tags, "i")
		}
		if s.Underline {
			tags = append(tags, "u")
		}
		if len(tags) == 0 {
			b.WriteString(s.Text)
			continue
		}
		fmt.Fprintf(&b, "<%s>%s</>", strings.Join(tags, ""), s.Text)
	}
	return b.String()
}

// The examples of fountain.io/syntax#emphasis, and what follows from its
// rules.
func TestEmphasis(t *testing.T) {
	for _, c := range [][2]string{
		// the page's examples
		{"*italics*", "<i>italics</>"},
		{"**bold**", "<b>bold</>"},
		{"***bold italics***", "<bi>bold italics</>"},
		{"_underline_", "<u>underline</>"},
		{"From what seems like only INCHES AWAY. _Steel's face FILLS the *Leupold Mark 4* scope_.",
			"From what seems like only INCHES AWAY. <u>Steel's face FILLS the </><iu>Leupold Mark 4</><u> scope</>."},
		{"_**THE END**_", "<bu>THE END</>"},
		{`Steel enters the code on the keypad: **\*9765\***`, "Steel enters the code on the keypad: <b>*9765*</>"},
		{"He dialed *69 and then *23, and then hung up.", "He dialed *69 and then *23, and then hung up."},
		{"He dialed *69 and then 23*, and then hung up.", "He dialed <i>69 and then 23</>, and then hung up."},
		{`He dialed *69 and then 23\*, and then hung up.`, "He dialed *69 and then 23*, and then hung up."},
		{"As he rattles off the long list, Brick and Steel *share a look.\nThis is going to be BAD.*",
			"As he rattles off the long list, Brick and Steel *share a look.\nThis is going to be BAD.*"},
		// mixing and nesting
		{"**bold *and italic* bold**", "<b>bold </><bi>and italic</><b> bold</>"},
		{"*italic **and bold** italic*", "<i>italic </><bi>and bold</><i> italic</>"},
		{"***bold** italic*", "<bi>bold</><i> italic</>"},
		{"_under **bold** line_", "<u>under </><bu>bold</><u> line</>"},
		{"**foo***bar*", "<b>foo</><i>bar</>"},
		// Markdown's rule of three: a run that can open and close pairs
		// with one whose length makes a multiple of three only if both are
		{"**,*’s***", "<b>,</><bi>’s</>"},
		{"**foo*bar*baz**", "<b>foo</><bi>bar</><b>baz</>"},
		{"*foo**bar**baz*", "<i>foo</><bi>bar</><i>baz</>"},
		{"*John*'s **O'Brien**'s", "<i>John</>'s <b>O'Brien</>'s"},
		// spaces count
		{"* not italic *", "* not italic *"},
		{"2 * 3 * 4", "2 * 3 * 4"},
		{"** not bold**", "** not bold**"},
		// unclosed markers are text
		{"*unclosed", "*unclosed"},
		{"**half*", "*<i>half</>"},
		{"a_b", "a_b"},
		// underscores within words are text; asterisks are markers
		{"snake_case_name", "snake_case_name"},
		{"un*frigging*believable", "un<i>frigging</>believable"},
		{"_word_s", "_word_s"},
		{"(_aside_)", "(<u>aside</>)"},
		// escapes: only of markers
		{`\_not underlined\_`, "_not underlined_"},
		{`C:\path\to`, `C:\path\to`},
		// lines are separate
		{"*one*\n_two_", "<i>one</>\n<u>two</>"},
		{"", ""},
	} {
		if got := show(Emphasis(c[0])); got != c[1] {
			t.Errorf("Emphasis(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestUnstyledAndEscape(t *testing.T) {
	if got := Unstyled(`**\*9765\*** and _x_`); got != "*9765* and x" {
		t.Errorf("Unstyled: %q", got)
	}
	for _, s := range []string{"*69 and 23*", "a_b_ *c*", `back\slash`, "**"} {
		if got := Unstyled(EscapeEmphasis(s)); got != s {
			t.Errorf("Unstyled(EscapeEmphasis(%q)) = %q", s, got)
		}
	}
}
