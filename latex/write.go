package latex

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// LaTeXWriter implements the writer.Writer interface for LaTeX output.
// It uses a Go text/template for rendering and requires a rules.Set
// for formatting elements.
type LaTeXWriter struct {
	Template string    // Path to the LaTeX template file
	Elements rules.Set // Configuration for elements (margins, fonts, etc.)
	// TitlePage places the title page's fields (nil: Fountain's); the
	// LaTeX shows their order and whether they are centred or at the foot.
	TitlePage *titlepage.Template
}

// Inline markup patterns for LaTeX output
// inlineMarkup turns Fountain's emphasis (lex.Emphasis) into LaTeX:
// the text escaped, in \textbf, \textit and \underline.
func inlineMarkup(text string) string {
	var b strings.Builder
	for _, s := range lex.Emphasis(text) {
		t := escapeLaTeX(s.Text)
		if s.Underline {
			t = `\underline{` + t + `}`
		}
		if s.Italic {
			t = `\textit{` + t + `}`
		}
		if s.Bold {
			t = `\textbf{` + t + `}`
		}
		b.WriteString(t)
	}
	return b.String()
}

// LaTeXTemplateData combines configuration and screenplay data for the template
type LaTeXTemplateData struct {
	Config     LaTeXConfig
	Screenplay lex.Screenplay
}

// LaTeXConfig holds the configuration values for the LaTeX template
type LaTeXConfig struct {
	// Page layout
	LeftMargin   float64
	RightMargin  float64
	TopMargin    float64
	BottomMargin float64

	// Element margins (in inches)
	ActionLeft   float64
	ActionRight  float64
	SpeakerLeft  float64
	SpeakerRight float64
	DialogLeft   float64
	DialogRight  float64
	ParenLeft    float64
	ParenRight   float64
	SceneLeft    float64
	SceneRight   float64
	TransLeft    float64
	TransRight   float64
	CenterLeft   float64
	CenterRight  float64
	LyricsLeft   float64
	LyricsRight  float64

	// Font configuration
	FontFamily string
	FontSize   float64

	// Dual dialogue configuration
	DualSpeakerLeft float64
	DualDialogLeft  float64
	DualParenLeft   float64
}

const defaultLaTeXTemplate = `\documentclass[{{printf "%.0f" .Config.FontSize}}pt]{article}
\usepackage[utf8]{inputenc}
\usepackage[T1]{fontenc}
\IfFileExists{lmodern.sty}{\usepackage{lmodern}}{} % not in every TeX (Debian has it apart)
\usepackage[letterpaper,
            left={{printf "%.1f" .Config.LeftMargin}}in,
            right={{printf "%.1f" .Config.RightMargin}}in,
            top={{printf "%.1f" .Config.TopMargin}}in,
            bottom={{printf "%.1f" .Config.BottomMargin}}in]{geometry}
\usepackage{setspace} % For line spacing
\usepackage{fancyhdr} % For page headers
\usepackage{array} % For dual dialogue tables

% Set up courier font for screenplay formatting
\usepackage{courier}
\renewcommand{\familydefault}{\ttdefault}

% Set up page headers and footers
\pagestyle{fancy}
\fancyhf{} % Clear all headers and footers
\fancyfoot[R]{\thepage.} % Page number in bottom right
\renewcommand{\headrulewidth}{0pt}
\renewcommand{\footrulewidth}{0pt}

% Define screenplay formatting commands with configurable indentation
\newcommand{\sceneheading}[1]{\noindent\hspace{ {{- printf "%.1f" .Config.SceneLeft -}}in}` +
	`\textbf{\MakeUppercase{#1}}\par\vspace{0.5\baselineskip}}
\newcommand{\action}[1]{\noindent\hspace{ {{- printf "%.1f" .Config.ActionLeft -}}in}` +
	`\parbox{ {{- printf "%.1f" (sub 8.5 .Config.LeftMargin .Config.RightMargin ` +
	`.Config.ActionLeft .Config.ActionRight) -}}in}{#1}` +
	`\par\vspace{\baselineskip}}
\newcommand{\character}[1]{\noindent\hspace{ {{- printf "%.1f" .Config.SpeakerLeft -}}in}` +
	`\textbf{\MakeUppercase{#1}}\par}
\newcommand{\dialogue}[1]{\noindent\hspace{ {{- printf "%.1f" .Config.DialogLeft -}}in}` +
	`\parbox{ {{- printf "%.1f" (sub 8.5 .Config.LeftMargin .Config.RightMargin ` +
	`.Config.DialogLeft .Config.DialogRight) -}}in}{#1}\par}
\newcommand{\parenthetical}[1]{\noindent\hspace{ {{- printf "%.1f" .Config.ParenLeft -}}in}\textit{#1}\par}
\newcommand{\transition}[1]{\noindent\hfill\textbf{\MakeUppercase{#1}}\par\vspace{\baselineskip}}
\newcommand{\centeredtext}[1]{\begin{center}#1\end{center}\par}

% Title page commands
\newcommand{\titletext}[1]{\begin{center}\textbf{\large #1}\end{center}\vspace{\baselineskip}}
\newcommand{\credittext}[1]{\begin{center}\textit{#1}\end{center}\vspace{0.5\baselineskip}}
\newcommand{\authorname}[1]{\begin{center}#1\end{center}\vspace{\baselineskip}}

% Dual dialogue environment using tabular with configurable spacing
\newenvironment{dualdialogue}{\noindent\begin{tabular}{` +
	`p{ {{- printf "%.1f" (div (sub 8.5 .Config.LeftMargin .Config.RightMargin ` +
	`.Config.ActionLeft .Config.ActionRight) 2.2) -}}in}` +
	`@{\hspace{0.3in}}` +
	`p{ {{- printf "%.1f" (div (sub 8.5 .Config.LeftMargin .Config.RightMargin ` +
	`.Config.ActionLeft .Config.ActionRight) 2.2) -}}in}}}` +
	`{\end{tabular}\par\vspace{\baselineskip}}
\newcommand{\leftcol}{}
\newcommand{\rightcol}{ & }

% Dual dialogue specific commands with configurable margins
\newcommand{\dualcharacter}[1]{\textbf{\MakeUppercase{#1}}\par}
\newcommand{\dualtext}[1]{#1\par}
\newcommand{\dualparenthetical}[1]{\textit{#1}\par}

\begin{document}

{{- $title := false -}}{{- $foot := false -}}
{{range .Screenplay}}
    {{if eq .Type "titlepage"}}{{$title = true}}
        \thispagestyle{empty}
        \vspace*{2in}
    {{else if eq .Type "Title"}}\titletext{ {{- .Contents -}} }
    {{else if eq .Type "Credit"}}\credittext{ {{- .Contents -}} }
    {{else if eq .Type "Author"}}\authorname{ {{- .Contents -}} }
    {{else if eq .Type "metasection"}}{{$foot = true}}
        \vfill
    {{else if eq .Type "scene"}}
        \sceneheading{ {{- .Contents -}} }
    {{else if eq .Type "action"}}
        \action{ {{- .Contents -}} }
    {{else if eq .Type "speaker"}}
        \character{ {{- .Contents -}} }
    {{else if eq .Type "dialog"}}
        \dialogue{ {{- .Contents -}} }
    {{else if eq .Type "paren"}}
        \parenthetical{ {{- .Contents -}} }
    {{else if eq .Type "trans"}}
        \transition{ {{- .Contents -}} }
    {{else if eq .Type "center"}}
        \centeredtext{ {{- .Contents -}} }
    {{else if eq .Type "newpage"}}{{$title = false}}{{$foot = false}}
        \newpage
    {{else if eq .Type "empty"}}
        \vspace{\baselineskip}
    {{else if eq .Type "dualspeaker_open"}}
        \begin{dualdialogue}
            \leftcol
    {{else if eq .Type "dualspeaker_next"}}
            \rightcol
    {{else if eq .Type "dualspeaker_close"}}
        \end{dualdialogue}
    {{else if eq .Type "dualspeaker"}}
        \dualcharacter{ {{- .Contents -}} }
    {{else if eq .Type "dualdialog"}}
        \dualtext{ {{- .Contents -}} }
    {{else if eq .Type "dualparen"}}
        \dualparenthetical{ {{- .Contents -}} }
    {{else if $foot}}\noindent {{.Contents}}\par
    {{else if $title}}\authorname{ {{- .Contents -}} }
    {{else}}
        % Ignore unhandled types
    {{end}}
{{end}}

\end{document}
`

// getLatexConfig creates LaTeXConfig from rules configuration
func (l *LaTeXWriter) getLatexConfig() LaTeXConfig {
	elements := l.Elements
	if elements == nil {
		elements = rules.Default
	}

	// Get format configurations
	action := elements.Get("action")
	speaker := elements.Get("speaker")
	dialog := elements.Get("dialog")
	paren := elements.Get("paren")
	scene := elements.Get("scene")
	trans := elements.Get("trans")
	center := elements.Get("center")
	lyrics := elements.Get("lyrics")

	// Get dual dialogue configurations
	dualSpeaker := elements.Get("dualspeaker")
	dualDialog := elements.Get("dualdialog")
	dualParen := elements.Get("dualparen")

	// Standard page margins
	pageLeftMargin := 1.0
	pageRightMargin := 1.0

	return LaTeXConfig{
		// Page layout - use standard margins
		LeftMargin:   pageLeftMargin,
		RightMargin:  pageRightMargin,
		TopMargin:    1.0,
		BottomMargin: 1.0,

		// Element margins - adjust for page margin to get absolute positioning
		ActionLeft:   action.Left - pageLeftMargin,
		ActionRight:  action.Right,
		SpeakerLeft:  speaker.Left - pageLeftMargin,
		SpeakerRight: speaker.Right,
		DialogLeft:   dialog.Left - pageLeftMargin,
		DialogRight:  dialog.Right,
		ParenLeft:    paren.Left - pageLeftMargin,
		ParenRight:   paren.Right,
		SceneLeft:    scene.Left - pageLeftMargin,
		SceneRight:   scene.Right,
		TransLeft:    trans.Left - pageLeftMargin,
		TransRight:   trans.Right,
		CenterLeft:   center.Left - pageLeftMargin,
		CenterRight:  center.Right,
		LyricsLeft:   lyrics.Left - pageLeftMargin,
		LyricsRight:  lyrics.Right,

		// Font configuration
		FontFamily: action.Font,
		FontSize:   action.Size,

		// Dual dialogue margins - adjust for page margin
		DualSpeakerLeft: dualSpeaker.Left - pageLeftMargin,
		DualDialogLeft:  dualDialog.Left - pageLeftMargin,
		DualParenLeft:   dualParen.Left - pageLeftMargin,
	}
}

// preprocessDualDialogue converts speaker/dialog elements to dualspeaker/dualdialog
// when they appear inside dual dialogue blocks
func preprocessDualDialogue(screenplay lex.Screenplay) lex.Screenplay {
	var result lex.Screenplay
	inDualDialogue := false

	for _, line := range screenplay {
		newLine := line

		// Track dual dialogue state
		switch line.Type {
		case "dualspeaker_open":
			inDualDialogue = true
		case "dualspeaker_close":
			inDualDialogue = false
		case "speaker":
			if inDualDialogue {
				newLine.Type = "dualspeaker"
			}
		case "dialog":
			if inDualDialogue {
				newLine.Type = "dualdialog"
			}
		case "paren":
			if inDualDialogue {
				newLine.Type = "dualparen"
			}
		}

		result = append(result, newLine)
	}

	return result
}

// printsNot reports whether the LaTeX leaves out lines of a type (see
// lex.Printable): its template prints these, and the title page's.
func printsNot(t lex.ElementType) bool {
	switch t {
	case lex.TypeScene, lex.TypeAction, lex.TypeSpeaker, lex.TypeDialog, lex.TypeParen, lex.TypeTrans, lex.TypeCenter:
		return false
	}
	return true
}

// Write converts the internal lex.Screenplay format to a LaTeX file.
// It implements the writer.Writer interface.
//
// Note: This function only generates the .tex file. Compiling it into a PDF
// requires a LaTeX distribution (e.g., TeX Live) with a screenplay package
// (like `screenwright` or `fountain-latex`) installed on the system.
// The PDF generation step is external to this Go program.
func (l *LaTeXWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// Preprocess screenplay to handle dual dialogue; notes are not printed
	screenplay = preprocessDualDialogue(lex.Printable(screenplay, printsNot))
	tpl := titlepage.Standard()
	if l.TitlePage != nil {
		tpl = *l.TitlePage
	}
	screenplay = tpl.Stream(screenplay) // its fields by key: the centred ones, then the foot's

	// Get template configuration from rules
	config := l.getLatexConfig()

	// Create combined template data
	data := LaTeXTemplateData{
		Config:     config,
		Screenplay: screenplay,
	}

	// Attempt to parse the template from the provided path, or use the default
	var tmpl *template.Template
	var err error

	// Create template with helper functions
	funcMap := template.FuncMap{
		"sub": func(a float64, rest ...float64) float64 {
			result := a
			for _, val := range rest {
				result -= val
			}
			return result
		},
		"div":    func(a, b float64) float64 { return a / b },
		"mul":    func(a, b float64) float64 { return a * b },
		"printf": fmt.Sprintf,
	}

	if l.Template != "" {
		tmpl, err = template.New("latexScreenplay").Funcs(funcMap).ParseFiles(l.Template)
		if err != nil {
			return fmt.Errorf("failed to parse LaTeX template file %s: %w", l.Template, err)
		}
	} else {
		tmpl, err = template.New("latexScreenplay").Funcs(funcMap).Parse(defaultLaTeXTemplate)
		if err != nil {
			return fmt.Errorf("failed to parse default LaTeX template: %w", err)
		}
	}

	// The text escaped, its emphasis as LaTeX commands
	for i := range data.Screenplay {
		data.Screenplay[i].Contents = inlineMarkup(data.Screenplay[i].Contents)
	}

	return tmpl.Execute(w, data)
}

// escapeLaTeX escapes characters that have special meaning in LaTeX.
// This is a basic implementation and might need to be extended for more complex scenarios.
func escapeLaTeX(s string) string {
	// Escape backslash FIRST to avoid double-escaping
	s = strings.ReplaceAll(s, "\\", "\\textbackslash{}")
	s = strings.ReplaceAll(s, "&", "\\&")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "$", "\\$")
	s = strings.ReplaceAll(s, "#", "\\#")
	s = strings.ReplaceAll(s, "_", "\\_")
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	s = strings.ReplaceAll(s, "~", "\\textasciitilde{}")
	s = strings.ReplaceAll(s, "^", "\\textasciicircum{}")
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
