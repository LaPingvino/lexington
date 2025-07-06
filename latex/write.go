package latex

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/rules"
)

// LaTeXWriter implements the writer.Writer interface for LaTeX output.
// It uses a Go text/template for rendering and requires a rules.Set
// for formatting elements.
type LaTeXWriter struct {
	Template string    // Path to the LaTeX template file
	Elements rules.Set // Configuration for elements (margins, fonts, etc.)
}

const defaultLaTeXTemplate = `\documentclass{scrartcl} % KOMA-Script article class
\usepackage[utf8]{inputenc}
\usepackage[T1]{fontenc}
\usepackage{lmodern}
\usepackage[a4paper,
            left={{.LeftMargin}}in,
            right={{.RightMargin}}in,
            top={{.TopMargin}}in,
            bottom={{.BottomMargin}}in]{geometry}
\usepackage{screenplay} % Requires a LaTeX screenplay package, e.g., screenwright or fountain-latex
\usepackage{setspace} % For line spacing

% Set up screenplay environment with custom fonts and sizes if needed
% \setmainfont{Courier New} % Requires fontspec and a system font
% \setsansfont{Arial}
% \setmonofont{Courier New}

\begin{document}

{{range .Screenplay}}
    {{if eq .Type "titlepage"}}
        \begin{titlepage}
    {{else if eq .Type "Title"}}\titletext{ {{.Contents}} }
    {{else if eq .Type "Credit"}}\credittext{ {{.Contents}} }
    {{else if eq .Type "Author"}}\authorname{ {{.Contents}} }
    {{else if eq .Type "metasection"}}
        \end{titlepage}
        \pagenumbering{arabic}
        \setcounter{page}{2} % Start page numbering from 2 after title page
    {{else if eq .Type "scene"}}
        \scenehead{ {{.Contents}} }
    {{else if eq .Type "action"}}
        \action{ {{.Contents}} }
    {{else if eq .Type "speaker"}}
        \character{ {{.Contents}} }
    {{else if eq .Type "dialog"}}
        \dialogue{ {{.Contents}} }
    {{else if eq .Type "paren"}}
        \parenthetical{ {{.Contents}} }
    {{else if eq .Type "transition"}}
        \transition{ {{.Contents}} }
    {{else if eq .Type "center"}}
        \center{ {{.Contents}} }
    {{else if eq .Type "newpage"}}
        \newpage
    {{else if eq .Type "empty"}}
        \vspace{\baselineskip} % Equivalent to a blank line
    {{else if eq .Type "dualspeaker_open"}}
        \begin{dualdialogue} % Assumes screenplay package supports this environment
            \leftcol
    {{else if eq .Type "dualspeaker_next"}}
            \rightcol
    {{else if eq .Type "dualspeaker_close"}}
        \end{dualdialogue}
    {{else}}
        % Fallback for unhandled types
        \textbf{UNHANDLED TYPE: {{.Type}}}: {{.Contents}}
    {{end}}
{{end}}

\end{document}
`

// Write converts the internal lex.Screenplay format to a LaTeX file.
// It implements the writer.Writer interface.
//
// Note: This function only generates the .tex file. Compiling it into a PDF
// requires a LaTeX distribution (e.g., TeX Live) with a screenplay package
// (like `screenwright` or `fountain-latex`) installed on the system.
// The PDF generation step is external to this Go program.
func (l *LaTeXWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// Define a struct to pass data to the template
	type TemplateData struct {
		Screenplay   lex.Screenplay
		LeftMargin   float64
		RightMargin  float64
		TopMargin    float64
		BottomMargin float64
		// Add more configurable elements from rules.Set if needed for the template
	}

	data := TemplateData{
		Screenplay: screenplay,
		// Assuming default margins from rules.Set for now, can be made dynamic
		LeftMargin:   1.5, // Standard screenplay left margin
		RightMargin:  1.0, // Standard screenplay right margin
		TopMargin:    1.0, // Standard screenplay top margin
		BottomMargin: 1.0, // Standard screenplay bottom margin
	}

	// Attempt to parse the template from the provided path, or use the default
	var tmpl *template.Template
	var err error

	if l.Template != "" {
		tmpl, err = template.ParseFiles(l.Template)
		if err != nil {
			return fmt.Errorf("failed to parse LaTeX template file %s: %w", l.Template, err)
		}
	} else {
		tmpl, err = template.New("latexScreenplay").Parse(defaultLaTeXTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse default LaTeX template: %w", err)
		}
	}

	// Escape problematic characters for LaTeX
	// This should be done carefully, ideally within the template or a custom function
	// passed to the template, to avoid over-escaping legitimate LaTeX commands.
	// For now, a simple replacement for common offenders.
	// A more robust solution might require a dedicated LaTeX escaping library or
	// a custom template function.
	for i := range data.Screenplay {
		data.Screenplay[i].Contents = escapeLaTeX(data.Screenplay[i].Contents)
	}

	return tmpl.Execute(w, data)
}

// escapeLaTeX escapes characters that have special meaning in LaTeX.
// This is a basic implementation and might need to be extended for more complex scenarios.
func escapeLaTeX(s string) string {
	s = strings.ReplaceAll(s, "&", "\\&")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "#", "\\#")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	s = strings.ReplaceAll(s, "~", "\\textasciitilde{}")
	s = strings.ReplaceAll(s, "^", "\\textasciicircum{}")
	s = strings.ReplaceAll(s, "\\", "\\textbackslash{}") // Escapes backslash itself
	s = strings.ReplaceAll(s, "<", "\\textless{}")
	s = strings.ReplaceAll(s, ">", "\\textgreater{}")
	// For quotes, apostrophes, etc., a more intelligent solution might be needed
	// as LaTeX often handles them contextually.
	s = strings.ReplaceAll(s, "...", "\\ldots{}") // Replace ellipsis with LaTeX equivalent
	s = strings.ReplaceAll(s, "--", "--")         // En dash
	s = strings.ReplaceAll(s, "---", "---")       // Em dash
	s = strings.ReplaceAll(s, "\"", "''")         // Simple quote replacement, might need context
	s = strings.ReplaceAll(s, "'", "`")           // Simple apostrophe replacement, might need context
	return s
}
