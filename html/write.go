package html

import (
	"fmt"
	"html/template"
	"io"

	"github.com/lapingvino/lexington/lex"
)

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
<title>Screenplay</title>
<meta charset="UTF-8">
<style>
body {
    font-family: "Courier New", Courier, monospace;
    font-size: 12pt;
    background-color: #fdfdfd;
    color: #111;
    max-width: 8.5in;
    margin: 0 auto;
}
.page {
    margin: 1in;
    min-height: 9in;
}
.scene-heading {
    text-transform: uppercase;
    font-weight: bold;
    margin-top: 1.5em;
    margin-bottom: 1em;
}
.action, .general {
    margin-top: 1em;
    margin-bottom: 1em;
    text-align: justify;
}
.speaker {
    text-transform: uppercase;
    text-align: center;
    margin-top: 1em;
    margin-bottom: 0;
}
.dialogue {
    margin-left: 2.5in;
    margin-right: 2.5in;
    text-align: left;
}
.parenthetical {
    margin-left: 3.1in;
    margin-right: 3.1in;
    text-align: left;
}
.transition {
    text-transform: uppercase;
    text-align: right;
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
{{range .}}
    {{if eq .Type "titlepage"}}<div class="title-page">
    {{else if eq .Type "Title"}}<h1>{{.Contents}}</h1>
    {{else if eq .Type "Credit"}}<p><em>{{.Contents}}</em></p>
    {{else if eq .Type "Author"}}<p>{{.Contents}}</p>
    {{else if eq .Type "metasection"}}</div><div class="newpage"></div><div class="page">
    {{else if eq .Type "scene"}}<div class="scene-heading">{{.Contents}}</div>
    {{else if eq .Type "action" "general"}}<div class="action">{{.Contents}}</div>
    {{else if eq .Type "speaker"}}<div class="speaker">{{.Contents}}</div>
    {{else if eq .Type "dialog" "lyrics"}}<div class="dialogue">{{.Contents}}</div>
    {{else if eq .Type "paren"}}<div class="parenthetical">{{.Contents}}</div>
    {{else if eq .Type "trans"}}<div class="transition">{{.Contents}}</div>
    {{else if eq .Type "center"}}<div class="center">{{.Contents}}</div>
    {{else if eq .Type "newpage"}}</div><div class="newpage"></div><div class="page">
    {{else if eq .Type "empty"}}<div class="empty"></div>
    {{else}}<div class="general">{{.Type}}: {{.Contents}}</div>{{end}}
{{end}}
</div>
</body>
</html>
`

// Write converts the internal lex.Screenplay format to a self-contained HTML file.
func Write(w io.Writer, screenplay lex.Screenplay) error {
	tmpl, err := template.New("screenplay").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse HTML template: %w", err)
	}

	return tmpl.Execute(w, screenplay)
}
