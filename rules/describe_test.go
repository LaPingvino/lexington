package rules

import (
	"reflect"
	"strings"
	"testing"
)

// tomlKeys are a struct type's keys as the TOML writes them.
func tomlKeys(t reflect.Type) []string {
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
		if name == "" {
			name = f.Name
		}
		keys = append(keys, name)
	}
	return keys
}

// Every field of the configuration, a language, a shape, a format and a
// format file, every element key and every cue rule is described, in
// the struct's order where there is one, and no description is of a
// field that is not there.
func TestDescribeCoversEverything(t *testing.T) {
	for _, c := range []struct {
		name   string
		typ    reflect.Type
		fields []Field
	}{
		{"TOMLConf", reflect.TypeOf(TOMLConf{}), ConfFields()},
		{"Language", reflect.TypeOf(Language{}), LanguageFields()},
		{"Shape", reflect.TypeOf(Shape{}), ShapeFields()},
		{"Format", reflect.TypeOf(Format{}), FormatFields()},
		{"presetFile", reflect.TypeOf(presetFile{}), FormatFileFields()},
	} {
		keys := tomlKeys(c.typ)
		var got []string
		for _, f := range c.fields {
			got = append(got, f.Key)
			if strings.TrimSpace(f.Doc) == "" || f.Kind == "" {
				t.Errorf("%s.%s: no doc or kind", c.name, f.Key)
			}
		}
		if c.name == "TOMLConf" || c.name == "presetFile" { // described in an order of their own
			if !sameSet(keys, got) {
				t.Errorf("%s: fields %v, described %v", c.name, keys, got)
			}
			continue
		}
		if !reflect.DeepEqual(keys, got) {
			t.Errorf("%s: fields %v, described %v", c.name, keys, got)
		}
	}
	var keys []string
	for _, f := range ElementKeys() {
		if !ConfigKey(f.Key).IsValid() {
			t.Errorf("element %q is not a valid key", f.Key)
		}
		keys = append(keys, f.Key)
	}
	for k := range Default {
		if !contains(keys, k) {
			t.Errorf("Default's %q is not described", k)
		}
	}
	for _, k := range []ConfigKey{KeyPrintNotes, KeyNoteColors, KeySection, KeyPagination} {
		if !contains(keys, k.String()) {
			t.Errorf("%q is not described", k)
		}
	}
	var cues []string
	for _, f := range CueRuleFields() {
		cues = append(cues, f.Key)
	}
	if !reflect.DeepEqual(cues, CueRules) {
		t.Errorf("cue rules %v, described %v", CueRules, cues)
	}
	if f, ok := Describe(FormatFields(), "Align"); !ok || len(f.Choices) != 3 {
		t.Errorf("Describe Align: %+v %v", f, ok)
	}
	if _, ok := Describe(FormatFields(), "Colour"); ok {
		t.Error("Describe found a field that is not there")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		if !contains(b, x) {
			return false
		}
	}
	return true
}
