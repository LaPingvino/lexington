package fountain

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// typeOf is the element the row holding want is read as, in lang.
func typeOf(t *testing.T, lang, text, want string) string {
	t.Helper()
	for _, l := range ParseIn(rules.DefaultConf().Language(lang), strings.NewReader(text)) {
		if strings.Contains(strings.ToUpper(l.Contents), strings.ToUpper(want)) {
			return l.Type
		}
	}
	t.Fatalf("%s: no row with %q in %q", lang, want, text)
	return ""
}

func TestCueRules(t *testing.T) {
	list := "\n\n/* Characters\n张三: 主角\n  age: 30\nبيتر\n*/\n"
	for _, c := range []struct {
		lang, text, row, want string
	}{
		// caps: English
		{"en", "Rain.\n\nANNA\nHello.\n", "ANNA", lex.TypeSpeaker},
		{"en", "Rain.\n\nAnna\nHello.\n", "Anna", lex.TypeAction},
		{"en", "Rain.\n\n@Anna\nHello.\n", "Anna", lex.TypeSpeaker},
		// list: the Characters block
		{"zh", "下雨。\n\n张三\n你好。" + list, "张三", lex.TypeSpeaker},
		{"zh", "下雨。\n\n李四\n你好。\n", "李四", lex.TypeAction},
		// parenthetical
		{"zh", "下雨。\n\n李四\n（笑）\n你好。\n", "李四", lex.TypeSpeaker},
		{"he", "גשם.\n\nדנה\n(בשקט)\nשלום.\n", "דנה", lex.TypeSpeaker},
		// extension
		{"zh", "下雨。\n\n王五（画外音）\n你好。\n", "王五", lex.TypeSpeaker},
		{"zh", "下雨。\n\n王五（大笑）\n你好。\n", "王五", lex.TypeAction},
		{"ja", "雨。\n\n太郎（ＯＦＦ）\nこんにちは。\n", "太郎", lex.TypeSpeaker},
		// repeat: twice shaped like a name
		{"zh", "下雨。\n\n赵六\n你好。\n\n赵六\n再见。\n", "赵六", lex.TypeSpeaker},
		// shape alone is not one of zh's rules, and a sentence is no name
		{"zh", "下雨。\n\n赵六走了。\n他哭。\n\n赵六走了。\n他笑。\n", "赵六走了", lex.TypeAction},
		// Hekaya's registry: a later bare name, not an earlier one
		{"ar", "مطر.\n\nسمير\nأهلا.\n\n@سمير\nقهوة.\n", "أهلا", lex.TypeAction},
		{"ar", "مطر.\n\n@سَمير\nقهوة.\n\nسمير\nكلام.\n", "كلام", lex.TypeDialog},
		{"ar", "مطر.\n\nبيتر\nكلام." + list, "كلام", lex.TypeDialog},
		// a registered name followed by a blank line is action (Fountain)
		{"ar", "@سمير\nقهوة.\n\nسمير\n\nكلام.\n", "كلام", lex.TypeAction},
		// English capitals in Hekaya
		{"ar", "مطر.\n\nJOHN\nHello.\n", "Hello", lex.TypeDialog},
	} {
		if got := typeOf(t, c.lang, c.text, c.row); got != c.want {
			t.Errorf("%s %q: %s, want %s", c.lang, c.row, got, c.want)
		}
	}
}

func TestTransitionRules(t *testing.T) {
	for _, c := range []struct {
		lang, line, want string
	}{
		{"en", "CUT TO:", lex.TypeTrans},
		{"en", "Cut to:", lex.TypeAction},
		{"en", "FADE OUT.", lex.TypeAction}, // Fountain: forced only
		{"ar", "- قطع -", lex.TypeTrans},
		{"ar", "قطع إلى:", lex.TypeTrans},
		{"ar", "- اختفاء تدريجي -", lex.TypeTrans},
		{"ar", "مزج إلى", lex.TypeTrans},
		{"ar", "قطع الخبز", lex.TypeAction},
		{"he", "דיזולב ל:", lex.TypeTrans},
		{"de", "SCHNITT AUF:", lex.TypeTrans},
		{"it", "STACCO SU:", lex.TypeTrans},
		{"zh", "淡出", lex.TypeTrans},
	} {
		if got := typeOf(t, c.lang, "Rain.\n\n"+c.line+"\n\nMore.\n", c.line); got != c.want {
			t.Errorf("%s %q: %s, want %s", c.lang, c.line, got, c.want)
		}
	}
	// blank lines around it, as Fountain has them
	if got := typeOf(t, "ar", "مطر.\n- قطع -\n\nأكثر.\n", "قطع"); got != lex.TypeAction {
		t.Errorf("a keyword without a blank line before: %s", got)
	}
}

func TestHeadingPatterns(t *testing.T) {
	for _, c := range []struct {
		lang, line string
		heading    bool
	}{
		{"zh", "场景1 小区楼下 日 外", true},
		{"zh", "13. 街道  黄昏  外  夏", true},
		{"zh", "1.日  外  清晨的田野间", true},
		{"zh", "1. 內景 阿強家客廳 夜", true},
		{"zh", "2. 大华宾馆房间   夜   内", true},
		{"zh", "内景 咖啡馆 夜", true},
		{"zh", "场景很美 他走了", false},
		{"de", "18. GERICHTSSAAL INNEN/TAG", true},
		{"de", "HOTEL DAVOR AUSSEN/NACHT", true},
		{"de", "Sie geht nach draußen.", false},
		{"fr", "5 - CHU GRENOBLE – INT/JOUR", true},
		{"fr", "2- BOIS (PRÈS DU CHALET) – EXT/JOUR", true},
		{"fr", "SÉQUENCE 1 – INT. JOUR", true},
		{"ar", "مشهد 12 خارج شقة إنجي/البسطة ن/د", true},
		{"ar", "مشهد (1) غرفة الضيوف ليل / داخلي", true},
		{"ar", "داخلي - قهوة بلدي - نهار", true},
		{"zh", "场景1 小区楼下 日 外 #1#", true},
		{"ar", "مشهد جميل في الليل", false},
		{"ru", "1. НАТ. ГОРОДСКАЯ ПЛОЩАДЬ. НОЧЬ", true},
		{"he", "1. פנים. משרד דפנה – יום.", true},
		{"it", "1. INT - CASA DI MATILDE - CUCINA - GIORNO", true},
		{"en", "1. INT. HOUSE - DAY", false}, // Fountain numbers with #1#
	} {
		got := typeOf(t, c.lang, "x\n\n"+c.line+"\n\ny\n", strings.ToUpper(strings.Fields(c.line)[0]))
		if (got == lex.TypeScene) != c.heading {
			t.Errorf("%s %q: %s, heading %v", c.lang, c.line, got, c.heading)
		}
	}
}

func TestTitleKeys(t *testing.T) {
	s := ParseIn(rules.DefaultConf().Language("ar"), strings.NewReader("العنوان: آخر أيام الصيف\nسيناريو: سمير\nDirection: rtl\n\nداخلي - قهوة - نهار\n"))
	got := map[string]string{}
	for _, l := range s {
		got[l.Type] = l.Contents
	}
	if got["Title"] != "آخر أيام الصيف" || got["Author"] != "سمير" || got["Direction"] != "rtl" {
		t.Errorf("title page: %v", got)
	}
}

func TestHekayaFixed(t *testing.T) {
	c := rules.DefaultConf()
	c.Languages["ar"] = rules.Language{Scenes: []string{"XX"}, Cues: []string{"caps"}}
	if got := c.Language("ar"); len(got.Scenes) != 1 || got.Has(rules.CueList) {
		t.Fatalf("ar as the writer set it: %+v", got)
	}
	h := c.Language(rules.Hekaya)
	if !h.RTL() || !h.Has(rules.CueList) || h.Scenes[0] != "داخلي" {
		t.Errorf("hekaya follows the writer's ar: %+v", h)
	}
}

func TestListedNames(t *testing.T) {
	text := "INT. HOUSE - DAY\n\n/* Characters\nANNA: the lead\n  age: 34\nben\n小明\n*/\n\n/* Notes\nx\n*/\n"
	// the Characters block before the notes is not at the end: only the
	// last block is read by At; EndBlocks finds the others
	if got := strings.Join(ListedNames("x\n\n/* Characters\nANNA: the lead\n  age: 34\nben\n小明\n*/\n"), ","); got != "ANNA,ben,小明" {
		t.Errorf("listed: %q", got)
	}
	if got := strings.Join(ListedNames(text), ","); got != "ANNA,ben,小明" {
		t.Errorf("listed before the notes: %q", got)
	}
}

// TestHekayaOpenSpec: where Hekaya's spec v0.1 is open, what Lexington
// reads (docs/LANGUAGES.md, "Hekaya, where its spec is open").
func TestHekayaOpenSpec(t *testing.T) {
	h := rules.DefaultConf().Language(rules.Hekaya)
	read := func(text, row string) string {
		t.Helper()
		for _, l := range ParseIn(h, strings.NewReader(text)) {
			if strings.Contains(strings.ToUpper(string(l.Contents)), strings.ToUpper(row)) {
				return l.Type
			}
		}
		t.Fatalf("no %q in %q", row, text)
		return ""
	}
	for _, c := range []struct{ name, text, row, want string }{
		{"1 مقدر is no scene word", "x\n\nمقدر له أن يعود.\n", "مقدر", lex.TypeAction},
		{"2 مزج إلى is a transition", "x\n\nمزج إلى\n\ny\n", "مزج", lex.TypeTrans},
		{"3 خارج الكادر is an extension", "x\n\n@نادية (خارج الكادر)\nأنا هنا.\n", "نادية", lex.TypeSpeaker},
		{"4 عودة للمشهد is a transition", "x\n\n- عودة للمشهد -\n\ny\n", "عودة", lex.TypeTrans},
		{"5 a registered name needs its speech right under it", "@سمير\nأهلا.\n\nسمير\n\nكلام.\n", "كلام", lex.TypeAction},
		{"5 the registry: an earlier bare name is action", "x\n\nسمير\nكلام.\n\n@سمير\nأهلا.\n", "كلام", lex.TypeAction},
		{"6 a heading needs a blank line before it", "قهوة.\nداخلي - شارع - ليل\n", "شارع", lex.TypeAction},
		{"7 a keyword transition needs blank lines around it", "x\n\n- قطع -\nسمير يمشي.\n", "قطع", lex.TypeAction},
		{"8 any title page key", "Producer: نادية\nالعنوان: فيلم\n\nداخلي - شارع - ليل\n", "نادية", "Producer"},
		{"9 == text is a synopsis", "x\n\n== الفصل الثاني\n\ny\n", "الفصل", "synopse"},
		{"11 Arabic digits number a scene", "x\n\nداخلي - قهوة بلدي - نهار #١٢#\n\ny\n", "قهوة", lex.TypeScene},
		{"12 capitals beyond ASCII are a cue", "x\n\nJOSÉ\nHola.\n", "Hola", lex.TypeDialog},
	} {
		if got := read(c.text, c.row); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	for _, l := range ParseIn(h, strings.NewReader("x\n\nداخلي - قهوة بلدي - نهار #١٢#\n")) {
		if _, n := lex.SceneNumber(string(l.Contents)); l.Type == lex.TypeScene && n != "١٢" {
			t.Errorf("number %q", n)
		}
	}
}

// ForceIn and UnforceIn with the language rules: Hekaya's samples, a
// Chinese script and the registry read the same in standard Fountain,
// and come back as written.
func TestForceWithRules(t *testing.T) {
	conf := rules.DefaultConf()
	std := conf.Language("en")
	texts := map[string]string{
		"registry": "مطر.\n\n@سمير\nأهلا.\n\nسمير\nكلام.\n\n- قطع -\n\nخارجي - شارع - ليل\n",
		"zh":       "场景1 咖啡馆 日 内\n\n张三坐在窗边。\n\n李四\n（笑）\n你来了。\n\n王五（画外音）\n我在。\n\n李四\n真的？\n\n淡出\n\n内景 家 夜\n",
	}
	langs := map[string]string{"registry": rules.Hekaya, "zh": "zh"}
	files, _ := filepath.Glob("../testdata/hekaya/*.hekaya")
	for _, f := range files {
		b, _ := os.ReadFile(f)
		texts[f], langs[f] = string(b), rules.Hekaya
	}
	types := func(text string, l rules.Language) string {
		var out []string
		body := !strings.Contains(text, ":") // the title page's keys are named by language: left out
		for _, r := range ParseIn(l, strings.NewReader(text)) {
			if body {
				out = append(out, r.Type)
			}
			body = body || r.Type == lex.TypeNewPage
		}
		return strings.Join(out, " ")
	}
	for name, text := range texts {
		l := conf.Language(langs[name])
		forced := ForceIn(text, l, std)
		if a, b := types(text, l), types(forced, std); a != b {
			t.Errorf("%s: standard Fountain reads otherwise:\n%s\n%s\n%s", name, a, b, forced)
		}
		back := UnforceIn(forced, l, std)
		if name == "registry" {
			// a later bare name forced with @ cannot be told from one the
			// writer forced: it stays, and reads the same
			if types(back, l) != types(text, l) || !strings.Contains(back, "\n@سمير\nكلام") {
				t.Errorf("registry: %s", back)
			}
			continue
		}
		if back != text {
			t.Errorf("%s: not back as written:\n%s", name, back)
		}
	}
}

// The cache's cast after each edit is the one read anew.
func TestCastCache(t *testing.T) {
	d := NewDialect(rules.DefaultConf().Language("zh"))
	c := d.NewCastCache()
	text := "场景1 咖啡馆 日 内\n\n李四\n你好。\n\n王五\n再见。\n\n李四\n真的？\n\n/* Characters\n张三: 主角\n*/\n"
	edits := []func(string) string{
		func(s string) string { return s },
		func(s string) string { return strings.Replace(s, "王五\n再见。", "李四\n再见。", 1) },
		func(s string) string { return strings.Replace(s, "\n\n李四\n真的？", "", 1) },
		func(s string) string { return strings.Replace(s, "张三: 主角", "张三: 主角\n赵六", 1) },
		func(s string) string { return "x\n\n" + s },
		func(s string) string {
			return strings.Replace(s, "\n\n/* Characters", "\n\n赵六\n嗯。\n\n/* Characters", 1)
		},
	}
	for i, e := range edits {
		text = e(text)
		lines := strings.Split(text, "\n")
		got, want := c.Cast(lines), d.CastOfLines(lines)
		if !maps.Equal(got.shaped, want.shaped) || !maps.Equal(got.listed, want.listed) {
			t.Errorf("edit %d: cached %v %v, anew %v %v", i, got.shaped, got.listed, want.shaped, want.listed)
		}
	}
}
