Lexington commandline tool for screenwriters
============================================

[![Go](https://github.com/LaPingvino/lexington/actions/workflows/go.yml/badge.svg)](https://github.com/LaPingvino/lexington/actions/workflows/go.yml)

Lexington helps you convert between Final Draft, Fountain and its own lex file formats, and output to PDF, HTML and ebook formats.

The tool supports dual dialogue (simultaneous dialog), configurable margins and fonts, and professional screenplay formatting. It integrates with pandoc for ebook generation and supports multiple output formats.

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

### Pre-defined Styles

- **default**: Standard screenplay format with industry-standard margins
- **compact**: Tighter margins suitable for web display or smaller pages

Use a pre-defined style:
```bash
lexington -i script.fountain -o script.html -to html -e compact
```

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

### EPUB Output
- Metadata integration (title, author, etc.)
- Chapter structure preservation
- Compatible with most e-readers

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
