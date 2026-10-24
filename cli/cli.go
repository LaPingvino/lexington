// Package cli is Lexington's command line, for the lexington command and
// for programs that offer the same conversions (Accolade, a graphical
// Lexington): Main runs it with arguments, Convert and Write run a
// conversion with Options, and Formats are the output formats.
package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/lexington/fdx"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/html"
	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/latex"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/linter"
	"github.com/LaPingvino/lexington/markdown"
	"github.com/LaPingvino/lexington/office"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/writer"
)

// Version information, shown by -version (the lexington command sets it
// from its build flags).
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// What some formats need.
const (
	needsPandoc  = "pandoc"
	needsLaTeX   = "LaTeX"
	needsCalibre = "Calibre" // its ebook-convert
)

// Format is an output format.
type Format struct {
	// Key is what -to takes, and the file extension.
	Key string
	// Name is how people call it.
	Name string
	// Needs is the program it is made with ("" for none): pandoc,
	// wkhtmltopdf, or a LaTeX (pdflatex, xelatex or lualatex).
	Needs string
	// Ext is the file extension, when it is not Key.
	Ext string
}

// Extension is the format's file extension.
func (f Format) Extension() string {
	if f.Ext != "" {
		return f.Ext
	}
	return f.Key
}

// Formats are the output formats, Lexington's own first.
var Formats = []Format{
	{Key: internal.FormatPDF, Name: "PDF"},
	{Key: "html", Name: "HTML"},
	{Key: "docx", Name: "Word (DOCX)"},
	{Key: "odt", Name: "OpenDocument (ODT)"},
	{Key: "fdx", Name: "Final Draft (FDX)"},
	{Key: "fountain", Name: "Fountain"},
	{Key: "latex", Name: "LaTeX", Ext: "tex"},
	{Key: "lex", Name: "Lexington's own (lex)"},
	{Key: "htmlpdf", Name: "PDF through HTML", Needs: "wkhtmltopdf", Ext: internal.FormatPDF},
	{Key: "latexpdf", Name: "PDF through LaTeX", Needs: needsLaTeX, Ext: internal.FormatPDF},
	{Key: "epub", Name: "EPUB", Needs: needsPandoc},
	{Key: "azw3", Name: "Kindle (AZW3)", Needs: needsCalibre},
	{Key: "mobi", Name: "Mobipocket (MOBI)", Needs: needsCalibre},
	{Key: "prc", Name: "Mobipocket (PRC)", Needs: needsCalibre},
	{Key: "fb2", Name: "FictionBook (FB2)", Needs: needsCalibre},
	{Key: "pdb", Name: "eReader (PDB)", Needs: needsCalibre},
	{Key: "lrf", Name: "Sony Reader (LRF)", Needs: needsCalibre},
	{Key: "lit", Name: "Microsoft Reader (LIT)", Needs: needsCalibre},
	{Key: "rtf", Name: "Rich Text (RTF)", Needs: needsPandoc},
	{Key: "markdown", Name: "Markdown", Needs: needsPandoc, Ext: "md"},
	{Key: "rst", Name: "reStructuredText", Needs: needsPandoc},
	{Key: "json", Name: "Pandoc JSON", Needs: needsPandoc},
	{Key: "native", Name: "Pandoc native", Needs: needsPandoc},
	{Key: "man", Name: "Manual page", Needs: needsPandoc},
	{Key: "textile", Name: "Textile", Needs: needsPandoc},
	{Key: "mediawiki", Name: "MediaWiki", Needs: needsPandoc},
	{Key: "org", Name: "Org mode", Needs: needsPandoc},
	{Key: "asciidoc", Name: "AsciiDoc", Needs: needsPandoc},
}

// FormatFor is the output format with key.
func FormatFor(key string) (Format, bool) {
	for _, f := range Formats {
		if f.Key == key {
			return f, true
		}
	}
	return Format{}, false
}

// Available reports whether the program a format needs is installed.
func (f Format) Available() bool {
	switch f.Needs {
	case "":
		return true
	case needsLaTeX:
		return latexCommand() != ""
	case needsCalibre:
		_, err := exec.LookPath("ebook-convert")
		return err == nil
	}
	_, err := exec.LookPath(f.Needs)
	return err == nil
}

func formatKeys(needs bool) string {
	var keys []string
	for _, f := range Formats {
		if (f.Needs != "") == needs {
			keys = append(keys, f.Key)
		}
	}
	return strings.Join(keys, ", ")
}

// Options are a conversion's settings: what the command line's flags set.
type Options struct {
	ConfigFile   string // the settings file (lexington.toml)
	FormatDir    string // folders of format (preset) files of one's own, as a path list, or ""
	SceneIn      string // the scene heading language of the input ("en")
	SceneOut     string // and of the output
	Elements     string // the element settings: a preset or a set from the settings file
	Page         string // paper for pdf, docx and odt: letter, a4, a5; "" for the preset's
	Input        string // file name, "-" for standard input
	Output       string // file name, "-" for standard output
	From         string // input format: fountain, lex, fdx, pdf
	To           string // output format (Formats)
	TemplatePath string // a template for HTML, FDX or LaTeX
	Lint         bool   // run the Fountain linter
}

// Main runs the command line with args (without the program's name) and
// returns the exit code.
func Main(ctx context.Context, args []string) int {
	o, ok, err := parseFlags(args)
	if err != nil {
		return 2
	}
	if !ok {
		return 0
	}
	if err := Convert(ctx, o); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}

// parseFlags reads the arguments; ok is false when there is nothing to
// convert (help, presets, version, dumped settings).
func parseFlags(args []string) (o Options, ok bool, err error) {
	fs := flag.NewFlagSet("lexington", flag.ContinueOnError)
	var dump, help, presets, version bool
	fs.StringVar(&o.ConfigFile, "config", "lexington.toml", "Configuration file to use.")
	fs.StringVar(&o.FormatDir, "formats", "", "Folders of format files of your own (name.toml, as Lexington's presets), as a path list, to use with -e.")
	fs.BoolVar(&dump, "dumpconfig", false,
		"Dump the default configuration to the location of --config to be adapted manually.")
	fs.StringVar(&o.SceneIn, "scenein", "", "Configuration to use for scene header detection on input.")
	fs.StringVar(&o.SceneOut, "sceneout", "", "Configuration to use for scene header detection on output.")
	fs.StringVar(&o.Elements, "e", "default",
		"Element settings to use: a preset (see -presets) or a set from the settings file.")
	fs.StringVar(&o.Page, "page", "",
		"Paper size for pdf, docx and odt: letter, a4 or a5 (default: the preset's suggestion, else letter).")
	fs.BoolVar(&presets, "presets", false, "List the presets for -e (stage play, radio, ...).")
	fs.StringVar(&o.Input, "i", "-", "Input from provided filename. - means standard input.")
	fs.StringVar(&o.Output, "o", "-", "Output to provided filename. - means standard output.")
	fs.StringVar(&o.From, "from", "", "Input file type. Choose from fountain, lex, fdx, pdf.")
	fs.StringVar(&o.To, "to", "",
		"Output file type. Choose from "+formatKeys(false)+", or external formats requiring other programs: "+
			formatKeys(true)+".")
	fs.BoolVar(&o.Lint, "lint", false, "Run the Fountain linter on the input file")
	fs.StringVar(&o.TemplatePath, "template", "",
		"Path to a custom template file (e.g., for HTML, FDX, or LaTeX output).")
	fs.BoolVar(&help, "help", false, "Show this help message")
	fs.BoolVar(&version, "version", false, "Show version information")
	if err := fs.Parse(args); err != nil {
		return o, false, err
	}
	switch {
	case help:
		fs.PrintDefaults()
	case presets:
		list, err := rules.PresetsIn(filepath.SplitList(o.FormatDir)...)
		if err != nil {
			log.Print(err)
		}
		for _, p := range list {
			fmt.Printf("%-12s %s\n%-12s %s\n\n", p.Key, p.Name, "", p.Description)
		}
	case version:
		log.Printf("Lexington version %s (commit: %s, built: %s)", Version, Commit, Date)
	case dump:
		if err := rules.Dump(o.ConfigFile); err != nil {
			log.Printf("Error dumping configuration: %v", err)
		} else {
			log.Printf("Configuration dumped to %s", o.ConfigFile)
		}
	default:
		return o, true, nil
	}
	return o, false, nil
}

// Convert reads the input, and writes it as the output, as the command
// line does. Formats and scene languages missing from o are taken from
// the file names (script.fr.fountain: French scene headings).
func Convert(ctx context.Context, o Options) error {
	detectFormats(&o)
	setDefaults(&o)
	conf := confOf(o)
	if _, ok := conf.Elements[o.Elements]; !ok {
		return fmt.Errorf("no element settings %q in %s or the presets (see -presets)", o.Elements, o.ConfigFile)
	}
	in, closeIn, err := openInput(o.Input)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := closeIn(); cerr != nil {
			log.Printf("Closing the input: %v", cerr)
		}
	}()
	screenplay, err := parseInput(o, conf, in)
	if err != nil {
		return err
	}
	if o.Lint && lint(screenplay) && o.To == "" && o.Output == "-" {
		return nil // only linting
	}
	if o.Output == "" {
		if ins := strings.Split(o.Input, "."); len(ins) > 0 && ins[0] != "" {
			o.Output = ins[0] + "." + o.To
		}
	}
	return write(ctx, o, conf, screenplay)
}

// Write writes a screenplay as o.To to o.Output (a file, or "-" for
// standard output), with o's settings.
func Write(ctx context.Context, screenplay lex.Screenplay, o Options) error {
	setDefaults(&o)
	conf := confOf(o)
	if _, ok := conf.Elements[o.Elements]; !ok {
		return fmt.Errorf("no element settings %q in %s or the presets", o.Elements, o.ConfigFile)
	}
	return write(ctx, o, conf, screenplay)
}

func write(ctx context.Context, o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, ok := FormatFor(o.To)
	if !ok {
		return fmt.Errorf("%s is not a supported output type; choose from %s, or with other programs %s",
			o.To, formatKeys(false), formatKeys(true))
	}
	if f.Needs != "" {
		return external(o, conf, screenplay)
	}
	w, err := createWriter(o, conf)
	if err != nil {
		return err
	}
	out, closeOut, err := openOutput(o.Output)
	if err != nil {
		return err
	}
	if err := w.Write(out, screenplay); err != nil {
		return errors.Join(err, closeOut())
	}
	if o.To == internal.FormatPDF {
		if warning := PDFWarning(screenplay); warning != "" {
			log.Print(warning)
		}
	}
	return closeOut()
}

// PDFWarning says what a PDF of the screenplay cannot print as written,
// and what to do instead; "" when it prints well.
func PDFWarning(s lex.Screenplay) string {
	missing, rtl := pdf.Unsupported(s)
	var why []string
	if len(missing) > 0 {
		shown := missing
		if len(shown) > 12 {
			shown = shown[:12]
		}
		why = append(why, fmt.Sprintf("its font has no %s (%d characters), which show as boxes", string(shown), len(missing)))
	}
	if rtl {
		why = append(why, "its right-to-left text (Hebrew, Arabic) comes out left to right")
	}
	if len(why) == 0 {
		return ""
	}
	return "The PDF cannot print this script as written: " + strings.Join(why, "; and ") +
		". Export HTML and print it from a browser (Save as PDF) instead."
}

func detectFormats(o *Options) {
	if ins := strings.Split(o.Input, "."); len(ins) > 1 {
		if o.From == "" {
			o.From = ins[len(ins)-1]
		}
		if len(ins) > 2 && o.SceneIn == "" {
			o.SceneIn = ins[len(ins)-2]
		}
	}
	if outs := strings.Split(o.Output, "."); len(outs) > 1 {
		if o.To == "" {
			o.To = outs[len(outs)-1]
		}
		if len(outs) > 2 && o.SceneOut == "" {
			o.SceneOut = outs[len(outs)-2]
		}
	}
}

func setDefaults(o *Options) {
	if o.ConfigFile == "" {
		o.ConfigFile = "lexington.toml"
	}
	if o.Elements == "" {
		o.Elements = "default"
	}
	if o.From == "" {
		o.From = internal.FormatFountain
	}
	if o.To == "" && o.Output == "-" {
		o.To = internal.FormatLex
	}
	if o.SceneIn == "" {
		o.SceneIn = "en"
	}
	if o.SceneOut == "" {
		o.SceneOut = "en"
	}
}

func openInput(name string) (io.Reader, func() error, error) {
	if name == "-" || name == "" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func openOutput(name string) (io.Writer, func() error, error) {
	if name == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func parseInput(o Options, conf rules.TOMLConf, input io.Reader) (lex.Screenplay, error) {
	switch o.From {
	case internal.FormatLex:
		return lex.Parse(input), nil
	case internal.FormatFountain:
		return fountain.Parse(conf.Scenes[o.SceneIn], input), nil
	case internal.FormatFDX:
		s, err := fdx.ParseWithError(input)
		if err != nil {
			return nil, fmt.Errorf("could not read FDX input: %w", err)
		}
		return s, nil
	case internal.FormatPDF:
		// a PDF is read at random places, so it is read whole first
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, fmt.Errorf("could not read the PDF: %w", err)
		}
		// scanned pages are OCRed with tesseract, if it is installed
		opts := pdfin.Options{Scenes: conf.Scenes[o.SceneIn]}
		if t, ok := pdfin.InstalledTesseract(); ok {
			if o.SceneIn != "en" && !t.UseLanguage(o.SceneIn) {
				log.Printf("Tesseract has no %s: scanned pages are read as English (install its traineddata)", o.SceneIn)
			}
			opts.OCR = t
		}
		opts.Skipped = func(page int, err error) { log.Printf("Left out page %d: %v", page, err) }
		s, err := pdfin.ReadWith(bytes.NewReader(data), int64(len(data)), opts)
		if err != nil {
			return nil, fmt.Errorf("could not read the PDF: %w", err)
		}
		return s, nil
	}
	return nil, fmt.Errorf("%s is not a valid input type", o.From)
}

// lint reports the linter's findings; it reports whether it ran.
func lint(screenplay lex.Screenplay) bool {
	l := linter.NewLinter()
	l.Lint(screenplay)
	if l.HasErrors() {
		log.Println(l.FormatErrors())
	} else {
		log.Println("Linting complete: No errors found.")
	}
	return true
}

// confOf is the settings with the format files' element sets, which take
// the place of the presets of their names.
func confOf(o Options) rules.TOMLConf {
	conf := rules.GetConf(o.ConfigFile)
	if o.FormatDir != "" {
		list, err := rules.PresetsIn(filepath.SplitList(o.FormatDir)...)
		if err != nil {
			log.Print(err)
		}
		for _, p := range list {
			conf.Elements[p.Key] = p.Elements
		}
	}
	return conf
}

// pageFor is the paper size: o.Page, or the one the preset suggests.
func pageFor(o Options) string {
	if o.Page != "" {
		return o.Page
	}
	list, _ := rules.PresetsIn(filepath.SplitList(o.FormatDir)...)
	for _, p := range list {
		if p.Key == o.Elements {
			return p.Page
		}
	}
	return ""
}

func createWriter(o Options, conf rules.TOMLConf) (writer.Writer, error) {
	elements := conf.Elements[o.Elements]
	switch o.To {
	case internal.FormatPDF:
		if o.Output == "-" {
			return nil, errors.New("cannot write PDF to standard output: give an output file (-o output.pdf)")
		}
		return &pdf.PDFWriter{OutputFile: o.Output, Elements: elements, Page: pageFor(o)}, nil
	case internal.FormatLex:
		return &lex.LexWriter{}, nil
	case internal.FormatFountain:
		return &fountain.FountainWriter{SceneConfig: conf.Scenes[o.SceneOut]}, nil
	case internal.FormatFDX:
		return &fdx.FDXWriter{TemplatePath: o.TemplatePath}, nil
	case internal.FormatHTML:
		return &html.HTMLWriter{Elements: elements, Page: pageFor(o)}, nil
	case internal.FormatLaTeX:
		return &latex.LaTeXWriter{Template: o.TemplatePath, Elements: elements}, nil
	case "docx":
		return &office.DOCXWriter{Elements: elements, Page: pageFor(o)}, nil
	case "odt":
		return &office.ODTWriter{Elements: elements, Page: pageFor(o)}, nil
	}
	return nil, fmt.Errorf("%s is not a supported output type", o.To)
}

// external makes the formats that need another program.
func external(o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	if o.Output == "-" || o.Output == "" {
		return fmt.Errorf("%s needs an output file (-o)", o.To)
	}
	switch o.To {
	case "htmlpdf":
		return htmlPDF(o, conf, screenplay)
	case "latexpdf":
		return latexPDF(o, conf, screenplay)
	}
	if f, _ := FormatFor(o.To); f.Needs == needsCalibre {
		return calibre(o, conf, screenplay)
	}
	return pandoc(o, screenplay)
}

// tempFile writes data to a temporary file with the pattern's extension;
// remove removes it.
func tempFile(pattern string, data []byte) (name string, remove func(), err error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", nil, err
	}
	remove = func() { os.Remove(f.Name()) }
	if _, err := f.Write(data); err != nil {
		f.Close()
		remove()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		remove()
		return "", nil, err
	}
	return f.Name(), remove, nil
}

func htmlPDF(o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	wkhtmltopdf, err := exec.LookPath("wkhtmltopdf")
	if err != nil {
		return errors.New("'htmlpdf' output requires wkhtmltopdf, but it could not be found in your system's PATH")
	}
	var buf bytes.Buffer
	if err = (&html.HTMLWriter{Elements: conf.Elements[o.Elements]}).Write(&buf, screenplay); err != nil {
		return fmt.Errorf("converting to HTML for wkhtmltopdf: %w", err)
	}
	name, remove, err := tempFile("lexington_*.html", buf.Bytes())
	if err != nil {
		return err
	}
	defer remove()
	cmd := exec.Command(wkhtmltopdf,
		"--page-size", "Letter",
		"--margin-top", "0.75in",
		"--margin-right", "0.75in",
		"--margin-bottom", "0.75in",
		"--margin-left", "0.75in",
		"--encoding", "UTF-8",
		"--print-media-type",
		name, o.Output)
	cmd.Stderr = os.Stderr
	log.Printf("Running wkhtmltopdf to create PDF from HTML...")
	return cmd.Run()
}

// latexCommand is the LaTeX installed: pdflatex, xelatex or lualatex.
func latexCommand() string {
	for _, c := range []string{"pdflatex", "xelatex", "lualatex"} {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

func latexPDF(o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	latexCmd := latexCommand()
	if latexCmd == "" {
		return errors.New("'latexpdf' output requires pdflatex, xelatex, or lualatex, " +
			"but none could be found in your system's PATH")
	}
	var buf bytes.Buffer
	lw := &latex.LaTeXWriter{Template: o.TemplatePath, Elements: conf.Elements[o.Elements]}
	if err := lw.Write(&buf, screenplay); err != nil {
		return fmt.Errorf("converting to LaTeX: %w", err)
	}
	// in a folder of its own: LaTeX writes its .aux and .log beside the PDF
	dir, err := os.MkdirTemp("", "lexington-latex-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	tex := filepath.Join(dir, "script.tex")
	if err = os.WriteFile(tex, buf.Bytes(), 0o644); err != nil {
		return err
	}
	log.Printf("Running %s to create PDF from LaTeX...", latexCmd)
	cmd := exec.Command(latexCmd, "-interaction=nonstopmode", "-output-directory", dir, tex)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	if err = cmd.Run(); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "script.pdf"))
	if err != nil {
		return err
	}
	return os.WriteFile(o.Output, data, 0o644)
}

func pandoc(o Options, screenplay lex.Screenplay) error {
	pandoc, err := exec.LookPath("pandoc")
	if err != nil {
		return fmt.Errorf("'%s' output requires pandoc, but it could not be found in your system's PATH", o.To)
	}
	title, author := titleAuthor(screenplay)
	var buf bytes.Buffer
	if err := (&markdown.MarkdownWriter{}).Write(&buf, screenplay); err != nil {
		return fmt.Errorf("converting to Markdown for pandoc: %w", err)
	}
	args := []string{"--from=markdown", "--to=" + o.To, "-o", o.Output}
	if title != "" {
		args = append(args, "--metadata", "title="+title)
	}
	if author != "" {
		args = append(args, "--metadata", "author="+author)
	}
	cmd := exec.Command(pandoc, args...)
	cmd.Stdin = &buf
	cmd.Stderr = os.Stderr
	log.Printf("Running pandoc to create %s...", o.Output)
	return cmd.Run()
}

// titleAuthor are the title page's title and author.
func titleAuthor(screenplay lex.Screenplay) (title, author string) {
	for _, line := range screenplay {
		switch line.Type {
		case "Title":
			title = line.Contents
		case "Author":
			author = line.Contents
		}
	}
	return title, author
}

// calibre makes the e-book formats with Calibre's ebook-convert, from
// Lexington's HTML (its layout and styles); a PRC is a MOBI.
func calibre(o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	convert, err := exec.LookPath("ebook-convert")
	if err != nil {
		return fmt.Errorf("'%s' output requires Calibre's ebook-convert, "+
			"but it could not be found in your system's PATH", o.To)
	}
	dir, err := os.MkdirTemp("", "lexington-ebook-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var buf bytes.Buffer
	if err = (&html.HTMLWriter{Elements: conf.Elements[o.Elements]}).Write(&buf, screenplay); err != nil {
		return fmt.Errorf("converting to HTML for ebook-convert: %w", err)
	}
	in := filepath.Join(dir, "script.html")
	if err = os.WriteFile(in, buf.Bytes(), 0o644); err != nil {
		return err
	}
	ext := o.To
	if ext == "prc" {
		ext = "mobi"
	}
	out := filepath.Join(dir, "book."+ext)
	args := []string{in, out}
	if title, author := titleAuthor(screenplay); title != "" || author != "" {
		if title != "" {
			args = append(args, "--title", title)
		}
		if author != "" {
			args = append(args, "--authors", author)
		}
	}
	cmd := exec.Command(convert, args...)
	cmd.Stderr = os.Stderr
	log.Printf("Running ebook-convert to create %s...", o.Output)
	if err = cmd.Run(); err != nil {
		return err
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return os.WriteFile(o.Output, data, 0o644)
}
