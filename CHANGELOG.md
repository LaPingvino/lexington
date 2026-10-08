# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### New Features
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
  WebAssembly one of the `ocrwasm` module.
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