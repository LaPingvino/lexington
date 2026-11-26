package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/lexington/bundle"
	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/kitsp"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/osf"
)

// InputFormat is a format Lexington reads a screenplay from.
type InputFormat struct {
	// Key is what -from takes.
	Key string
	// Name is how people call it.
	Name string
	// Exts are its files' extensions, the first its own.
	Exts []string
	// Folder: a folder with its extension is one too (Name.fadein/).
	Folder bool
}

// Inputs are the formats Lexington reads screenplays from, by their own
// readers (the bundles, .highland, .accolade, .textbundle and .textpack,
// are read for the Fountain in them: bundle.Types).
var Inputs = []InputFormat{
	{Key: internal.FormatFountain, Name: "Fountain", Exts: []string{".fountain", ".spmd"}},
	{Key: internal.FormatFDX, Name: "Final Draft (FDX)", Exts: []string{".fdx"}},
	{Key: internal.FormatPDF, Name: "Screenplay (PDF)", Exts: []string{".pdf"}},
	{Key: "fadein", Name: "Fade In", Exts: []string{".fadein"}, Folder: true},
	{Key: "osf", Name: "Open Screenplay Format (OSF)", Exts: []string{".osf"}},
	{Key: "trelby", Name: "Trelby", Exts: []string{".trelby"}},
	{Key: "kitsp", Name: "KIT Scenarist", Exts: []string{".kitsp"}},
	{Key: internal.FormatLex, Name: "Lexington's own (lex)", Exts: []string{".lex"}},
}

// InputFor is the input format a file's name has, by its extension.
func InputFor(name string) (InputFormat, bool) {
	ext := strings.ToLower(filepath.Ext(strings.TrimRight(name, `/\`)))
	for _, f := range Inputs {
		for _, e := range f.Exts {
			if e == ext {
				return f, true
			}
		}
	}
	return InputFormat{}, false
}

func inputKey(key string) bool {
	for _, f := range Inputs {
		if f.Key == key {
			return true
		}
	}
	return false
}

// Sniff is the input format a file's first bytes say it is in: a KIT
// project's database, a Fade In zip, Trelby's #Version line, a PDF, Final
// Draft's or OSF's XML; "" for none of them (Fountain, or a bundle).
func Sniff(head []byte) string {
	switch {
	case kitsp.IsProject(head):
		return "kitsp"
	case bytes.HasPrefix(head, []byte("%PDF")):
		return internal.FormatPDF
	case bytes.HasPrefix(head, []byte("PK\x03\x04")):
		if _, err := osf.FadeInDocument(head); err == nil {
			return "fadein"
		}
		return ""
	}
	text := bytes.TrimLeft(bytes.TrimPrefix(head, []byte("\ufeff")), " \t\r\n")
	switch {
	case bytes.HasPrefix(text, []byte("#Version ")):
		return "trelby"
	case !bytes.HasPrefix(text, []byte("<")):
		return ""
	case bytes.Contains(head, []byte("<FinalDraft")):
		return internal.FormatFDX
	case bytes.Contains(head, []byte(osf.DocumentType)):
		return "osf"
	}
	return ""
}

// sniffInput sets o.From from the input's content when its name does not
// name a format Lexington reads; it reads a file whole for that, so that
// a zip's end is there too (standard input is kept, in o.in).
func sniffInput(o *Options) error {
	if inputKey(o.From) || o.in != nil {
		return nil
	}
	var data []byte
	var err error
	if o.Input == "" || o.Input == "-" {
		if data, err = io.ReadAll(os.Stdin); err != nil {
			return err
		}
		o.in = data
	} else if st, serr := os.Stat(o.Input); serr == nil && st.IsDir() {
		return nil
	} else if data, err = os.ReadFile(o.Input); err != nil {
		return err
	}
	if k := Sniff(data); k != "" {
		o.From = k
	} else if o.From != "" && !inputKey(o.From) {
		o.From = internal.FormatFountain // a .txt, or a name of another's
	}
	return nil
}

// openFadeIn reads a .fadein input (a zip or a folder, or a zip on
// standard input) as the OSF document in it.
func openFadeIn(o *Options) error {
	if o.From != "fadein" {
		return nil
	}
	var doc []byte
	var err error
	switch {
	case o.in != nil:
		doc, err = osf.FadeInDocument(o.in)
	case o.Input == "" || o.Input == "-":
		var data []byte
		if data, err = io.ReadAll(os.Stdin); err == nil {
			doc, err = osf.FadeInDocument(data)
		}
	default:
		doc, err = osf.ReadFadeIn(o.Input)
	}
	if err != nil {
		return err
	}
	o.in, o.From = doc, "osf"
	return nil
}

// writeFadeIn writes the screenplay into the .fadein at o.Output as its
// OSF document, as -form asks (else as it is, else a zip), keeping what
// else one there holds.
func writeFadeIn(o Options, screenplay lex.Screenplay) error {
	form, err := bundle.ParseForm(o.BundleForm)
	if err != nil {
		return err
	}
	return osf.WriteFadeIn(o.Output, form, (&osf.OSFWriter{Version: o.OSFVersion}).Document(screenplay))
}

// ReadScreenplay reads a screenplay from a file in any format Lexington
// reads (Inputs, and the bundles), by its name or else its content, as
// the command line does; o's settings (the scene language, -loose) are
// the reading's, its Input the file.
func ReadScreenplay(ctx context.Context, path string, o Options) (lex.Screenplay, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o.Input = path
	if o.ConfigFile == "" {
		o.ConfigFile = "lexington.toml"
	}
	src, err := readBundleInput(&o)
	if err != nil {
		return nil, err
	}
	if src != nil && src.Kind() != bundle.Fountain {
		return nil, errors.New(filepath.Base(path) + " holds " + src.Kind().String() + ", not a screenplay")
	}
	conf := confOf(o)
	detectFormats(&o, conf)
	if err := sniffInput(&o); err != nil {
		return nil, err
	}
	if err := openFadeIn(&o); err != nil {
		return nil, err
	}
	setDefaults(&o)
	in, closeIn, err := openInput(o)
	if err != nil {
		return nil, err
	}
	defer closeIn()
	return parseInput(o, conf, in)
}
