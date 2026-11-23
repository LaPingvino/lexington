package lex

import (
	"strconv"
	"strings"
)

// Storyboard frames, in Storyboard Fountain's notation (setpixel's
// storyboard-fountain, MIT): a note on its own line under the line it
// illustrates,
//
//	[[board: shot-01.png, duration: 2.50, continued: true]]
//
// the picture's file (in the folder a [[dataPath: …]] note in the
// script's head names, else beside the script), how long it shows in
// seconds (when set by hand) and whether it carries on from the frame
// before. Plain Fountain notes: other programs keep them as they are.

// TypePicture is a picture printed in the script's body: a storyboard
// frame (pdf.PDFWriter.Pictures turns a board note into one). Its
// Contents is the picture's name.
const TypePicture ElementType = "picture"

// Board is a storyboard frame's note.
type Board struct {
	File      string
	Duration  float64 // in seconds; 0: not set
	Continued bool
}

// ParseBoard reads a note's inside (what is between "[[" and "]]") as a
// storyboard frame: ok when it starts with "board:".
func ParseBoard(note string) (b Board, ok bool) {
	note = strings.TrimSpace(note)
	if len(note) < 6 || !strings.EqualFold(note[:6], "board:") {
		return Board{}, false
	}
	for i, part := range strings.Split(note[6:], ",") {
		part = strings.TrimSpace(part)
		if i == 0 {
			b.File = part
			continue
		}
		k, v, _ := strings.Cut(part, ":")
		v = strings.TrimSpace(v)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "duration":
			b.Duration, _ = strconv.ParseFloat(v, 64)
		case "continued":
			b.Continued = strings.EqualFold(v, "true")
		}
	}
	return b, b.File != ""
}

// String is the note's inside as Storyboard Fountain writes it.
func (b Board) String() string {
	s := "board: " + b.File
	if b.Duration > 0 {
		s += ", duration: " + strconv.FormatFloat(b.Duration, 'f', 2, 64)
	}
	if b.Continued {
		s += ", continued: true"
	}
	return s
}

// BoardOf is the storyboard frame a line is, if it is one: a line of
// nothing but a board note.
func BoardOf(l Line) (Board, bool) {
	if !IsNoteLine(l) {
		return Board{}, false
	}
	ns := Notes(l.Contents)
	if l.Type == TypeNote && len(ns) == 0 {
		ns = []string{l.Contents}
	}
	if len(ns) != 1 {
		return Board{}, false
	}
	return ParseBoard(ns[0])
}

// WithPictures is the screenplay with each storyboard frame's line (a
// board note alone on its line) a TypePicture row of its file, for the
// frames printed.
func WithPictures(s Screenplay) Screenplay {
	out := make(Screenplay, 0, len(s))
	for _, l := range s {
		if b, ok := BoardOf(l); ok {
			l = Line{Type: TypePicture, Contents: b.File, Source: l.Source, Revision: l.Revision}
		}
		out = append(out, l)
	}
	return out
}
