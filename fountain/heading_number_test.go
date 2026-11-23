package fountain

import (
	"testing"

	"github.com/LaPingvino/lexington/rules"
)

// A heading's own number is read and written in its words, the rest of
// the line as it was typed: per language, put in, changed, left out.
func TestHeadingNumber(t *testing.T) {
	c := rules.DefaultConf()
	for _, tc := range []struct {
		lang, row, n, want string
		ok                 bool
	}{
		// changed in place, the spaces, a note and Fountain's #n# kept (equal)
		{"zh", "场景3  厨房  日  内", "4", "场景4  厨房  日  内", true},
		{"zh", "场景3 厨房 日 内 #3# [[看]]", "12A", "场景12A 厨房 日 内 #12A# [[看]]", true},
		{"zh", "12. 街道 黄昏 外", "13", "13. 街道 黄昏 外", true},
		{"he", "1. פנים. משרד דפנה – יום.", "2", "2. פנים. משרד דפנה – יום.", true},
		{"hi-Deva", "3. रसोई/दिन/अंदर", "4", "4. रसोई/दिन/अंदर", true},
		{"ar", "مشهد ١٢ شقة إنجي نهار/داخلي", "13", "مشهد ١٣ شقة إنجي نهار/داخلي", true},
		{"zh", "场景十二 厨房 日 内", "13", "场景十三 厨房 日 内", true},
		{"zh", ".场景2 厨房 日 内", "3", ".场景3 厨房 日 内", true},
		// left out, with its label
		{"zh", "场景3  厨房 日 内", "", "厨房 日 内", true},
		{"zh", "场景3 厨房 日 内 #3#", "", "厨房 日 内", true},
		{"he", "1. פנים. משרד דפנה – יום.", "", "פנים. משרד דפנה – יום.", true},
		{"hi-Deva", "3. रसोई/दिन/अंदर", "", "रसोई/दिन/अंदर", true},
		{"de", "18. GERICHTSSAAL INNEN/TAG", "", "GERICHTSSAAL INNEN/TAG", true},
		// put in by the pattern it is without one
		{"zh", "厨房  日 内", "5", "场景5 厨房  日 内", true},
		{"he", "פנים. משרד דפנה – יום.", "7", "7. פנים. משרד דפנה – יום.", true},
		{"hi-Deva", "रसोई/दिन/अंदर", "2", "2. रसोई/दिन/अंदर", true},
		{"de", "GERICHTSSAAL INNEN/TAG", "18", "18. GERICHTSSAAL INNEN/TAG", true},
		{"it", "INT. CUCINA - GIORNO #4#", "5", "5. INT. CUCINA - GIORNO #5#", true},
		// no number in its words: English, Hindi written as English
		{"en", "INT. KITCHEN - DAY #3#", "4", "INT. KITCHEN - DAY #3#", false},
		{"hi", "INT. KITCHEN - DAY", "4", "INT. KITCHEN - DAY", false},
		{"zh", "厨房 日 内", "", "厨房 日 内", false},
		{"zh", "他在外面等着。", "3", "他在外面等着。", false},
	} {
		d := NewDialect(c.Language(tc.lang))
		got, ok := d.WithHeadingNumber(tc.row, tc.n)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: WithHeadingNumber(%q, %q) = %q, %v; want %q, %v", tc.lang, tc.row, tc.n, got, ok, tc.want, tc.ok)
		}
		if !tc.ok || tc.n == "" {
			continue
		}
		if n, has := d.HeadingNumber(got); !has || n != tc.n {
			t.Errorf("%s: HeadingNumber(%q) = %q, %v", tc.lang, got, n, has)
		}
		if !d.Heading(got) {
			t.Errorf("%s: %q is no heading", tc.lang, got)
		}
	}
}

// The number reads in the digits it is written in, as Western digits;
// lettered as a locked script letters them.
func TestHeadingNumberRead(t *testing.T) {
	c := rules.DefaultConf()
	for _, tc := range []struct{ lang, row, n string }{
		{"zh", "场景十二 厨房 日 内", "12"},
		{"zh", "场景一百零五 厨房 日 内", "105"},
		{"zh", "场景10AA 厨房 日 内", "10AA"},
		{"zh", "场景10aA 厨房 日 内", "10aA"},
		{"zh", "场景AB11 厨房 日 内", "AB11"},
		{"ar", "مشهد ١٢ شقة إنجي نهار/داخلي", "12"},
		{"hi-Deva", "3. रसोई/दिन/अंदर", "3"},
	} {
		d := NewDialect(c.Language(tc.lang))
		if n, ok := d.HeadingNumber(tc.row); !ok || n != tc.n {
			t.Errorf("%s: HeadingNumber(%q) = %q, %v; want %q", tc.lang, tc.row, n, ok, tc.n)
		}
	}
	// a number before the place, not part of it
	if p, _ := NewDialect(c.Language("hi-Deva")).HeadingParts("3. रसोई/दिन/अंदर"); p["n"] != "3" || p["place"] != "रसोई" {
		t.Errorf("hi-Deva parts: %v", p)
	}
	for _, v := range []int{1, 10, 12, 20, 21, 100, 105, 110, 999} {
		if got, ok := chineseValue(chineseNumber(v)); !ok || got != v {
			t.Errorf("%d: %s reads %d", v, chineseNumber(v), got)
		}
	}
	if d := NewDialect(c.Language("zh")); !d.TakesNumber("厨房 日 内") || d.TakesNumber("他在外面等着。") {
		t.Error("zh: TakesNumber")
	}
	if NewDialect(c.Language("en")).TakesNumber("INT. KITCHEN - DAY") {
		t.Error("en takes a number in its words")
	}
}
