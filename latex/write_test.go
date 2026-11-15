package latex

import "testing"

func TestInlineMarkup(t *testing.T) {
	for in, want := range map[string]string{
		"_**THE END**_":              `\textbf{\underline{THE END}}`,
		`**\*9765\***`:               `\textbf{*9765*}`,
		"*69 and then *23":           `*69 and then *23`,
		"a *b_c* & d":                `a \textit{b\_c} \& d`,
		"**bold *and italic* bold**": `\textbf{bold }\textbf{\textit{and italic}}\textbf{ bold}`,
	} {
		if got := inlineMarkup(in); got != want {
			t.Errorf("inlineMarkup(%q) = %q, want %q", in, got, want)
		}
	}
}
