package cli

import (
	"testing"

	"github.com/LaPingvino/lexington/rules"
)

// The part of a file's name before its extension is its language only
// when it names one: script.back.fountain is English Fountain, written
// with no needless dots, as script.fountain is.
func TestLanguageFromName(t *testing.T) {
	conf := rules.DefaultConf()
	conf.Languages["mine"] = conf.Language("en")
	for _, c := range []struct{ in, out, sceneIn, sceneOut string }{
		{"a.zh.fountain", "b.he.fountain", "zh", "he"},
		{"a.back.fountain", "b.back.fountain", "", ""},
		{"film.v2.fdx", "film.de-CH.fountain", "", "de-CH"},
		{"a.mine.fountain", "b.hekaya", "mine", rules.Hekaya},
	} {
		o := Options{Input: c.in, Output: c.out}
		detectFormats(&o, conf)
		if o.SceneIn != c.sceneIn || o.SceneOut != c.sceneOut {
			t.Errorf("%s → %s: read in %q, written in %q; want %q, %q",
				c.in, c.out, o.SceneIn, o.SceneOut, c.sceneIn, c.sceneOut)
		}
	}
}
