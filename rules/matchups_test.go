package rules

import "testing"

func TestTranslate(t *testing.T) {
	c := DefaultConf()
	for _, x := range []struct{ word, from, to, want string }{
		{"INT", "en", "nl", "BIN."},
		{"EXT.", "en", "eo", "EKST."},
		{"INT./EXT", "en", "nl", "BIN./BUI."},
		{"I/E", "en", "de", "INT./EXT."},
		{"EXT/INT", "en", "ru", "НАТ./ИНТ."},
		{"EST", "en", "de", "ETABL."},
		{"EST", "en", "ar", "لقطة تأسيسية"},
		{"EST.", "it", "en", "EXT."}, // esterno
		{"BI", "nl", "en", "INT."},
		{"BI/BU", "nl", "en", "INT./EXT."},
		{"EN.", "eo", "en", "INT."},
		{"И/Н", "ru", "en", "INT./EXT."},
	} {
		if got, ok := c.Translate(x.word, x.from, x.to); !ok || got != x.want {
			t.Errorf("%s %s->%s: %q %v, want %q", x.word, x.from, x.to, got, ok, x.want)
		}
	}
	if got, ok := c.Translate("EST", "en", "nl"); ok {
		t.Errorf("EST en->nl: %q, want none", got)
	}
	// without matchups, the slash trick
	c.Scenes["xx"] = []string{"IN", "UT", "IN/UT"}
	if got, ok := c.Translate("INT.", "en", "xx"); !ok || got != "IN" {
		t.Errorf("en->xx: %q %v", got, ok)
	}
}
