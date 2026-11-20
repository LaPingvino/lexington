package lex

import (
	"testing"
	"time"
)

func TestRevisionNames(t *testing.T) {
	for n, want := range map[int]string{0: "White", 1: "Blue", 8: "Cherry", 9: "Second Blue", 16: "Second Cherry", 17: "Third Blue"} {
		if got := RevisionName(n); got != want {
			t.Errorf("%d: %q, want %q", n, got, want)
		}
		if RevisionIndex(want) != n {
			t.Errorf("index of %q", want)
		}
	}
	if RevisionColor("2nd Pink Rev.") != "Pink" || RevisionColor("Tan") != "" {
		t.Error("colours")
	}
	s := RevisionSet{Name: "Blue", Date: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
	r := &Revisions{}
	if h := r.HeaderFor(s); h != "Blue Revision – 20/11/26" {
		t.Errorf("header %q", h)
	}
	r = &Revisions{Header: "{set} Rev. {date}", DateLayout: "01/02/06"}
	if h := r.HeaderFor(s); h != "Blue Rev. 11/20/26" {
		t.Errorf("header %q", h)
	}
	if h := r.HeaderFor(RevisionSet{Name: "Pink"}); h != "Pink Rev." {
		t.Errorf("undated %q", h)
	}
	if h := (&Revisions{Header: "-"}).HeaderFor(s); h != "" {
		t.Errorf("none: %q", h)
	}
}
