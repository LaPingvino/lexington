// Package cli is Lexington's command line, for the lexington command and
// for programs that offer the same conversions (Accolade, a graphical
// Lexington): Main runs it with arguments, Convert and Write run a
// conversion with Options, and Formats are the output formats.
package cli

import (
	"bytes"
	"cmp"
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

	"github.com/LaPingvino/lexington/bundle"
	"github.com/LaPingvino/lexington/celtx"
	"github.com/LaPingvino/lexington/fdx"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/html"
	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/kitsp"
	"github.com/LaPingvino/lexington/latex"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/linter"
	"github.com/LaPingvino/lexington/markdown"
	"github.com/LaPingvino/lexington/office"
	"github.com/LaPingvino/lexington/osf"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/pdfin"
	"github.com/LaPingvino/lexington/report"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
	"github.com/LaPingvino/lexington/trelby"
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
	{Key: "fadein", Name: "Fade In"},
	{Key: "osf", Name: "Open Screenplay Format (OSF)"},
	{Key: "trelby", Name: "Trelby"},
	{Key: "kitsp", Name: "KIT Scenarist"},
	{Key: "celtx", Name: "Celtx"},
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

func inputKeys() string {
	var keys []string
	for _, f := range Inputs {
		keys = append(keys, f.Key)
	}
	return strings.Join(keys, ", ")
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
	ConfigFile   string          // the settings file (lexington.toml)
	Conf         *rules.TOMLConf // if not nil, the settings themselves, in place of ConfigFile's
	FormatDir    string          // folders of format (preset) files of one's own, as a path list, or ""
	SceneIn      string          // the scene heading language of the input ("en")
	SceneOut     string          // and of the output
	Direction    string          // "rtl" or "ltr" for the script's lines with no direction of their own; "" for its language's (or its title page's Direction:)
	Elements     string          // the element settings: a preset or a set from the settings file
	Page         string          // paper for pdf, docx and odt: letter, a4, a5; "" for the preset's
	Input        string          // file name, "-" for standard input
	Output       string          // file name, "-" for standard output
	From         string          // input format (Inputs' keys: fountain, fdx, pdf, fadein, osf, trelby, kitsp, celtx, lex)
	To           string          // output format (Formats)
	TemplatePath string          // a template for HTML, FDX or LaTeX
	Lint         bool            // run the Fountain linter
	// Script is the script to read of a project with several (a Celtx
	// project): its number from 1 or its title; "" the first.
	Script string
	// Loose reads Fountain input as Lexington did before 1.19
	// (fountain.ParseLoose), not as the spec has it.
	Loose bool
	// KeepLoose writes the Fountain input, as Fountain, so that it reads
	// under the spec as Lexington before 1.19 read it (fountain.KeepLoose:
	// blank lines and force marks added), instead of converting it.
	KeepLoose bool
	// Strip is what to take out of the Fountain input before anything
	// else (fountain.ParseStrip: a comma list of options and presets,
	// "clean" for Accolade's data only); "" takes nothing out. Fountain
	// out is the stripped text as it is, not rewritten.
	Strip string
	// Colors prints scene headings and sections in their notes' colors
	// (rules.Set.WithColors).
	Colors bool
	// ColorStyle is how colors show (lex.ColorStyle: line, marker,
	// background, text); giving one switches colors on.
	ColorStyle string
	// ColorScope is what a marker, background or text color covers: heading (the
	// default) or block (the scene, or the section whole, with the color).
	ColorScope string
	// ColorTint puts a light tint under a line's block. A scope or a tint
	// switches colors on too.
	ColorTint bool
	// ColorPatterns draws a line's bar in its color's pattern
	// (lex.BarPattern), for those who tell colors apart badly.
	ColorPatterns bool
	// Pagination is how pdf, docx, odt and printed html break pages
	// (rules.ParsePagination): "all" (keep headings, names and
	// parentheticals with what follows, transitions with what is before,
	// two lines of a paragraph on each side of a break, speeches broken
	// with (MORE) and CONT'D), "none", or some of keep, widows and more
	// joined with "+". "" is the format's own (all, unless its settings
	// say otherwise).
	Pagination string
	// Tags keep only the scenes and sections with one of them as a note
	// (lex.FilterTags).
	Tags []string
	// Character and Sides keep a character's sides (lex.Sides): Sides is
	// full, cue or isolated, needed with a Character; CueWords, for cue,
	// the last words of the speech before (0: all of it).
	Character string
	Sides     string
	CueWords  int
	// Number numbers the scenes without a number (lex.NumberScenes):
	// "count" on from the last whole number, "10A" or "A11" as a locked
	// script does (lex.NumberStyle); "" leaves them.
	Number string
	// Report writes a production report as CSV (report.Kinds: scenes,
	// locations, characters, cast) instead of the script.
	Report string
	// Revisions prints the lines' revisions (lex.Line.Revision): the PDF's
	// marks, changed pages' headers, those pages only or on coloured
	// paper; the FDX's <Revisions> and RevisionIDs. nil: none.
	Revisions *lex.Revisions
	// Lock keeps the locked pages in the PDF (A-pages for what grew) and
	// writes them as the FDX's <LockedPages>; LockFile is a file it is
	// read from (lex.ParsePageLock). LockPages writes, instead of the
	// script, the lock of its pages as the PDF prints them (with a lock:
	// locked again, its A-pages kept).
	Lock      *lex.PageLock
	LockFile  string
	LockPages bool
	// Continueds prints the scenes' CONTINUEDs in the PDF (a lock's own
	// setting wins).
	Continueds bool
	// Breakdown is the script's breakdown categories and elements, and
	// LineTags the words tagged as them by the rows' Source line
	// (fountain.ParseLines): the FDX's <TagData> and TagNumbers.
	Breakdown *lex.TagSet
	LineTags  map[int][]lex.LineTag
	// SceneLengths writes each scene's page and length in eighths in the
	// FDX's SceneProperties, as the PDF prints it with these settings.
	SceneLengths bool
	// MergeOmitted prints a run of OMITTED headings as one, "12-14
	// OMITTED 12-14" (lex.MergeOmittedRuns), in every format but FDX,
	// Fountain and lex, which keep one heading a number; the pages, the
	// lock and the measures are the merged PDF's.
	MergeOmitted bool
	// Record, writing a PDF, is filled with its pages as drawn
	// (pdf.Recorder): what a program shows of it without reading it.
	Record *pdf.Drawing
	// Resume, writing a PDF, draws its pages into Record from a page's
	// start (pdf.Resume): no PDF is made, Output is not written.
	Resume *pdf.Resume
	// Pictures, writing a PDF, prints the storyboard frames under their
	// lines: the pictures' files by the name their board notes give
	// (pdf.PDFWriter.Pictures; nil: not printed).
	Pictures map[string][]byte
	// OSFVersion is the Open Screenplay Format version an osf or fadein
	// is written in: 4.0 (""), 2.0 or 1.2.
	OSFVersion string
	// BundleForm is the form a bundle written (-o x.highland, .accolade,
	// .textbundle, .textpack) takes: "zip", "folder", or "" for the
	// bundle's own when it is rewritten, else its type's (bundle.FormOf).
	BundleForm string
	// IncludeFrom is the file Highland's {{INCLUDE: file}}s are relative
	// to ("": Input; neither, the working folder). Every output that
	// prints the script puts the files in (ResolveIncludes).
	IncludeFrom string
	// Header and Footer are a {{HEADER: …}} and {{FOOTER: …}}'s value
	// for a script that has none of its own (lex/highland.go): its own
	// wins.
	Header, Footer string
	// TitlePage is the title page template's name (rules.TOMLConf
	// TitlePage: one's own or a preset, standard, bold, poster); "" the
	// standard one, Fountain's.
	TitlePage string
	// TitlePictures are the files the title page's pictures name, by
	// their names (Picture: art.png). nil: read from beside Input, or
	// from its bundle (as it is, or in assets/).
	TitlePictures map[string][]byte

	in      []byte         // the input's text, read from a bundle (nil: Input's)
	src     *bundle.Bundle // the bundle the input was read from
	tpl     *titlepage.Template
	sink    *bytes.Buffer // where the output goes, to be put in a bundle
	carried *carried      // the settings an .accolade input brought (cli/bundle.go)
}

// colorScheme is the color scheme o asks for, on the settings' own.
func colorScheme(o Options, sch lex.ColorScheme) (lex.ColorScheme, error) {
	var err error
	if o.ColorStyle != "" {
		if sch, err = lex.ParseColorScheme(o.ColorStyle); err != nil {
			return sch, err
		}
	}
	switch strings.ToLower(o.ColorScope) {
	case "":
	case "heading":
		sch.Block = false
	case "block":
		sch.Block = true
	default:
		return sch, fmt.Errorf("no color scope %q: choose heading or block", o.ColorScope)
	}
	if o.ColorTint {
		sch.Tint = true
	}
	if o.ColorPatterns {
		sch.Patterns = true
	}
	return lex.ParseColorScheme(sch.String()) // a combination that is one
}

// tagList is a flag that can be given more than once.
type tagList []string

func (t *tagList) String() string     { return strings.Join(*t, ", ") }
func (t *tagList) Set(v string) error { *t = append(*t, v); return nil }

// filter applies o's filters to a screenplay, made ready to print
// (forPrint): the tags, then the sides.
func filter(o Options, s lex.Screenplay) (lex.Screenplay, error) {
	s = forPrint(o, s) // its includes, its header and footer
	if (o.Character == "") != (o.Sides == "") {
		return nil, errors.New("give a character's sides as -character NAME with -sides full, cue or isolated")
	}
	switch strings.ToLower(o.Number) {
	case "":
	case "count", "yes", "true":
		s = lex.NumberScenes(s, lex.NumberOptions{})
	case "10a", "a11", "locked":
		s = lex.NumberScenes(s, lex.NumberOptions{Locked: true, Style: lex.ParseNumberStyle(o.Number)})
	default:
		return nil, fmt.Errorf("-number %s: give count, 10A or A11", o.Number)
	}
	if len(o.Tags) > 0 {
		s = lex.FilterTags(s, o.Tags...)
	}
	if o.Character != "" {
		mode, err := lex.ParseSidesMode(o.Sides)
		if err != nil {
			return nil, err
		}
		s = lex.Sides(s, o.Character, mode, o.CueWords)
	}
	return s, nil
}

// Main runs the command line with args (without the program's name) and
// returns the exit code.
func Main(ctx context.Context, args []string) int {
	if len(args) > 0 && args[0] == "check" {
		return checkBundles(args[1:])
	}
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

// flagState is what the flags set: the options, and the flags that are
// not a conversion's.
type flagState struct {
	o                                        Options
	dump, help, presets, titlePages, version bool
	scripts                                  bool
}

// newFlagSet is the command line's flags, set into st.
func newFlagSet(st *flagState) *flag.FlagSet {
	fs := flag.NewFlagSet("lexington", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: lexington [flags] IN [OUT], or lexington -i IN -o OUT [flags]: each format as its name, else (IN) its content, says")
		fs.PrintDefaults()
	}
	fs.StringVar(&st.o.ConfigFile, "config", "lexington.toml", "Configuration file to use.")
	fs.StringVar(&st.o.FormatDir, "formats", "", "Folders of format files of your own (name.toml, as Lexington's presets), as a path list, to use with -e.")
	fs.BoolVar(&st.dump, "dumpconfig", false,
		"Dump the default configuration to the location of --config to be adapted manually.")
	fs.StringVar(&st.o.SceneIn, "scenein", "", "Configuration to use for scene header detection on input.")
	fs.StringVar(&st.o.SceneOut, "sceneout", "", "Configuration to use for scene header detection on output.")
	fs.StringVar(&st.o.Elements, "e", "",
		"Element settings to use: a preset (see -presets) or a set from the settings file "+
			"(default: an .accolade's own format, else default).")
	fs.StringVar(&st.o.Page, "page", "",
		"Paper size for pdf, docx and odt: letter, a4 or a5 (default: the preset's suggestion, else letter).")
	fs.StringVar(&st.o.Header, "header", "",
		"The page's header for a script with no {{HEADER: …}} of its own, as Highland writes one: \"Title | %p.\" (%p the page, %pp the last page, %# the section; left | centre | right).")
	fs.StringVar(&st.o.Footer, "footer", "", "The page's footer for a script with no {{FOOTER: …}} of its own, as -header.")
	fs.BoolVar(&st.presets, "presets", false, "List the presets for -e (stage play, radio, ...).")
	fs.StringVar(&st.o.TitlePage, "titlepage", "",
		"The title page template: standard (Fountain's, the default), bold, poster, or one of the [TitlePages] in the settings file (see -titlepages).")
	fs.BoolVar(&st.titlePages, "titlepages", false, "List the title page templates for -titlepage.")
	fs.StringVar(&st.o.Input, "i", "-", "Input from provided filename. - means standard input. "+
		"A bundle (.accolade, .highland, .textbundle, .textpack, zipped or a folder) is read for its text; a .fadein too, zipped or a folder.")
	fs.StringVar(&st.o.Output, "o", "-", "Output to provided filename. - means standard output. "+
		"A bundle (.accolade, .highland, .textbundle, .textpack) gets the script as Fountain, all else in it kept; a .fadein its OSF document.")
	fs.StringVar(&st.o.BundleForm, "form", "",
		"The form of a bundle written: zip or folder (default: the bundle's own when rewritten, else zip, a .textbundle a folder).")
	fs.StringVar(&st.o.From, "from", "", "Input file type: "+inputKeys()+" (default: as the input's name, else its content, says).")
	fs.StringVar(&st.o.Script, "script", "",
		"The script to read of a Celtx project with several (a screenplay and its stage play): its number from 1 or its title (default: the first; -scripts lists them).")
	fs.BoolVar(&st.scripts, "scripts", false, "List the scripts of the Celtx project IN, for -script.")
	fs.StringVar(&st.o.OSFVersion, "osfversion", "",
		"The Open Screenplay Format version osf and fadein are written in: 5.0 (the default, Fade In 5's), 4.0, 2.0 or 1.2.")
	fs.StringVar(&st.o.To, "to", "",
		"Output file type. Choose from "+formatKeys(false)+", or external formats requiring other programs: "+
			formatKeys(true)+".")
	fs.BoolVar(&st.o.Lint, "lint", false, "Run the Fountain linter on the input file")
	fs.BoolVar(&st.o.Loose, "loose", false,
		"Read Fountain as Lexington before 1.19 did: a scene word starts a heading anywhere, capitals after action are a name, any line ending in \" TO:\" is a transition, a line of spaces ends a speech.")
	fs.BoolVar(&st.o.KeepLoose, "keeploose", false,
		"Write the Fountain input so that it reads under the spec as Lexington before 1.19 read it (blank lines and force marks added), instead of converting it; says how many places changed.")
	fs.StringVar(&st.o.Strip, "strip", "",
		"Take out of the Fountain input, as a comma list: Accolade's data always (its end blocks, the language line, [[from: …]] links), and "+
			stripHelp()+". Fountain out (the default with -strip) is the text as it was, less what went.")
	fs.BoolVar(&st.o.Colors, "colors", false,
		"Print scene headings and sections in the colors of their notes ([[red]], [[COLOR BLUE]], [[#3366ff]]).")
	fs.StringVar(&st.o.ColorStyle, "colorstyle", "",
		"How -colors shows a color: line (a bar beside the scene, the default), marker, background or text (or with -colorscope, -colortint: marker+block, line+tint, ...); switches colors on.")
	fs.StringVar(&st.o.ColorScope, "colorscope", "",
		"What a marker, background or text color covers: heading (the default) or block (the scene, or a section whole, with the color).")
	fs.BoolVar(&st.o.ColorTint, "colortint", false, "With the line style: a light tint under the block too.")
	fs.BoolVar(&st.o.ColorPatterns, "colorpatterns", false,
		"With the line style: each color's bar in a pattern of its own (dashes, dots, double, ...), not told apart by hue alone.")
	fs.StringVar(&st.o.Pagination, "pagination", "",
		"How pages break in pdf, docx, odt and printed html: all (the default: headings, names and parentheticals never last on a page, transitions never first, two lines of a paragraph on each side of a break, speeches broken after a sentence with (MORE) and CONT'D), none, or keep, widows and more joined with + (keep+widows).")
	fs.Var((*tagList)(&st.o.Tags), "tag",
		"Keep only the scenes and sections with this note ([[London]]) or color, with all under them; give it more than once for any of several.")
	fs.StringVar(&st.o.Character, "character", "", "Keep this character's sides; needs -sides.")
	fs.StringVar(&st.o.Sides, "sides", "",
		"With -character: full (the scenes they speak in), cue (their speeches after their cues) or isolated (their speeches only).")
	fs.IntVar(&st.o.CueWords, "cuewords", 0, "With -sides cue: the cue's last words only, this many (0: the whole speech before).")
	fs.StringVar(&st.o.Number, "number", "",
		"Number the scenes without a number: count (on from the last whole number), or as a locked script, keeping the numbers there are: 10A (a new scene after 10 is 10A, the default style) or A11 (before 11).")
	fs.StringVar(&st.o.Report, "report", "",
		"Write a report as CSV instead of the script: scenes (number, heading, page, length in eighths, who speaks), locations, characters or cast (numbered by size). Pages are measured as -e and -page print them.")
	fs.StringVar(&st.o.LockFile, "lock", "",
		"Keep the locked pages in this file (as -lockpages writes them): each page's breaks and number, A-pages (12A) for text that grew; the FDX gets them as <LockedPages>.")
	fs.BoolVar(&st.o.LockPages, "lockpages", false,
		"Write the lock of the script's pages as the PDF prints them (with -lock: locked again, A-pages kept), instead of the script.")
	fs.BoolVar(&st.o.Continueds, "continueds", false,
		"Print the scenes' CONTINUEDs in the PDF: (CONTINUED) at the foot of a page a scene goes on from, CONTINUED: at the top of the next.")
	fs.BoolVar(&st.o.MergeOmitted, "mergeomitted", false,
		"Print a run of OMITTED scenes as one heading, 12-14 OMITTED (not in FDX, Fountain or lex, which keep one a number).")
	fs.BoolVar(&st.o.SceneLengths, "lengths", false,
		"Write each scene's page and length in eighths into the FDX (SceneProperties Page and Length), measured as -e and -page print them.")
	fs.StringVar(&st.o.TemplatePath, "template", "",
		"Path to a custom template file (e.g., for HTML, FDX, or LaTeX output).")
	fs.BoolVar(&st.help, "help", false, "Show this help message")
	fs.BoolVar(&st.version, "version", false, "Show version information")
	return fs
}

// parseFlags reads the arguments; ok is false when there is nothing to
// convert (help, presets, version, dumped settings). The files can be
// given as -i and -o, or plainly, IN [OUT], with the flags before,
// between or after them; not both ways at once.
func parseFlags(args []string) (o Options, ok bool, err error) {
	st := &flagState{}
	fs := newFlagSet(st)
	var files []string
	for {
		if err := fs.Parse(args); err != nil {
			return st.o, false, err
		}
		if fs.NArg() == 0 {
			break
		}
		files, args = append(files, fs.Arg(0)), fs.Args()[1:]
	}
	if len(files) > 0 {
		named := false
		fs.Visit(func(f *flag.Flag) { named = named || f.Name == "i" || f.Name == "o" })
		switch {
		case named:
			err = fmt.Errorf("give the files as IN [OUT] or as -i and -o, not both: %s", strings.Join(files, " "))
		case len(files) > 2:
			err = fmt.Errorf("one input and one output, not %d files: %s", len(files), strings.Join(files, " "))
		}
		if err != nil {
			fmt.Fprintln(fs.Output(), err)
			return st.o, false, err
		}
		st.o.Input = files[0]
		if len(files) == 2 {
			st.o.Output = files[1]
		}
	}
	return st.after(fs)
}

// after does what the flags that are no conversion's ask.
func (st *flagState) after(fs *flag.FlagSet) (Options, bool, error) {
	o := st.o
	switch {
	case st.help:
		fs.Usage()
	case st.presets:
		list, err := rules.PresetsIn(filepath.SplitList(o.FormatDir)...)
		if err != nil {
			log.Print(err)
		}
		for _, p := range list {
			fmt.Printf("%-12s %s\n%-12s %s\n\n", p.Key, p.Name, "", p.Description)
		}
	case st.titlePages:
		conf := rules.GetConf(o.ConfigFile)
		for _, k := range conf.TitlePageNames() {
			t, _ := conf.TitlePage(k)
			fmt.Printf("%-12s %s\n%-12s %s\n\n", k, cmp.Or(t.Name, k), "", t.Description)
		}
	case st.scripts:
		if err := listScripts(o.Input); err != nil {
			return o, false, err
		}
	case st.version:
		log.Printf("Lexington version %s (commit: %s, built: %s)", Version, Commit, Date)
	case st.dump:
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
	if o.ConfigFile == "" {
		o.ConfigFile = "lexington.toml"
	}
	src, err := readBundleInput(&o)
	if err != nil {
		return err
	}
	if o.Output == "" && o.KeepLoose && src != nil {
		o.Output = o.Input // back into the bundle
	}
	if isBundleOutput(o.Output) {
		return convertToBundle(ctx, o, src)
	}
	if src != nil && src.Kind() != bundle.Fountain {
		return fmt.Errorf("%s holds %s (%s), not a screenplay: Lexington can only repack it into another bundle",
			filepath.Base(o.Input), src.Kind(), src.Text)
	}
	return convert(ctx, o)
}

func convert(ctx context.Context, o Options) error {
	conf := confOf(o)
	detectFormats(&o, conf)
	if err := sniffInput(&o); err != nil {
		return err
	}
	if err := openFadeIn(&o); err != nil {
		return err
	}
	setDefaults(&o)
	if _, ok := conf.Elements[o.Elements]; !ok && o.carried != nil && o.carried.chosen {
		// one of Accolade's formats (TV series: the screenplay with
		// buttons), which an .accolade does not carry
		log.Printf("%s: its format %q is Accolade's own; printed as the screenplay", filepath.Base(o.Input), o.Elements)
		o.Elements = rules.ScreenplayPreset
	}
	if _, ok := conf.Elements[o.Elements]; !ok {
		return fmt.Errorf("no element settings %q in %s or the presets (see -presets)", o.Elements, o.ConfigFile)
	}
	in, closeIn, err := openInput(o)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := closeIn(); cerr != nil {
			log.Printf("Closing the input: %v", cerr)
		}
	}()
	if o.Strip != "" {
		stripped, err := stripInput(o, conf, in)
		if err != nil {
			return err
		}
		if o.KeepLoose || o.To == internal.FormatFountain {
			return writeFountain(o, conf, stripped)
		}
		in = strings.NewReader(stripped)
	} else if o.KeepLoose {
		return keepLoose(o, conf, in)
	}
	screenplay, err := parseInput(o, conf, in)
	if err != nil {
		return err
	}
	if o.Direction == "" {
		o.Direction = lex.TitleDirection(screenplay) // Hekaya's Direction: rtl
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
	if isBundleOutput(o.Output) {
		return writeToBundle(ctx, screenplay, o)
	}
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
	screenplay, err := filter(o, screenplay)
	if err != nil {
		return err
	}
	if o.Lock == nil && o.LockFile != "" {
		data, err := os.ReadFile(o.LockFile)
		if err != nil {
			return err
		}
		if o.Lock = lex.ParsePageLock(string(data)); o.Lock == nil {
			return fmt.Errorf("no locked pages in %s", o.LockFile)
		}
	}
	if err := titlePageFor(&o, conf, screenplay); err != nil {
		return err
	}
	if o.Report != "" {
		return writeReport(o, conf, screenplay)
	}
	if o.LockPages {
		return writeLock(o, conf, screenplay)
	}
	if o.Colors || o.ColorStyle != "" || o.ColorScope != "" || o.ColorTint || o.ColorPatterns {
		// the element settings with colors on, for every writer
		set := conf.Elements[o.Elements]
		sch, err := colorScheme(o, set.ColorScheme())
		if err != nil {
			return err
		}
		elements := map[string]rules.Set{}
		for k, set := range conf.Elements {
			elements[k] = set
		}
		elements[o.Elements] = set.WithColorScheme(sch)
		conf.Elements = elements
	}
	if o.Pagination != "" {
		p, err := rules.ParsePagination(o.Pagination)
		if err != nil {
			return err
		}
		elements := map[string]rules.Set{}
		for k, set := range conf.Elements {
			elements[k] = set
		}
		elements[o.Elements] = conf.Elements[o.Elements].WithPagination(p)
		conf.Elements = elements
	}
	f, ok := FormatFor(o.To)
	if !ok {
		return fmt.Errorf("%s is not a supported output type; choose from %s, or with other programs %s",
			o.To, formatKeys(false), formatKeys(true))
	}
	if mergesOmitted(o) {
		screenplay = lex.MergeOmittedRuns(screenplay)
	}
	if f.Needs != "" {
		return external(o, conf, screenplay)
	}
	w, err := createWriter(o, conf)
	if err != nil {
		return err
	}
	if fw, ok := w.(*fdx.FDXWriter); ok && o.SceneLengths {
		lw, err := lockWriter(o, conf)
		if err != nil {
			return err
		}
		for _, l := range lw.Measure(screenplay) {
			fw.Scenes = append(fw.Scenes, fdx.SceneProperty{Page: l.Page, Length: l.Eighths})
		}
	}
	if fw, ok := w.(*fdx.FDXWriter); ok && usesLastPage(screenplay) { // a header's %pp
		lw, err := lockWriter(o, conf)
		if err != nil {
			return err
		}
		if pages := lw.Pages(screenplay); len(pages) > 0 {
			fw.LastPage = pages[len(pages)-1].Label
		}
	}
	if o.Resume != nil && o.To == internal.FormatPDF {
		return w.Write(nil, screenplay) // the drawing only
	}
	if o.To == "fadein" && o.Output != "-" && o.sink == nil {
		return writeFadeIn(o, screenplay) // its other files kept, a zip or a folder
	}
	out, closeOut, err := openOutput(o)
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

// Report makes the screenplay's production reports, its pages measured as
// o's PDF would print them (its element settings, paper and pagination:
// ro.PDF, when not given), its headings read in o.SceneIn's language
// (ro.Scenes, when not given).
func Report(screenplay lex.Screenplay, o Options, ro report.Options) (report.Report, error) {
	setDefaults(&o)
	conf := confOf(o)
	if _, ok := conf.Elements[o.Elements]; !ok {
		return report.Report{}, fmt.Errorf("no element settings %q in %s or the presets", o.Elements, o.ConfigFile)
	}
	screenplay, err := filter(o, screenplay)
	if err != nil {
		return report.Report{}, err
	}
	return makeReport(o, conf, screenplay, ro)
}

func makeReport(o Options, conf rules.TOMLConf, screenplay lex.Screenplay, ro report.Options) (report.Report, error) {
	if ro.PDF == nil {
		elements := conf.Elements[o.Elements]
		if o.Pagination != "" {
			p, err := rules.ParsePagination(o.Pagination)
			if err != nil {
				return report.Report{}, err
			}
			elements = elements.WithPagination(p)
		}
		ro.PDF = &pdf.PDFWriter{Elements: elements, Page: pageFor(o), MergeOmitted: o.MergeOmitted, RTL: rtlFor(conf, o), Language: fontLanguage(o)}
	}
	if ro.Scenes == nil {
		ro.Scenes = conf.Scenes(o.SceneIn)
	}
	return report.Make(screenplay, ro), nil
}

// writeLock writes the lock of the script's pages to o.Output.
func writeLock(o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	w, err := lockWriter(o, conf)
	if err != nil {
		return err
	}
	text := w.LockPages(screenplay).Text() + "\n"
	if o.sink != nil {
		_, err := o.sink.WriteString(text)
		return err
	}
	if o.Output == "-" || o.Output == "" {
		_, err := os.Stdout.WriteString(text)
		return err
	}
	return os.WriteFile(o.Output, []byte(text), 0o644)
}

// lockWriter is the PDF writer that lays the script out as o prints it,
// to lock or list its pages.
func lockWriter(o Options, conf rules.TOMLConf) (*pdf.PDFWriter, error) {
	elements := conf.Elements[o.Elements]
	if o.Pagination != "" {
		p, err := rules.ParsePagination(o.Pagination)
		if err != nil {
			return nil, err
		}
		elements = elements.WithPagination(p)
	}
	return &pdf.PDFWriter{Elements: elements, Page: pageFor(o), Lock: o.Lock, Continueds: o.Continueds,
		Revisions: o.Revisions, MergeOmitted: o.MergeOmitted, RTL: rtlFor(conf, o), Language: fontLanguage(o)}, nil
}

// LockPages are the pages of a screenplay as the PDF prints it with o's
// settings, locked (with o.Lock: locked again, its A-pages kept).
func LockPages(screenplay lex.Screenplay, o Options) (lex.PageLock, error) {
	w, err := pagesWriter(&o)
	if err != nil {
		return lex.PageLock{}, err
	}
	screenplay, err = filter(o, screenplay)
	if err != nil {
		return lex.PageLock{}, err
	}
	return w.LockPages(screenplay), nil
}

// Pages are the pages of a screenplay as the PDF prints it with o's
// settings: their numbers (a lock's) and revisions.
func Pages(screenplay lex.Screenplay, o Options) ([]pdf.Page, error) {
	w, err := pagesWriter(&o)
	if err != nil {
		return nil, err
	}
	screenplay, err = filter(o, screenplay)
	if err != nil {
		return nil, err
	}
	return w.Pages(screenplay), nil
}

func pagesWriter(o *Options) (*pdf.PDFWriter, error) {
	setDefaults(o)
	conf := confOf(*o)
	if _, ok := conf.Elements[o.Elements]; !ok {
		return nil, fmt.Errorf("no element settings %q in %s or the presets", o.Elements, o.ConfigFile)
	}
	return lockWriter(*o, conf)
}

// writeReport writes o.Report as CSV to o.Output.
func writeReport(o Options, conf rules.TOMLConf, screenplay lex.Screenplay) error {
	r, err := makeReport(o, conf, screenplay, report.Options{})
	if err != nil {
		return err
	}
	t, err := r.Table(strings.ToLower(o.Report))
	if err != nil {
		return err
	}
	out, closeOut, err := openOutput(o)
	if err != nil {
		return err
	}
	if err := t.WriteCSV(out); err != nil {
		return errors.Join(err, closeOut())
	}
	return closeOut()
}

// PDFWarning says what a PDF of the screenplay cannot print as written,
// and what to do instead; "" when it prints well.
func PDFWarning(s lex.Screenplay) string {
	missing, _ := pdf.Unsupported(s)
	scripts := pdf.Unshaped(s)
	var why []string
	if len(missing) > 0 {
		shown := missing
		if len(shown) > 12 {
			shown = shown[:12]
		}
		why = append(why, fmt.Sprintf("no font on this computer has %s (%d characters), which print as blanks", string(shown), len(missing)))
	}
	if len(scripts) > 0 {
		why = append(why, fmt.Sprintf("its %s letters that join would print unjoined: the PDF cannot shape them", strings.Join(scripts, ", ")))
	}
	if len(why) == 0 {
		return ""
	}
	return "The PDF cannot print this script as written: " + strings.Join(why, "; and ") +
		". Export HTML and print it from a browser (Save as PDF) instead."
}

// detectFormats reads the formats (and scene languages) from the file
// names alone: script.en.fountain — a dot in a folder's name is no part of it.
func detectFormats(o *Options, conf rules.TOMLConf) {
	defer func() {
		if o.Strip != "" && o.To == "" {
			o.To = internal.FormatFountain // a clean copy of the script
		}
	}()
	if lang, ok := conf.ExtensionLanguage(filepath.Base(o.Input)); ok {
		// a language's own extension ([Extensions]: film.hekaya is
		// Fountain in Hekaya's Arabic)
		if o.From == "" {
			o.From = internal.FormatFountain
		}
		if o.SceneIn == "" {
			o.SceneIn = lang
		}
	} else if ins := strings.Split(filepath.Base(o.Input), "."); len(ins) > 1 {
		if o.From == "" {
			o.From = ins[len(ins)-1]
		}
		if len(ins) > 2 && o.SceneIn == "" {
			o.SceneIn = ins[len(ins)-2]
		}
	}
	if lang, ok := conf.ExtensionLanguage(filepath.Base(o.Output)); ok {
		if o.To == "" {
			o.To = internal.FormatFountain
		}
		if o.SceneOut == "" {
			o.SceneOut = lang
		}
	} else if outs := strings.Split(filepath.Base(o.Output), "."); len(outs) > 1 {
		if o.To == "" {
			o.To = outs[len(outs)-1]
		}
		if len(outs) > 2 && o.SceneOut == "" {
			o.SceneOut = outs[len(outs)-2]
		}
	}
}

// rtlFor tells whether the script's lines with no direction of their own
// run right to left: o.Direction, else its language's.
func rtlFor(conf rules.TOMLConf, o Options) bool {
	switch strings.ToLower(strings.TrimSpace(o.Direction)) {
	case "rtl":
		return true
	case "ltr":
		return false
	}
	return conf.RTL(o.SceneIn)
}

// fontLanguage is the language the PDF asks the computer's fonts in
// (pdf.PDFWriter.Language): the script's, "" for the standard one
// (English, Fountain's own), which leaves the choice of a Han face to the
// computer's default.
func fontLanguage(o Options) string {
	if o.SceneIn == "en" {
		return ""
	}
	return o.SceneIn
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

func openInput(o Options) (io.Reader, func() error, error) {
	if o.in != nil {
		return bytes.NewReader(o.in), func() error { return nil }, nil
	}
	name := o.Input
	if name == "-" || name == "" {
		return os.Stdin, func() error { return nil }, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

func openOutput(o Options) (io.Writer, func() error, error) {
	if o.sink != nil {
		return o.sink, func() error { return nil }, nil
	}
	name := o.Output
	if name == "-" {
		return os.Stdout, func() error { return nil }, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// keepLoose writes the Fountain input as fountain.KeepLoose has it, to
// o.Output (the input itself when it is "": read whole first).
func keepLoose(o Options, conf rules.TOMLConf, in io.Reader) error {
	if o.From != internal.FormatFountain {
		return fmt.Errorf("-keeploose reads Fountain, not %s", o.From)
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	text := string(data)
	crlf := strings.Contains(text, "\r\n") // the lines added end as the others do
	kept, places := fountain.KeepLooseIn(strings.ReplaceAll(text, "\r\n", "\n"), conf.Language(o.SceneIn))
	if crlf {
		kept = strings.ReplaceAll(kept, "\n", "\r\n")
	}
	if o.Output == "" {
		o.Output = o.Input
	}
	out, closeOut, err := openOutput(o)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(out, kept); err != nil {
		return errors.Join(err, closeOut())
	}
	if places == 0 {
		log.Print("The script reads the same under the spec: nothing changed")
	} else {
		log.Printf("Changed %d places so the script reads as before 1.19", places)
	}
	return closeOut()
}

// stripInput is the Fountain input as fountain.Strip has it, with what
// o.Strip says taken out.
func stripInput(o Options, conf rules.TOMLConf, in io.Reader) (string, error) {
	if o.From != internal.FormatFountain {
		return "", fmt.Errorf("-strip reads Fountain, not %s", o.From)
	}
	so, err := fountain.ParseStrip(o.Strip)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return "", err
	}
	return fountain.StripIn(string(data), conf.Language(o.SceneIn), so), nil
}

// writeFountain writes the stripped input to o.Output as it is (with
// -keeploose as fountain.KeepLoose has it): a copy, never over the input.
func writeFountain(o Options, conf rules.TOMLConf, text string) error {
	if o.Output == "" || (o.Input != "-" && o.Output != "-" && sameFile(o.Input, o.Output)) {
		return errors.New("-strip writes a copy: give -o another file than the input")
	}
	if o.KeepLoose {
		crlf := strings.Contains(text, "\r\n")
		kept, places := fountain.KeepLooseIn(strings.ReplaceAll(text, "\r\n", "\n"), conf.Language(o.SceneIn))
		if crlf {
			kept = strings.ReplaceAll(kept, "\n", "\r\n")
		}
		text = kept
		log.Printf("Changed %d places so the script reads as before 1.19", places)
	}
	out, closeOut, err := openOutput(o)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(out, text); err != nil {
		return errors.Join(err, closeOut())
	}
	return closeOut()
}

// sameFile reports whether two names are one file.
func sameFile(a, b string) bool {
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	aa, _ := filepath.Abs(a)
	ab, _ := filepath.Abs(b)
	return aa == ab
}

// stripHelp lists -strip's presets and options for -help.
func stripHelp() string {
	var parts []string
	for _, p := range fountain.StripPresets {
		parts = append(parts, p.Key+" ("+p.Help+")")
	}
	var keys []string
	for _, k := range fountain.StripKeys {
		keys = append(keys, k.Key+" ("+k.Help+")")
	}
	return "the presets " + strings.Join(parts, ", ") + "; or the options " + strings.Join(keys, ", ") + ". They add up: writer,notes"
}

func parseInput(o Options, conf rules.TOMLConf, input io.Reader) (lex.Screenplay, error) {
	switch o.From {
	case internal.FormatLex:
		return lex.Parse(input), nil
	case internal.FormatFountain:
		if o.Loose {
			return fountain.ParseLoose(conf.Scenes(o.SceneIn), input), nil
		}
		return fountain.ParseIn(conf.Language(o.SceneIn), input), nil
	case internal.FormatFDX:
		s, err := fdx.ParseWithError(input)
		if err != nil {
			return nil, fmt.Errorf("could not read FDX input: %w", err)
		}
		return s, nil
	case "osf":
		s, err := osf.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("could not read the OSF document: %w", err)
		}
		return s, nil
	case "trelby":
		s, err := trelby.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("could not read the Trelby screenplay: %w", err)
		}
		return s, nil
	case "kitsp":
		s, err := kitsp.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("could not read the KIT Scenarist project: %w", err)
		}
		return s, nil
	case "celtx":
		return readCeltx(o, input)
	case internal.FormatPDF:
		// a PDF is read at random places, so it is read whole first
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, fmt.Errorf("could not read the PDF: %w", err)
		}
		// scanned pages are OCRed with tesseract, if it is installed
		opts := pdfin.Options{Scenes: conf.Scenes(o.SceneIn)}
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
	var conf rules.TOMLConf
	if o.Conf != nil {
		conf = o.Conf.WithPresets()
	} else {
		conf = rules.GetConf(o.ConfigFile)
	}
	if o.FormatDir != "" {
		list, err := rules.PresetsIn(filepath.SplitList(o.FormatDir)...)
		if err != nil {
			log.Print(err)
		}
		for _, p := range list {
			conf.Elements[p.Key] = p.Elements
		}
	}
	if c := o.carried; c != nil {
		conf = conf.With(c.conf)
		for _, p := range c.presets(o) {
			if _, ok := c.formats[p.Key]; ok {
				conf.Elements[p.Key] = p.Elements
			}
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
	if o.carried != nil {
		list = o.carried.presets(o)
	}
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
		if o.Output == "-" && o.Resume == nil {
			return nil, errors.New("cannot write PDF to standard output: give an output file (-o output.pdf)")
		}
		return &pdf.PDFWriter{OutputFile: o.Output, Elements: elements, Page: pageFor(o), Revisions: o.Revisions,
			Lock: o.Lock, Continueds: o.Continueds, MergeOmitted: o.MergeOmitted, RTL: rtlFor(conf, o), Language: fontLanguage(o),
			Record: o.Record, Resume: o.Resume, Pictures: o.Pictures, TitlePage: o.tpl, TitlePictures: o.TitlePictures}, nil
	case internal.FormatLex:
		return &lex.LexWriter{}, nil
	case internal.FormatFountain:
		lang := conf.Language(o.SceneOut)
		return &fountain.FountainWriter{SceneConfig: conf.Scenes(o.SceneOut), Language: &lang}, nil
	case internal.FormatFDX:
		return &fdx.FDXWriter{TemplatePath: o.TemplatePath, Revisions: o.Revisions, Lock: o.Lock,
			Tags: o.Breakdown, LineTags: o.LineTags}, nil
	case internal.FormatHTML:
		return &html.HTMLWriter{Elements: elements, Page: pageFor(o), RTL: rtlFor(conf, o), TitlePage: o.tpl,
			TitlePictures: o.TitlePictures}, nil
	case internal.FormatLaTeX:
		return &latex.LaTeXWriter{Template: o.TemplatePath, Elements: elements, TitlePage: o.tpl}, nil
	case "osf":
		return &osf.OSFWriter{Version: o.OSFVersion}, nil
	case "fadein":
		return &fadeInWriter{osf.OSFWriter{Version: o.OSFVersion}}, nil
	case "trelby":
		return &trelby.TrelbyWriter{}, nil
	case "kitsp":
		return &kitsp.KITWriter{}, nil
	case "celtx":
		return &celtx.CeltxWriter{Tags: o.Breakdown, LineTags: o.LineTags}, nil
	case "docx":
		return &office.DOCXWriter{Elements: elements, Page: pageFor(o), TitlePage: o.tpl, TitlePictures: o.TitlePictures}, nil
	case "odt":
		return &office.ODTWriter{Elements: elements, Page: pageFor(o), TitlePage: o.tpl, TitlePictures: o.TitlePictures}, nil
	}
	return nil, fmt.Errorf("%s is not a supported output type", o.To)
}

// mergesOmitted tells whether o prints OMITTED runs as one in its
// format: not FDX, Fountain or lex, which keep one heading a number, nor
// the PDF, whose writer merges them itself (in its measures too).
func mergesOmitted(o Options) bool {
	switch o.To {
	case internal.FormatFDX, internal.FormatFountain, internal.FormatLex, internal.FormatPDF:
		return false
	}
	return o.MergeOmitted
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

// writeAs writes the screenplay as format (HTML or LaTeX, which another
// program makes o.To from) with o's settings, its title page template
// among them, as createWriter makes that format's writer.
func writeAs(format string, o Options, conf rules.TOMLConf, screenplay lex.Screenplay) (*bytes.Buffer, error) {
	o.To = format
	w, err := createWriter(o, conf)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := w.Write(&buf, screenplay); err != nil {
		return nil, err
	}
	return &buf, nil
}

// runProgram runs another program, its output shown on standard error
// as it comes; when it fails, the error says the program and the line
// that tells why: a TeX error (a line that starts "! "), else its last.
func runProgram(cmd *exec.Cmd) error {
	var said bytes.Buffer
	out := io.MultiWriter(os.Stderr, &said)
	cmd.Stderr = out
	if cmd.Stdout == nil {
		cmd.Stdout = out
	}
	err := cmd.Run()
	if err == nil {
		return nil
	}
	why := ""
	for _, line := range strings.Split(said.String(), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "! ") {
			why = line
			break
		}
		if line != "" {
			why = line
		}
	}
	name := filepath.Base(cmd.Path)
	if why == "" {
		return fmt.Errorf("%s: %w", name, err)
	}
	return fmt.Errorf("%s: %w: %s", name, err, why)
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
	buf, err := writeAs(internal.FormatHTML, o, conf, screenplay)
	if err != nil {
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
	log.Printf("Running wkhtmltopdf to create PDF from HTML...")
	return runProgram(cmd)
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
	buf, err := writeAs(internal.FormatLaTeX, o, conf, screenplay)
	if err != nil {
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
	if err = runProgram(cmd); err != nil {
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
	if err := (&markdown.MarkdownWriter{TitlePage: o.tpl}).Write(&buf, screenplay); err != nil {
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
	log.Printf("Running pandoc to create %s...", o.Output)
	return runProgram(cmd)
}

// titleAuthor are the title page's title and author.
func titleAuthor(screenplay lex.Screenplay) (title, author string) {
	for _, line := range screenplay {
		switch line.Type {
		case "Title":
			title = lex.Unstyled(lex.StripNotes(line.Contents))
		case "Author":
			author = lex.Unstyled(lex.StripNotes(line.Contents))
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
	buf, err := writeAs(internal.FormatHTML, o, conf, screenplay)
	if err != nil {
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
	log.Printf("Running ebook-convert to create %s...", o.Output)
	if err = runProgram(cmd); err != nil {
		return err
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return os.WriteFile(o.Output, data, 0o644)
}

// fadeInWriter writes a .fadein's zip, its OSF document alone in it (to
// standard output; a file gets writeFadeIn).
type fadeInWriter struct{ osf.OSFWriter }

func (w *fadeInWriter) Write(out io.Writer, s lex.Screenplay) error {
	data, err := osf.FadeInZip(w.Document(s), nil)
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}
