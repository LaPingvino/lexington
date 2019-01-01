// The lex format is basically a parse tree for screenplays, which enables quick debugging.
package lex

type Screenplay []Line

type Line struct {
	Type     string
	Contents string
}
