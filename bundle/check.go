package bundle

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Report is what Check finds in a bundle: Problems keep other programs
// (or Lexington) from reading it; Notes are worth knowing, no harm.
type Report struct {
	Bundle   *Bundle // nil when it could not be read
	Problems []string
	Notes    []string
}

// OK tells whether the bundle has no problems.
func (r Report) OK() bool { return len(r.Problems) == 0 }

// String is the report as lines, the bundle's outline first.
func (r Report) String() string {
	var s strings.Builder
	if b := r.Bundle; b != nil {
		kind := "Fountain"
		if b.Markdown() {
			kind = "Markdown"
		}
		form := "a folder"
		if b.Zipped {
			form = "a zip, its files at the root"
			if b.Prefix != "" {
				form = "a zip, its files in " + b.Prefix
			}
		}
		fmt.Fprintf(&s, "%s: %s; text %s (%s), %d files\n", filepath.Base(b.Path), form,
			orNone(b.Text), kind, len(b.Files))
	}
	for _, p := range r.Problems {
		s.WriteString("problem: " + p + "\n")
	}
	for _, n := range r.Notes {
		s.WriteString("note: " + n + "\n")
	}
	if r.OK() {
		s.WriteString("ok\n")
	}
	return s.String()
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// Check validates the bundle at p: that it reads as a zip or a folder,
// its layout is clear, it holds a text file, its info.json reads, and its
// names stay inside it.
func Check(p string) Report {
	var r Report
	st, err := os.Stat(p)
	if err != nil {
		r.Problems = append(r.Problems, err.Error())
		return r
	}
	b, err := Read(p)
	if err != nil {
		r.Problems = append(r.Problems, err.Error())
		return r
	}
	r.Bundle = b
	if b.Zipped {
		zr, _ := zip.NewReader(bytes.NewReader(b.Raw), int64(len(b.Raw)))
		tops, junk, root := map[string]bool{}, false, false
		for _, f := range zr.File {
			name := strings.TrimSuffix(f.Name, "/")
			if !fs.ValidPath(name) || strings.Contains(f.Name, `\`) {
				r.Problems = append(r.Problems, fmt.Sprintf("entry %q points outside the bundle", f.Name))
				continue
			}
			if IsJunk(f.Name) {
				junk = true
				continue
			}
			top, _, nested := strings.Cut(f.Name, "/")
			if nested {
				tops[top] = true
			} else {
				root = true
			}
		}
		if !root && len(tops) > 1 && b.Prefix == "" {
			r.Problems = append(r.Problems, fmt.Sprintf("%d top-level folders and no files at the root: which is the bundle is unclear", len(tops)))
		}
		if root && b.Prefix == "" && len(b.Files) > 0 {
			r.Notes = append(r.Notes, "its files are at the zip's root; TextBundle's own reader expects them in one Name.textbundle/ folder")
		}
		if junk {
			r.Notes = append(r.Notes, "it holds macOS's __MACOSX/ or .DS_Store entries (ignored, copied through)")
		}
	}
	if t, ok := TypeOf(p); ok && t.Form != b.Form() && t.Ext != ".highland" && t.Ext != ".accolade" {
		r.Notes = append(r.Notes, fmt.Sprintf("a %s is a %s elsewhere; this one is a %s (Lexington and Accolade read both)", t.Ext, t.Form, b.Form()))
	} else if t.Ext == ".highland" && st.IsDir() {
		r.Notes = append(r.Notes, "Highland itself opens the zip, not this folder form")
	}
	if b.Text == "" {
		r.Problems = append(r.Problems, "no text file (text.fountain, text.md…) at the bundle's root")
	} else if _, err := b.ReadText(); err != nil {
		r.Problems = append(r.Problems, "the text file cannot be read: "+err.Error())
	}
	switch readable, strict := InfoReadable(b.Info); {
	case b.Info == nil:
		r.Notes = append(r.Notes, "no info.json (TextBundle's spec asks for one; readers here manage without)")
	case !readable:
		r.Problems = append(r.Problems, "info.json is no JSON object")
	default:
		if !strict {
			r.Notes = append(r.Notes, "info.json is not strict JSON (a trailing comma?): strict readers refuse it")
		}
		if v, ok := InfoValue(b.Info, "version"); !ok || string(v) != "2" {
			r.Notes = append(r.Notes, "info.json's version is not 2 (TextBundle's)")
		}
		if t, ok := InfoString(b.Info, "type"); ok && b.Text != "" {
			t = strings.ToLower(t)
			ext := strings.ToLower(path.Ext(b.Text))
			md := ext == ".md" || ext == ".markdown"
			if fountain := ext == ".fountain"; fountain && strings.Contains(t, "markdown") || md && strings.Contains(t, "fountain") {
				r.Notes = append(r.Notes, fmt.Sprintf("info.json's type %q and the text file %s disagree (the file's name wins)", t, b.Text))
			}
		}
	}
	return r
}
