package fountain

import (
	"slices"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// TestHindi: Hindi in Latin letters, in Devanagari, and both, read as
// real scripts write them (docs/LANGUAGES.md, "Hindi").
func TestHindi(t *testing.T) {
	latin := "INT. MUMBAI APARTMENT - LIVING ROOM - NIGHT\n\nArjun khidki ke paas khada hai.\n\nARJUN (V.O.)\nTumhein lagta hai main yahaan apni marzi se aaya hoon?\n\nCUT TO:\n\nEXT. DHARAVI LANES - DAY\n\nAndar se awaaz aati hai.\n"
	deva := "शिक्षा अधिकारी का दफ़्तर (बाहरी कक्ष)/दिन/अंदर\n\nरजनी कमरे में आती है।\n\nरजनी\n(चपरासी से)\nसाहब हैं?\n\nकट टू\n\n3. घर का आँगन / रात / बाहर\n\nबाहर बारिश हो रही है।\n\nअंदर. रसोई - सुबह\n\nअंदर से आवाज़ आती है।\n\nINT. OFFICE - DAY\n\nANNA\nनमस्ते।\n\n(डिजॉल्व टू)\n\nEXT. ROAD - NIGHT\n\nफ़ेड आउट\n"
	for _, c := range []struct {
		langs     []string
		text, row string
		want      string
	}{
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "MUMBAI APARTMENT", lex.TypeScene},
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "khidki", lex.TypeAction},
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "ARJUN (V.O.)", lex.TypeSpeaker},
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "marzi", lex.TypeDialog},
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "CUT TO:", lex.TypeTrans},
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "DHARAVI", lex.TypeScene},
		{[]string{"hi-Latn", "hi-Deva", "hi"}, latin, "awaaz", lex.TypeAction},
		{[]string{"hi-Deva", "hi"}, deva, "शिक्षा अधिकारी", lex.TypeScene},
		{[]string{"hi-Deva", "hi"}, deva, "कमरे में", lex.TypeAction},
		{[]string{"hi-Deva", "hi"}, deva, "चपरासी", lex.TypeParen},
		{[]string{"hi-Deva", "hi"}, deva, "साहब", lex.TypeDialog},
		{[]string{"hi-Deva", "hi"}, deva, "कट टू", lex.TypeTrans},
		{[]string{"hi-Deva", "hi"}, deva, "आँगन", lex.TypeScene},
		{[]string{"hi-Deva", "hi"}, deva, "बारिश", lex.TypeAction},
		{[]string{"hi-Deva", "hi"}, deva, "रसोई", lex.TypeScene},
		{[]string{"hi-Deva", "hi"}, deva, "आवाज़", lex.TypeAction},
		{[]string{"hi-Deva", "hi"}, deva, "OFFICE", lex.TypeScene},
		{[]string{"hi-Deva", "hi"}, deva, "ANNA", lex.TypeSpeaker},
		{[]string{"hi-Deva", "hi"}, deva, "नमस्ते", lex.TypeDialog},
		{[]string{"hi-Deva", "hi"}, deva, "डिजॉल्व", lex.TypeTrans},
		{[]string{"hi-Deva", "hi"}, deva, "ROAD", lex.TypeScene},
		{[]string{"hi-Deva", "hi"}, deva, "फ़ेड आउट", lex.TypeTrans},
	} {
		for _, lang := range c.langs {
			if got := typeOf(t, lang, c.text, c.row); got != c.want {
				t.Errorf("%s %q: %s, want %s", lang, c.row, got, c.want)
			}
		}
	}
}

// TestHindiWords: Devanagari's extensions make a cue, beside English's,
// and each way of writing has its title page keys (docs/LANGUAGES.md).
func TestHindiWords(t *testing.T) {
	deva := "शीर्षक: बंदर और मगरमच्छ\nश्रेय: लेखन\nपटकथा: Accolade\nमसौदे की तिथि: 23.11.2026\n\nनदी किनारा/दिन/बाहर\n\nबंदर पेड़ पर बैठा है।\n\nकथावाचक (वॉयस ओवर)\nबहुत पुरानी बात है।\n\nमगरमच्छ (ऑफ़ स्क्रीन)\nदोस्त!\n\nबंदर (जारी)\nकौन है?\n\nANNA (O.S.)\nनमस्ते।\n"
	for _, lang := range []string{"hi-Deva", "hi"} {
		for row, want := range map[string]string{"कथावाचक": lex.TypeSpeaker, "मगरमच्छ (": lex.TypeSpeaker,
			"बंदर (जारी)": lex.TypeSpeaker, "ANNA": lex.TypeSpeaker, "दोस्त": lex.TypeDialog} {
			if got := typeOf(t, lang, deva, row); got != want {
				t.Errorf("%s %q: %s, want %s", lang, row, got, want)
			}
		}
	}
	latin := "Sheershak: Bandar aur Magarmachh\nShrey: likhit\nPataktha: Accolade\nMasaude ki tithi: 23.11.2026\n\nEXT. NADI KINARA - DIN\n\nBandar ped par baitha hai.\n"
	for _, c := range []struct{ lang, text string }{{"hi-Deva", deva}, {"hi-Latn", latin}, {"hi", deva}, {"hi", latin}} {
		got := map[string]bool{}
		for _, r := range ParseIn(rules.DefaultConf().Language(c.lang), strings.NewReader(c.text)) {
			got[r.Type] = true
		}
		for _, k := range []string{"Title", "Credit", "Author", "Draft date"} {
			if !got[k] {
				t.Errorf("%s: no %s in %v", c.lang, k, got)
			}
		}
	}
}

// TestHindiWrites: each Hindi writes its own words first and reads
// English's too (Joop, 2026-11-23): hi English Fountain's, hi-Latn its
// Hindi in Latin letters, hi-Deva its Devanagari.
func TestHindiWrites(t *testing.T) {
	c := rules.DefaultConf()
	hi, latn, deva := c.Language("hi"), c.Language("hi-Latn"), c.Language("hi-Deva")
	if hi.HeadingForm() != rules.SceneForm || hi.Matchup[0] != "INT." || hi.Times[0] != "DAY" || hi.Extensions[0] != "V.O." ||
		hi.Transitions[len(hi.Transitions)-1] != "FADE OUT." || hi.TitleKeys["Author"] != "Author" || hi.TitleKeys["Title"] != "Title" {
		t.Errorf("hi does not write English first: %q %q %q %q %q", hi.HeadingForm(), hi.Times[:2], hi.Extensions[:2], hi.Transitions, hi.TitleKeys)
	}
	if latn.Times[0] != "DIN" || !slices.Contains(latn.Times, "DAY") || latn.TitleKeys["Sheershak"] != "Title" {
		t.Errorf("hi-Latn: %q %q", latn.Times, latn.TitleKeys)
	}
	if deva.Times[0] != "दिन" || !slices.Contains(deva.Times, "NIGHT") || deva.Extensions[0] != "वॉयस ओवर" || !slices.Contains(deva.Extensions, "CONT'D") {
		t.Errorf("hi-Deva: %q %q", deva.Times, deva.Extensions)
	}
	english := "Title: The Well\nAuthor: Accolade\nDraft date: 1\n\nEXT. VILLAGE WELL - NIGHT\n\nRaat ho chuki hai.\n\nMEERA\n(dheere se)\nKaun hai?\n\nRAVI (O.S.)\nMain hoon.\n\nMEERA (CONT'D)\nAndar aao.\n\nCUT TO:\n\nINT. MEERA'S HOUSE - DAY\n\n> FADE OUT.\n"
	for _, lang := range []string{"hi", "hi-Latn", "hi-Deva"} {
		for row, want := range map[string]string{"VILLAGE WELL": lex.TypeScene, "MEERA": lex.TypeSpeaker, "RAVI (O.S.)": lex.TypeSpeaker,
			"MEERA (CONT'D)": lex.TypeSpeaker, "Kaun": lex.TypeDialog, "CUT TO:": lex.TypeTrans, "HOUSE": lex.TypeScene,
			"FADE OUT": lex.TypeTrans, "The Well": "Title", "Raat ho": lex.TypeAction} {
			if got := typeOf(t, lang, english, row); got != want {
				t.Errorf("%s %q: %s, want %s", lang, row, got, want)
			}
		}
	}
	if got := typeOf(t, "hi", "INT. ROOM - DAY\n\nFADE OUT.\n", "FADE OUT"); got != lex.TypeTrans {
		t.Errorf("hi: FADE OUT. read as %s", got)
	}
}
