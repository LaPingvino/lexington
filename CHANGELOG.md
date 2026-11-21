# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.22.0] - 2026-11-21

### Added
- **`-strip`: a clean copy of a Fountain script** (`fountain.Strip`). It
  always takes out Accolade's data — its end blocks (Notes, Source,
  Adaptation, Production, Characters), the language line after them and
  the `[[from: …]]` source links, in a kept boneyard too — and, as a comma
  list says, more: `notes`, `boneyard` (as Parse reads one: from a line
  starting with `/*` to the line that closes it; an omitted scene's kept
  text too), `numbers`,
  `omitted` (the OMITTED headings with the scenes kept under them),
  `titlepage`, `outline` (sections and synopses), or the presets `clean`
  (Accolade's data only), `reading` (notes, outline) and `writer` (scene
  numbers, OMITTED); they add up (`writer,notes`). Fountain out (the
  default with `-strip`) is the text as it was, byte for byte where
  nothing went; other formats convert what is left. A line left with
  nothing goes, with a blank line where two would be left; a line that
  would read as another element gets its force mark. Never over the
  input. `fountain.StripOptions`, `StripKeys`, `StripPresets`,
  `ParseStrip`.
- **Accolade's end blocks in one place**: `fountain.EndBlocks` (the
  blocks a script ends with, from the notes back, in any order, and where
  the script proper ends), `EndBlockNames`, `EndBlockMarks` /
  `BlockMarks.At` (the plain scan Accolade used), `LanguageLine` and
  `IsSourceLink`. Accolade finds its blocks with them, so what counts as
  one is said once.

## [1.21.0] - 2026-11-21

### Added
- **The PDF prints right to left** (scripts that need no shaping:
  Hebrew). A line reads right to left by its own direction mark or first
  strong letter, a line with neither (`…`, `12.`) by its language's
  `[Directions]` (`pdf.PDFWriter.RTL`, set from `-scenein`). Its levels
  are resolved over the whole row by the Unicode bidi algorithm
  (go-text/typesetting's `bidi`, a new dependency, already Fyne's), each
  wrapped line reordered (L2) with brackets and `<>«»` mirrored, so mixed
  Hebrew/Latin lines and numbers read right both ways; a Latin line with
  Hebrew words in it keeps its side. Points stay with their letters (a
  point is reversed with its letter, and Courier Badi draws a point over
  the cell after it). A right-to-left line's element is mirrored: its
  margins swapped and its alignment turned (action and dialogue end at
  the right margin, a transition starts at the left, the scene numbers
  follow the heading), a speech's (MORE) on its side, a scene's
  CONTINUEDs mirrored, a dual dialogue whose first speaker reads right to
  left mirrored with that speaker on the right. Lines wrap where the
  pagination counts them, so pages, locks and scene lengths are the same
  in either direction (a speech broken after a sentence keeps its
  direction on the next page). Direction marks and other bidi controls
  on a reordered line set the order and print as nothing (Courier Badi
  gives them a blank cell). Scripts with no right-to-left text print as
  before (the examples' text, positions and page images unchanged). The
  page number, the revision marks and the color bar stay where they are.
  GPOS mark positioning is not applied: each point stands at its default
  place, so the shin and sin dots both sit over the letter's middle (the
  sin dot a little higher) and are hard to tell apart, a holam stands
  over the vav's stem, a dagesh in a vav (shuruk) is hidden in its
  stroke.

### Changed
- **`pdf.Unsupported` reports shaping, not direction**: its second result
  is true for letters that join (Unicode Joining_Type D, R or L, from
  ArabicShaping.txt: Arabic, Syriac, N'Ko, Mongolian…; `pdf/joining.go`,
  `go generate`), and `cli.PDFWarning` says those print unjoined. A
  Hebrew script no longer warns, so it makes its PDF natively.

## [1.20.0] - 2026-11-21

### Changed
- **Courier Badi 1.100** is the embedded font (Regular, Italic, Bold,
  Bold Italic): Hebrew letters, final forms, points and cantillation
  (from Cousine, fitted to Courier Badi's width), U+05EF and Bold
  anchors for the marks. Hebrew no longer counts as missing from the
  font in `pdf.Unsupported`; the PDF still warns that right-to-left
  text prints left to right (native PDF bidi comes next).

## [1.19.0] - 2026-11-21

### Changed
- **Fountain is read as the spec has it** on four points where Lexington
  read more loosely: a scene heading needs a blank line before it (a line
  starting with `INT.` right under action is action); so does a
  character cue (capitals right under action are action); a transition
  is in capitals, ends in `TO:` and has blank lines around it (`cut to:`,
  `... and DISSOLVE TO:` at the end of action are action); a line of two
  spaces (or more) within dialogue keeps the speech going, as a blank
  dialogue line (`lex.TypeDialog` with no text; the Fountain writer
  writes it as two spaces again, Final Draft as an empty Dialogue
  paragraph and back; PDF, HTML and DOCX print it as a blank line). Forced elements (`. @ > ! ~`) are unchanged, the
  scene words are still each language's from the configuration, the
  title page and the rest read as before. Over ~24,000 lines of real
  scripts, examples and documentation 13 lines read otherwise, nearly all
  where the old reading was wrong (dialogue starting "Open…" read as a
  heading through another language's word, "We CUT back to:" ending
  action read as a transition).
- **The Fountain writer writes spec-valid Fountain**: a heading or a
  name without a blank line before it gets its force mark (`.`, `@`), a
  transition that would not read as one bare gets `>`, action that would
  read as a heading `!`.

### Added
- **Scripts that relied on the old reading:** `fountain.ParseLoose` and
  `ParseLinesLoose` read as Lexington did before; `fountain.KeepLoose(text,
  scenes)` writes a script so that the spec reads it as the old reading
  did — a blank line before a heading or a name that had none (or its
  force mark, where a blank line would change the line before), `>` for
  a transition, a line of spaces that ended a speech emptied — and says
  in how many places (0, and the text unchanged, when both read it
  alike). Accolade's *Keep It As It Was* is this.
- **`-keeploose`** writes the Fountain input so (`-o`, or over the input
  with `-o ""`; CR LF line ends kept), and **`-loose`** reads it the old way for one conversion.

## [1.18.0] - 2026-11-20

### Added
- **A language's direction, as the last resort only**
  (`rules.TOMLConf.Directions`, `[Directions]` in the configuration:
  `"rtl"` or `"ltr"`; the default configuration has `ar` and `he` right
  to left, the rest left to right). `RTL(lang)` and `Direction(lang)` read
  it; a language a configuration does not name has the default's. A line takes it
  only when it has no direction of its own — no direction mark, no strong
  letter (`lex.OwnDirection`, `lex.LineRTL`): `…`, `2.`, a bare number.
  Every other line follows its own characters.
- **The HTML writer** (`HTMLWriter.RTL`, set by the command line from the
  script's language, `-scenein`) gives such a line `dir="rtl"` in a
  right-to-left language; the others keep `dir="auto"`.
- A language's direction travels with it: `Part` carries it (`"ltr"`
  too, the language in full), `With`, `Items`, `Of` and `Renamed` know it.

## [1.17.0] - 2026-11-20

### Added
- **A configuration that travels with a script** (`rules/layer.go`).
  `TOMLConf.Part(langs, sets)` is what a script in those languages, laid
  out with those element sets, uses (their scene words and matchups, the
  sets, the `Standard`); `Encode` writes a configuration as TOML (empty
  parts left out), `Decode` reads one from bytes; `With(layer)` lays one
  over another (the layer's languages, sets and Standard win; neither is
  changed). `Items`, `Of`, `Same` and `Renamed` list a configuration's
  languages, sets and Standard, take one out, compare and rename it: a
  program can copy a script's settings into its own without knowing the
  fields. A field added to `TOMLConf` travels by being added there.
- **`rules.WithPresetFiles(list, files)`:** format files given as text by
  key, as in a folder of `PresetsIn` (a Base among them resolved first).
- **`TOMLConf.WithPresets()`:** the presets among a configuration's
  element sets, the configuration itself unchanged.
- **`cli.Options.Conf`:** the settings themselves, in place of
  `ConfigFile`'s (a program with its own, layered ones).

### Changed
- `TOMLConf`'s fields are `omitempty` in TOML: an encoded part has only
  what it holds.

## [1.16.0] - 2026-11-20

### Added
- **`fountain.Force` and `fountain.Unforce`: a language's script as
  standard Fountain and back.** A script read with one language's scene
  words is a dialect of Fountain; `Force(text, from, to)` adds Fountain's
  force marks (`.` heading, `!` action, `@` character) to each line the
  words `to` would read as another element, deciding by parsing both ways
  line by line, the lines otherwise as written (no writer round trip,
  which would put headings in capitals and rewrite the title page).
  `Unforce` takes back exactly those marks: one the other words need too,
  or that the writer put where they need none, stays. Dialogue has no
  force mark: such a line stays as it is. Only the first line of a run that
  reads otherwise is marked before reading again, so a line that only
  followed a misread one gets none.
- **`Standard` in the configuration** (`rules.TOMLConf.Standard`,
  `StandardLanguage()`): the language whose scene words are standard
  Fountain's, `"en"` by default.

### Fixed
- The Fountain reader reads with the scene words it is given, also for
  telling a title page field from a scene heading, instead of the shared
  `fountain.Scene`, which it still sets for `CheckScene`'s callers: a
  script is read with its own words even while another is read in
  another language (the write to `fountain.Scene` itself is still shared).

## [1.15.0] - 2026-11-20

### Removed
- **v1.13.0's merged OMITTED form is no longer read.** `lex.OmittedRange`,
  `lex.SplitOmittedRanges`, `fountain.SplitOmitted`, the Fountain
  reader's splitting of `.OMITTED #12-14#` into its scenes, and
  `NumberOptions.Omitted` (NumberScenes/NumberLabels/Renumber reading
  "12-14" on an OMITTED heading as a run of numbers) are gone. Such a
  heading is now one scene labelled "12-14", a fixed label like any other
  dashed one: read, numbered and printed as written, never split, never
  renumbered or dropped, never a neighbour. Nobody used the form in
  production, and undoing it could break legitimate dashed labels.
  Printing a run of OMITTED scenes as one (`-mergeomitted`,
  `lex.MergeOmittedRuns`) stays: a print option, not a text form.

## [1.14.0] - 2026-11-20

### Changed
- **Scene numbers are labels.** The numbering engine (`NumberScenes`,
  `NumberLabels`, `SceneNumberBetween`, `Renumber`) reads one scheme: a
  whole number (no leading zero) with letters after it (10A, 10AA, 10aA)
  or capitals before it (A11, AB11, A11B); `lex.InNumberScheme` tells.
  Every other label (Fountain's I-1-A, 1-A, 1., 1.1, PROLOGUE, 01, 10a,
  Storyboarder's 3-x2k9q) is fixed: never renumbered (by any strategy,
  OMITTED or not), never given to another scene, never a neighbour (the
  nearest scheme numbers are).
- **No ranges as scene numbers.** v1.13.0's `.OMITTED #12-14#` form is
  superseded: the text keeps one `.OMITTED #n#` a scene. `OmittedMerge`,
  `Renumbered.Merged`, `NumberRange` and `CompareNumbers` are gone.
  An old merged heading (`lex.OmittedRange`: an OMITTED heading, both
  ends scheme numbers) is read as its scenes when Fountain is parsed
  (`lex.SplitOmittedRanges`: one heading a `/* Omitted n` block, its first
  and last kept), and `fountain.SplitOmitted` rewrites it in the text;
  until then locked numbering never gives a number inside it.
- **Number lists by the script's order** (`lex.ParseNumberSet(s, labels)`):
  a range X-Y is every scene from the one labelled X to the one labelled Y
  as they appear, whatever their labels ("4-2" where 4 comes first);
  Y before X is an error, not swapped. Ranges join with "–", "..", "…"
  or a spaced " - "; a lone "-" only where both sides are labels of the
  script and it splits one way. `Has` takes a scene index; `Unknown`,
  `Errors` and `Read` say how the list was read. `RenumberOptions.Reserved`
  are labels only.

### New Features
- **OMITTED runs printed as one** (`lex.MergeOmittedRuns`,
  `pdf.PDFWriter.MergeOmitted`, `cli.Options.MergeOmitted`,
  `-mergeomitted`): consecutive OMITTED headings print as one
  `12-14 OMITTED 12-14` line in PDF, HTML, DOCX, ODT and the rest, its
  pages, lock and measures alike (`lex.OmittedRunSizes` gives the
  measures back to each heading); FDX, Fountain and lex keep one heading
  a number.

### Fixes
- A long scene number in the PDF's right margin (a run printed as one,
  `I-1-A–I-1-C`) is moved in to end a quarter inch before the page's
  edge instead of running off it.

## [1.13.0] - 2026-11-20

### New Features
- **Renumbering strategies** (`lex.Renumber`, `lex.RenumberOptions`): the
  scenes that get a new number (`Free`; the others keep theirs), letters
  folded or kept (10, 10A, 11 → 9, 9A, 10), OMITTED dropped, kept or
  merged (`OmittedDrop`, `OmittedKeep`, `OmittedMerge`; never a range
  holding another scene's number), reserved numbers never given (a range
  reserves its whole numbers, its letters stay free). Free scenes count on from the pinned scene before them, and
  get a letter between their neighbours where a pinned number forces it.
- **Ranges of scene numbers**: `.OMITTED #12-14#` for a run of omitted
  scenes (`lex.NumberRange`, `lex.CompareNumbers`); prints as
  `12-14 OMITTED 12-14`, FDX `Number="12-14"`. Locked numbering never gives
  a number inside a range, and takes a range as a neighbour (its last
  before, its first after). `fountain.SplitOmitted` makes a merged run one
  OMITTED heading a block again.
- **Number lists** (`lex.ParseNumberSet`): `5, 12A, 20-24`, a range for
  every number from its first to its last.

### Fixes
- Locked numbering looks for letters up to two more than its neighbours
  have, then takes the next number past those taken, rather than looking
  for ever where a range takes every number between two scenes.

## [1.12.0] - 2026-11-20

### New Features
- **Strips** (`Report.Strips`, `report.StripColour`): each scene's strip,
  INT/EXT (by the language's place words: EXT., INT./EXT. and EST. are
  outside) and day or night (NIGHT, EVENING, DUSK … in the known
  languages; CONTINUOUS and LATER take the scene before's), coloured as the
  generic stripboard convention has it (white INT. DAY, yellow EXT. DAY,
  blue INT. NIGHT, green EXT. NIGHT: Wikipedia's "Production board",
  StudioBinder), with its cast by number.
- **The shooting schedule's reports** (`report.Schedule`, `report.Day`):
  the one-liner (`Report.OneLinerTable`, `report.WriteOneLinerPDF`: each
  day's head, its strips in their colours, the day break with its total in
  eighths) and the Day Out of Days (`Report.DOOD`, `Report.DOODTable`,
  `report.WriteDOODPDF`: SW, W, WF, SWF and H as Movie Magic Scheduling
  codes them, days worked and held, start and finish), as CSV (stable
  layouts, golden files) and PDF.
- **Shoot days as iCalendar** (`report.WriteICS`): an event a dated day
  (from the call for 12 hours, floating local time; a whole day without a
  call), the scenes in the description, the notes as its location; RFC
  5545 folding and escaping.

## [1.11.0] - 2026-11-20

### New Features
- **Breakdown categories** (`lex.TagCategories`, `lex.TagCategoryOf`):
  Final Draft's tag categories as Final Draft 13 writes them (Ids, names,
  numbers; Movie Magic's names), with the colours Final Draft (a Version 5
  file) and Fade In's OSF 4.0 share; older names, Ids and Beat's keys read
  as the same categories.
- **FDX tags** (`FDXWriter.Tags`, `FDXWriter.LineTags`, `fdx.ParseTags`):
  `<TagData>` (categories, definitions, tags) and `TagNumber` on the
  tagged words' runs, as Beat writes them; read back with the words and
  their place in the line (`lex.LineTag`, `lex.TagPlain`, `lex.TagIndex`).
  Movie Magic Scheduling reads only headings and speaking characters from
  an FDX (its documentation); the tags are for Final Draft and the others.
- **Scene lengths in FDX** (`FDXWriter.Scenes`, `cli.Options.SceneLengths`,
  `-lengths`): each heading's `SceneProperties Page` and `Length` in
  eighths, as the PDF prints them.
- **Breakdown sheets and element reports** (`report.Breakdown`,
  `Report.Sheets`, `Breakdown.SheetTable`, `Report.ElementTable`,
  `report.WriteSheetsPDF`): a sheet per scene (number, INT/EXT, location,
  time, page, eighths, synopsis, cast by number, elements by category) as
  CSV and as a PDF in the classic layout; each element with its scenes.
  `Scene.Synopsis` is the scene's first synopsis.

## [1.10.0] - 2026-11-20

### New Features
- **Locked pages and A-pages** (`lex.PageLock`, `lex.LockedPage`,
  `pdf.PDFWriter.Lock`): once a script goes out, each page keeps its
  number and its first line. Text that grows past a locked page goes onto
  A-pages, named as A-scenes are (`lex.PageLabelBetween`: 12A, 12B; 12AA
  between 12A and 12B; 12aA between 12 and 12A, as Final Draft writes it);
  text that shrinks leaves the page short; a page whose text is gone gives
  its number to the page before ("12-13", as Scripto and Final Draft do).
  Pages after the last locked one count on (121). A page is anchored by
  words, not offsets: the scene it starts in, its first words and the words
  before it (`lex.PageAnchor(s)`, `lex.LocatePages`), so edits elsewhere,
  in any program, do not move it; it is found by its first words, else
  after the words before it (its first line edited), else it is gone.
  `pdf.PDFWriter.LockPages` locks the pages as printed (with a lock:
  locked again, its A-pages kept, Final Draft's Lock A Pages);
  `pdf.PDFWriter.Pages` lists the pages with their numbers and revisions.
  A lock carries its paper and CONTINUEDs, and the writer prints a locked
  script so. `PageLock.Text` / `lex.ParsePageLock` are its text form
  (`pages: locked, letter` and a line per page).
- **Revised pages with locks**: a changed page's A-pages are revised
  pages too, numbered so ("12A" in the header and in revised pages only);
  so is a "12-13" page, and a page before one whose first line is marked
  (a deletion at its foot).
- **Scene CONTINUEDs** (`pdf.PDFWriter.Continueds`, `-continueds`):
  "(CONTINUED)" under a page's last line when its scene goes on, and
  "CONTINUED:" (then "CONTINUED: (2)" …) at the next page's top, with the
  scene's number in both margins. A page keeps its last line for it.
- **Page numbers** are printed without a title page too (from the second
  page on), and a page no longer starts with a blank line.
- **FDX `<LockedPages>`**: `FDXWriter.Lock` writes each page found at its
  `Position` (characters of the text runs plus one per paragraph end, as
  Final Draft's own file has them; `LockLevel` and `LevelIndex` for an
  A-page are a guess); `fdx.ParseLockedPages` reads them back as anchors.
- **Command line**: `-lockpages` writes the lock of the script's pages
  (as printed; with `-lock`, locked again), `-lock file` keeps them in
  the PDF and the FDX. Programs: `cli.Options.Lock`, `.Continueds`,
  `cli.LockPages` and `cli.Pages` (the pages as the options print them).

## [1.9.0] - 2026-11-20

### New Features
- **Production revisions** (`lex.Line.Revision`, `lex.Revisions`,
  `lex.RevisionSet`): a line's revision set, printed in the PDF as an
  asterisk at the right margin (7.75" on Letter, as Final Draft; right of a
  numbered heading's number) on every line it prints on, and the changed
  pages' header on the page number's line ("Blue Revision – 20/11/26";
  `Revisions.Header` with `{set}` and `{date}`, `DateLayout`).
  `Revisions.PagesOnly` writes only the changed pages, numbered as in the
  whole script (`pdf.PDFWriter.RevisedPages` lists them;
  `pdf.ErrNoRevisedPages` when there are none); `Revisions.PageColors`
  tints them in their set's paper colour. `cli.Options.Revisions` passes
  them to the PDF and FDX writers.
- **The revision colours** in the WGA's order (`lex.RevisionName`,
  `lex.RevisionIndex`, `lex.RevisionColor`, `lex.RevisionPaper`,
  `lex.RevisionInk`): White, Blue, Pink, Yellow, Green, Goldenrod, Buff,
  Salmon, Cherry, then Second Blue … Second Cherry, Third Blue
  (Quote-Unquote's "The WGA script revision order"; Final Draft's knowledge
  base "Per the WGA west…", seen as a search snippet only).
- **FDX revisions**: `FDXWriter.Revisions` writes `<Revisions>` (Final
  Draft's attributes, its paper colours, a set named "Blue Rev. 20/11/26")
  and each revised paragraph's runs' `RevisionID`; no `<DeletedText>`.
  `fdx.ParseRevisions` reads a line's set from its runs' `RevisionID` (the
  latest) or a `<DeletedTextLocation>` in it (its Offset counts the text
  runs' characters and one per paragraph end), and the sets the text uses
  ("Blue Rev. (mm/dd/yy)" read as Blue).
- **`fountain.ParseLines`**: Parse with each row's `lex.Line.Source`, the
  text's line it was read from, so that what a caller knows per line (a
  revision) can go on the rows.

## [1.8.0] - 2026-11-20

### New Features
- **Page eighths** (`lex.Eighths`, `lex.EighthsOf`): a length of script in
  eighths of a page, written as productions write it (`4/8`, `1`,
  `1 3/8`, never reduced): to the nearest eighth, and at least one.
- **Scenes measured as printed** (`pdf.PDFWriter.Measure`, `pdf.Measure`):
  each scene's page and length, laid out as the PDF prints it (its rules,
  paper and pagination; nothing is written). A scene runs from its
  heading to the next one's; the room the pagination leaves at a page's
  foot is the scene's before it, the rest of a page after a forced break
  (`===`) no one's.
- **Production reports** (the `report` package, `cli.Report`, `-report`):
  scenes (number, heading with INT/EXT, location and time apart, page,
  eighths, who speaks, OMITTED kept as such), locations (their scenes,
  INT/EXT, times, eighths), characters (scenes, first and last, speeches,
  words, eighths; other names merged with `report.Options.Aliases`) and
  the cast list numbered by size (`Report.Cast`). Each is a `Table`,
  written as CSV in a fixed layout (`report.SceneColumns`, ...: a header
  row of the columns' keys, lists joined with `; `, CRLF) or as an HTML
  page to print (`report.WriteHTML`).

## [1.7.0] - 2026-11-20

### New Features
- **Scene numbers for a locked script** (`lex.NumberScenes`,
  `lex.NumberLabels`, `lex.SceneNumberBetween`, `-number`,
  `cli.Options.Number`): number the scenes without a number, on from the
  last whole number (`count`, what Accolade's export did), or keeping
  every number there is, as a shooting script must. A scene between 10
  and 11 is then 10A, the next 10B, one between 10A and 10B 10AA, and
  between 10 and 10A 10aA (`lex.NumberAfter`, the default, Final Draft's
  form); or A11, B11, AB11 between A11 and B11, AA11 between 10 and A11
  (`lex.NumberBefore`, Movie Magic Screenwriter's "flipped"). A scene
  before the first is A1, one after the last the next number, and no
  number is given twice (OMITTED scenes keep theirs).
  `lex.WithSceneNumber` puts `#n#` in a heading, before its notes.
- **Omitted scenes** (`fountain.OmitScene`, `fountain.RestoreScene`,
  `lex.OmittedNumber`): the heading becomes `.OMITTED #12#` and the scene
  goes into a `/* Omitted 12 … */` boneyard under it, to come back whole.
  FDX writes it as a Scene Heading `OMITTED` with `Number="12"`.

### Fixed
- FDX wrote Fountain's boneyard (`/* … */`, notes blocks too) as General
  paragraphs, which Final Draft prints. It is left out now, as in print.

## [1.6.0] - 2026-11-19

### New Features
- **Text color over the whole block** (`-colorstyle text -colorscope block`,
  `text+block`): a scene's or section's lines in its color, not only its
  heading, in HTML, PDF, DOCX and ODT.
- **Page breaks as screenplay software makes them** (`-pagination`,
  `rules.Pagination`, `cli.Options.Pagination`; on by default): scene
  headings, sections, character names and parentheticals are never the
  last thing on a page and keep the first two lines of what follows; a
  transition never starts a page; dual dialogue is never broken; a
  paragraph keeps two lines on each side of a break (widow/orphan
  control); in the PDF a speech breaks after a sentence with `(MORE)` and
  `NAME (CONT'D)`. Word gets keepNext/keepLines/widowControl, Writer
  keep-with-next/keep-together/widows/orphans, printed HTML
  break-after/break-inside/orphans/widows. `-pagination none` (or a
  hidden `pagination` rule) prints as before; `keep`, `widows` and `more`
  can be chosen apart.
- `pdfin.Pages`: the text lines of each page of a PDF, where printed.
- **PDF as prose** (`pdfin.ReadProse`, `pdfin.ReadProseFile`): a story,
  article or book read as running text, not as a screenplay, OCR included.
  Lines wrapped by the page become paragraphs again (a hyphen-broken word
  joined), headings are told by their size or a bold line of their own,
  bulleted and numbered lists kept, page numbers and running headers and
  footers left out (a chapter's number in large or bold type is kept). In
  scans, a line's size is Tesseract's line height (hOCR `x_size`).
  `Prose.Markdown` writes it as Markdown.

### Fixed
- Text in the PDF's standard fonts (Times, Helvetica), which need not
  give their glyphs' widths, was measured at Courier's: lines seemed to
  end too far right. Their widths come from gofpdf's metrics now.
- A dot in a folder's name no longer reads as a scene language: the
  formats and languages come from the file's own name
  (`script.en.fountain`), not its path. Scene headings in such folders
  were read as action.
- A scene heading, character name or parenthetical could be the last line
  of a PDF page; in docx and odt the heading kept only with the blank
  paragraph after it.

## [1.5.1] - 2026-11-15

### Fixed
- **Colour bands and bars over every element**: the background band and the
  bar were placed by the action's margins only, so in formats that put names
  or dialogue further out (Inverse, the radio and stage play formats) text
  stood outside the band and left of the bar. They now span the leftmost to
  the rightmost margin of the elements.

## [1.5.0] - 2026-11-15

### New Features
- **Color blocks apart, and patterns** (`-colorpatterns`, `+patterns`):
  a gap (the blank line) between two color blocks, also of the same
  color, in every format (`lex.ColorBlocks`, `Line.Block`); each of
  Beat's twelve colors' bars in a pattern of its own, others by a hash
  (`lex.BarPattern`), Word's and ODF's nearest border styles in docx
  and odt.
- **Color blocks** (`-colorscope heading|block`, `-colortint`; schemes
  such as `marker+block`, `background+block`, `line+tint`): a marker or
  background over the whole scene, or section, with the color, page by
  page, and a light tint under a line's block, lighter than a background
  but as readable (`lex.ColorScheme`, `lex.ColorTint`).
- **Color styles** (`-colorstyle line|marker|background|text`, the
  `notecolors` rule's `Style`): a bar beside the scene down every page
  (the default), a marker behind the heading, a band across it, or the
  text. Colors on sections color their scenes. Tints and text colors
  keep WCAG contrast (`lex.ColorFor`): 7:1 for black text on a tint,
  4.5:1 for colored text, 3:1 for the bar.
- **Colors** (`-colors`, a set's `notecolors` rule; off by default):
  scene headings and sections in the color their notes give them, as
  Beat reads and shows them, in PDF, HTML, DOCX and ODT. FDX reads and
  writes Final Draft's scene color (SceneProperties Color).
- **Tags** (`-tag`, `lex.FilterTags`): only the scenes and sections
  with a note such as `[[London]]` (or a color), with all under them.
- **Sides** (`-character NAME -sides full|cue|isolated`, `-cuewords`,
  `lex.Sides`): a character's scenes, their speeches after their cues,
  or their speeches only.
- **Layout** (`layout` package): a screenplay laid out as printed lines,
  for front-ends that draw it themselves (Accolade's preview, a
  terminal): each line with its column and width in characters (Courier,
  ten to the inch, from the action's left margin), its alignment, and
  its text as styled spans (Fountain's bold, italic and underline); page
  breaks; the title page's title block and meta block; dual dialogue as
  two columns. The margins, alignment, style, prefix, postfix and hidden
  elements come from the same rules as the PDF writer.
  `Line.Padded()` gives the line as fixed-width text.
- **Presets** for other kinds of scripts, with `-e`: `stageplay` and
  `stageplay-uk`, `musical`, `radio` (BBC radio drama, hoorspel),
  `comic` (full script), `bd` (bande dessinée), `transcript`, and
  `inverse`, the "Character: text" layout from 2022. They need no
  configuration file (one can still override them by name); `-presets`
  lists them, `rules.Presets` gives them to programs. Each is a small
  TOML file in `rules/presets`. docs/presets.md describes them and how
  to write each kind of script, with examples in `examples/` (also a
  TV episode).
- **PDF input** (`pdfin` package, `-from pdf` or a `.pdf` input): a
  printed screenplay read back into a script, from Lexington, Final
  Draft, Highland, Fade In, Word or LibreOffice PDFs. The elements are
  told apart by their indents from the action's margin (dialogue,
  parentheticals, names, transitions at the right, centred text), blank
  lines by the gaps, wrapped lines joined; dual dialogue, scene numbers
  in the margins, the title page and forced page breaks are recognised;
  page numbers, (MORE), CONTINUED and the repeated NAME (CONT'D) at page
  breaks are dropped. Scanned scripts are read with OCR
  (`pdfin.Options.OCR`, hOCR in): the page images (JPEG, fax, plain
  pixels) go to tesseract when it is installed (`pdfin.Tesseract`, the
  command line uses it), or to any other OCR, such as the built-in
  WebAssembly one of the `ocrwasm` module. Scans in JBIG2 (most
  scanned PDFs) are decoded by a pure Go port of pdf.js's decoder
  (`internal/jbig2`, Apache 2.0), checked pixel for pixel against
  poppler and against the SerenityOS conformance files.
  The PDF reading is github.com/ledongthuc/pdf, vendored in
  internal/pdf with fixes: fonts that map the code 0x0A, 40-bit RC4
  encryption (old PDFs that only restrict copying), text drawn in form
  XObjects. A page that cannot be read is left out (and reported)
  instead of failing the import; running headers and footers are
  dropped.
- The **Fountain writer** keeps dual dialogue (`^`), centred text
  (`>...<`) and transitions that need `>`; dual dialogue lost its `^`
  and gained blank lines.
- **DOCX and ODT** written natively (`office` package), no pandoc
  needed: every element a named paragraph style (Scene Heading,
  Character, Dialogue, ...) for easy restyling in Word or LibreOffice,
  the rules' margins, alignment and styles, scene numbers at the right
  margin, page numbers from the first page after the title page, dual
  dialogue as a borderless table. `-page letter|a4|a5` sets the paper;
  a preset can suggest one (the musical: A5, a folded booklet), and
  positions scale with the page's width. The PDF takes `-page` too
  (`PDFWriter.Page`).
- `layout.Paragraphs`: the printed elements before they are broken into
  lines, for writers that leave that to a word processor.
- Toki Pona scene headings (`tok`).

### Fixed
- **Hidden lines leave no blank lines**: sections, synopses, the
  boneyard and notes, where a format does not print them, are "ignored
  completely" as Fountain has it: blank, section, blank prints one blank
  line, a run of them the same, none at the top of the script or a page
  (`lex.Printable` takes the format's `hides`, `rules.Set.Hides`).
  Formats that print sections keep them and their blank lines.
- **Color bars beside text only**: a block's bar, band or marker is
  drawn on a page from its first line of text there to its last, never
  on into blank lines at a page's foot or top, nor as a stub beside
  nothing; in the PDF following where the text lands (it may break over
  pages), in HTML, DOCX and ODT none on blank lines at a block's or a
  page's edge (`lex.TrimBlocks`).
- **Emphasis** follows fountain.io exactly, in every writer, through one
  reader (`lex.Emphasis`): bold, italics and underlining mix and nest
  (`_**THE END**_`), `\*` is an asterisk, a marker with a space on its
  inner side is text (`He dialed *69 and then *23`), nothing goes over
  a line break, and an underscore within a word is text. The PDF no
  longer reads a `<` in emphasised text as a tag.
- **FDX** is read as Final Draft shows it: in capitals where it says
  AllCaps (`edward (v.O.)` is `EDWARD (V.O.)`), with scene numbers
  (`#12A#`), script notes and hidden text as Fountain notes, Final
  Draft 13's outline, summaries and notes as sections, synopses and
  notes, act breaks centred. Literal asterisks and underscores are
  escaped. Written FDX has scene numbers, the paragraph types' settings
  and notes as script notes.
- **Fountain notes** (`[[...]]`) are never printed any more, in a line
  (a heading, action, dialogue, a section, a synopsis) or on a line of
  their own, over several lines too (an unclosed `[[` is left as it
  is). They stay in the text: Fountain and lex written back keep them.
  `lex.StripNotes` takes them out, every writer prints through
  `lex.Printable`, and `lex.SceneNumber` and `lex.SectionText` leave
  them out, so `INT. HOUSE - DAY [[red]] #12#` is still scene 12.
  `lex.NoteColor` gives a heading's, section's or synopsis's color the
  way Beat reads it (`[[COLOR BLUE]]`, `[[red]]`, `[[#3366ff]]`).
  Lines of notes print only with a `printnotes` rule; the old `note`
  rule is gone from the defaults. Fountain written back now keeps the
  `=` of a synopsis, and Markdown no longer prints the boneyard.
- **Dual dialogue** in PDFs was printed too far left: the dialogue of
  the left column started at 1" from the page's edge, left of the 1.5"
  margin. The columns are now those of afterwriting, Better Fountain and
  screenplain: 2.5" wide at 2" and 5", with the dual rules' left
  margins counted from 1" (dialogue at the column's edge, paren 0.3",
  speaker 0.5" in). Lines wrap within their column (the speaker and
  paren no longer run 0.3-0.5" past it, and styled lines no longer wrap
  at the page's margin across the other column), and the block is
  followed by the usual blank line only.
- **Scene numbers** (`INT. HOUSE - DAY #1A#`) are printed in both
  margins of the PDF, as in other screenwriting software, instead of as
  part of the heading; the layout has them as `Line.SceneNumber`.
- **Sections** print without their `#`s.
- A section or forced scene heading with a colon right after the title
  page (`# Part 1: the kitchen`) was taken for a title page field.
- Fountain's **boneyard** (`/* ... */` on lines of their own) was
  printed as action. It is now kept as `boneyard` lines, written back by
  the Fountain writer and hidden by the others.
- An all-caps line in a speech (`GOTCHA!`, `KRAKOOM!`) was taken for
  a new character instead of dialogue.
- The layout counts from the set's leftmost margin, so elements left of
  the action's (radio's names) no longer give negative indents.
- An unknown `-e` set is an error instead of an empty PDF.
- **Fonts** in the PDF: the rules' `Font` was ignored (always Courier).
  Now Helvetica (Arial), Times and Courier name the PDF's standard
  fonts, for Western European text (a line with other characters stays
  in Courier Badi), and a `.ttf`/`.otf` path loads that font with its
  -Bold/-Italic/-BoldItalic siblings. The default lyrics are in
  Helvetica, as the default rules always had it.

## [1.4.0] - 2026-10-03

### New Features
- **Real bold in PDFs**: the font package now bundles Courier Badi Bold and
  Bold Italic (`font.CourierBadiBold`, `font.CourierBadiBoldItalic`), so
  scene headings and other bold text are bold instead of regular.

### Changed
- Courier Badi updated to v1.010 for all four styles (same metrics,
  13 more glyphs, smaller files).

## [1.3.0] - 2026-10-03

### New Features
- **FDX writer**: Final Draft documents now have `DocumentType="Script"`, a
  `<TitlePage>` (title, credit and author centred, other fields below),
  `<DualDialogue>` for dual dialogue, Final Draft style names
  (`Italic`, `Bold+Italic`), `Alignment="Center"`, page breaks
  (`StartsNewPage`) and `Lyrics` paragraphs. Blank lines between blocks no
  longer become empty paragraphs; extra blank lines are kept. The default
  output is written by the XML encoder; custom templates still work.
- **FDX reader**: `fdx.ParseWithError` reports invalid files (the CLI now
  says so instead of writing an empty script); the title page, dual
  dialogue, bold/italic/underline, centred text, page breaks and lyrics are
  read back, and blank lines are added between blocks for Fountain output.
  Fountain -> FDX -> Fountain round trips are tested on all test scripts.

### Bug Fixes
- **Fountain title page**: `FADE IN:` after the title page or opening a
  script is no longer dropped as an empty title field; a scene heading with
  a colon no longer starts a title page; indented lines continue a field's
  value (multi-line Title, Contact); leading blank lines no longer make an
  empty first page.
- **Fountain parser**: indented parentheticals are parentheticals, not
  dialogue; indented forced transitions and centred text (`> FADE OUT.`)
  are recognised; `NAME ^` pairs only with the dialogue block directly
  before it.
- **PDF**: title page fields other than title, credit and author (Contact,
  Draft date, ...) are printed instead of hidden.
- **HTML**: script text is escaped (a `<` or `&` broke the page); the whole
  title page is shown; no empty page after the title page.

## [1.2.1] - 2025-07-09

### Bug Fixes
- **Title Page Processing**: Fixed title page element processing to work without explicit TypeTitlePage marker
  - Title and author metadata now correctly extracted for epub/pandoc output from all fountain files
  - Added support for additional title page elements: Source, Contact, Draft date, Notes, Copyright
  - Removed dependency on inTitlePage state that was preventing some fountain files from working correctly
  - Ensures proper metadata extraction for pandoc-based outputs (epub, mobi, docx, etc.)

## [1.2.0] - 2025-07-09

### New Features
- **Enhanced Markdown Writer**: Improved dialogue block formatting with blockquote markers
  - Dialogue elements (speaker names, dialogue lines, parentheticals) are now grouped in blockquotes
  - Action lines remain separate from dialogue blocks for better visual distinction
  - Uses `> ` prefix for all dialogue elements creating clear visual hierarchy

### Bug Fixes
- **Dual Dialogue HTML**: Fixed width styling in dual dialogue HTML output (removed double %% in CSS)

### Testing
- **Complete Test Coverage**: Added comprehensive test suite for markdown writer
  - Tests for dialogue block formatting, dual dialogue, inline markup processing
  - Edge case handling for empty screenplays, title pages, and special elements
  - 100% test coverage for all markdown writer functionality

## [1.1.0] - 2025-07-06

### Major Features
- **Complete Inline Markup Support**: Fixed and enhanced inline formatting across all output formats
  - Fountain-style markup (`**bold**`, `*italic*`, `_underline_`, `***bold-italic***`) now works consistently
  - Fixed critical regex capture group issue that was preventing markup conversion
  - All output formats (HTML, LaTeX, PDF, Markdown, FDX) now properly process inline formatting

### New Features
- **FDX Inline Formatting**: Added complete inline markup support for Final Draft XML format
  - Bold text: `**bold**` → `<Text Style="Bold">bold</Text>`
  - Italic text: `*italic*` → `<Text AdornmentStyle="-1">italic</Text>`
  - Underline text: `_underline_` → `<Text Style="Underline">underline</Text>`
  - Multiple Text elements for complex formatting within paragraphs

### Fixed
- **Regex Capture Groups**: Changed all `$1` references to `${1}` in regex replacements
  - LaTeX Writer: Now generates proper LaTeX commands (`\textbf{}`, `\textit{}`, `\underline{}`)
  - HTML Writer: Now generates proper HTML tags (`<b>`, `<i>`, `<u>`)
  - PDF Writer: Now processes HTML-style markup correctly
  - Markdown Writer: Now preserves Markdown formatting with HTML fallback for underline
- **FDX Writer**: Previously showed raw fountain markup, now generates proper Final Draft styling
- **Template Processing**: Fixed placeholder approach in LaTeX to prevent conflicts with escaping

### Enhanced
- **Code Quality**: Refactored complex functions to reduce cyclomatic complexity
- **Linting**: Fixed all linting issues including line length and complexity warnings
- **Testing**: Comprehensive testing across all output formats with various markup combinations

### Technical Improvements
- Consistent inline markup processing across all writers
- Proper XML attribute handling in FDX format
- Enhanced template readability with proper line breaks
- Better error handling and code organization

## [1.0.5] - 2025-07-05

### Fixed
- **HTML Template Issues**: Fixed whitespace control and unknown element handling
  - Added proper whitespace control using `{{- .Contents -}}` for all content elements
  - Unknown elements are now properly ignored instead of displaying debug information
  - Improved HTML output readability with strategic newlines for easier debugging
- **Template Consistency**: HTML template now follows same patterns as LaTeX template fixes
- **Debug Output**: Enhanced HTML template structure for better debugging experience

### Technical Improvements
- Cleaner HTML output with proper line breaks and formatting
- Consistent whitespace handling across all HTML elements
- Better template debugging capabilities without affecting functionality
- All existing tests continue to pass with no functionality changes

## [1.0.2] - 2024-01-06

### Fixed
- **Go Version Compatibility**: Updated from Go 1.24 to Go 1.23 for better ecosystem compatibility
- **Code Quality Issues**: Fixed all golangci-lint errors and warnings
  - Resolved 18 errcheck issues by properly handling error return values
  - Fixed 7 staticcheck issues by using switch statements instead of if-else chains
  - Removed 1 unused function from fountain/parse.go
- **Error Handling**: Improved error handling in deferred file operations
- **CI/CD Compatibility**: Updated GitHub Actions workflows to use Go 1.23

### Technical Improvements
- Enhanced golangci-lint configuration for modern linter versions
- Better error propagation in temporary file handling
- Improved code maintainability with proper error checking
- All tests continue to pass with no functionality changes

## [1.0.1] - 2024-01-06

### Changed
- **Code Quality**: Significantly reduced cyclomatic complexity across the codebase
  - Refactored `main()` function from complexity 33 to <10 by breaking into focused functions
  - Refactored `fountain.Parse()` from complexity 25 to <10 using state-based approach
  - Refactored `fountain.Write()` from complexity 21 to <10 using helper functions
  - Improved code maintainability and readability without changing functionality
- **Function Organization**: Split large functions into smaller, single-purpose functions
  - `parseFlags()`, `setupIO()`, `parseInput()`, `convertOutput()` for main functionality
  - State-based parsing with `ParseState` struct for fountain parsing
  - Helper functions for each output type in fountain writing

### Technical Improvements
- Better separation of concerns with focused, testable functions
- Improved error handling with clearer error propagation
- Enhanced code structure following Go best practices
- All existing functionality preserved with 100% test compatibility

### Fixed
- **Go Report Card**: Addressed gofmt complexity warnings
- **Code Maintainability**: Reduced technical debt from high-complexity functions

## [1.0.0] - 2024-01-06

### Added
- **Dual Dialogue Support**: Complete implementation of side-by-side dual dialogue formatting
  - PDF output with proper column positioning using industry standards
  - HTML output with table-based dual dialogue structure
  - Support for parentheticals within dual dialogue
  - Multiple dual dialogue blocks within a single screenplay
- **Modern Go Features**: Updated to Go 1.24 with modern language features
  - Generic utility functions (Filter, Map, Find, Contains, Unique, GroupBy)
  - Type aliases and constants for better code readability
  - Enhanced error handling with better context and wrapping
  - Context support for graceful shutdown
- **Enhanced Output Formats**:
  - HTML-to-PDF conversion via wkhtmltopdf
  - LaTeX-to-PDF conversion via pdflatex/xelatex/lualatex
  - Configurable HTML writer with industry-standard margins
  - Improved LaTeX writer with dual dialogue support
- **Font Management**: Modern font loading using Go's embed directive
- **Configuration System**: Enhanced TOML configuration with validation
- **Version Information**: Added `-version` flag to display build information
- **GitHub Workflows**: CI/CD pipelines for testing and releasing

### Fixed
- **Dual Dialogue Rendering**: Fixed overlapping columns in PDF output
- **Title Page Detection**: Proper handling of files with and without title pages
- **Scene Heading Parsing**: Improved scene detection logic using generic utilities
- **Error Handling**: Better error messages with context throughout the application
- **Memory Management**: Fixed slice allocation issues in generic utilities

### Changed
- **Go Version**: Updated minimum requirement to Go 1.24
- **Dependencies**: Updated all dependencies to latest versions
- **Code Structure**: Modernized codebase with generics and improved patterns
- **Testing**: Enhanced test coverage with comprehensive dual dialogue tests
- **Documentation**: Updated README with improved installation and usage instructions

### Removed
- **Deprecated Code**: Removed old bindata.go in favor of embed.go
- **Legacy Patterns**: Replaced outdated error handling patterns

### Technical Improvements
- Industry-standard dual dialogue column positioning (left: 1.5"-3.5", right: 4.5"-6.5")
- Proper X/Y coordinate positioning instead of margin manipulation in PDF output
- Type-safe configuration handling with validation
- Context-aware operations with signal handling
- Comprehensive test suite with 100% coverage for critical functionality

### Performance
- Optimized dual dialogue buffer handling using generic utilities
- Improved error propagation with proper error wrapping
- Enhanced template rendering with better error context

---

This release represents the first stable version of Lexington with complete dual dialogue support and modern Go features. The application now meets professional screenplay formatting standards and provides a solid foundation for future enhancements.