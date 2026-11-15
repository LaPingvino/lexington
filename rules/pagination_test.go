package rules

import "testing"

// Pagination: all of it without a rule (the settings files from before
// it), none with a hidden one, what the Style names otherwise; as written,
// read back.
func TestPagination(t *testing.T) {
	if got := Default.Pagination(); got != AllPagination {
		t.Errorf("default: %s", got)
	}
	if got := (Set{"pagination": {Hide: true}}).Pagination(); got != (Pagination{}) {
		t.Errorf("hidden: %s", got)
	}
	for text, want := range map[string]Pagination{
		"": AllPagination, "all": AllPagination, "none": {}, "off": {}, "keep": {Keep: true},
		"widows+more": {Widows: true, More: true}, "Keep + Widows": {Keep: true, Widows: true},
	} {
		got, err := ParsePagination(text)
		if err != nil || got != want {
			t.Errorf("%q: %s, %v", text, got, err)
		}
		if back, _ := ParsePagination(got.String()); back != got {
			t.Errorf("%q: %s reads back as %s", text, got, back)
		}
		if set := Default.WithPagination(want); set.Pagination() != want || !IsSwitch("pagination") {
			t.Errorf("%q: the set's is %s", text, set.Pagination())
		}
	}
	if _, err := ParsePagination("tidy"); err == nil {
		t.Error("tidy: no error")
	}
}
