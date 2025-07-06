package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lapingvino/lexington/fdx"
	"github.com/lapingvino/lexington/fountain"
	"github.com/lapingvino/lexington/html"
	"github.com/lapingvino/lexington/latex" // New import for the LaTeX writer
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/linter"
	"github.com/lapingvino/lexington/markdown" // New import for the Markdown writer
	"github.com/lapingvino/lexington/pdf"
	"github.com/lapingvino/lexington/rules"
	"github.com/lapingvino/lexington/writer"
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
}

func main() {
	start := time.Now()
	defer func() {
		log.Printf("Conversion took %v", time.Since(start))
	}()

	config := flag.String("config", "lexington.toml", "Configuration file to use.")
	dump := flag.Bool("dumpconfig", false, "Dump the default configuration to the location of --config to be adapted manually.")
	scenein := flag.String("scenein", "", "Configuration to use for scene header detection on input.")
	sceneout := flag.String("sceneout", "", "Configuration to use for scene header detection on output.")
	elements := flag.String("e", "default", "Element settings from settings file to use.")
	input := flag.String("i", "-", "Input from provided filename. - means standard input.")
	output := flag.String("o", "-", "Output to provided filename. - means standard output.")
	from := flag.String("from", "", "Input file type. Choose from fountain, lex, fdx.")
	to := flag.String("to", "", "Output file type. Choose from pdf, lex, fountain, fdx, html, latex, or external formats requiring pandoc: epub, mobi, docx, odt, rtf, markdown, rst, json, native, man, textile, mediawiki, org, asciidoc.")
	lint := flag.Bool("lint", false, "Run the Fountain linter on the input file")
	templatePath := flag.String("template", "", "Path to a custom template file (e.g., for HTML, FDX, or LaTeX output).") // New flag
	help := flag.Bool("help", false, "Show this help message")
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	if *dump {
		err := rules.Dump(*config)
		if err != nil {
			log.Println("Error dumping configuration: ", err)
		}
		log.Println("Configuration dumped to ", *config)
		return
	}

	ins := strings.Split(*input, ".")
	if len(ins) > 1 && *from == "" {
		*from = ins[len(ins)-1]
	}
	if len(ins) > 2 && *scenein == "" {
		*scenein = ins[len(ins)-2]
	}

	outs := strings.Split(*output, ".")
	if len(outs) > 1 && *to == "" {
		*to = outs[len(outs)-1]
	}
	if len(outs) > 2 && *sceneout == "" {
		*sceneout = outs[len(outs)-2]
	}

	if *from == "" {
		*from = "fountain"
	}
	if *to == "" && *output == "-" {
		*to = "lex"
	}
	if *scenein == "" {
		*scenein = "en"
	}
	if *sceneout == "" {
		*sceneout = "en"
	}

	log.Printf("Scenein: %s ; Sceneout: %s ;\n", *scenein, *sceneout)

	var infile io.Reader
	var outfile io.Writer
	var i lex.Screenplay

	conf := rules.GetConf(*config)

	if *input == "-" {
		infile = os.Stdin
		log.Println("Reading from Stdin")
	} else {
		var err error
		file, err := os.Open(*input)
		if err != nil {
			log.Println("Error opening file: ", err)
			return
		}
		defer file.Close()
		infile = file
	}

	if *output == "-" {
		outfile = os.Stdout
		log.Println("Writing to Stdout")
	} else {
		if *output == "" && len(ins) > 0 && ins[0] != "" {
			*output = ins[0] + "." + *to
		}
		var err error
		outputFile, err := os.Create(*output)
		if err != nil {
			log.Println("Error creating output file: ", err)
			return
		}
		defer outputFile.Close()
		outfile = outputFile
	}

	log.Println("Input type is ", *from)
	switch *from {
	case "lex":
		i = lex.Parse(infile)
	case "fountain":
		i = fountain.Parse(conf.Scenes[*scenein], infile)
	case "fdx":
		i = fdx.Parse(infile)
	default:
		log.Printf("%s is not a valid input type", *from)
		return
	}

	// Run linter if requested
	if *lint {
		l := linter.NewLinter()
		l.Lint(i)
		if l.HasErrors() {
			log.Println(l.FormatErrors())
			// If only linting and errors are found, exit
			if *to == "" && *output == "-" {
				return
			}
		} else {
			log.Println("Linting complete: No errors found.")
			if *to == "" && *output == "-" { // If no output format specified, and not writing to file, imply lint-only
				return // If just linting, exit after report.
			}
		}
	}

	log.Println("Output type is ", *to)

	var outputWriter writer.Writer // Declare the interface variable
	var err error                  // Declare err here for broader scope

	switch *to {
	case "pdf":
		if *output == "-" {
			log.Println("Cannot write PDF to standard output. Please provide an output filename (e.g., -o output.pdf).")
			return // Exit because PDF output to stdout is not supported.
		}
		outputWriter = &pdf.PDFWriter{OutputFile: *output, Elements: conf.Elements[*elements]}
	case "lex":
		outputWriter = &lex.LexWriter{}
	case "fountain":
		outputWriter = &fountain.FountainWriter{SceneConfig: conf.Scenes[*sceneout]}
	case "fdx":
		// FDXWriter will be updated to use a template
		outputWriter = &fdx.FDXWriter{TemplatePath: *templatePath} // Pass template path
	case "html":
		// HTMLWriter already handles its internal template
		outputWriter = &html.HTMLWriter{}
	case "latex":
		outputWriter = &latex.LaTeXWriter{Template: *templatePath, Elements: conf.Elements[*elements]}
	default:
		// Check if the format is one that should be handled by pandoc.
		if pandocFormats[*to] {
			pandoc, err := exec.LookPath("pandoc")
			if err != nil {
				log.Printf("Error: '%s' output requires pandoc, but it could not be found in your system's PATH.", *to)
				return
			}

			// Extract title and author from screenplay for pandoc metadata
			var title, author string
			for _, line := range i {
				if line.Type == "Title" {
					title = line.Contents
				} else if line.Type == "Author" {
					author = line.Contents
				}
			}

			// Convert the screenplay to Markdown format in memory.
			var markdownBuffer bytes.Buffer
			markdownWriter := &markdown.MarkdownWriter{}
			err = markdownWriter.Write(&markdownBuffer, i)
			if err != nil {
				log.Printf("Error converting to Markdown format for pandoc: %v", err)
				return
			}

			// Prepare and run the pandoc command.
			cmdArgs := []string{"--from=markdown", "--to=" + *to, "-o", *output}
			if title != "" {
				cmdArgs = append(cmdArgs, "--metadata", "title="+title)
			}
			if author != "" {
				cmdArgs = append(cmdArgs, "--metadata", "author="+author)
			}
			cmd := exec.Command(pandoc, cmdArgs...)
			cmd.Stdin = &markdownBuffer
			cmd.Stderr = os.Stderr // Pipe pandoc's errors to our stderr.

			log.Printf("Running pandoc to create %s...", *output)
			err = cmd.Run()
			if err != nil {
				log.Printf("Error executing pandoc command: %v", err)
			}
			return
		} else {
			log.Printf("%s is not a supported output type. Choose from: pdf, lex, fountain, fdx, html, latex, or external formats requiring pandoc: epub, mobi, docx, odt, rtf, markdown, rst, json, native, man, textile, mediawiki, org, asciidoc.\n", *to)
			return
		}
	}

	// Execute the write operation using the interface
	err = outputWriter.Write(outfile, i)
	if err != nil {
		log.Printf("Error writing output file: %v", err)
	}
}
