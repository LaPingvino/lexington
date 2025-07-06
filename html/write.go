package html

import (
	"fmt"
	"html/template"
	"io"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// HTMLWriter implements the writer.Writer interface for HTML output.
// It uses a rules.Set for configurable formatting elements.
type HTMLWriter struct {
	Elements rules.Set // Configuration for elements (margins, fonts, etc.)
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
    margin-left: {{.Config.SceneLeft}}in;
    margin-right: {{.Config.SceneRight}}in;
    margin-top: 1.5em;
    margin-bottom: 1em;
}
.action, .general {
    margin-left: {{.Config.ActionLeft}}in;
    margin-right: {{.Config.ActionRight}}in;
    margin-top: 1em;
    margin-bottom: 1em;
    text-align: justify;
}
.speaker {
    text-transform: uppercase;
    margin-left: {{.Config.SpeakerLeft}}in;
    margin-right: {{.Config.SpeakerRight}}in;
    margin-top: 1em;
    margin-bottom: 0;
}
.dialogue {
    margin-left: {{.Config.DialogLeft}}in;
    margin-right: {{.Config.DialogRight}}in;
    text-align: left;
}
.parenthetical {
    margin-left: {{.Config.ParenLeft}}in;
    margin-right: {{.Config.ParenRight}}in;
    text-align: left;
}
.transition {
    text-transform: uppercase;
    text-align: {{.Config.TransAlign}};
    margin-left: {{.Config.TransLeft}}in;
    margin-right: {{.Config.TransRight}}in;
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
.center {
	text-align: center;
	margin-left: {{.Config.CenterLeft}}in;
	margin-right: {{.Config.CenterRight}}in;
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
	margin-left: {{.Config.DualSpeakerLeft}}in;
	margin-right: {{.Config.DualSpeakerRight}}in;
	margin-top: 0;
}
.dual-dialogue .dialogue {
	margin-left: {{.Config.DualDialogLeft}}in;
	margin-right: {{.Config.DualDialogRight}}in;
}
.dual-dialogue .parenthetical {
	margin-left: {{.Config.DualParenLeft}}in;
	margin-right: {{.Config.DualParenRight}}in;
}
.lyrics {
    {{.Config.LyricsStyle}}
    margin-left: {{.Config.LyricsLeft}}in;
    margin-right: {{.Config.LyricsRight}}in;
    text-align: left;
}
@media print {
    .newpage {
        page-break-after: always;
    }
}
</style>
</head>
<body>
<div class="page">
{{- range .Screenplay -}}
    {{- if eq .Type "titlepage" -}}<div class="title-page">{{-
    else if eq .Type "Title" -}}<h1>{{.Contents}}</h1>{{-
    else if eq .Type "Credit" -}}<p><em>{{.Contents}}</em></p>{{-
    else if eq .Type "Author" -}}<p>{{.Contents}}</p>{{-
    else if eq .Type "metasection" -}}</div><div class="newpage"></div><div class="page">{{-
    else if eq .Type "scene" -}}<div class="scene-heading">{{.Contents}}</div>{{-
    else if eq .Type "action" "general" -}}<div class="action">{{.Contents}}</div>{{-
    else if eq .Type "speaker" -}}<div class="speaker">{{.Contents}}</div>{{-
    else if eq .Type "dialog" -}}<div class="dialogue">{{.Contents}}</div>{{-
    else if eq .Type "lyrics" -}}<div class="lyrics">{{.Contents}}</div>{{-
    else if eq .Type "paren" -}}<div class="parenthetical">{{.Contents}}</div>{{-
    else if eq .Type "trans" -}}<div class="transition">{{.Contents}}</div>{{-
    else if eq .Type "center" -}}<div class="center">{{.Contents}}</div>{{-
    else if eq .Type "newpage" -}}</div><div class="newpage"></div><div class="page">{{-
    else if eq .Type "empty" -}}<div class="empty"></div>{{-
    else if eq .Type "dualspeaker_open" -}}<table class="dual-dialogue"><tr><td>{{-
    else if eq .Type "dualspeaker_next" -}}</td><td>{{-
    else if eq .Type "dualspeaker_close" -}}</td></tr></table>{{-
    else -}}<div class="general">{{.Type}}: {{.Contents}}</div>{{- end -}}
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

	transAlign := "right"
	switch trans.Align {
	case "L":
		transAlign = "left"
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
	tmpl, err := template.New("screenplay").Parse(htmlTemplateString)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	if err := tmpl.Execute(w, templateData); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}

	return nil
}
