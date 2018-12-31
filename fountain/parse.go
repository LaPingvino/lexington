package parse

import (
	"strings"
)

var example = `
INT. HOUSE - DAY

MARY
I can't believe how easy it is to write in Fountain.

TOM
(typing)
Look! I just made a parenthetical!

SOMETHING HAPPENS!

(what? I don't know...)

EXT. GARDEN

TOM
What am I doing here now?
To be honest, I have absolutely no idea!
  
And that means really no idea!
`

var action = map[string]struct {
	Left, Width float64
}{
	"action":  {1.5, 6},
	"speaker": {4.2, 3.3},
	"dialog":  {2.9, 3.3},
	"scene":   {1.5, 6},
	"paren":   {3.6, 2},
	"trans":   {6, 1.5},
	"note":    {1.5, 6},
	"allcaps": {1.5, 6},
	"empty":   {1.5, 6},
}

var tr func(string) string

type Tree struct {
	PDF *gofpdf.Fpdf
	F   []struct {
		Format string
		Text   string
	}
}

func (t Tree) pr(a string, text string) {
	line(t.PDF, action[a].Left, action[a].Width, text)
}

func (t Tree) Render() {
	for _, row := range t.F {
		t.pr(row.Format, row.Text)
	}
}

func line(pdf *gofpdf.Fpdf, jump, width float64, text string) {
	pdf.SetX(jump)
	pdf.MultiCell(width, 0.19, tr(text), "", "aligned", false)
}

func (t *Tree) ParseString(play string) {
	toParse := strings.Split(play, "\n")
	for i, row := range toParse {
		action := "action"
		if row == strings.ToUpper(row) {
			action = "allcaps"
		}
		if row == "" {
			action = "empty"
		} else {
			if i > 0 {
				switch t.F[i-1].Format {
				case "allcaps":
					t.F[i-1].Format = "speaker"
					if row[0] == '(' && row[len(row)-1] == ')' {
						action = "paren"
					} else {
						action = "dialog"
					}
				case "paren", "dialog":
					action = "dialog"
				}
			}
		}
		t.F = append(t.F, struct{ Format, Text string }{action, row})
	}
}
