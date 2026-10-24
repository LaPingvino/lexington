package fountain_test

import (
	"strings"
	"testing"

	. "github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

// Scene headings in Hebrew, Chinese and Japanese (a word ending in a sign
// needs nothing after it).
func TestScenesOtherScripts(t *testing.T) {
	conf := rules.DefaultConf()
	for lang, heading := range map[string]string{
		"he": "פנים. סדנה. יום.", "zh": "内景 客厅 - 日", "ja": "○喫茶店・店内（夜）", "ar": "داخلي - منزل - نهار",
	} {
		s := Parse(conf.Scenes[lang], strings.NewReader(heading+"\n\nSomething happens.\n"))
		if len(s) == 0 || s[0].Type != "scene" {
			t.Errorf("%s: %q read as %+v", lang, heading, s)
		}
	}
	if StartsScene("INTERIOR", "INT") || !StartsScene("INT. HOUSE", "INT") {
		t.Error("English words need a space or a dot after them")
	}
}

func TestAllCaps(t *testing.T) {
	for s, want := range map[string]bool{"JOHN": true, "R2-D2": true, "John": false, "שרה": false, "سارة": false, "123": false} {
		if AllCaps(s) != want {
			t.Errorf("AllCaps(%q) = %v", s, !want)
		}
	}
}
