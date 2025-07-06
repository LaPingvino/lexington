package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lapingvino/lexington/fdx"
	"github.com/lapingvino/lexington/fountain"
	"github.com/lapingvino/lexington/html"
	"github.com/lapingvino/lexington/latex"
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/linter"
	"github.com/lapingvino/lexington/markdown"
	"github.com/lapingvino/lexington/pdf"
	"github.com/lapingvino/lexington/rules"
	"github.com/lapingvino/lexington/writer"
)

// Version information (set by build flags)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// pandocFormats lists the formats that are delegated to the pandoc command.
var pandocFormats = map[string]bool{
	"epub":      true,
	"mobi":      true,
	"docx":      true,
	"odt":       true,
	"rtf":       true,
	"markdown":  true,
	"rst":       true,
	"json":      true,
	"native":    true,
	"man":       true,
	"textile":   true,
	"mediawiki": true,
	"org":       true,
	"asciidoc":  true,
	"htmlpdf":   true,
	"latexpdf":  true,
}

// Config holds all command-line configuration
type Config struct {
	ConfigFile   string
	Dump         bool
	SceneIn      string
	SceneOut     string
	Elements     string
	Input        string
	Output       string
	From         string
	To           string
	Lint         bool
	TemplatePath string
	Help         bool
	ShowVersion  bool
}

// IOFiles holds input and output file handles
type IOFiles struct {
	Input  io.Reader
	Output io.Writer
	Closer func() error
}

func main() {
	ctx, cancel := setupContext()
	defer cancel()

	start := time.Now()
	defer func() {
		log.Printf("Conversion took %v", time.Since(start))
	}()

	config := parseFlags()
	if handleEarlyExits(config) {
		return
	}

	detectFormats(config)
	setDefaults(config)
	log.Printf("Scenein: %s ; Sceneout: %s ;\n", config.SceneIn, config.SceneOut)

	conf := rules.GetConf(config.ConfigFile)
	ioFiles, err := setupIO(config)
	if err != nil {
		log.Printf("Error setting up I/O: %v", err)
		return
	}
	defer func() {
		if err := ioFiles.Closer(); err != nil {
			log.Printf("Error closing files: %v", err)
		}
	}()

	screenplay := parseInput(config, conf, ioFiles.Input)
	if screenplay == nil {
		return
	}

	if config.Lint {
		if handleLinting(*screenplay, config) {
			return
		}
	}

	if err := convertOutput(ctx, config, conf, ioFiles.Output, *screenplay); err != nil {
		log.Printf("Error during conversion: %v", err)
	}
}

func setupContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	return ctx, cancel
}

func parseFlags() *Config {
	config := &Config{}
	flag.StringVar(&config.ConfigFile, "config", "lexington.toml", "Configuration file to use.")
	flag.BoolVar(&config.Dump, "dumpconfig", false, "Dump the default configuration to the location of --config to be adapted manually.")
	flag.StringVar(&config.SceneIn, "scenein", "", "Configuration to use for scene header detection on input.")
	flag.StringVar(&config.SceneOut, "sceneout", "", "Configuration to use for scene header detection on output.")
	flag.StringVar(&config.Elements, "e", "default", "Element settings from settings file to use.")
	flag.StringVar(&config.Input, "i", "-", "Input from provided filename. - means standard input.")
	flag.StringVar(&config.Output, "o", "-", "Output to provided filename. - means standard output.")
	flag.StringVar(&config.From, "from", "", "Input file type. Choose from fountain, lex, fdx.")
	flag.StringVar(&config.To, "to", "", "Output file type. Choose from pdf, lex, fountain, fdx, html, latex, or external formats requiring pandoc: epub, mobi, docx, odt, rtf, markdown, rst, json, native, man, textile, mediawiki, org, asciidoc, htmlpdf, latexpdf.")
	flag.BoolVar(&config.Lint, "lint", false, "Run the Fountain linter on the input file")
	flag.StringVar(&config.TemplatePath, "template", "", "Path to a custom template file (e.g., for HTML, FDX, or LaTeX output).")
	flag.BoolVar(&config.Help, "help", false, "Show this help message")
	flag.BoolVar(&config.ShowVersion, "version", false, "Show version information")
	flag.Parse()
	return config
}

func handleEarlyExits(config *Config) bool {
	if config.Help {
		flag.PrintDefaults()
		return true
	}

	if config.ShowVersion {
		log.Printf("Lexington version %s (commit: %s, built: %s)", version, commit, date)
		return true
	}

	if config.Dump {
		err := rules.Dump(config.ConfigFile)
		if err != nil {
			log.Printf("Error dumping configuration: %v", err)
		}
		log.Printf("Configuration dumped to %s", config.ConfigFile)
		return true
	}

	return false
}

func detectFormats(config *Config) {
	if ins := strings.Split(config.Input, "."); len(ins) > 1 {
		if config.From == "" {
			config.From = ins[len(ins)-1]
		}
		if len(ins) > 2 && config.SceneIn == "" {
			config.SceneIn = ins[len(ins)-2]
		}
	}

	if outs := strings.Split(config.Output, "."); len(outs) > 1 {
		if config.To == "" {
			config.To = outs[len(outs)-1]
		}
		if len(outs) > 2 && config.SceneOut == "" {
			config.SceneOut = outs[len(outs)-2]
		}
	}
}

func setDefaults(config *Config) {
	if config.From == "" {
		config.From = "fountain"
	}
	if config.To == "" && config.Output == "-" {
		config.To = "lex"
	}
	if config.SceneIn == "" {
		config.SceneIn = "en"
	}
	if config.SceneOut == "" {
		config.SceneOut = "en"
	}
}

func setupIO(config *Config) (*IOFiles, error) {
	ioFiles := &IOFiles{
		Closer: func() error { return nil },
	}

	if config.Input == "-" {
		ioFiles.Input = os.Stdin
		log.Println("Reading from Stdin")
	} else {
		file, err := os.Open(config.Input)
		if err != nil {
			return nil, err
		}
		ioFiles.Input = file
		ioFiles.Closer = file.Close
	}

	if config.Output == "-" {
		ioFiles.Output = os.Stdout
		log.Println("Writing to Stdout")
	} else {
		if config.Output == "" {
			if ins := strings.Split(config.Input, "."); len(ins) > 0 && ins[0] != "" {
				config.Output = ins[0] + "." + config.To
			}
		}
		outputFile, err := os.Create(config.Output)
		if err != nil {
			return nil, err
		}
		ioFiles.Output = outputFile
		oldCloser := ioFiles.Closer
		ioFiles.Closer = func() error {
			if err := outputFile.Close(); err != nil {
				log.Printf("Error closing output file: %v", err)
			}
			return oldCloser()
		}
	}

	return ioFiles, nil
}

func parseInput(config *Config, conf rules.TOMLConf, input io.Reader) *lex.Screenplay {
	log.Printf("Input type is %s", config.From)

	var screenplay lex.Screenplay
	switch config.From {
	case "lex":
		screenplay = lex.Parse(input)
	case "fountain":
		screenplay = fountain.Parse(conf.Scenes[config.SceneIn], input)
	case "fdx":
		screenplay = fdx.Parse(input)
	default:
		log.Printf("%s is not a valid input type", config.From)
		return nil
	}

	return &screenplay
}

func handleLinting(screenplay lex.Screenplay, config *Config) bool {
	l := linter.NewLinter()
	l.Lint(screenplay)

	if l.HasErrors() {
		log.Println(l.FormatErrors())
		if config.To == "" && config.Output == "-" {
			return true
		}
	} else {
		log.Println("Linting complete: No errors found.")
		if config.To == "" && config.Output == "-" {
			return true
		}
	}

	return false
}

func convertOutput(ctx context.Context, config *Config, conf rules.TOMLConf, output io.Writer, screenplay lex.Screenplay) error {
	log.Printf("Output type is %s", config.To)

	if pandocFormats[config.To] {
		return handlePandocOutput(config, conf, screenplay)
	}

	outputWriter := createWriter(config, conf)
	if outputWriter == nil {
		log.Printf("%s is not a supported output type. Choose from: pdf, lex, fountain, fdx, html, latex, or external formats requiring pandoc: epub, mobi, docx, odt, rtf, markdown, rst, json, native, man, textile, mediawiki, org, asciidoc, htmlpdf, latexpdf.\n", config.To)
		return nil
	}

	select {
	case <-ctx.Done():
		log.Printf("Operation cancelled: %v", ctx.Err())
		return ctx.Err()
	default:
		return outputWriter.Write(output, screenplay)
	}
}

func createWriter(config *Config, conf rules.TOMLConf) writer.Writer {
	switch config.To {
	case "pdf":
		if config.Output == "-" {
			log.Println("Cannot write PDF to standard output. Please provide an output filename (e.g., -o output.pdf).")
			return nil
		}
		return &pdf.PDFWriter{OutputFile: config.Output, Elements: conf.Elements[config.Elements]}
	case "lex":
		return &lex.LexWriter{}
	case "fountain":
		return &fountain.FountainWriter{SceneConfig: conf.Scenes[config.SceneOut]}
	case "fdx":
		return &fdx.FDXWriter{TemplatePath: config.TemplatePath}
	case "html":
		return &html.HTMLWriter{Elements: conf.Elements[config.Elements]}
	case "latex":
		return &latex.LaTeXWriter{Template: config.TemplatePath, Elements: conf.Elements[config.Elements]}
	default:
		return nil
	}
}

func handlePandocOutput(config *Config, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	if config.To == "htmlpdf" {
		return handleHTMLPDF(config, conf, screenplay)
	}
	if config.To == "latexpdf" {
		return handleLaTeXPDF(config, conf, screenplay)
	}
	return handleStandardPandoc(config, screenplay)
}

func handleHTMLPDF(config *Config, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	wkhtmltopdf, err := exec.LookPath("wkhtmltopdf")
	if err != nil {
		log.Printf("Error: 'htmlpdf' output requires wkhtmltopdf, but it could not be found in your system's PATH.")
		return err
	}

	var htmlBuffer bytes.Buffer
	htmlWriter := &html.HTMLWriter{Elements: conf.Elements[config.Elements]}
	if err := htmlWriter.Write(&htmlBuffer, screenplay); err != nil {
		log.Printf("Error converting to HTML format for wkhtmltopdf: %v", err)
		return err
	}

	tempFile, err := os.CreateTemp("", "lexington_*.html")
	if err != nil {
		log.Printf("Error creating temporary HTML file: %v", err)
		return err
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			log.Printf("Error removing temporary HTML file: %v", err)
		}
	}()

	if _, err := tempFile.Write(htmlBuffer.Bytes()); err != nil {
		log.Printf("Error writing HTML content: %v", err)
		return err
	}
	if err := tempFile.Close(); err != nil {
		log.Printf("Error closing temporary HTML file: %v", err)
		return err
	}

	cmdArgs := []string{
		"--page-size", "Letter",
		"--margin-top", "0.75in",
		"--margin-right", "0.75in",
		"--margin-bottom", "0.75in",
		"--margin-left", "0.75in",
		"--encoding", "UTF-8",
		"--print-media-type",
		tempFile.Name(),
		config.Output,
	}
	cmd := exec.Command(wkhtmltopdf, cmdArgs...)
	cmd.Stderr = os.Stderr

	log.Printf("Running wkhtmltopdf to create PDF from HTML...")
	return cmd.Run()
}

func handleLaTeXPDF(config *Config, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	var latexCmd string
	if _, err := exec.LookPath("pdflatex"); err == nil {
		latexCmd = "pdflatex"
	} else if _, err := exec.LookPath("xelatex"); err == nil {
		latexCmd = "xelatex"
	} else if _, err := exec.LookPath("lualatex"); err == nil {
		latexCmd = "lualatex"
	} else {
		log.Printf("Error: 'latexpdf' output requires pdflatex, xelatex, or lualatex, but none could be found in your system's PATH.")
		return err
	}

	var latexBuffer bytes.Buffer
	latexWriter := &latex.LaTeXWriter{Template: config.TemplatePath, Elements: conf.Elements[config.Elements]}
	if err := latexWriter.Write(&latexBuffer, screenplay); err != nil {
		log.Printf("Error converting to LaTeX format: %v", err)
		return err
	}

	tempFile, err := os.CreateTemp("", "lexington_*.tex")
	if err != nil {
		log.Printf("Error creating temporary LaTeX file: %v", err)
		return err
	}
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			log.Printf("Error removing temporary LaTeX file: %v", err)
		}
	}()

	if _, err := tempFile.Write(latexBuffer.Bytes()); err != nil {
		log.Printf("Error writing LaTeX content: %v", err)
		return err
	}
	if err := tempFile.Close(); err != nil {
		log.Printf("Error closing temporary LaTeX file: %v", err)
		return err
	}

	log.Printf("Running %s to create PDF from LaTeX...", latexCmd)
	cmd := exec.Command(latexCmd, "-output-directory", ".", "-jobname", strings.TrimSuffix(config.Output, ".pdf"), tempFile.Name())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd.Run()
}

func handleStandardPandoc(config *Config, screenplay lex.Screenplay) error {
	pandoc, err := exec.LookPath("pandoc")
	if err != nil {
		log.Printf("Error: '%s' output requires pandoc, but it could not be found in your system's PATH.", config.To)
		return err
	}

	var title, author string
	for _, line := range screenplay {
		switch line.Type {
		case "Title":
			title = line.Contents
		case "Author":
			author = line.Contents
		}
	}

	var markdownBuffer bytes.Buffer
	markdownWriter := &markdown.MarkdownWriter{}
	if err := markdownWriter.Write(&markdownBuffer, screenplay); err != nil {
		log.Printf("Error converting to Markdown format for pandoc: %v", err)
		return err
	}

	cmdArgs := []string{"--from=markdown", "--to=" + config.To, "-o", config.Output}
	if title != "" {
		cmdArgs = append(cmdArgs, "--metadata", "title="+title)
	}
	if author != "" {
		cmdArgs = append(cmdArgs, "--metadata", "author="+author)
	}

	cmd := exec.Command(pandoc, cmdArgs...)
	cmd.Stdin = &markdownBuffer
	cmd.Stderr = os.Stderr

	log.Printf("Running pandoc to create %s...", config.Output)
	return cmd.Run()
}
