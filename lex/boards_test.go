package lex

import "testing"

func TestParseBoard(t *testing.T) {
	for _, c := range []struct {
		note string
		want Board
		ok   bool
	}{
		{"board: shot-01.png", Board{File: "shot-01.png"}, true},
		{" board: a b.png, duration: 2.50, continued: true", Board{File: "a b.png", Duration: 2.5, Continued: true}, true},
		{"Board: x.jpg, continued: false", Board{File: "x.jpg"}, true},
		{"board:", Board{}, false},
		{"from: #:~:text=x", Board{}, false},
		{"a note", Board{}, false},
	} {
		got, ok := ParseBoard(c.note)
		if got != c.want || ok != c.ok {
			t.Errorf("%q: %+v %v", c.note, got, ok)
		}
		if ok {
			if again, _ := ParseBoard(got.String()); again != got {
				t.Errorf("%q: written %q, read back %+v", c.note, got.String(), again)
			}
		}
	}
	if s := (Board{File: "f.png", Duration: 2.5, Continued: true}).String(); s != "board: f.png, duration: 2.50, continued: true" {
		t.Errorf("written %q", s)
	}
}

func TestWithPictures(t *testing.T) {
	s := Screenplay{
		{Type: TypeAction, Contents: "She runs.", Source: 1},
		{Type: TypeAction, Contents: "[[board: run.png, duration: 1.00]]", Source: 2},
		{Type: TypeNote, Contents: "[[a note]]", Source: 3},
		{Type: TypeAction, Contents: "Rain [[board: no.png]]", Source: 4},
	}
	got := WithPictures(s)
	if got[1] != (Line{Type: TypePicture, Contents: "run.png", Source: 2}) {
		t.Errorf("the frame's row: %+v", got[1])
	}
	if got[2] != s[2] || got[3] != s[3] || got[0] != s[0] {
		t.Errorf("other rows changed: %+v", got)
	}
}
