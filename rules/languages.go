package rules

import (
	"maps"
	"slices"
)

// The default configuration's languages, each in full. Where each word
// comes from is in docs/LANGUAGES.md; a word no source gave is marked
// there as Lexington's own choice.

// english is Fountain's own language (fountain.io/syntax).
func english() Language {
	return Language{
		Scenes:         []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
		Matchup:        []string{"INT.", "EXT.", "INT./EXT.", "EXT./INT.", "EST."},
		Headings:       []string{},
		Settings:       []string{},
		Times:          []string{},
		Direction:      "ltr",
		Cues:           []string{CueForced, CueCaps},
		Extensions:     []string{"V.O.", "O.S.", "O.C.", "CONT'D", "CONT’D"},
		Shape:          Shape{Words: 4, Ends: ".!?"},
		Repeat:         2,
		TransitionEnds: []string{" TO:"},
		Transitions:    []string{},
		TitleKeys:      map[string]string{},
	}
}

// custom is what a language the default configuration does not have
// starts from: English's rules, without scene words of its own.
func custom() Language {
	l := english()
	l.Scenes, l.Matchup = nil, nil
	return l
}

// latin is a language written in capitals like English, with its own
// scene words and matchup.
func latin(scenes, matchup []string, extensions, transitions []string, ends ...string) Language {
	l := english()
	l.Scenes, l.Matchup = scenes, matchup
	if extensions != nil {
		l.Extensions = extensions
	}
	if transitions != nil {
		l.Transitions = transitions
	}
	if ends != nil {
		l.TransitionEnds = ends
	}
	return l
}

// noCaps are the cue rules of a language written without capitals
// (Joop, LOOP.md 2026-11-20): a name in the list, before a
// parenthetical, with an extension, or one that comes back; and caps,
// which never reads a name without capitals but keeps a Latin cue in
// capitals (ANNA in a Hebrew script) reading as before.
var noCaps = []string{CueForced, CueCaps, CueList, CueParenthetical, CueExtension, CueRepeat}

// hekaya is Hekaya's Arabic in full: its keywords.ts and spec v0.1
// (michaelkmalak/hekaya, MIT), with English Fountain beside it as
// Hekaya reads it too.
func hekaya() Language {
	return Language{
		Scenes: []string{"داخلي", "خارجي", "لقطة تأسيسية", "داخلي/خارجي", "خارجي/داخلي", "د/خ",
			"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
		Matchup:   []string{"داخلي", "خارجي", "داخلي/خارجي", "خارجي/داخلي", "لقطة تأسيسية"},
		Headings:  []string{},
		Settings:  []string{},
		Times:     []string{"نهار", "ليل", "صباح", "مساء", "غروب", "فجر", "ظهر", "عصر", "منتصف الليل", "قبل الفجر", "بعد الغروب"},
		Direction: "rtl",
		Cues:      []string{CueForced, CueCaps, CueList},
		Extensions: []string{"صوت من خارج المشهد", "ص.خ", "خارج الشاشة", "خ.ش", "خارج الكادر", "تابع",
			"V.O.", "O.S.", "O.C.", "CONT'D"},
		Shape:          Shape{Words: 3, Ends: ".!?؟،؛:"},
		Repeat:         2,
		TransitionEnds: []string{" TO:"},
		Transitions: []string{
			"قطع", "قطع إلى", "قطع مباشر", "قطع مفاجئ", "قطع متطابق", "قطع قافز", "قطع خاطف",
			"تلاشي", "اختفاء تدريجي", "ظهور تدريجي", "تلاشي إلى أسود", "تلاشي إلى أبيض", "تلاشي إلى",
			"مزج", "مزج إلى", "مزج متطابق", "ذوبان",
			"مسح", "مسح إلى", "وميض", "وميض إلى",
			"قزحية دخول", "قزحية خروج",
			"تداخل مع", "تجميد الكادر", "شاشة منقسمة", "تحول تدريجي", "انتقال",
			"عودة للمشهد", "عودة إلى",
		},
		TransitionMarks: "-:",
		TitleKeys: map[string]string{
			"العنوان": "Title", "سيناريو": "Author", "كتابة": "Author", "تأليف": "Author",
			"المصدر": "Source", "مسودة": "Draft", "تاريخ المسودة": "Draft date", "تاريخ": "Date",
			"التاريخ": "Date", "تواصل": "Contact", "بيانات الاتصال": "Contact", "حقوق النشر": "Copyright",
			"ملاحظات": "Notes",
		},
	}
}

// with is l with its headings' patterns and words, and its cue and
// transition words where given (nil keeps English's).
func (l Language) with(headings, settings, times, extensions, transitions, ends []string) Language {
	l.Headings, l.Settings, l.Times = headings, settings, times
	if extensions != nil {
		l.Extensions = extensions
	}
	if transitions != nil {
		l.Transitions = transitions
	}
	if ends != nil {
		l.TransitionEnds = ends
	}
	return l
}

// Hindi (docs/LANGUAGES.md, "Hindi"): scripts in Latin letters keep
// English Fountain's words (INT. MUMBAI APARTMENT - NIGHT, CUT TO:, V.O.)
// with the dialogue in Hindi; Devanagari scripts write the heading place
// first, inside/outside last ("शिक्षा अधिकारी का दफ़्तर (बाहरी कक्ष)/दिन/अंदर",
// NCERT's अभिव्यक्ति और माध्यम), "कट टू", "(डिजॉल्व टू)"; many keep INT./EXT.
// in English beside Devanagari dialogue. hi is both at once.

// hindiTimes are the time words in Latin letters: English's, and the
// Hindi words written so (Lexington's own spelling).
var hindiTimes = []string{"DAY", "NIGHT", "MORNING", "EVENING", "DUSK", "DAWN", "DIN", "RAAT", "SUBAH", "SHAAM"}

// hindiLatin is Hindi in Latin letters: English Fountain, the Hindi time
// words beside English's.
func hindiLatin() Language {
	l := english()
	l.Times = slices.Clone(hindiTimes)
	return l
}

// devanagari are Devanagari Hindi's own words: scene words (with their
// dot, so a line starting "अंदर से…" stays action), the heading patterns,
// inside/outside and time words, transitions, title keys.
var devanagari = struct {
	scenes, matchup, headings, settings, times, transitions []string
	ends                                                    []string
	titleKeys                                               map[string]string
}{
	scenes:   []string{"अंदर.", "बाहर.", "अंदर/बाहर.", "बाहर/अंदर."},
	matchup:  []string{"अंदर.", "बाहर.", "अंदर/बाहर.", "बाहर/अंदर.", ""},
	headings: []string{"{place}/{time}/{io}", "{place}/{time}/{io} {any}", "{n}. {place}/{time}/{io}", "{n}. {place}/{time}/{io} {any}"},
	settings: []string{"अंदर", "बाहर", "अंदर/बाहर", "बाहर/अंदर", "INT", "EXT", "INT.", "EXT."},
	times:    []string{"दिन", "रात", "सुबह", "शाम", "दोपहर"},
	// ending on a fade out: New's template closes with the last
	transitions: []string{"कट टू", "डिजॉल्व टू", "डिज़ॉल्व टू", "फ़ेड इन", "फेड इन", "फेड आउट", "फ़ेड आउट"},
	ends:        []string{" TO:", " टू", " टू:"},
	titleKeys: map[string]string{"शीर्षक": "Title", "लेखक": "Author", "पटकथा": "Author", "स्रोत": "Source",
		"तिथि": "Date", "संपर्क": "Contact"},
}

// hindiDevanagari is Hindi in Devanagari: its own words, English
// Fountain's scene words, extensions and " TO:" accepted beside them;
// cues without capitals (a Latin name in capitals still is one).
func hindiDevanagari() Language {
	d := devanagari
	return Language{
		Scenes:          slices.Concat(d.scenes, english().Scenes),
		Matchup:         slices.Clone(d.matchup),
		Headings:        slices.Clone(d.headings),
		Settings:        slices.Clone(d.settings),
		Times:           slices.Clone(d.times),
		Direction:       "ltr",
		Cues:            slices.Clone(noCaps),
		Extensions:      english().Extensions,
		Shape:           Shape{Words: 3, Ends: ".!?,;:।॥"},
		Repeat:          2,
		TransitionEnds:  slices.Clone(d.ends),
		Transitions:     slices.Clone(d.transitions),
		TransitionMarks: "():",
		TitleKeys:       maps.Clone(d.titleKeys),
	}
}

// hindi is Hindi in either script or both mixed: Devanagari's rules with
// the Latin time words too, and English's scene words first (INT.: what
// most Hindi scripts write).
func hindi() Language {
	l := hindiDevanagari()
	l.Scenes = slices.Concat(english().Scenes, devanagari.scenes)
	l.Matchup = slices.Clone(english().Matchup)
	l.Times = slices.Concat(hindiTimes, devanagari.times)
	return l
}

// none is an empty list: a field that has nothing, said so.
var none = []string{}

// defaultLanguages are the default configuration's languages. The
// sources of their words are in docs/LANGUAGES.md.
func defaultLanguages() map[string]Language {
	ar := hekaya()
	// real Arabic scripts also number the scene and end on time/setting:
	// "مشهد 12  خارج شقة إنجي/البسطة  ن/د", "مشهد (1)  غرفة الضيوف  ليل / داخلي"
	ar.Headings = []string{"مشهد {n} {place} {time}/{io}", "مشهد ({n}) {place} {time}/{io}"}
	ar.Settings = []string{"داخلي", "خارجي", "د", "خ"}
	ar.Times = []string{"نهار", "ليل", "ن", "ل", "صباح", "مساء", "غروب", "فجر", "ظهر", "عصر", "منتصف الليل", "قبل الفجر", "بعد الغروب"}
	ls := map[string]Language{
		"en": english(),
		// "1. INT - CASA DI MATILDE - CUCINA - GIORNO" (cinemaperlascuola), STACCO SU (Solinas)
		"it": latin([]string{"INT", "EST", "INT./EST", "INT/EST", "EST/INT", "EST./INT", "I/E"},
			[]string{"INT.", "EST.", "INT./EST.", "EST./INT.", ""}, nil, nil).with( // EST. is esterno
			[]string{"{n}. {io} {any}"}, none, []string{"GIORNO", "NOTTE"},
			[]string{"VOCE FUORI CAMPO", "CONTINUA", "SEGUE", "V.O.", "O.S.", "CONT'D"}, nil, []string{" TO:", " SU:", " SU"}),
		"nl": latin([]string{"BIN", "BUI", "BI", "BU", "OPEN", "BIN./BUI", "BUI./BIN", "BIN/BUI", "BI/BU"},
			[]string{"BIN.", "BUI.", "BIN./BUI.", "BUI./BIN.", ""}, nil, nil).with(
			none, none, []string{"DAG", "NACHT", "SCHEMER"},
			[]string{"OFF", "O.S.", "V.O.", "VERVOLGT", "VERVOLG", "CONT'D"}, nil, nil),
		// "18. GERICHTSSAAL INNEN/TAG" (MURER), SCHNITT AUF: (de.wikibooks)
		"de": latin([]string{"INT", "EXT", "ETABL", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E", "INNEN", "AUSSEN"},
			[]string{"INT.", "EXT.", "INT./EXT.", "EXT./INT.", "ETABL."}, nil, nil).with(
			[]string{"{n}. {place} {io}/{time}", "{place} {io}/{time}"}, []string{"INNEN", "AUSSEN", "AUßEN", "INNEN/AUSSEN"},
			[]string{"TAG", "NACHT", "ABEND", "MORGEN"},
			[]string{"OFF", "ON", "FORTSETZUNG", "V.O.", "O.S.", "CONT'D"}, []string{"SCHNITT", "SCHNITT:"}, []string{" TO:", " AUF:"}),
		// "5 - CHU GRENOBLE – INT/JOUR" (Anatomie d'une chute), "SÉQUENCE 1 – INT. JOUR" (Quinzaine)
		"fr": latin([]string{"INT", "EXT", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"},
			[]string{"INT.", "EXT.", "INT./EXT.", "EXT./INT.", ""}, nil, nil).with(
			[]string{"{n} - {place} - {io}/{time}", "SÉQUENCE {n} - {io}. {time}"}, []string{"INT", "EXT", "EXT+INT", "INT+EXT"},
			[]string{"JOUR", "NUIT", "SOIR", "MATIN", "FIN DE JOURNÉE"},
			[]string{"OFF", "V.O.", "O.S.", "SUITE", "CONT'D"}, []string{"FONDU AU NOIR", "FONDU AU NOIR."}, nil),
		"eo": latin([]string{"EN.", "ENE", "EKST", "EK", "EN/EKST", "EKST/EN", "EKST./EN", "EN./EKST"},
			[]string{"EN.", "EKST.", "EN./EKST.", "EKST./EN.", ""}, nil, nil).with(none, none, none, nil, nil, nil),
		// "1. НАТ. ГОРОДСКАЯ ПЛОЩАДЬ. НОЧЬ" (kinodramaturg)
		"ru": latin([]string{"ИНТ", "НАТ", "ИНТ/НАТ", "ИНТ./НАТ", "НАТ/ИНТ", "НАТ./ИНТ", "ЭКСТ", "И/Н", "Н/И"},
			[]string{"ИНТ.", "НАТ.", "ИНТ./НАТ.", "НАТ./ИНТ.", ""}, nil, nil).with(
			[]string{"{n}. {io} {any}"}, none, []string{"ДЕНЬ", "НОЧЬ"},
			[]string{"ЗА КАДРОМ", "ПРОДОЛЖАЕТ", "V.O.", "O.S.", "CONT'D"}, nil, nil),
		"tok": latin([]string{"INSA", "KON", "INSA/KON", "I./K", "I/K", "KON/INSA", "K./I", "K/I"},
			[]string{"INSA.", "KON.", "INSA/KON.", "KON/INSA.", ""}, nil, nil).with(none, none, none, nil, nil, nil),
		"ar":      ar,
		"hi-Latn": hindiLatin(),
		"hi-Deva": hindiDevanagari(),
		"hi":      hindi(),
		// "1. פנים. משרד דפנה – יום." (dlg.co.il), דיזולב ל: / קאט ל (akolnoa)
		"he": {
			Scenes:          []string{"פנים", "חוץ", "פנים/חוץ", "חוץ/פנים"},
			Headings:        []string{"{n}. {io} {any}"},
			Settings:        none,
			Times:           []string{"יום", "לילה"},
			Matchup:         []string{"פנים", "חוץ", "פנים/חוץ", "חוץ/פנים", ""},
			Direction:       "rtl",
			Cues:            noCaps,
			Extensions:      []string{"V.O.", "O.S.", "CONT'D", "MORE"},
			Shape:           Shape{Words: 3, Ends: ".!?,;:"},
			Repeat:          2,
			TransitionEnds:  []string{" TO:", " ל:"},
			Transitions:     []string{"קאט ל", "קאט", "פייד אין", "פייד אאוט"},
			TransitionMarks: ":",
			TitleKeys:       map[string]string{},
		},
		// "场景1  小区楼下  日  外" (XISU thesis), "13. 街道  黄昏  外  夏" (孔雀),
		// "1.日  外  清晨的田野间" (那山那人那狗), "1. 內景 阿強家客廳 夜" (Taiwan award
		// template), "場：1 景：捷運淡水站大廳" (Taiwan contest template)
		"zh": {
			Scenes:         []string{"内景", "外景", "内景/外景", "外景/内景", "內景", "內景/外景", "外景/內景"},
			Headings:       []string{"场景{n} {place} {time} {io}", "{n}. {place} {time} {io}", "{n}. {place} {time} {io} {any}", "{n}.{time} {io} {place}", "{n}. {io} {place} {time}", "場：{n} {any}", "场：{n} {any}"},
			Settings:       []string{"内", "外", "內", "内景", "外景", "內景", "内/外", "外/内"},
			Times:          []string{"日", "夜", "晨", "昏", "黄昏", "黃昏"},
			Matchup:        []string{"内景", "外景", "内景/外景", "外景/内景", ""},
			Direction:      "ltr",
			Cues:           noCaps,
			Extensions:     []string{"画外音", "畫外音", "画外", "OS", "VO", "V.O.", "O.S."},
			Shape:          Shape{Chars: 6, Ends: "。！？…，；：!?,;:"},
			Repeat:         2,
			TransitionEnds: []string{" TO:"},
			Transitions:    []string{"淡入", "淡出", "淡入：", "淡出："},
			TitleKeys:      map[string]string{},
		},
		// Japanese headings (hashira) start with a circle: ○走る阪堺電車・外観（朝）
		// (Sakai City), ○警察署・外観（夜） (ja.wikipedia 脚本)
		"ja": {
			Scenes:         []string{"○", "◯"},
			Headings:       none,
			Settings:       none,
			Times:          []string{"朝", "昼", "夕", "夜"},
			Matchup:        []string{"", "", "", "", ""},
			Direction:      "ltr",
			Cues:           noCaps,
			Extensions:     []string{"OFF", "M", "Ｍ", "N", "Ｎ", "ＯＦＦ", "V.O.", "O.S."},
			Shape:          Shape{Chars: 8, Ends: "。！？…、!?,"},
			Repeat:         2,
			TransitionEnds: []string{" TO:"},
			Transitions:    []string{"F・I", "F・O", "Ｆ・Ｉ", "Ｆ・Ｏ"},
			TitleKeys:      map[string]string{},
		},
	}
	for k, l := range ls {
		ls[k] = l.clone()
	}
	return ls
}

// DefaultLanguages are the default configuration's languages, each in
// full (a copy).
func DefaultLanguages() map[string]Language { return maps.Clone(defaultLanguages()) }
