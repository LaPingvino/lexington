// Lexington is a command line tool to convert between several formats used for screenwriting.
// When you write a screenplay in Fountain, you can use this tool to convert it into other formats.
// The tool uses an internal format called lex which can be used to tweak the output.
// Run the compiled tool with --help to get information about usage.
package main

import (
	"github.com/lapingvino/lexington/fountain"
	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/pdf"
	"github.com/lapingvino/lexington/rules"

	"flag"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

func main() {
	start := time.Now()
	defer func () {
		log.Printf("Conversion took %v", time.Since(start))
	}()
	config := flag.String("config", "lexington.toml", "Configuration file to use.")
	dump := flag.Bool("dumpconfig", false, "Dump the default configuration to the location of --config to be adapted manually.")
	scenein := flag.String("scenein", "", "Configuration to use for scene header detection on input.")
	sceneout := flag.String("sceneout", "", "Configuration to use for scene header detection on output.")
	elements := flag.String("e", "default", "Element settings from settings file to use.")
	input := flag.String("i", "-", "Input from provided filename. - means standard input.")
	output := flag.String("o", "-", "Output to provided filename. - means standard output.")
	from := flag.String("from", "", "Input file type. Choose from fountain, lex [, fdx]. Formats between angle brackets are planned to be supported, but are not supported by this binary.")
	to := flag.String("to", "", "Output file type. Choose from pdf (built-in, doesn't support inline markup), lex (helpful for troubleshooting and correcting fountain parsing), fountain, [html, epub*, mobi*, docx*, odt*, fdx]. Formats marked with a little star need an additional external tool to work. Formats between angle brackets are planned to be supported, but are not supported by this binary.")
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
	if len(ins)>1 && *from == "" {
		*from = ins[len(ins)-1]
	}
	if len(ins)>2 && *scenein == "" {
		*scenein = ins[len(ins)-2]
	}

	outs := strings.Split(*output, ".")
	if len(outs)>1 && *to == "" {
		*to = outs[len(outs)-1]
	}
	if len(outs)>2 && *sceneout == "" {
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
		infile, err = os.Open(*input)
		if err != nil {
			log.Println("Error opening file: ", err)
			return
		}
	}

	if *output == "-" {
		outfile = os.Stdout
		log.Println("Writing to Stdout")
	} else {
		if *output == "" && len(ins) > 0 && ins[0] != "" {
			*output = ins[0] + "." + *to
		}
		var err error
		outfile, err = os.Create(*output)
		if err != nil {
			log.Println("Error creating output file: ", err)
			return
		}
	}

	log.Println("Input type is ", *from)
	switch *from {
	case "lex":
		i = lex.Parse(infile)
	case "fountain":
		i = fountain.Parse(conf.Scenes[*scenein], infile)
	default:
		log.Printf("%s is not a valid input type", *from)
	}

	log.Println("Output type is ", *to)
	switch *to {
	case "pdf":
		if *output == "-" && len(ins) > 0 && ins[0] != "" {
			*output = ins[0] + ".pdf"
		}
		if *output == "-" {
			log.Println("Cannot write PDF to standard output")
			return
		}
		pdf.Create(*output, conf.Elements[*elements], i)
	case "lex":
		lex.Write(i, outfile)
	case "fountain":
		fountain.Write(outfile, conf.Scenes[*sceneout], i)
	default:
		log.Printf("%s is not a valid output type", *to)
	}
}
