# Languages

A script's language decides how Lexington reads its Fountain: what a scene
heading looks like, what makes a line a character's cue or a transition,
its title page's keys and the way its text runs. These are named rules,
chosen per language in the configuration (`[Languages.xx]`); the parser has
no code for any one language. `script.zh.fountain` is read with Chinese's,
`-scenein zh` says so for any file, and a `.hekaya` file is read with
Hekaya's fixed rules (below).

A language is named by its BCP 47 tag, so one language can have a key per
script: `hi-Latn`, `hi-Deva`, `hi`. A tag reads as itself in any case and
with `_` for `-` (`hi-deva`, `hi_Deva`), else as itself without its last
subtag, down to the language: `script.hi-Deva-IN.fountain` is read with
`hi-Deva`'s rules, `de-CH` with `de`'s, `hi-Arab` with `hi`'s
(`TOMLConf.Lookup`, `rules.Tag`).

Every language carries its whole definition: `lexington -dumpconfig`
writes each one in full, and a script that travels with its configuration
(`TOMLConf.Part`, Accolade's `.accolade`) takes its language in full. A
field a configuration file leaves out is the default's for that language
(English's rules, with no scene words, for a language the default does not
have), so a file written before a field existed reads as it did. The
tables of older files — `[Scenes]`, `[Matchups]`, `[Directions]` — are
read into `[Languages]`.

## The configuration

```toml
[Languages.zh]
Scenes = ["内景", "外景", "内景/外景", "外景/内景"]
Headings = ["场景{n} {place} {time} {io}", "{n}. {place} {time} {io}"]
Settings = ["内", "外", "内景", "外景"]
Times = ["日", "夜", "晨", "昏", "黄昏"]
Matchup = ["内景", "外景", "内景/外景", "外景/内景", ""]
Direction = "ltr"
Cues = ["forced", "caps", "list", "parenthetical", "extension", "repeat"]
Extensions = ["画外音", "画外", "OS", "VO"]
Shape = { Words = 0, Chars = 6, Ends = "。！？…，；：" }
Repeat = 2
TransitionEnds = [" TO:"]
Transitions = ["淡入", "淡出"]
TransitionMarks = ""
TitleKeys = {}
```

| field | what it is |
|---|---|
| `Scenes` | the words a scene heading starts with, followed by a dot or a space (or anything, after a word ending in a sign: `○`) |
| `Headings` | patterns for headings that do not start with a word (below) |
| `NewHeading` | the pattern a new heading is written by (below); empty: the first of `Headings`, else Fountain's `{io} {place} - {time}` |
| `Settings`, `Times` | the words for inside/outside and for the time of day that the patterns' `{io}` and `{time}` take (`{io}` takes `Scenes` too) |
| `Matchup` | the scene words by role — inside, outside, both, both the other way, establishing — for translating headings and the statistics |
| `Direction` | `rtl` or `ltr`: what a line with no direction of its own takes |
| `Cues` | the cue rules (below) |
| `Extensions` | the language's words for a cue's extensions (V.O., O.S., CONT'D), for the `extension` rule |
| `Shape`, `Repeat` | what the `shape` and `repeat` rules take |
| `TransitionEnds` | what a transition ends in (` TO:`) |
| `Transitions` | transitions as a whole line (`قطع`, `FONDU AU NOIR`) |
| `TransitionMarks` | signs that may stand around one of those (`-` for `- قطع -`, `:`) |
| `TitleKeys` | the language's title page keys and the standard key each is (`العنوان = "Title"`); the standard keys work in every language |

### Cue rules

A line after a blank line, with text right under it, is a character's cue
when one of its language's rules reads it so:

| rule | the line |
|---|---|
| `forced` | starts with `@` (always, named or not) |
| `caps` | is in capitals: capital letters and no small ones (Fountain's rule; scripts without capitals never are) |
| `list` | is a name in the script's character list: the names in its `/* Characters */` block (Accolade's Characters tab), and every name read as a cue before it (Hekaya's Character Registry: the first `@سمير` makes a later `سمير` a cue) |
| `parenthetical` | is shaped like a name and a parenthetical follows directly (`(quietly)`, `（笑）`) |
| `extension` | ends in one of the language's `Extensions` in brackets: `ANNA (V.O.)`, `张三（画外音）` |
| `shape` | is shaped like a name: at most `Shape.Words` words and `Shape.Chars` characters (0: any number), none of the sentence ends in `Shape.Ends` |
| `repeat` | passes `shape` at least `Repeat` times in the script |

Names are compared without their extension and dual dialogue's `^`, in
capitals, with their spaces as one, Arabic without its vowel marks and with
one alef (`أ إ آ ٱ` as `ا`), as Hekaya's registry compares them. English
and the other languages written in capitals keep Fountain's `caps`; Hebrew,
Chinese and Japanese take `forced, list, parenthetical, extension, repeat`;
Arabic is Hekaya's `forced, caps, list`. In Go: `fountain.NewDialect(lang)`
reads lines by a language's rules (`Heading`, `Transition`, `Cue` with the
script's `Cast`), which is what Accolade's editor reads with too.

### Transitions

A line with blank lines around it, with no small letters, that ends in one
of `TransitionEnds` or is one of `Transitions` (its `TransitionMarks`
around it left out). `>` forces one anywhere, as in Fountain. English has
only ` TO:`: `FADE OUT.` stays action unless forced, as Fountain reads it.

### Heading patterns

A pattern is a heading's shape with slots: `{n}` a scene number (digits —
Western, Arabic, Persian, Chinese — and a letter after them, `15B`),
`{io}` one of `Settings` or `Scenes`, `{time}` one of `Times`, `{place}`
and `{any}` any text. A space is any spaces; a dash is any dash (`-`, `–`,
`—`) with or without spaces; a slash with or without them. A pattern line
is a heading after a blank line, as a scene word's is; `#1#` at its end is
its Fountain scene number. The patterns exist because real scripts write
headings in other orders (below); Fountain's own way (`INT. PLACE - DAY`,
the number in `#1#`) is read as it always was.

A `{n}` takes a letter before its digits or after them (`A11`, `11A`): a
locked script's scene put in between.

### Writing a heading

`NewHeading` is the pattern a program writes a new heading by
(Accolade's Insert › New Scene…): one of `Headings`, or one that starts
with a scene word. Left empty it is the first of `Headings`, else
Fountain's own form, `rules.SceneForm` (`{io} {place} - {time}`: INT.
KITCHEN - DAY). Japanese writes `○{place}（{time}）`, its hashira
(○走る阪堺電車・外観（朝）, Sakai City; ○警察署・外観（夜）, ja.wikipedia
脚本). It is only for writing: `Scenes` and `Headings` are what reads one.

In Go: `Language.HeadingForm()` is the pattern; `Language.IOWords(form)`
the words its `{io}` takes, the commonest first (the `Matchup` words for a
form that starts with `{io}`, else `Settings` and then `Matchup`);
`rules.FillPattern(form, values)` the heading, a slot with no value left
out with the text joining it; `rules.ParsePattern` and `JoinPattern` take
a pattern apart and together again, byte for byte; and
`fountain.Dialect.HeadingParts(line)` reads a heading back into its slots
(a scene word's heading too: its word, the place, the time after the
last dash).

## File extensions

A language can have a file extension of its own: `[Extensions]` maps an
extension (without its dot) to a language, and a file with it is read
with that language's rules, as `film.ar.fountain` is read with Arabic's.
The default configuration has Hekaya's (`hekaya = "hekaya"`); one's own
add to it or change one, and a script that carries its configuration
(`TOMLConf.Part`) carries the extensions of its language.

```toml
[Extensions]
hekaya = "hekaya"   # the default's
kissa = "ar"        # one's own: film.kissa is Fountain in Arabic
```

`TOMLConf.ExtensionLanguage(name)`, `ExtensionOf(lang)`,
`FileExtensions()`; the command line reads them for `-i` and `-o`, and
Accolade for opening, Save As and renaming.

### Other Fountain dialects with their own extension

Looked for 2026-11-22 (GitHub, npm, crates.io, fountain.io's app list;
fetched normally, nothing was blocked). Besides Hekaya, two tools are
Fountain-derived with an extension of their own, read from their sources:

- **Wrap** (github.com/eprovst/wrap, GPL-3.0, "currently not actively
  maintained"), `.wrap`: "Wrap builds upon [Fountain] to add support for
  different languages and stageplays". The language is chosen per file by
  a `Language:` title key (Dutch `bin.`/`bui.`/`open.`, German `etabl.`,
  Italian `amb.`/`int./est.`), transitions may end in `NAAR:`, `A:`, `À:`,
  `ZU:`; acts and localised (MORE)/(CONT'D). Not a default entry: a `.wrap`
  file's language is in its title page, not its extension, which
  Lexington's table cannot say (a `wrap = "nl"` of one's own reads Dutch
  ones).
- **Playfount** (github.com/tucnak/vim-playfount, MIT), "a dialect of
  Fountain", Russian and Ukrainian (`ИНТ.`, `ЭКСТ.`, `ІНТ.`, transitions in
  `К:`), parentheticals on the cue's line, `% Title` title pages. Its
  vim plugin matches `*.play*`; it names no extension of its own, so no
  entry.

Found but not Fountain by their own account, or not a text dialect:
Script Editor for Obsidian (Hebrew, `.script`, its own prefixes),
EasyScript (Chinese, `.sp`), draM. (Turkish, `.dram`, zipped JSON),
Scriptty (Malayalam, `.screenplay`, JSON). Localised Fountain that keeps
`.fountain` or `.txt`: in剧本/injuben (Chinese, `外景 地点 - 时间`,
`角色：台词`), Write Now (Chinese, Japanese), K-Fountain (Korean),
Scénario Atelier (French `INTÉRIEUR`/`EXTÉRIEUR`), ften-web (Indian
two-column). So the default table has Hekaya only.

## What reads differently from 1.26

- Hebrew, Chinese and Japanese cues without `@`: a name in the
  Characters list, before a parenthetical, with an extension, or that
  comes back (`repeat`); Arabic reads Hekaya's way (a name after its
  first `@`). A Latin name in capitals is a cue as before.
- Arabic transitions without `>` (`- قطع -`), Hebrew ones ending in
  ` ל:`, Chinese `淡入`/`淡出`, Japanese `F・I`/`F・O`, German `SCHNITT`,
  `… AUF:`, French `FONDU AU NOIR`, Italian `… SU:`.
- Headings by the patterns: numbered Chinese, Arabic, Italian, Russian and
  Hebrew headings, German and French place-first ones; German lines
  starting with `INNEN` or `AUSSEN` after a blank line are headings.
- Arabic title keys (`العنوان` is the title).

## How real scripts write scene headings

Researched 2026-11-22 for this rule set; fetched normally (sites that
refused automated requests were skipped, not worked around). Quotes are
verbatim; the Arabic and Japanese layouts are given in reading order.

| language | what real scripts do | sources |
|---|---|---|
| Chinese (mainland) | numbered, place first, then time, then 内/外; or time first, place last: `场景1    小区楼下    日    外`, `13. 街道  黄昏  外  夏` (《孔雀》), `1.日  外  清晨的田野间` (《那山那人那狗》), `2. 大华宾馆房间   夜   内`. Cues on the dialogue's line with a full-width colon (`小慧：你为什么躲着不见我？`); `淡入`, `[画外音]` | thesis screenplay, Xi'an International Studies University, zwxy.xisu.edu.cn/__local/9/79/53/8C038E0EDC07DD0F22C105DD80D_16629828_19A00.doc; amber-zhang.ghost.io/fen-chang-jing-ju-ben-ge-shi/ (quoting the films); sohu.com/a/452935672_693248 (a translated US guide: `内 起居室——日`, `淡入：`, `淡出`) |
| Chinese (Taiwan) | `1. 內景 阿強家客廳 夜` (number, 內景/外景, place, time); the cue on its own line or with a colon; contest templates `場：1 景：捷運淡水站大廳` / `時：日 人：…` | 48th 優良電影劇本 template, ghsa.org.tw/esa/datas/upload/finder/files/20251112144310612.pdf; ed.arte.gov.tw/uploadfile/NewsFile/2369L_劇本格式(範例).pdf |
| Japanese | `○` + place, `・` sub-place, time in `（）`: `○走る阪堺電車・外観（朝）`, `○警察署・外観（夜）`; cues `里子「…」`, `Ｍ「…」` (monologue), N, （OFF）, F・I, F・O | Sakai City's published script, city.sakai.lg.jp/…/omnibus_pdf.pdf; ja.wikipedia.org/wiki/脚本; koubo.jp/article/17953 |
| Hebrew | interior/exterior first, numbered: `1. פנים. משרד דפנה – יום.`; also `חוץ. יום. גינתו של יוסף`; cue on its own line; `דיזולב ל:`, `קאט ל` | dlg.co.il/knowledge-base/sequence/; icinemaspot.com/index.php?p=tuts&tut=53; akolnoa.blogspot.com/2012/05/blog-post_1153.html; he.wikipedia.org/wiki/תסריט |
| Arabic | numbered first, place in the middle, time/setting last: `مشهد 12  خارج شقة إنجي/البسطة  ن/د` (ن/د = نهار/داخلي), `مشهد (1)  غرفة الضيوف  ليل / داخلي`; cues on their own line or `إنجي : …`; Hekaya: `داخلي - قهوة بلدي - نهار` | Egyptian series شقة 6, episode 5 (a public Google Doc); University of Diyala lecture شكل السيناريو, colarts.uodiyala.edu.iq; michaelkmalak.github.io/hekaya |
| German | place first, `INNEN/TAG` at the end: `18. GERICHTSSAAL INNEN/TAG`, `HOTEL DAVOR AUSSEN/NACHT`; also `INNEN, BÜRO – TAG`; `(OFF)`, `SCHNITT`, `SCHNITT AUF:` | MURER shooting script, drehbuchforum.at/media/drehbuch_murer_shooting_script.pdf; de.wikibooks.org/wiki/Wie_schreibe_ich_ein_Drehbuch/_Das_Layout_eines_Drehbuches |
| French | place first: `5 - CHU GRENOBLE – INT/JOUR`, `2- BOIS (près du chalet) – EXT/JOUR`; or `SÉQUENCE 1 – INT. JOUR` with the place on the next line; `(OFF)`, `FONDU AU NOIR` | Anatomie d'une chute, wp.filmlinc.org/wp-content/uploads/2023/12/ANATOMY-OF-A-FALL-Script-French-English.pdf; quinzaine-cineastes.fr workshop scripts |
| Italian | numbered, prefix: `1. INT - CASA DI MATILDE E ANGHELA - CUCINA - GIORNO`, `5. INT. CASA DELLA VEDOVA, CAMERA MARCO - GIORNO`; `(voce fuori campo)`, `(CONTINUA)`, `STACCO SU` | cinemaperlascuola.istruzione.it/…/Sceneggiatura-Episodio-4-Il-castello-dei-segreti.pdf; premiosolinas.it/…/sceneggiatura-2015.pdf |
| Dutch | prefix: `EXT. KAPSALON – DAG`, `INT. KANTOOR PETER – DAG`; `off`, `O.S.`, `V.O.`, `VERVOLGT` | havicinfo.nl/images/pdf/workshops/Speelfilm_maken-zonder_achtergrond.pdf |
| Russian | numbered prefix, full stops: `1. НАТ. ГОРОДСКАЯ ПЛОЩАДЬ. НОЧЬ`; `(за кадром)`, `(продолжает)` | «Самый лучший день», kinodramaturg.ru |
| English | `INT./EXT. PLACE - TIME`, Fountain's | fountain.io/syntax; finaldraft.com/blog/everything-that-should-and-should-not-be-in-a-scene-heading |
| Hindi | in Latin letters, English sluglines (`INT. MUMBAI APARTMENT - LIVING ROOM - NIGHT`); in Devanagari, place first, inside/outside last: `शिक्षा अधिकारी का दफ़्तर (बाहरी कक्ष)/दिन/अंदर`, `कट टू`, `(डिजॉल्व टू)` | below, "Hindi" |
| Esperanto, toki pona | no evidence found; Lexington's own words | — |

So other word orders are in real use — Chinese (place or time first),
Arabic (number first, setting last), German and French (place first) —
and the default configuration has patterns for those it found. Not read:
a cue on the same line as its speech (`张三：…`, `里子「…」`, `إنجي : …`),
which Fountain has no element for; Taiwan's two-line labelled block reads
its first line (`場：1 …`) as the heading. Words marked here as
Lexington's own are not from a source: German `ABEND`, `MORGEN`; French
`NUIT`, `SOIR`, `MATIN`; Hebrew and Japanese time words beyond the
examples, and its fades `פייד אין`, `פייד אאוט`; Chinese `OS`/`VO`; Esperanto's and toki pona's.

## Hindi

Three keys, one per way of writing (Joop, 2026-11-23):

- **`hi-Latn`**, Hindi in Latin letters, what most Hindi Fountain scripts
  are: English Fountain as it is — `INT./EXT.`, `CUT TO:`, `V.O.`,
  cues in capitals — with the dialogue in Hindi
  ("Tumhein lagta hai main yahaan apni marzi se aaya hoon?"). Its time
  words add `DIN`, `RAAT`, `SUBAH`, `SHAAM` to English's (Lexington's own
  spelling; no source wrote the time in Hindi in Latin letters). No Hindi
  scene words in Latin letters: none were found, and `ANDAR`/`BAHAR`
  before a space would make every action line starting "Andar se…" a
  heading. Its title page keys are the Devanagari ones in Latin letters
  (below).
- **`hi-Deva`**, Hindi in Devanagari: headings place first, time, then
  inside/outside, slash-separated, as NCERT's textbook writes them
  (`शिक्षा अधिकारी का दफ़्तर (बाहरी कक्ष)/दिन/अंदर`; also numbered,
  `3. घर का आँगन / रात / बाहर`); `अंदर`/`बाहर` for inside and outside,
  `दिन`, `रात`, `सुबह`, `शाम` (and `दोपहर`) for the time; the transitions
  `कट टू` and `(डिजॉल्व टू)` (its brackets are `TransitionMarks`). English's
  scene words, extensions and ` TO:` are kept: many Indian scripts write
  English headings and action with Devanagari dialogue. Cues without
  capitals (`list`, `parenthetical`, `extension`, `repeat`), as Hebrew's;
  a Latin name in capitals is a cue as before; `।` and `॥` end a sentence.
- **`hi`**, both at once: `hi-Deva`'s rules with English's scene words
  first and `hi-Latn`'s time words and title keys too; a writer writes in either script or
  mixes them. Everything `hi-Latn` and `hi-Deva` read, `hi` reads
  (`TestHindiBoth`).

No standard for Devanagari screenplay terms was found: Hindi guides and
the textbook write `INT.`/`EXT.` and the transitions in English or in
English transliterated (`कट टू`). A Hindi course book's glossary of film
terms (below, MANUU) has `कट`/`कट टू`, `फेड इन`, `फेड आउट`, `डिजॉल्व`, and
`वॉयस ओवर` (also `वायस ओवर`) as a narrator's or a character's voice over the
picture; it gives them as terms, not as written in a cue.

Sourced in `hi-Deva`: the heading's order and words (NCERT), `कट टू` and
`डिजॉल्व टू` (NCERT, MANUU), `फेड इन`/`फेड आउट` (MANUU), the extension
`वॉयस ओवर`/`वायस ओवर` (the term from MANUU; writing it in the cue's brackets,
`कथावाचक (वॉयस ओवर)`, is Lexington's own, as English writes `(V.O.)`).
Lexington's own, from no source: the dotted scene words
`अंदर.`, `बाहर.`, `अंदर/बाहर.`, `बाहर/अंदर.` (a heading starting with
them, `अंदर. रसोई - सुबह`, for New's template and for translating headings;
the dot keeps "अंदर से…" action), `दोपहर`, the fades spelled with a nukta
`फ़ेड इन`/`फ़ेड आउट`, the extensions `ऑफ़ स्क्रीन` (`ऑफ स्क्रीन`; O.S.,
transliterated as the fades are) and `जारी` (CONT'D, "continuing"), and every
title key: `शीर्षक` (Title), `श्रेय` (Credit), `लेखक` and `पटकथा` (Author),
`स्रोत` (Source), `मसौदा` (Draft), `मसौदे की तिथि` (Draft date), `तिथि` and
`दिनांक` (Date), `संपर्क` (Contact), `टिप्पणियाँ` (Notes), `कॉपीराइट`
(Copyright). English's extensions (`V.O.`, `O.S.`, `CONT'D`) are still read,
after the Hindi ones.

`hi-Latn` keeps English Fountain's extensions and transitions (sourced:
proofscript.in; Hindi in Latin letters writes them in English). Its title
keys, Lexington's own, are the Devanagari ones in Latin letters, in the
spelling of the examples above (no diacritics): `Sheershak` (Title),
`Shrey` (Credit), `Lekhak` and `Pataktha` (Author), `Srot` (Source),
`Masauda` (Draft), `Masaude ki tithi` (Draft date), `Tithi` (Date),
`Sampark` (Contact), `Tippaniyan` (Notes); the keys are read in any case.
A native reader's correction of any of these is welcome.

Not read: the textbook's `रजनी : …`, a cue on its speech's line (as
Chinese's `张三：…`), and its scene labels in words (`दृश्य-चार`).

Sources (fetched 2026-11-23, normally; a site that refused was skipped):
the NCERT textbook अभिव्यक्ति और माध्यम's chapter कथा-पटकथा, with the
screenplay of रजनी, hindwi.org/creative-writing/katha-pataktha-unknown-creative-writing
(the heading's order: number, place, time दिन/रात/सुबह/शाम, inside or
outside); proofscript.in/learn/indian-screenplay-format-template/ (English
sluglines, dialogue in English, in Hindi in Latin letters or in
Devanagari); hollywoodreporterindia.com, "Why Is There a Dearth of Hindi
in Hindi Film Scripts?" (Hindi film dialogue mostly written in Roman
letters). Maulana Azad National Urdu University, *जनसंचार माध्यम एवं हिंदी*
(Jansanchar Madhyam evam Hindi, MA Hindi, 4th semester, self-learning
material), dde.manuu.edu.in/sites/default/files/DDE/DDE-SelfLearnmaterial/MA-Hindi-4th-Sem/Jansanchar_Madhyam_evam_Hindi.pdf:
its glossary "सिनेमा की तकनीकी भाषा" (`कट/कट टू`, `फेड आउट`, `फेड इन`,
`डिजॉल्व`, `वाइप`) and its pages on `वॉयस ओवर`/`वायस ओवर` (fetched
2026-11-23). baou.edu.in's पटकथा लेखन course (HNDSEC-201) refused the
connection and was not read.

## Title page keys

Every language but English has its own title page keys (`TitleKeys`), each
the standard key it is: Esperanto `Titolo`, `Aŭtoro`, `Dato de malneto`;
German `Titel`, `Drehbuch`, `Fassung`; Chinese `片名`, `编剧`; Japanese
`題名`, `脚本`; and so on. The standard keys (`Title`, `Credit`, `Author`,
`Source`, `Draft date`, `Contact`, `Notes`, `Copyright`…) are read in every
language too, so a script may mix them. Arabic's are Hekaya's, Hindi's in
its section above; the others
are Lexington's own choice, each language's usual word for the field, not
from a source: a native reader's correction is welcome. The key is
followed by an ASCII colon (`编剧: 张三`); a full-width `：` is not read as
one. Force (a language's text as standard Fountain) leaves the title page
as written, keys included.

## Hekaya

[Hekaya](https://github.com/michaelkmalak/hekaya) (MIT, spec v0.1) is
Fountain in Arabic. Lexington's default Arabic is Hekaya's full set — its
scene words (with English Fountain's beside them), transitions with their
dashes (`- قطع -`), extensions (`ص.خ`, `خ.ش`, `خارج الكادر`, `تابع`), title
keys (`العنوان`, `سيناريو`/`كتابة`/`تأليف`, `المصدر`, …), the cue rules
`forced, caps, list` (the list filled by the script's own `@` uses: the
Character Registry), right to left, `Direction:` on the title page, Arabic
digits in `#١٢#` — plus the numbered headings real Arabic scripts use. That
Arabic is the writer's to change; the fixed language `hekaya` is not: a
`.hekaya` file (`.ar.fountain` read so) is always read with Hekaya's own
rules. `Direction: rtl` on a title page sets the direction of the lines
with none of their own (`lex.TitleDirection`).

Hekaya's samples and templates are in `testdata/hekaya` (MIT, with its
licence and the commit they come from); the PDF prints them shaped
(`pdf/arabic.go`) and `TestArabicShapedAsHarfBuzz` checks every glyph.

### Hekaya, where its spec is open

Where the spec, its code (`keywords.ts`, `rules.ts`, `lexer.ts`) and its
compatibility page disagree or say nothing, Lexington chose: what Fountain
does where Hekaya follows Fountain, otherwise the reading least likely to
misparse a real Arabic script. Each is tested (`TestHekayaOpenSpec`) and
drafted as an issue for Hekaya in `docs/hekaya-issues/`.

| # | open point | Lexington |
|---|---|---|
| 1 | `مقدر` is a scene word on the compatibility page only, not in the spec or the code | not a scene word: an action line starting "مقدر له…" would become a heading |
| 2 | `مزج إلى` is a transition in the code only, not in the spec's table | a transition, like `مسح إلى` and `وميض إلى` |
| 3 | `خارج الكادر` is O.C. in the spec, O.S. in the code | an extension, kept as written; meant as O.C. (the spec) |
| 4 | `عودة للمشهد` is BACK TO SCENE in the spec, BACK TO in the code | a transition, kept as written; meant as BACK TO SCENE |
| 5 | a registered name is a cue when "followed by dialogue" (spec); the code takes any later non-blank line, even after a blank one, and only names registered earlier | Fountain's: the speech right under it; earlier `@` uses only (the spec's "subsequent") |
| 6 | the code reads a heading on any line; Fountain needs a blank line before | a blank line before (Fountain) |
| 7 | keyword transitions need a blank line before (spec); Fountain wants blank lines around a transition | blank lines around (Fountain): a one-word line of action or dialogue stays what it is |
| 8 | the code's title page takes only its listed keys; Fountain any key | any key (Fountain) |
| 9 | `== text` is neither a synopsis nor a page break in the code | a synopsis (Fountain) |
| 10 | direction: "Mixed → determined by title page language", which is not defined | each line by its own letters; a line with none by `Direction:`, else right to left |
| 11 | scene numbers in Arabic digits: the spec accepts them, says nothing of renumbering | kept as written (`#١٢#`); renumbering writes Western digits |
| 12 | "standard Fountain UPPERCASE detection works" — the code's is ASCII only (`[A-Z]`) | any capitals (Fountain): `JOSÉ` is a cue |

Not open: `>text<` is centred text and `>text` a forced transition in both,
as in Fountain (Hekaya's code also drops a forced transition's trailing
colon; Lexington keeps the text as written).
