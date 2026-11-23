package fountain

import (
	"testing"

	"github.com/LaPingvino/lexington/lex"
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
		{[]string{"hi-Latn", "hi"}, latin, "MUMBAI APARTMENT", lex.TypeScene},
		{[]string{"hi-Latn", "hi"}, latin, "khidki", lex.TypeAction},
		{[]string{"hi-Latn", "hi"}, latin, "ARJUN (V.O.)", lex.TypeSpeaker},
		{[]string{"hi-Latn", "hi"}, latin, "marzi", lex.TypeDialog},
		{[]string{"hi-Latn", "hi"}, latin, "CUT TO:", lex.TypeTrans},
		{[]string{"hi-Latn", "hi"}, latin, "DHARAVI", lex.TypeScene},
		{[]string{"hi-Latn", "hi"}, latin, "awaaz", lex.TypeAction},
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
