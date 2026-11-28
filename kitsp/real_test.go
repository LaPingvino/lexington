package kitsp

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/internal/formattest"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// The projects in testdata/real-kit are Lexington's, saved by KIT
// Scenarist 0.7.2 itself (its README).

func realKIT(t *testing.T, name string) lex.Screenplay {
	t.Helper()
	data, err := os.ReadFile("testdata/real-kit/" + name + ".kitsp")
	if err != nil {
		t.Fatal(err)
	}
	return parse(t, data)
}

func has(s lex.Screenplay, typ lex.ElementType, contents string) bool {
	return slices.ContainsFunc(s, func(l lex.Line) bool { return l.Type == typ && l.Contents == contents })
}

// TestRealKITEnglish: KIT keeps the emphasis, dual dialogue (its
// splitter_start, splitter and splitter_end), a note, a transition and a
// centred line (as its title element).
func TestRealKITEnglish(t *testing.T) {
	s := realKIT(t, "lexington-check.en")
	for _, w := range []struct {
		typ      lex.ElementType
		contents string
	}{
		{lex.TypeAction, "A small kitchen. Sunlight. **Bold action**, *italic action*, _underlined action_ and " +
			"***bold italic***."},
		{lex.TypeDialog, "Did you see the *letter*?"},
		{lex.TypeDialog, "Which **letter**?"},
		{lex.TypeNote, "[[A note for the writer: this note should survive or be dropped cleanly.]]"},
		{lex.TypeCenter, "THE END OF THE BEGINNING"},
		{lex.TypeTrans, "CUT TO:"},
		{lex.TypeSpeaker, "MARIA (V.O.)"},
		{"Title", "Lexington Check"},
		{"Contact", "example@example.org"},
	} {
		if !has(s, w.typ, w.contents) {
			t.Errorf("no %s %q", w.typ, w.contents)
		}
	}
	var dual []string
	in := false
	for _, l := range s {
		switch l.Type {
		case lex.TypeDualOpen:
			in = true
		case lex.TypeDualClose:
			in = false
		}
		if in && l.Type != lex.TypeEmpty {
			dual = append(dual, string(l.Type)+": "+l.Contents)
		}
	}
	formattest.Same(t, "dual dialogue", []string{
		"dualspeaker_open: ", "speaker: MARIA", "dialog: The one on the table.", "dualspeaker_next: ",
		"speaker: JONAS", "dialog: Oh. That one.",
	}, dual)
}

// TestRealKITArabic: KIT keeps Hekaya's acts and sequences as folders,
// a synopsis, notes, lyrics, emphasis and Arabic-Indic scene numbers.
func TestRealKITArabic(t *testing.T) {
	s := realKIT(t, "layla.ar")
	for _, w := range []struct {
		typ      lex.ElementType
		contents string
	}{
		{lex.TypeSection, "# الفصل الأول"},
		{lex.TypeSection, "## تسلسل أ: الاكتشاف"},
		{lex.TypeSection, "# الفصل التاني"},
		{lex.TypeSynopsis, "سمير يكتشف إن الحفلة اتلغت ويحاول ينقذ الموقف."},
		{lex.TypeNote, "[[ملاحظة: محتاجين نحدد شكل الشقة بتفصيل أكتر في المسودة الجاية]]"},
		{lex.TypeLyrics, "يا ليلة الفرح والعيد"},
		{lex.TypeDialog, "نعمل **إحنا** الحفلة!"},
		{lex.TypeScene, "خارجي - شارع المعز - ليل #١#"},
		{lex.TypeScene, "خارجي - سطح عمارة قديمة - ليل #٢#"},
	} {
		if !has(s, w.typ, w.contents) {
			t.Errorf("no %s %q", w.typ, w.contents)
		}
	}
}

// TestRealKITDialects: what KIT saved of the Russian, Hebrew and Chinese
// scripts reads as their rows, as Lexington wrote them before 1.67.3.
func TestRealKITDialects(t *testing.T) {
	ru := realKIT(t, "proverka.ru")
	if !has(ru, lex.TypeScene, "НАТ. САД - ДЕНЬ") || !has(ru, lex.TypeAction, "СМЕНА КАДРА:") {
		t.Errorf("ru: %v", formattest.Rows(ru, nil))
	}
	he := realKIT(t, "bdika.he")
	if !has(he, lex.TypeSpeaker, "מריה") || !has(he, lex.TypeParen, "(בשקט)") {
		t.Errorf("he: %v", formattest.Rows(he, nil))
	}
	zh := realKIT(t, "jiancha.zh")
	if !has(zh, lex.TypeScene, "1. 厨房 晨 内") || !has(zh, lex.TypeSpeaker, "玛丽亚（画外音）") || !has(zh, lex.TypeDialog, "（轻声）") {
		t.Errorf("zh: %v", formattest.Rows(zh, nil))
	}
}

// TestDialectRoundTrip: a Russian, Hebrew, Chinese and Arabic script,
// through a KIT project and back to Fountain in its own language, reads
// as the same rows; it has no force mark its reading does not need, and
// every cue in a script without capitals keeps its @.
func TestDialectRoundTrip(t *testing.T) {
	conf := rules.DefaultConf()
	for name, lang := range map[string]string{
		"proverka.ru.fountain": "ru", "bdika.he.fountain": "he", "jiancha.zh.fountain": "zh", "layla.hekaya": rules.Hekaya,
	} {
		t.Run(lang, func(t *testing.T) {
			src, err := os.ReadFile("testdata/real-kit/" + name)
			if err != nil {
				t.Fatal(err)
			}
			l := conf.Language(lang)
			orig := fountain.ParseIn(l, bytes.NewReader(src))
			back := parse(t, write(t, orig))
			var b bytes.Buffer
			if err := (&fountain.FountainWriter{SceneConfig: conf.Scenes(lang), Language: &l}).Write(&b, back); err != nil {
				t.Fatal(err)
			}
			again := fountain.ParseIn(l, bytes.NewReader(b.Bytes()))
			formattest.Same(t, lang, kitRows(orig), kitRows(again))
			out := b.String()
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, ".") && !strings.Contains(string(src), "\n"+line+"\n") {
					t.Errorf("forced heading %q", line)
				}
			}
			if lang != "ru" {
				for _, l := range again {
					if l.Type == lex.TypeSpeaker && !strings.Contains(out, "@"+l.Contents) {
						t.Errorf("cue %q without its @", l.Contents)
					}
				}
			}
		})
	}
}
