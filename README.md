# Lexington v1.0 - Professional Screenplay Formatter

[![CI](https://github.com/LaPingvino/lexington/actions/workflows/ci.yml/badge.svg)](https://github.com/LaPingvino/lexington/actions/workflows/ci.yml)
[![Release](https://github.com/LaPingvino/lexington/actions/workflows/release.yml/badge.svg)](https://github.com/LaPingvino/lexington/actions/workflows/release.yml)
[![License](https://img.shields.io/github/license/lapingvino/lexington)](LICENSE)

**Lexington** is a professional command-line tool for screenwriters that converts between multiple screenplay formats with industry-standard formatting. Built with modern Go and featuring comprehensive dual dialogue support.


**Try it in your browser:** [lapingvino.github.io/lexington](https://lapingvino.github.io/lexington/) converts a screenplay without installing anything; your script stays on your computer.

## 🎯 Key Features

- **✅ Dual Dialogue Support**: Perfect side-by-side formatting in PDF and HTML
- **📄 Multiple Formats**: Fountain, FDX (Final Draft), LEX, PDF, HTML, LaTeX, EPUB
- **🎨 Professional Output**: Industry-standard margins and typography
- **⚙️ Highly Configurable**: Customize fonts, margins, and styling
- **🌍 International**: Multi-language scene heading support
- **🚀 Modern Codebase**: Built with Go 1.24, featuring generics and robust error handling

## Installation

Make sure to have Go installed first and then run:

```bash
go install github.com/lapingvino/lexington@latest
```

This installs Lexington to your go/bin directory. If this directory is in your execution path, you can then run it directly.

## Basic Usage

Convert a Fountain screenplay to PDF (multiple options):
```bash
# Direct PDF generation (built-in, supports dual dialogue); the shortest form,
# each format told by the file's name (else the input's content)
lexington inputfile.fountain outputfile.pdf
# the same with -i and -o (not mixed with plain files)
lexington -i inputfile.fountain -o outputfile.pdf -to pdf

# HTML to PDF (requires wkhtmltopdf, preserves CSS styling)
lexington -i inputfile.fountain -o outputfile.pdf -to htmlpdf

# LaTeX to PDF (requires LaTeX, high-quality typesetting)
lexington -i inputfile.fountain -o outputfile.pdf -to latexpdf
```

Convert to HTML with custom configuration:
```bash
lexington -i script.fountain -o script.html -to html -config custom.toml
```

For a complete overview of command line options:
```bash
lexington -help
```

## Features

- **Format Support**: Fountain, Final Draft (FDX), LEX, PDF, HTML, EPUB, LaTeX
- **Multiple PDF Options**: Direct PDF, HTML-to-PDF, LaTeX-to-PDF conversion
- **Dual Dialogue**: Proper formatting of simultaneous character dialogue in HTML and direct PDF
- **Configurable Styling**: Customize margins, fonts, and layout through configuration files
- **International Support**: Scene headings in multiple languages
- **Pandoc Integration**: Seamless ebook generation with proper metadata
- **Professional Formatting**: Industry-standard screenplay layout

### Reading Fountain: the spec's rules

Lexington reads Fountain as the spec ([fountain.io/syntax](https://fountain.io/syntax))
has it. Since 1.19 that includes four points where it used to read more
loosely:

- a scene heading needs a blank line before it (`INT. HOUSE` right under
  action is action);
- so does a character cue (a line in capitals right under action is
  action);
- a transition is in capitals, ends in `TO:` and has blank lines around
  it (`cut to:` and `... and DISSOLVE TO:` at the end of action are
  action);
- a line of two spaces within dialogue keeps the speech going, as a blank
  line of it.

Forced elements (`.` `@` `>` `!` `~`) are what they say anywhere, and the
scene words are still each language's from the configuration. A script
that relied on the old reading can be written so that it reads as it did,
with blank lines and force marks added (`fountain.KeepLoose`; Accolade's
*Keep It As It Was*):

```bash
lexington -keeploose -i old.fountain -o kept.fountain   # says how many places changed
lexington -loose -i old.fountain -o old.pdf              # or read it the old way once
```

The Fountain writer writes what the spec reads back the same: a heading
or a name without a blank line before it gets its force mark, a
transition without blank lines around it `>`.

### A clean copy: -strip

`-strip` takes out of a Fountain script what one hands on without — for
another app, an actor, a reader — and writes the rest as it was, byte for
byte (with `-to pdf`, `html`, … it converts what is left). It always takes
out Accolade's data: its end blocks (`/* Notes */` and, before it,
`/* Source */`, `/* Adaptation */`, `/* Production */`, `/* Characters */`,
`/* Fix-Up */`),
the language line after them (`/* Language: eo */`) and the links to the
source story (`[[from: …]]`, wherever they are, in a boneyard kept too). It takes a comma list of presets and
options, which add up:

| key | takes out |
|---|---|
| `clean` | Accolade's data only |
| `reading` | a reading copy: `notes` and `outline` |
| `writer` | the writer's version: `numbers` and `omitted` |
| `notes` | every note `[[…]]` |
| `boneyard` | every boneyard `/* … */` as the parser reads one (from a line starting with `/*` to the line that closes it), an omitted scene's kept text too |
| `numbers` | the scene numbers `#12#` |
| `omitted` | the OMITTED headings, with the scene kept under each |
| `titlepage` | the title page |
| `outline` | sections `#` and synopses `=` |

```bash
lexington -strip clean -i film.fountain -o film-clean.fountain
lexington -strip writer,notes -i film.fountain -o draft.fountain
lexington -strip reading -i film.fountain -o reading.pdf
```

A line that held only what went goes with it (and a blank line, where
two would be left); a line that would read as another element without it
gets its force mark, so what is left reads as it did. It never writes over
its input. In Go: `fountain.Strip(text, scenes, fountain.StripOptions{…})`,
`fountain.ParseStrip("writer,notes")`, `fountain.StripPresets`; where
Accolade's blocks are, `fountain.EndBlocks` (Accolade finds them with it
too).

## Configuration

Lexington supports extensive customization through TOML configuration files. You can control margins, fonts, alignment, and styling for all screenplay elements.

### Using Configuration Files

Create a custom configuration file:
```bash
lexington -dumpconfig -config my_config.toml
```

Use your configuration:
```bash
lexington -i script.fountain -o script.html -to html -config my_config.toml
```

### Configuration Options

The configuration file allows you to customize:

- **Margins**: Left and right margins for all elements (in inches)
- **Fonts**: Font family and size for each element type
- **Alignment**: Left, center, or right alignment
- **Styling**: Bold, italic, or normal text styling
- **Dual Dialogue**: Specialized formatting for simultaneous dialogue

### Example Configuration

```toml
[Elements.default.action]
Left = 1.5            # Left margin in inches
Right = 1.0           # Right margin in inches
Font = "CourierPrime"
Size = 12.0
Align = "L"           # L=Left, R=Right, C=Center

[Elements.default.speaker]
Left = 3.7            # Character names positioned at 3.7"
Right = 1.5
Font = "CourierPrime"
Size = 12.0

[Elements.default.dialog]
Left = 2.5            # Dialogue indented from character names
Right = 1.5
Font = "CourierPrime"
Size = 12.0
```

### Languages: scene headings, cues, transitions

Each language in `[Languages]` carries its whole definition: the words a
scene heading starts with and heading patterns with slots for other word
orders (`"{n}. {place} {io}/{time}"`, as German and French scripts write
them), the rules a character's cue is read by (`caps` for English;
`list`, `parenthetical`, `extension`, `repeat` for languages without
capitals; `forced` @ always), what a transition ends in or is, the title
page's keys and the direction. A file named `script.eo.fountain` is read
with Esperanto's, a `.hekaya` file with Hekaya's fixed Arabic rules. See
[docs/LANGUAGES.md](docs/LANGUAGES.md) for every rule, the research behind
the default languages, and Hekaya.

```toml
Standard = "en"

[Languages.zh]
Scenes = ["内景", "外景", "内景/外景", "外景/内景"]
Headings = ["场景{n} {place} {time} {io}", "{n}. {place} {time} {io}"]
Settings = ["内", "外"]
Times = ["日", "夜"]
Cues = ["forced", "list", "parenthetical", "extension", "repeat"]
Extensions = ["画外音"]
Shape = { Chars = 6, Ends = "。！？…，" }
Repeat = 2
TransitionEnds = [" TO:"]
Transitions = ["淡入", "淡出"]
```

`Standard` names the language whose words are standard Fountain's
(`"en"`): a script in another language is a dialect of Fountain, and
`fountain.ForceIn(text, from, to)` writes it so that the standard rules
read the same elements (force marks — `.EN. DOMO`, `!INT. is English`,
`@张三`, `> - قطع -` — only where needed, the lines otherwise as written);
`fountain.UnforceIn` takes those marks back. Configuration files with the
older `[Scenes]`, `[Matchups]` and `[Directions]` tables are read as
before.

### A language's direction

Every line runs the way its own characters say: a direction mark at its
start (RLM U+200F, LRM U+200E), or else its first strong letter
(Hebrew and Arabic right to left, Latin left to right), so a file mixing
directions stays mixed. Only a line with neither — `…`, `2.`, a bare
number — takes its language's direction, from `[Directions]`: `"rtl"` or
`"ltr"`, each language's `Direction` (or a title page's `Direction:`, as
Hekaya writes it). The default configuration has `ar` and `he` right to
left, and a language one's configuration does not name keeps the
default's; a custom language sets its own. The HTML
and PDF writers use it (`-scenein` names the script's language).

The PDF prints a right-to-left line from the right: its element's
margins swapped and its alignment turned (a transition on the left, a
scene's CONTINUEDs, a dual dialogue's first speaker on the right), its
words in reading order by the Unicode bidi algorithm, numbers and Latin
in it kept, brackets turned. Hebrew prints this way, points and all
(Courier Badi has them; without shaping each point stands at its default
place). Arabic is shaped through Courier Badi's own OpenType features
(joining forms, lam-alef; go-text's HarfBuzz port), so it prints joined;
other scripts whose letters join (Syriac, N'Ko, Mongolian…) are not
shaped: for them Lexington warns and suggests printing the HTML from a
browser.

Devanagari (Hindi, Marathi, Nepali, Sanskrit) is shaped too, through
Courier Badi's features (from Noto Sans Devanagari): conjuncts, half
forms, the i-matra before its consonant, reph and rakar. Courier Badi
keeps it monospaced by cell — a letter or a spacing vowel sign takes a
cell, a virama, a mark or a joiner none, as Accolade's editor counts —
and the PDF wraps a line by those cells. The glyphs are printed as
shaped, each run with its text as written (`ActualText`), so a copy, a
search, `pdftotext` and `-from pdf` read the words back.

```toml
[Languages.fa]
Direction = "rtl"   # a language of one's own
```

### Presets: stage plays, radio, comics and more

Besides the screenplay (`default`), Lexington has presets for other
kinds of scripts: US and UK stage plays, musicals, radio drama
(hoorspel), comics, bande dessinée, transcripts, and the "Character:
text" layout `inverse`. They work without a configuration file:

```bash
lexington -presets
lexington -e musical -i musical.fountain -o musical.pdf
```

See [docs/presets.md](docs/presets.md) for each, how to write such a
script in Fountain, and [examples/](examples) for a sample of every kind.

### Title page templates

Where each title page key goes, and how it looks, is a template's: placed by key, never by the order the keys
are written in. `standard` is Fountain's (Title, Credit, Author and Source centred, the rest at the foot, as
the language's `TitleCentred` has it); `bold` and `poster` (a picture above a big title) come with Lexington;
one's own are `[TitlePages.name]` in `lexington.toml`. A field whose value is `![alt](file.png)` prints the
picture, from beside the script or its bundle.

```bash
lexington -titlepages                       # the templates
lexington -i film.fountain -o film.pdf -titlepage poster
```

All of it: [docs/TITLEPAGE.md](docs/TITLEPAGE.md).

## Output Formats

### HTML Output
- Self-contained HTML files with embedded CSS
- Configurable styling and margins
- Proper dual dialogue table formatting
- Print-friendly layouts

### PDF Output
- **Direct PDF**: Professional screenplay formatting with dual dialogue support
- **HTML to PDF**: Uses wkhtmltopdf for CSS-styled output (requires wkhtmltopdf)
- **LaTeX to PDF**: Uses pdflatex/xelatex for high-quality typesetting (requires LaTeX)
- Industry-standard page layouts and proper spacing

### Showing the PDF's pages without a PDF reader

The PDF writers (the script's, and the reports' sheets, one-liner and Day
Out of Days) draw through `pdf.Drawer`, the few gofpdf calls they make. A
`pdf.Recorder` forwards each to gofpdf — which still measures, breaks and
places everything, so the PDF is the same byte for byte — and records
what landed on each page: text runs at their baselines, alignment and
cell margins resolved, the string as printed (right-to-left lines in
visual order, the Hebrew presentation forms), and boxes (fills, borders,
underlines, with their alpha and blend), in points. A program gets this
`pdf.Drawing` with the PDF in one pass (`PDFWriter.Record`,
`cli.Options.Record`, `report.DrawSheetsPDF` and the like) and can show
the pages itself: `pdf.Rasterizer` paints them as images in the fonts
the PDF embeds (the standard fonts, which it does not, in the Go fonts,
stretched to their width in the PDF). A test compares the painted pages
with pdftoppm's of the PDF itself, over the examples, the test inputs,
Hebrew, colours, revisions and the reports.

### Page breaks

Pages break as screenplay software breaks them (on by default, in the
PDF, docx, odt and printed HTML):

- **keep**: a scene heading, a printed section, a character name and a
  parenthetical are never the last thing on a page, but go over with the
  first two lines of what follows; a transition is never the first thing
  on a page, but goes with the line before it; dual dialogue is never
  broken.
- **widows**: a paragraph broken over two pages leaves at least two lines
  on each.
- **more**: a speech broken over two pages breaks after a sentence, with
  `(MORE)` below it and `NAME (CONT'D)` above the rest (PDF only; Word,
  Writer and browsers keep a speech whole on the next page instead).

`-pagination` chooses: `all` (the default), `none` (each page fills to
its foot), or parts joined with `+` (`keep+widows`). A format's settings
can do the same with a `pagination` rule (`Style = "keep+widows"`, or
`Hide = true` for none); programs set `cli.Options.Pagination`.
Forced page breaks (`===`) are kept as they are.

```bash
lexington -pagination none -i script.fountain -o script.pdf
```

### Reading PDFs

Lexington reads a printed screenplay back into a script: from its own
PDFs and those of Final Draft, Highland, Fade In, Word or LibreOffice.
It tells the elements apart by their indents, recognises dual dialogue,
scene numbers and the title page, and leaves out page numbers, (MORE)
and (CONT'D) at page breaks:

```bash
lexington -i script.pdf -o script.fountain
```

A scanned script (only images) is read with OCR when tesseract is
installed; programs can pass any OCR (`pdfin.Options`).

### Bundles: Accolade, Highland, TextBundle

A bundle is a script's text file (`text.fountain`, or `text.md`) with `info.json`, `assets/` and whatever
other programs keep beside it. It comes in two forms that hold the same files, a zip or a plain folder:

| Type | Zip | Folder |
|---|---|---|
| Accolade | `film.accolade` | `film.accolade/` |
| Highland 2 | `film.highland` | `film.highland/` |
| TextBundle | `film.textpack` | `film.textbundle/` |

Any bundle works as the input of every output format, and as an output that gets the script as Fountain:

```bash
lexington -i film.accolade -o film.pdf          # the script inside, printed
lexington -i film.fountain -o film.highland     # a new Highland file
lexington -i film.highland -o film.textbundle   # the same files, unzipped
lexington -i film.accolade -o film-folder.accolade -form folder
lexington check film.accolade                   # is it whole and readable?
```

- **Reading:** a zip and a folder are told apart by what is on disk. A `.highland` that is plain text, as
  some real ones are, reads as Fountain. Highland 2's zip is read and written as its maker and others
  describe it; no file saved by Highland 2 has been tried yet, nor whether Highland opens the folder form.
- **An `.accolade`'s own settings:** the language words and format it carries (`assets/lexington/`, the
  format named in `info.json`) are laid over the machine's settings for that script, as Accolade does:
  `lexington -i film.accolade -o film.pdf` prints it as it looks in Accolade. `-e` still picks another
  format. A format of Accolade's own it does not carry (TV series) prints as the screenplay.
- **Back to plain Fountain:** `lexington -i film.accolade -o film.fountain` gives the script as it is in
  the bundle, Accolade's own blocks (Production, Characters, Notes, …) and links included.
- **The text:** written into a bundle as it is, byte for byte, unless an option changes it (`-strip`,
  `-number`, `-tag`, …). Then Lexington writes it anew as Fountain.
- **Everything else is kept:** drafts, sources, sidecars, assets and files no one here knows. The output's
  own files are kept when it already exists, else the input bundle's.
- **The form:** a bundle is written back in the form it has. A new one gets its type's form (a `.textbundle`
  is a folder, the others are zips) unless `-form zip` or `-form folder` says otherwise.
- **Markdown and plain-text bundles** (Highland's Manuscript, Novel and other prose templates, and its Plain
  Text template's `text.txt`) can be repacked, but not printed.
- **Highland:** checked against real `.highland` files (`bundle/testdata/highland`). A zip holds one
  `Name.textbundle/` folder, named after the file on every save.
- **Highland's directives** (in any Fountain file, as Highland 2 does them): `{{INCLUDE: file}}` prints
  another file where its line is (relative to the including file; nested; a cycle or a missing file is a
  line in print and a warning in `lexington check`); `{{HEADER: …}}` and `{{FOOTER: …}}` ("Left | Centre |
  Right"; `%p` the page, `%pp` the last page, `%#` the section, `%none` off) replace the page number;
  `{{PAGENUMBER: 10}}` numbers from there; `{{SERIES}}` counts 1, 2, 3 (`{{SERIES: 5}}` sets it);
  `{{TIMESTAMP}}` is the day ("Sep 28, 2026"); `{{TOC}}` lists the sections with their pages. Markers
  `{{%m: …}}` and `{{REVISION}}` tags print nothing. The Fountain written keeps them all as written.
  `-header` and `-footer` give a header and footer to a script without its own. HTML and FDX have one
  header and footer for all pages (the script's first).
- **For programs:** the `bundle` package does all of this (`bundle.Read`, `bundle.Write`, `bundle.Check`).

The layout is described roughly, as a draft that will change, in
[Accolade's docs/FORMAT.md](https://github.com/LaPingvino/accolade/blob/main/docs/FORMAT.md).

### Other programs' formats: Fade In, Trelby, KIT Scenarist

`lexington IN OUT` (or `-i IN -o OUT`) converts any format Lexington reads into any it writes, each told
by its name, else (the input) by its content; the other flags go before, between or after the files, and
`IN` alone writes to standard output:

```bash
lexington script.pdf script.fountain                 # the shortest form
lexington -i script.pdf -o script.fountain           # any screenplay PDF, read back to Fountain
lexington -i film.fadein -o film.fdx                  # Fade In to Final Draft
lexington -i film.trelby -o film.kitsp                # Trelby to KIT Scenarist
lexington -i film.fountain -o film.fadein -form folder    # an unzipped .fadein
lexington -i film.fountain -o film.osf -osfversion 1.2    # an older Fade In's OSF
lexington -i download -o film.fountain                # no extension: told by its content
```

| Program | Read | Written | What it keeps |
|---|---|---|---|
| Fade In (`.fadein`, zipped or a folder; `.osf`) | OSF 1.2, 2.x, 3.0, 4.0 | OSF 4.0 (`-osfversion 2.0`, `1.2`) | all printed elements and emphasis, dual dialogue, scene numbers, page breaks, notes, a scene's synopsis; the title page |
| Trelby (`.trelby`) | formats 1–3 | format 3 | the text of each element and its type, notes, the title page; no emphasis, dual dialogue, scene numbers or page breaks |
| KIT Scenarist (`.kitsp`) | its projects (0.7) | a new project, as KIT 0.7.2 makes one | all elements and emphasis, dual dialogue, scene numbers and colours, folders as sections, notes as review comments, the title page's fields; no page breaks |

- **Fade In:** a `.fadein` is a zip holding the script as `document.xml`, in Fade In's Open Screenplay
  Format; like a `.highland` it can be a folder, and is written back in the form it has, what else is in it
  kept. OSF has no outline: a section (`# Act One`) and a synopsis not on a scene heading go in the next
  paragraph's note, which Fade In does not print either. The format is known from the OSF SDK's samples
  and checked on real `.fadein` files (OSF 3.0); no file Lexington wrote has been opened in Fade In itself
  yet. In 1.2, whose emphasis is tags in the text (`<b>`), a `<` or `>` of the text's own is written
  `&lt;`, a guess: no 1.2 file seen has one.
- **Trelby:** as Trelby's own `fileformat.txt` and loader describe it. Its act break is a first-level
  section; deeper sections and synopses are notes that start with `##` and `=`, which read back as what
  they were. A centred line is action, lyrics are dialogue.
- **KIT Scenarist:** a project is an SQLite database, read and written by Lexington itself (no SQLite
  library, so it works in the browser too). Written projects pass `sqlite3`'s integrity check; none has
  been opened in KIT itself yet. The title page maps as KIT's own Fountain import maps it (Credit is its
  genre, Draft date its year).
- **For programs:** `cli.ReadScreenplay` reads a file in any of these formats as the command line does;
  `cli.Inputs` and `cli.Formats` list what is read and written; the `osf`, `trelby` and `kitsp` packages do
  each format alone.

### Word and LibreOffice (DOCX, ODT)

Lexington writes `.docx` and `.odt` itself, without pandoc, ready to be
edited further: every kind of element is a paragraph style of its own
(Scene Heading, Action, Character, Dialogue, Parenthetical, ...), so
changing a style changes the whole script, and dual dialogue is a
borderless table. The margins, styles and alignment are those of the
element settings or preset, on US Letter, A4 or A5 (`-page`):

```bash
lexington -i script.fountain -o script.docx
lexington -e musical -i show.fountain -o show.odt   # A5, the musical's suggestion
lexington -page a4 -i script.fountain -o script.docx
```

### EPUB Output
- Metadata integration (title, author, etc.)
- Chapter structure preservation
- Compatible with most e-readers

## Colors, Tags and Sides

Fountain notes are never printed, but they can color and select:

```bash
# scene headings and sections in their notes' colors ([[red]],
# [[COLOR BLUE]], [[#3366ff]], as Beat writes them); off by default.
# A scene without a color takes its section's. -colorstyle chooses how:
#   line        a bar beside the scene, down every page (the default)
#   marker      a highlighter behind the heading
#   background  a band behind the heading, across the text
#   text        the heading in the color
# -colorscope block: a marker or background over the whole block, not
# just the heading; -colortint: a light tint under a line's block.
# A block is a scene, or a section whole when the color is on it (a
# scene's own color inside it wins for its span), page by page.
# Tints and colors keep black text readable (WCAG contrast).
lexington -colors -i script.fountain -o script.pdf
lexington -colorstyle marker -i script.fountain -o script.docx
lexington -colorstyle background -colorscope block -i script.fountain -o script.pdf
lexington -colortint -i script.fountain -o script.html   # line+tint
# Blocks are always apart: a gap (a blank line) between one block and the
# next, also of the same color. -colorpatterns gives each color's bar a
# pattern of its own (dashes, dots, double, ...), so that it does not
# rest on hue alone; docx and odt use their nearest border styles.
lexington -colorpatterns -i script.fountain -o script.pdf

# only the scenes and sections with a note [[London]] or [[New York]]
# (whole note, any case; a color is a tag too), with all under them
lexington -tag London -tag "New York" -i script.fountain -o london.pdf

# an actor's sides; the mode is needed:
#   full      every scene the character speaks in, whole
#   cue       their speeches, each after the speech before it
#             (-cuewords 5: only its last five words)
#   isolated  their speeches only
lexington -character EDWARD -sides cue -cuewords 5 -i bigfish.fdx -o edward.pdf

# number the scenes that have none: on from the last whole number
# (count), or as a locked shooting script, keeping every number there is:
# a new scene between 10 and 11 is 10A (then 10B; between 10A and 10B,
# 10AA), or with A11, A11 (then B11; between 10 and A11, AA11); one
# before the first is A1, one after the last the next number
lexington -number 10A -i script.fountain -o script.pdf
```

A scene number is a label: Fountain allows `#110A#`, `#1-A#`, `#I-1-A#`,
`#1.#`. The numbering (`-number`, `lex.NumberScenes`, `lex.Renumber`) reads
one scheme, the one it writes: a whole number with letters after it (10A,
10AA, 10aA) or capitals before it (A11, AB11). Every other label is fixed:
never renumbered, never given to another scene, and no neighbour (a new
scene between 10, PROLOGUE and 11 is 10A).

An omitted scene is written `.OMITTED #12#`, the scene itself in a
boneyard under it (`fountain.OmitScene`, `fountain.RestoreScene`); it
prints as OMITTED with its number, in FDX too. The text keeps one OMITTED
heading a scene; `-mergeomitted` prints a run of them as one,
`12-14 OMITTED 12-14` (PDF, HTML, DOCX, ODT and the rest; FDX keeps one
heading a number):

```bash
lexington -mergeomitted -i script.fountain -o script.pdf
```

Version 1.13.0 wrote such a run into the text as `.OMITTED #12-14#`; it is
read as its scenes (one heading for each `/* Omitted 13` block under it),
and `fountain.SplitOmitted` rewrites it so.

`lex.Renumber` numbers the scenes again by a strategy: which scenes get a
new number (the others are pinned), A-scenes folded into whole numbers or
kept lettered, OMITTED ones dropped or kept, labels reserved (sent out,
never given again). Free scenes count on from the pinned one before them
and take a letter where a pinned number forces it.
`lex.ParseNumberSet` reads a typed list against the script's labels,
`5, 12A, 20-24`: a range runs from scene to scene in the script's order,
whatever their labels.

Production reports come out as CSV: `-report scenes` (number, heading
with INT/EXT, location and time apart, page, length in eighths of a page,
who speaks), `locations`, `characters` or `cast` (numbered by size). The
pages are measured as `-e`, `-page` and `-pagination` print the PDF; a
scene is at least 1/8 and rounded to the nearest eighth
(`lex.EighthsOf`, `pdf.PDFWriter.Measure`, the `report` package):

```bash
lexington -report scenes -i script.fountain -o scenes.csv
```

Locked pages: `-lockpages` writes where each page starts (by its words,
so edits elsewhere do not move it); `-lock` keeps those pages after
edits, text that grows going onto A-pages (12A, 12B) and a page whose
text is gone giving its number to the one before (12-13). The FDX gets
them as `<LockedPages>`. `-continueds` prints the scenes' (CONTINUED)
and CONTINUED: where a scene breaks over pages:

```bash
lexington -i script.fountain -to pdf -lockpages -continueds -o pages.txt
lexington -i script.fountain -to pdf -lock pages.txt -o script.pdf
```

The title page stays, scene numbers are the script's own, and the
sections above a scene that is kept stay as headings. In a settings
file, a set's `notecolors` rule switches colors on, its `Style` the
color scheme: `line`, `marker+block`, `background+block`, `line+tint`,
`line+patterns`, ... Programs use
`lex.FilterTags`, `lex.Sides` and `rules.Set.WithColors`. In FDX, a
scene's color is Final Draft's scene color either way.

## Dual Dialogue

Lexington properly handles dual dialogue (simultaneous character speech) using the `^` syntax:

```fountain
ALICE
I can't believe this is happening.

BOB ^
Neither can I.

ALICE
What do we do now?

BOB
I guess we figure it out together.
```

This creates properly formatted side-by-side dialogue in HTML and PDF outputs.

## Testing

The `testdata/` directory contains comprehensive test files:

- `testdata/input/`: Source files for testing different features
- `testdata/output/`: Generated outputs (not tracked in git)

Run tests with various input files:
```bash
# Test basic screenplay formatting
lexington -i testdata/input/basic_screenplay.fountain -to html -o test.html

# Test dual dialogue
lexington -i testdata/input/dual_dialogue.fountain -to html -o dual.html

# Test complex formatting
lexington -i testdata/input/complex_screenplay.fountain -to pdf -o complex.pdf
```

## Using the command line from Go

The command line is the `cli` package, so other programs can offer the
same conversions (Accolade does, as a graphical Lexington):
`cli.Main(ctx, args)` runs it with arguments, `cli.Convert(ctx, opts)`
converts a file, `cli.Write(ctx, screenplay, opts)` writes a parsed
screenplay, and `cli.Formats` lists the output formats and the programs
some of them need. With `Options.Record` set, writing a PDF also gives
its pages as drawn (see *Showing the PDF's pages*).

## Contributing

Feel free to contribute! Areas where help is particularly welcome:

- Additional output formats
- Enhanced PDF page break handling
- More configuration options
- Bug fixes and improvements

## Dependencies

- **Go**: Required for building and running
- **pandoc**: Required for EPUB and some other output formats
- **wkhtmltopdf**: Optional, required for HTML-to-PDF conversion (`-to htmlpdf`)
- **LaTeX**: Optional, required for LaTeX-to-PDF conversion (`-to latexpdf`)

### Installing Dependencies

```bash
# Install wkhtmltopdf for HTML to PDF
# On Ubuntu/Debian:
sudo apt-get install wkhtmltopdf

# On macOS:
brew install wkhtmltopdf

# On NixOS or with nix:
nix-shell -p wkhtmltopdf

# Install LaTeX for LaTeX to PDF
# On Ubuntu/Debian:
sudo apt-get install texlive-latex-base texlive-fonts-recommended

# On macOS:
brew install basictex

# On NixOS or with nix:
nix-shell -p texlive.combined.scheme-medium
```

For a more complete overview of the command line options, use `lexington -help`.
