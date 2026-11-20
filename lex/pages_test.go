package lex

import "testing"

func TestPageLockText(t *testing.T) {
	l := PageLock{Paper: "a4", Continueds: true, Pages: []LockedPage{
		{Label: "1", Words: "int house day she"},
		{Label: "2", Scene: "1", Before: "int house day she", Words: "runs out"},
		{Label: "2A", Scene: "3B", Nth: 1, Before: "so he left", Words: "and then"},
	}}
	text := "scene numbers: locked, 10A\n" + l.Text() + "\nrevision: Blue, 2026-11-20"
	back := ParsePageLock(text)
	if back == nil || back.Paper != "a4" || !back.Continueds || len(back.Pages) != 3 || back.Pages[2] != l.Pages[2] {
		t.Fatalf("read back %+v from\n%s", back, text)
	}
	if from, to, ok := PageLockLines(text); !ok || from != 1 || to != 5 {
		t.Fatalf("lines %d-%d %v", from, to, ok)
	}
	if ParsePageLock("pages: unlocked") != nil || ParsePageLock("nothing") != nil {
		t.Fatal("a lock where there is none")
	}
}

// TestLocatePages: a page is found by its words in its scene, by the words
// before it when its own changed, near; and gone when its text is.
func TestLocatePages(t *testing.T) {
	s := Screenplay{
		{Type: TypeScene, Contents: "INT. HOUSE - DAY #1#"},
		{Type: TypeAction, Contents: "Anna opens the door and looks out at the rain falling on the garden."},
		{Type: TypeAction, Contents: "Ben comes in from the kitchen with two cups of tea and a plate."},
		{Type: TypeScene, Contents: "EXT. GARDEN - NIGHT #2#"},
		{Type: TypeAction, Contents: "The rain stops. Everything is quiet now and the moon comes out."},
	}
	lock := PageLock{Pages: PageAnchors(s, []PagePos{{0, 0}, {2, 0}, {4, 3}})}
	if lock.Pages[2].Scene != "2" || lock.Pages[2].Words != "everything is quiet now and the moon comes" {
		t.Fatalf("anchor %+v", lock.Pages[2])
	}
	if got := LocatePages(s, &lock); got[1] != (PagePos{2, 0}) || got[2] != (PagePos{4, 3}) {
		t.Fatalf("located %v", got)
	}
	edited := append(Screenplay(nil), s...)
	edited[2].Contents = "Carla runs in from the yard with two cups of tea and a plate."
	if got := LocatePages(edited, &lock); got[1] != (PagePos{2, 0}) {
		t.Fatalf("first words edited: %v", got)
	}
	gone := append(append(Screenplay(nil), s[:2]...), s[3:]...)
	if got := LocatePages(gone, &lock); got[1] != Missing || got[2] != (PagePos{3, 3}) {
		t.Fatalf("a page's text gone: %v", got)
	}
}
