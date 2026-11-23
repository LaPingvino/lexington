package lex

import (
	"regexp"
	"strings"
)

// Highland's directives are {{NAME}} or {{NAME: value}} in the text
// (Highland 2's cheat sheet; highwater-too's parser, which matches
// Highland's PDFs). Those that print nothing where they stand are taken
// out of the printed text here: the page's header and footer
// ({{HEADER: …}}, {{FOOTER: …}}), the page number to go on with
// ({{PAGENUMBER: n}}), a table of contents ({{TOC}}), another file put in
// ({{INCLUDE: file}}), markers ({{%m: label}}) and the tags around a
// revision ({{REVISION: #ee2d36}}…{{/REVISION}}, the text between them
// kept). What they ask for is not done (no header, no included file);
// the Fountain written keeps them as they are. Directives that print a
// value ({{SERIES}}, {{TIMESTAMP}}) and unknown ones are left in.
var directive = regexp.MustCompile(`(?i)\{\{\s*(?:%m|header|footer|pagenumber|toc|include|/\s*revision|revision)\s*(?::[^{}]*)?\}\}`)

// HasDirectives reports whether s has one of Highland's unprinted
// directives in it.
func HasDirectives(s string) bool { return strings.Contains(s, "{{") && directive.MatchString(s) }

// StripDirectives takes Highland's unprinted directives out of s, the
// white space around them as StripNotes does: "An action. {{%m: x}}" is
// "An action.", "{{REVISION: #ee2d36}}New.{{/REVISION}} Old." is "New.
// Old.".
func StripDirectives(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	ms := directive.FindAllStringIndex(s, -1)
	if ms == nil {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range ms {
		// a revision's tags go alone, the text around them as it is
		name := strings.TrimLeft(strings.ToLower(s[m[0]+2:m[1]]), " \t/")
		revision := strings.HasPrefix(name, "revision")
		before := s[last:m[0]]
		if !revision {
			before = strings.TrimRightFunc(before, isBlank)
		}
		b.WriteString(before)
		last = m[1]
		if !revision && (b.Len() == 0 || strings.HasSuffix(b.String(), "\n")) {
			for last < len(s) && isBlank(rune(s[last])) {
				last++
			}
		}
	}
	b.WriteString(s[last:])
	return b.String()
}

// IsDirectiveLine reports whether a line is nothing but Highland's
// unprinted directives (and notes): never printed, the blank lines around
// it as around a line of notes.
func IsDirectiveLine(l Line) bool {
	if !printsText(l.Type) || !HasDirectives(l.Contents) {
		return false
	}
	return strings.TrimSpace(StripNotes(StripDirectives(l.Contents))) == ""
}
