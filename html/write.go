package html

import (
	"fmt"
	"html/template"
	"io"
	"regexp"
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// HTMLWriter implements the writer.Writer interface for HTML output.
// It uses a rules.Set for configurable formatting elements.
type HTMLWriter struct {
	Elements rules.Set // Configuration for elements (margins, fonts, etc.)
	Page     string    // the paper to print on: letter (""), a4, a5
}

// Inline markup patterns for HTML output
var (
	bolditalic = regexp.MustCompile(`\*{3}([^\*\n]+)\*{3}`)
	bold       = regexp.MustCompile(`\*{2}([^\*\n]+)\*{2}`)
	italic     = regexp.MustCompile(`\*{1}([^\*\n]+)\*{1}`)
	underline  = regexp.MustCompile(`_{1}([^\*\n]+)_{1}`)
)

// processInlineMarkup converts fountain-style inline markup to HTML
func processInlineMarkup(text string) template.HTML {
	// Script text is not HTML: escape it before adding markup tags
	text = template.HTMLEscapeString(text)

	// Only process if the text contains markup characters
	if !strings.ContainsAny(text, "*_") {
		return template.HTML(text)
	}

	// Apply replacements in order: bold+italic first, then bold, then italic, then underline
	// This prevents conflicts between overlapping patterns
	text = bolditalic.ReplaceAllString(text, "<b><i>$1</i></b>")
	text = bold.ReplaceAllString(text, "<b>$1</b>")
	text = italic.ReplaceAllString(text, "<i>$1</i>")
	text = underline.ReplaceAllString(text, "<u>$1</u>")

	return template.HTML(text)
}

// htmlTemplateString is the template for HTML output with configurable CSS
const htmlTemplateString = `
<!DOCTYPE html>
<html>
<head>
<title>Screenplay</title>
<meta charset="UTF-8">
<style>
body {
    font-family: "{{.Config.FontFamily}}", Courier, monospace;
    font-size: {{.Config.FontSize}}pt;
    background-color: #fdfdfd;
    color: #111;
    margin: 0;
    padding: 0;
}
.page {
    margin: {{.Config.PageMargins}};
    min-height: 9in;
    max-width: 8.5in;
    margin-left: auto;
    margin-right: auto;
}
.scene-heading {
    text-transform: uppercase;
    {{.Config.SceneStyle}}
    margin-inline-start: {{.Config.SceneLeft}}in;
    margin-inline-end: {{.Config.SceneRight}}in;
    margin-top: 1.5em;
    margin-bottom: 1em;
}
.action, .general {
    margin-inline-start: {{.Config.ActionLeft}}in;
    margin-inline-end: {{.Config.ActionRight}}in;
    margin-top: 1em;
    margin-bottom: 1em;
    text-align: justify;
}
.speaker {
    text-transform: uppercase;
    margin-inline-start: {{.Config.SpeakerLeft}}in;
    margin-inline-end: {{.Config.SpeakerRight}}in;
    margin-top: 1em;
    margin-bottom: 0;
}
.dialogue {
    margin-inline-start: {{.Config.DialogLeft}}in;
    margin-inline-end: {{.Config.DialogRight}}in;
    text-align: start;
}
.parenthetical {
    margin-inline-start: {{.Config.ParenLeft}}in;
    margin-inline-end: {{.Config.ParenRight}}in;
    text-align: start;
}
.transition {
    text-transform: uppercase;
    text-align: {{.Config.TransAlign}};
    margin-inline-start: {{.Config.TransLeft}}in;
    margin-inline-end: {{.Config.TransRight}}in;
    margin-top: 1.5em;
    margin-bottom: 1.5em;
}
.empty {
    height: 1em;
}
.title-page {
	text-align: center;
	height: 100vh;
	display: flex;
	flex-direction: column;
	justify-content: center;
	align-items: center;
}
.title-page h1 {
	margin-bottom: 1em;
}
.title-page p {
	margin: 0.5em 0;
}
.title-meta {
	align-self: flex-start;
	text-align: start;
	margin-top: 3in;
}
.title-meta p {
	margin: 0;
}
.center {
	text-align: center;
	margin-inline-start: {{.Config.CenterLeft}}in;
	margin-inline-end: {{.Config.CenterRight}}in;
}
.dual-dialogue {
	width: {{.Config.DualDialogueWidth}}%;
	margin-top: 1em;
	margin-bottom: 1em;
}
.dual-dialogue td {
	width: 50%;
	vertical-align: top;
	padding: 0 {{.Config.DualCellPadding}}em;
}
.dual-dialogue .speaker {
	margin-inline-start: {{.Config.DualSpeakerLeft}}in;
	margin-inline-end: {{.Config.DualSpeakerRight}}in;
	margin-top: 0;
}
.dual-dialogue .dialogue {
	margin-inline-start: {{.Config.DualDialogLeft}}in;
	margin-inline-end: {{.Config.DualDialogRight}}in;
}
.dual-dialogue .parenthetical {
	margin-inline-start: {{.Config.DualParenLeft}}in;
	margin-inline-end: {{.Config.DualParenRight}}in;
}
.lyrics {
    {{.Config.LyricsStyle}}
    margin-inline-start: {{.Config.LyricsLeft}}in;
    margin-inline-end: {{.Config.LyricsRight}}in;
    text-align: start;
}
.scene-heading { position: relative; }
.scene-number { position: absolute; top: 0; font-weight: normal; }
.scene-number.left { inset-inline-end: calc(100% + 0.25in); }
.scene-number.right { inset-inline-start: calc(100% + 0.25in); }
/* printed as Lexington's PDF is: the paper's size, an inch above and
   below, the elements' indents from the paper's edge, one line (12 pt)
   for each blank line, page numbers top right from page 2 */
@page {
    size: {{.Config.PrintPage}};
    margin: 1in 0 1in 0;
    @top-right { content: counter(page) "."; margin-right: 1in; vertical-align: bottom; padding-bottom: 0.35in; }
}
@page :first { @top-right { content: none; } }
@page title { counter-increment: page 0; @top-right { content: none; } }
@media print {
    body { background: none; line-height: 12pt; }
    .page { margin: 0; max-width: none; min-height: 0; }
    .page:has(.title-page) { page: title; }
    /* as the PDF: centred between 1.5 and 1 in, the title a line below 4 in
       from the top, the details 2 in from the bottom */
    .title-page { height: {{.Config.PrintHeight}}; justify-content: flex-start; position: relative; margin: 0 1in 0 1.5in; }
    .title-page h1 { font-size: inherit; font-weight: normal; margin: calc(3in + 12pt) 0 0; }
    .title-page p { margin: 0; }
    .title-page em { font-style: normal; }
    .title-meta { position: absolute; left: 0; top: calc({{.Config.PrintHeight}} - 1in); margin: 0; }
    .page:has(.title-page) + .newpage { display: none; } /* the title page breaks already */
    .scene-heading, .action, .general, .speaker, .dialogue, .parenthetical,
    .transition, .center, .lyrics, .dual-dialogue { margin-top: 0; margin-bottom: 0; }
    .empty { height: 12pt; margin: 0; }
    .newpage { page-break-after: always; break-after: page; }
}
</style>
</head>
<body>
<div class="page">
{{- $inTitle := false -}}{{- $inMeta := false -}}
{{- range .Screenplay -}}
    {{- if eq .Type "titlepage" -}}
<div class="title-page">{{- $inTitle = true -}}{{-
    else if eq .Type "Title" -}}
<h1>{{- processInlineMarkup .Contents -}}</h1>{{-
    else if eq .Type "Credit" -}}
<p><em>{{- processInlineMarkup .Contents -}}</em></p>{{-
    else if eq .Type "Author" -}}
<p>{{- processInlineMarkup .Contents -}}</p>{{-
    else if eq .Type "metasection" -}}
<div class="title-meta">{{- $inMeta = true -}}{{-
    else if eq .Type "scene" -}}
{{- $h := sceneHeading .Contents -}}
<div class="scene-heading" dir="auto">{{- processInlineMarkup $h.Text -}}{{- if $h.Number -}}<span class="scene-number left">{{ $h.Number }}</span><span class="scene-number right">{{ $h.Number }}</span>{{- end -}}</div>{{-
    else if eq .Type "action" "general" -}}
<div class="action" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "speaker" -}}
<div class="speaker" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "dialog" -}}
<div class="dialogue" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "lyrics" -}}
<div class="lyrics" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "paren" -}}
<div class="parenthetical" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "trans" -}}
<div class="transition" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "center" -}}
<div class="center" dir="auto">{{- processInlineMarkup .Contents -}}</div>{{-
    else if eq .Type "newpage" -}}
{{- if $inMeta -}}</div>{{- $inMeta = false -}}{{- end -}}
{{- if $inTitle -}}</div>{{- $inTitle = false -}}{{- end -}}
</div>
<div class="newpage"></div>
<div class="page">{{-
    else if eq .Type "empty" -}}
<div class="empty"></div>{{-
    else if eq .Type "dualspeaker_open" -}}
<table class="dual-dialogue"><tr><td>{{-
    else if eq .Type "dualspeaker_next" -}}
</td><td>{{-
    else if eq .Type "dualspeaker_close" -}}
</td></tr></table>{{-
    else if $inMeta -}}
<p>{{- processInlineMarkup .Contents -}}</p>{{-
    else -}}{{- end -}}
{{- end -}}
</div>
</body>
</html>
`

// HTMLTemplateData combines configuration and screenplay data for the template
type HTMLTemplateData struct {
	Config     TemplateConfig
	Screenplay lex.Screenplay
}

// TemplateConfig holds the configuration values for the HTML template
type TemplateConfig struct {
	// Font configuration
	FontFamily string
	FontSize   float64

	// Page layout
	PageMargins string
	PrintPage   string // the CSS @page size: letter, A4, A5
	PrintHeight string // the paper's height within its margins

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

	// Dual dialogue configuration
	DualDialogueWidth int
	DualCellPadding   float64
	DualSpeakerLeft   float64
	DualSpeakerRight  float64
	DualDialogLeft    float64
	DualDialogRight   float64
	DualParenLeft     float64
	DualParenRight    float64

	// Style configuration
	SceneStyle  template.CSS
	TransAlign  string
	LyricsStyle template.CSS
}

// getTemplateConfig creates TemplateConfig from rules configuration
func (h *HTMLWriter) getTemplateConfig() TemplateConfig {
	elements := h.Elements
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

	// Use industry standard margins directly - HTML can handle them
	// The page width will be set appropriately to accommodate these margins
	htmlActionLeft := action.Left
	htmlSpeakerLeft := speaker.Left
	htmlDialogLeft := dialog.Left
	htmlParenLeft := paren.Left

	// Get dual dialogue specific configuration
	dualSpeaker := elements.Get("dualspeaker")
	dualDialog := elements.Get("dualdialog")
	dualParen := elements.Get("dualparen")

	// Use configured dual dialogue margins
	dualSpeakerLeft := dualSpeaker.Left
	dualDialogLeft := dualDialog.Left
	dualParenLeft := dualParen.Left

	// Build CSS style strings
	sceneStyle := template.CSS("font-weight: normal;")
	if scene.Style == "b" || scene.Style == "B" {
		sceneStyle = template.CSS("font-weight: bold;")
	}

	transAlign := "end" // start and end: the other way round in right-to-left text
	switch trans.Align {
	case "L":
		transAlign = "start"
	case "C":
		transAlign = "center"
	}

	lyricsStyle := template.CSS("font-style: normal;")
	if lyrics.Style == "i" || lyrics.Style == "I" {
		lyricsStyle = template.CSS("font-style: italic;")
	}
	if lyrics.Font != "" && lyrics.Font != "Courier" {
		lyricsStyle = template.CSS(fmt.Sprintf("font-style: italic; font-family: %s;", lyrics.Font))
	}

	// Create page margins string (top, right, bottom, left) - use standard margins
	pageMargins := "1in 0.5in 1in 0.5in"

	return TemplateConfig{
		// Font configuration
		FontFamily: action.Font,
		FontSize:   action.Size,

		// Page layout
		PageMargins: pageMargins,
		PrintPage:   printPage(h.Page),
		PrintHeight: printHeight(h.Page),

		// Element margins (industry standard)
		ActionLeft:   htmlActionLeft,
		ActionRight:  action.Right,
		SpeakerLeft:  htmlSpeakerLeft,
		SpeakerRight: speaker.Right,
		DialogLeft:   htmlDialogLeft,
		DialogRight:  dialog.Right,
		ParenLeft:    htmlParenLeft,
		ParenRight:   paren.Right,
		SceneLeft:    scene.Left,
		SceneRight:   scene.Right,
		TransLeft:    trans.Left,
		TransRight:   trans.Right,
		CenterLeft:   center.Left,
		CenterRight:  center.Right,
		LyricsLeft:   lyrics.Left,
		LyricsRight:  lyrics.Right,

		// Dual dialogue configuration
		DualDialogueWidth: 100,
		DualCellPadding:   0.5,
		DualSpeakerLeft:   dualSpeakerLeft,
		DualSpeakerRight:  dualSpeaker.Right,
		DualDialogLeft:    dualDialogLeft,
		DualDialogRight:   dualDialog.Right,
		DualParenLeft:     dualParenLeft,
		DualParenRight:    dualParen.Right,

		// Style configuration
		SceneStyle:  sceneStyle,
		TransAlign:  transAlign,
		LyricsStyle: lyricsStyle,
	}
}

// Write converts the internal lex.Screenplay format to a self-contained HTML file.
// It implements the writer.Writer interface.
func (h *HTMLWriter) Write(w io.Writer, screenplay lex.Screenplay) error {
	// Get template configuration from rules
	config := h.getTemplateConfig()

	// Create combined template data
	templateData := HTMLTemplateData{
		Config:     config,
		Screenplay: screenplay,
	}

	// Parse and execute the HTML template with better error context
	tmpl, err := template.New("screenplay").Funcs(template.FuncMap{
		"processInlineMarkup": processInlineMarkup,
		"sceneHeading":        sceneHeading,
	}).Parse(htmlTemplateString)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	if err := tmpl.Execute(w, templateData); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return nil
}

// printPage is the CSS @page size for a paper name.
func printPage(page string) string {
	switch strings.ToLower(page) {
	case "a4":
		return "A4"
	case "a5":
		return "A5"
	}
	return "letter"
}

// printHeight is the paper's height within an inch above and below.
func printHeight(page string) string {
	switch strings.ToLower(page) {
	case "a4":
		return "9.69in"
	case "a5":
		return "6.27in"
	}
	return "9in"
}

// heading is a scene heading's text and its number (#12#), if any.
type heading struct{ Text, Number string }

func sceneHeading(contents string) heading {
	text, number := lex.SceneNumber(contents)
	return heading{strings.TrimSpace(text), number}
}
