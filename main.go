package main

import (
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/pdf"
	"github.com/lapingvino/lexington/rules"
)

func main() {
	pdf.Create("fountain.pdf", rules.Default, lex.Screenplay{})
}
