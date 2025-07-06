// Lexington is a command line tool to convert between several formats used for screenwriting.
// When you write a screenplay in Fountain, you can use this tool to convert it into other formats.
// The tool uses an internal format called lex which can be used to tweak the output.
// Run the compiled tool with --help to get information about usage.
package main

import (
	"github.com/lapingvino/lexington/fdx"
	"github.com/lapingvino/lexington/fountain"
	"github.com/lapingvino/lexington/html"
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/linter" // New import for the linter
	"github.com/lapingvino/lexington/pdf"
	"github.com/lapingvino/lexington/rules"
	"github.com/lapingvino/lexington/writer" // New import for the pluggable template system

	"flag"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

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
	from := flag.String("from", "", "Input file type. Choose from fountain, lex, fdx. Formats between angle brackets are planned to be supported, but are not supported by this binary.")
	to := flag.String("to", "", "Output file type. Choose from pdf, lex, fountain, fdx, html, or external formats requiring pandoc: epub, mobi, docx, odt, rtf, md, latex.")
	lint := flag.Bool("lint", false, "Run the Fountain linter on the input file")
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
			// Exit here if linting errors are found and it's solely a linting run
			// Or continue if we want to allow generation despite lint errors.
			// For now, let's exit as requested.
			return
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
		// pdf.PDFWriter will encapsulate the logic for PDF generation.
		// It will ignore the io.Writer 'outfile' passed to its Write method for now,
		// and use the 'output' filename directly until the underlying PDF creation
		// supports writing to an io.Writer.
		if *output == "-" {
			log.Println("Cannot write PDF to standard output. Please provide an output filename (e.g., -o output.pdf).")
			return // Exit because PDF output to stdout is not supported.
		}
		// NOTE: pdf.PDFWriter needs to be defined in lexington/pdf/write.go (or similar)
		// and implement writer.Writer.
		outputWriter = &pdf.PDFWriter{OutputFile: *output, Elements: conf.Elements[*elements]}
	case "lex":
		// lex.LexWriter will encapsulate lex.Write.
		// NOTE: lex.LexWriter needs to be defined in lexington/lex/write.go
		// and implement writer.Writer.
		outputWriter = &lex.LexWriter{}
	case "fountain":
		// fountain.FountainWriter will encapsulate fountain.Write.
		// NOTE: fountain.FountainWriter needs to be defined in lexington/fountain/write.go
		// and implement writer.Writer. It will also need to handle SceneConfig.
		outputWriter = &fountain.FountainWriter{SceneConfig: conf.Scenes[*sceneout]}
	case "fdx":
		// fdx.FDXWriter will encapsulate fdx.Write.
		// NOTE: fdx.FDXWriter needs to be defined in lexington/fdx/write.go
		// and implement writer.Writer.
		outputWriter = &fdx.FDXWriter{}
	case "html":
		// html.HTMLWriter will encapsulate html.Write.
		// NOTE: html.HTMLWriter needs to be defined in lexington/html/write.go
		// and implement writer.Writer.
		outputWriter = &html.HTMLWriter{}
	default:
		log.Printf("%s is not a supported output type. Choose from: pdf, lex, fountain, fdx, html.", *to)
		return
	}

	// Check if an outputWriter was successfully assigned
	if outputWriter == nil {
		log.Println("Failed to initialize output writer. This should not happen for supported types.")
		return
	}

	// Execute the write operation using the interface
	err = outputWriter.Write(outfile, i)
	if err != nil {
		log.Printf("Error writing output file: %v", err)
	}
}
