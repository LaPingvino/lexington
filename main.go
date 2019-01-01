// Lexington is a command line tool to convert between several formats used for screenwriting.
// The tool uses an internal format called lex which can be used to tweak the output.
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
)

func main() {
	input := flag.String("i", "-", "Input from provided filename")
	output := flag.String("o", "-", "Output to provided filename")
	from := flag.String("from", "fountain", "Input file type. Choose from fountain, fdx, lex")
	to := flag.String("to", "pdf", "Output file type. Choose from pdf, html, epub, mobi, docx, odt, fountain, fdx, lex")
	help := flag.Bool("help", false, "Show this help message")
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	var infile io.Reader
	var outfile io.Writer
	var i lex.Screenplay

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
		i = fountain.Parse(infile)
	default:
		log.Printf("%s is not a valid input type", *from)
	}

	log.Println("Output type is ", *to)
	switch *to {
	case "pdf":
		if *output == "-" {
			log.Println("Cannot write PDF to standard output")
			return
		}
		pdf.Create(*output, rules.Default, i)
	case "lex":
		lex.Write(i, outfile)
	default:
		log.Printf("%s is not a valid output type", *to)
	}
}
