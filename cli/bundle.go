package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/lexington/bundle"
	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/internal"
	"github.com/LaPingvino/lexington/lex"
)

// Bundles (package bundle) on the command line: one as -i is read for its
// text, which every output takes; one as -o gets the script as Fountain,
// all else in it (info.json, assets, sidecars, drafts, files no one here
// knows) kept: the output's own when it is there, else the input bundle's.

// isBundleInput tells whether an input is a bundle: by its name, or a
// folder.
func isBundleInput(name string) bool {
	if name == "" || name == "-" {
		return false
	}
	if bundle.Is(name) {
		return true
	}
	st, err := os.Stat(name)
	return err == nil && st.IsDir()
}

// isBundleOutput tells whether an output is a bundle, by its name.
func isBundleOutput(name string) bool {
	return name != "" && name != "-" && bundle.Is(name)
}

// readBundleInput reads o's input if it is a bundle: its text becomes the
// input, as Fountain (a language's text, text.eo.fountain, in its
// language), and nil is returned for any other input.
func readBundleInput(o *Options) (*bundle.Bundle, error) {
	if o.in != nil || !isBundleInput(o.Input) {
		return nil, nil
	}
	b, err := bundle.Read(o.Input)
	if errors.Is(err, bundle.ErrNotZip) && strings.EqualFold(filepath.Ext(o.Input), ".highland") {
		if o.From == "" { // Fountain with Highland's name: read as it is
			o.From = internal.FormatFountain
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text, err := b.ReadText()
	if err != nil {
		return nil, err
	}
	o.in, o.src = text, b
	if o.From == "" {
		o.From = internal.FormatFountain
	}
	// text.eo.fountain: a script in that language
	if parts := strings.Split(strings.ToLower(b.Text), "."); len(parts) == 3 && parts[2] == "fountain" && o.SceneIn == "" {
		o.SceneIn = parts[1]
	}
	return b, nil
}

// transforms tells whether o changes the script, so that a bundle's text
// is written anew (as Fountain), not copied as it is.
func transforms(o Options) bool {
	return o.Strip != "" || o.KeepLoose || o.Loose || len(o.Tags) > 0 || o.Character != "" || o.Number != "" ||
		(o.From != "" && o.From != internal.FormatFountain) ||
		(o.SceneOut != "" && o.SceneOut != o.SceneIn)
}

// convertToBundle writes the input into the bundle o.Output: as it is
// when nothing changes it (Fountain in, a bundle's text, any Markdown),
// else as Lexington writes Fountain.
func convertToBundle(ctx context.Context, o Options, src *bundle.Bundle) error {
	form, err := bundle.ParseForm(o.BundleForm)
	if err != nil {
		return err
	}
	if o.Report != "" || o.LockPages {
		return fmt.Errorf("a bundle holds a script, not a report or a lock: give -o another file than %s", filepath.Base(o.Output))
	}
	if o.To != "" && o.To != internal.FormatFountain && !bundle.Is("."+o.To) {
		return fmt.Errorf("a bundle holds Fountain: -to %s cannot go in %s", o.To, filepath.Base(o.Output))
	}
	if o.Strip != "" && o.Input != "-" && sameFile(o.Input, o.Output) {
		return errors.New("-strip writes a copy: give -o another file than the input")
	}
	o.To = internal.FormatFountain
	if o.From == "" {
		probe := o
		probe.Output = "x.fountain"
		detectFormats(&probe, confOf(o))
		o.From = probe.From
	}
	base, err := outputBase(o.Output, src)
	if err != nil {
		return err
	}
	var text []byte
	kind := bundle.Fountain
	if src != nil {
		kind = src.Kind()
	}
	if !transforms(o) {
		if text = o.in; text == nil {
			r, closeIn, err := openInput(o)
			if err != nil {
				return err
			}
			text, err = io.ReadAll(r)
			if err := errors.Join(err, closeIn()); err != nil {
				return err
			}
		}
	} else {
		if kind != bundle.Fountain {
			return fmt.Errorf("%s holds %s: it can be repacked, not converted", filepath.Base(o.Input), kind)
		}
		var buf bytes.Buffer
		o.sink = &buf
		if err := convert(ctx, o); err != nil {
			return err
		}
		text = buf.Bytes()
	}
	return saveBundle(base, o.Output, form, text, kind)
}

// outputBase is the bundle a bundle written to dst keeps the rest of: the
// one at dst, else the input's (src, nil for none).
func outputBase(dst string, src *bundle.Bundle) (*bundle.Bundle, error) {
	if _, err := os.Stat(dst); err == nil {
		b, err := bundle.Read(dst)
		if err != nil {
			return nil, fmt.Errorf("%s is there and no bundle (%w): not replaced", filepath.Base(dst), err)
		}
		return b, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return src, nil
}

// saveBundle writes text as base's text into dst (a new bundle for base
// nil, or one with no text).
func saveBundle(base *bundle.Bundle, dst string, form bundle.Form, text []byte, kind bundle.Kind) error {
	if base != nil && base.Text != "" && base.Kind() != kind {
		return fmt.Errorf("%s holds %s (%s); the input is %s", filepath.Base(dst), base.Kind(), base.Text, kind)
	}
	if base != nil && base.Text == "" {
		base = nil
	}
	_, err := bundle.SaveText(base, dst, form, text, kind == bundle.Markdown, "")
	return err
}

// writeToBundle is Write into a bundle: the screenplay as Fountain.
func writeToBundle(ctx context.Context, screenplay lex.Screenplay, o Options) error {
	form, err := bundle.ParseForm(o.BundleForm)
	if err != nil {
		return err
	}
	base, err := outputBase(o.Output, nil)
	if err != nil {
		return err
	}
	setDefaults(&o)
	conf := confOf(o)
	filtered, err := filter(o, screenplay)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var buf bytes.Buffer
	w := &fountain.FountainWriter{SceneConfig: conf.Scenes(o.SceneOut)}
	if err := w.Write(&buf, filtered); err != nil {
		return err
	}
	return saveBundle(base, o.Output, form, buf.Bytes(), bundle.Fountain)
}

// checkBundles is `lexington check bundle...`: each bundle's outline and
// what is wrong with it; the exit code 1 if anything is.
func checkBundles(args []string) int {
	if len(args) == 0 || args[0] == "-help" || args[0] == "-h" {
		fmt.Fprintln(os.Stderr, "usage: lexington check file... (a bundle: .accolade, .highland, .textbundle, .textpack, zipped or a folder; or a script, for its {{INCLUDE}}s)")
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	code := 0
	for _, p := range args {
		if !isBundleInput(p) { // a script: its includes
			text, err := os.ReadFile(p)
			if err != nil {
				fmt.Printf("%s:\nproblem: %v\n", p, err)
				code = 1
				continue
			}
			r := bundle.Report{Warnings: checkIncludes(p, text, Options{})}
			fmt.Print(filepath.Base(p) + ": a script\n" + r.String())
			continue
		}
		r := bundle.Check(p)
		if b := r.Bundle; b != nil && b.Kind() == bundle.Fountain {
			if text, err := b.ReadText(); err == nil {
				r.Warnings = checkIncludes(p, text, Options{})
			}
		}
		out := r.String()
		if r.Bundle == nil {
			out = p + ":\n" + out
		}
		fmt.Print(out)
		if !r.OK() {
			code = 1
		}
	}
	return code
}
