Lexington commandline tool for screenwriters
============================================

Lexington helps you convert between Final Draft, Fountain and its own lex file formats, and output to PDF, HTML and ebook formats.

At the moment the Fountain parser should be pretty decent, although still lacking features like simultaneous dialog, and inline markup is not yet supported and might never be. Also the PDF output doesn't do anything to keep pieces from your screenplay together. Feel free to contribute and help me out in knowing how best to handle this!

To run the tool, make sure to have Go installed first and then run

`go install github.com/lapingvino/lexington@latest`

to install Lexington to your go/bin directory. If this directory is in your execution path, you can then run it like

`lexington -i inputfile.fountain -o outputfile.pdf -to pdf`

For a more complete overview of the command line options, use `lexington -help`.
