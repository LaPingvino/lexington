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
# Direct PDF generation (built-in, supports dual dialogue)
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
`/* Source */`, `/* Adaptation */`, `/* Production */`, `/* Characters */`),
the language line after them (`/* Language: eo */`) and the links to the
source story (`[[from: …]]`). It takes a comma list of presets and
options, which add up:

| key | takes out |
|---|---|
| `clean` | Accolade's data only |
| `reading` | a reading copy: `notes` and `outline` |
| `writer` | the writer's version: `numbers` and `omitted` |
| `notes` | every note `[[…]]` |
| `boneyard` | every boneyard `/* … */`, an omitted scene's kept text too |
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

### Scene heading languages

`[Scenes]` lists, per language, the words a scene heading starts with
(`en = ["INT", "EXT", …]`, `eo = ["EN.", "EKST", …]`); a file named
`script.eo.fountain` is read with Esperanto's. `Standard` names the
language whose words are standard Fountain's (`"en"`): a script in
another language is a dialect of Fountain, and `fountain.Force(text,
from, to)` writes it so that the standard words read the same elements
(force marks — `.EN. DOMO`, `!INT. is English` — only where needed, the
lines otherwise as written); `fountain.Unforce` takes those marks back.

```toml
Standard = "en"

[Scenes]
eo = ["EN.", "ENE", "EKST", "EK", "EN/EKST", "EKST/EN", "EKST./EN", "EN./EKST"]
```

### A language's direction

Every line runs the way its own characters say: a direction mark at its
start (RLM U+200F, LRM U+200E), or else its first strong letter
(Hebrew and Arabic right to left, Latin left to right), so a file mixing
directions stays mixed. Only a line with neither — `…`, `2.`, a bare
number — takes its language's direction, from `[Directions]`: `"rtl"` or
`"ltr"`. The default configuration has `ar` and `he` right to left, and a
language one's configuration does not name keeps the default's (so `he =
"ltr"` turns Hebrew around); a custom language sets its own. The HTML
and PDF writers use it (`-scenein` names the script's language).

The PDF prints a right-to-left line from the right: its element's
margins swapped and its alignment turned (a transition on the left, a
scene's CONTINUEDs, a dual dialogue's first speaker on the right), its
words in reading order by the Unicode bidi algorithm, numbers and Latin
in it kept, brackets turned. Hebrew prints this way, points and all
(Courier Badi has them; without shaping each point stands at its default
place). Scripts whose letters join (Arabic, Syriac, N'Ko, Mongolian…)
need shaping the PDF does not do yet: for them Lexington warns and
suggests printing the HTML from a browser.

```toml
[Directions]
ar = "rtl"
he = "rtl"
fa = "rtl"   # a language of one's own
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
some of them need.

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
