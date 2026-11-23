// Package bundle reads and writes script bundles: the TextBundle family.
// A bundle is a script's text file (text.fountain, text.md…) with
// info.json, assets/ and whatever else other programs keep beside it, in
// one of two forms that hold the same files:
//
//   - a zip (a .textpack, a .highland, an .accolade), its files at the
//     zip's root or inside one top-level Name.textbundle/ folder;
//   - a plain folder (a .textbundle; and Name.highland/ or Name.accolade/
//     as folders, the zipped ones unzipped).
//
// Reading tells the two apart by what is on disk; writing keeps the form a
// bundle was read in unless told otherwise, and copies every entry it does
// not write through as it is (zip entries raw, not recompressed).
package bundle

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Form is how a bundle is stored.
type Form int

const (
	// Auto: a bundle's own form (for the file it was read from), else what
	// is on disk at the name, else the type's.
	Auto Form = iota
	Zip
	Dir
)

func (f Form) String() string {
	switch f {
	case Zip:
		return "zip"
	case Dir:
		return "folder"
	}
	return "auto"
}

// ParseForm reads a form's name: zip, folder (or dir, directory), auto
// ("" too).
func ParseForm(s string) (Form, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return Auto, nil
	case "zip", "zipped":
		return Zip, nil
	case "folder", "dir", "directory":
		return Dir, nil
	}
	return Auto, fmt.Errorf("%q is no bundle form: zip or folder", s)
}

// Type is a kind of bundle, by its extension.
type Type struct {
	Ext  string // ".textbundle"
	Name string // "TextBundle"
	// Form is the form a new bundle of the type gets.
	Form Form
	// Screenplay: the type's text is Fountain when nothing else says
	// (TextBundle's own default is Markdown).
	Screenplay bool
}

// Types are the bundles known: TextBundle's two (.textbundle the folder,
// .textpack the zip), Highland's and Accolade's.
var Types = []Type{
	{Ext: ".textbundle", Name: "TextBundle", Form: Dir},
	{Ext: ".textpack", Name: "TextPack", Form: Zip},
	{Ext: ".highland", Name: "Highland", Form: Zip, Screenplay: true},
	{Ext: ".accolade", Name: "Accolade", Form: Zip, Screenplay: true},
}

// TypeOf is the type a name has, by its extension (a trailing slash
// aside).
func TypeOf(name string) (Type, bool) {
	ext := strings.ToLower(filepath.Ext(strings.TrimRight(name, `/\`)))
	for _, t := range Types {
		if t.Ext == ext {
			return t, true
		}
	}
	return Type{}, false
}

// Is tells whether a name is a bundle's, by its extension.
func Is(name string) bool {
	_, ok := TypeOf(name)
	return ok
}

// FormOf is the form a bundle at p has: as it is on disk (a folder, a
// file), else as the name asks (a trailing slash: a folder), else the
// type's (a zip for an unknown one).
func FormOf(p string) Form {
	if st, err := os.Stat(p); err == nil {
		if st.IsDir() {
			return Dir
		}
		if st.Size() > 0 {
			return Zip
		}
	}
	if strings.HasSuffix(p, "/") || strings.HasSuffix(p, string(filepath.Separator)) {
		return Dir
	}
	if t, ok := TypeOf(p); ok {
		return t.Form
	}
	return Zip
}

// ErrNotZip: the file is no zip (a .highland that is plain Fountain).
var ErrNotZip = errors.New("not a zip file")

// The UTIs info.json's "type" has for the kinds of text (plain text's as
// Highland writes it with text.txt, for its Plain Text template).
const (
	FountainUTI = "com.quoteunquoteapps.fountain"
	MarkdownUTI = "net.daringfireball.markdown"
	PlainUTI    = "public.plain.text"
)

// HighlandKey is the key of Highland 2's own settings in info.json (its
// "templateName" among them).
const HighlandKey = "com.quoteunquoteapps.highland2"

// Kind is the kind of text a bundle holds.
type Kind int

const (
	Fountain Kind = iota
	Markdown
	Plain // plain text: Highland's Plain Text template's text.txt
)

func (k Kind) String() string {
	switch k {
	case Markdown:
		return "Markdown"
	case Plain:
		return "plain text"
	}
	return "Fountain"
}

// Yield, if set, is called for every entry a zip is written with (a
// browser's page gets to draw meanwhile).
var Yield func()

// Bundle is a bundle as read.
type Bundle struct {
	Path   string
	Zipped bool
	// Raw is a zip's bytes as read or written: what is copied through.
	Raw []byte
	// ModTime and Size are a zip's file's as read or written (Changed).
	ModTime time.Time
	Size    int64
	// Prefix is the folder holding the bundle's files in a zip
	// ("Name.textbundle/"), "" for files at its root.
	Prefix string
	// Root are the files at the bundle's root, Files all its files,
	// relative to it ('/' between), sorted.
	Root, Files []string
	// Text is the text file (PickText's), "" for none.
	Text string
	// Info is info.json as it is, nil for none; InfoName its name.
	Info     []byte
	InfoName string

	zfiles  map[string]*zip.File // by full name
	comment string
}

// Entry is one file or folder in a bundle: Name its path in the zip
// (prefix included) or in the folder, '/' between, a folder's ending in
// '/'.
type Entry struct {
	Name string
	Zip  *zip.File // in a zip
	File string    // in a folder: the file on disk
}

// File is a file written into a bundle: Name relative to the bundle, Data
// nil and a Name ending in '/' for a folder.
type File struct {
	Name string
	Data []byte
}

// IsJunk tells whether an entry is what macOS leaves in zips: never the
// bundle's own, though copied through.
func IsJunk(name string) bool {
	return strings.HasPrefix(name, "__MACOSX/") || path.Base(name) == ".DS_Store"
}

// Read reads the bundle at p: a folder, else a zip.
func Read(p string) (*Bundle, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return ReadDir(p)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	b, err := ReadZip(p, data)
	if err == nil && st.Size() == int64(len(data)) {
		b.ModTime, b.Size = st.ModTime(), st.Size()
	}
	return b, err
}

// ReadZip reads a zipped bundle from its bytes (p is its name).
func ReadZip(p string, data []byte) (*Bundle, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, ErrNotZip
	}
	b := &Bundle{Path: p, Zipped: true, Raw: data, zfiles: map[string]*zip.File{}, comment: zr.Comment}
	tops := map[string]bool{}
	hasRootFile := false
	for _, f := range zr.File {
		if IsJunk(f.Name) {
			continue
		}
		top, _, nested := strings.Cut(f.Name, "/")
		if nested {
			tops[top] = true
		}
		if f.FileInfo().IsDir() {
			continue
		}
		b.zfiles[f.Name] = f
		hasRootFile = hasRootFile || !nested
	}
	// the layout: files at the root, else one top-level folder (the
	// *.textbundle one if there are several)
	if !hasRootFile {
		var cands []string
		for t := range tops {
			cands = append(cands, t)
		}
		if len(cands) > 1 {
			cands = slices.DeleteFunc(cands, func(t string) bool {
				return !strings.EqualFold(filepath.Ext(t), ".textbundle")
			})
		}
		if len(cands) == 1 {
			b.Prefix = cands[0] + "/"
		}
	}
	for name := range b.zfiles {
		if rel, ok := strings.CutPrefix(name, b.Prefix); ok {
			b.Files = append(b.Files, rel)
		}
	}
	b.fill()
	return b, nil
}

// ReadDir reads a bundle folder.
func ReadDir(dir string) (*Bundle, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", filepath.Base(dir))
	}
	b := &Bundle{Path: dir}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, _ := filepath.Rel(dir, p)
			b.Files = append(b.Files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	b.fill()
	return b, nil
}

func (b *Bundle) fill() {
	slices.Sort(b.Files)
	for _, n := range b.Files {
		if !strings.Contains(n, "/") {
			b.Root = append(b.Root, n)
		}
	}
	b.Text = PickText(b.Root, nil)
	for _, n := range b.Root {
		if strings.EqualFold(n, "info.json") {
			b.InfoName = n
			if data, err := b.ReadFile(n); err == nil {
				b.Info = data
				if b.Info == nil {
					b.Info = []byte{}
				}
			}
		}
	}
}

// ReadFile reads one of the bundle's files (rel relative to it).
func (b *Bundle) ReadFile(rel string) ([]byte, error) {
	if !fs.ValidPath(rel) {
		return nil, fs.ErrNotExist
	}
	if !b.Zipped {
		return os.ReadFile(filepath.Join(b.Path, filepath.FromSlash(rel)))
	}
	f := b.zfiles[b.Prefix+rel]
	if f == nil {
		return nil, fs.ErrNotExist
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// ReadText is the bundle's text file's bytes.
func (b *Bundle) ReadText() ([]byte, error) {
	if b.Text == "" {
		return nil, fmt.Errorf("%s holds no text file (text.fountain, text.md…)", filepath.Base(b.Path))
	}
	return b.ReadFile(b.Text)
}

// Markdown tells whether the bundle's text is Markdown (TextMarkdown).
func (b *Bundle) Markdown() bool { return TextMarkdown(b.Text, b.Info, b.Path) }

// Kind is the kind of text the bundle holds (TextKind).
func (b *Bundle) Kind() Kind { return TextKind(b.Text, b.Info, b.Path) }

// Template is the Highland template info.json names ("Screenplay",
// "Manuscript", "Stage Play"…), "" for none.
func (b *Bundle) Template() string {
	raw, ok := InfoValue(b.Info, HighlandKey)
	if !ok {
		return ""
	}
	s, _ := InfoString(raw, "templateName")
	return s
}

// Form is the form the bundle is in.
func (b *Bundle) Form() Form {
	if b.Zipped {
		return Zip
	}
	return Dir
}

// Changed tells whether a zip's file changed on disk since it was read or
// written here (always true for a folder, or a zip read from bytes).
func (b *Bundle) Changed() bool {
	if !b.Zipped || b.ModTime.IsZero() {
		return true
	}
	st, err := os.Stat(b.Path)
	return err != nil || st.Size() != b.Size || !st.ModTime().Equal(b.ModTime)
}

// Entries are the bundle's files and folders, in their order (a zip's)
// or the folder's walk.
func (b *Bundle) Entries() ([]Entry, error) {
	if b == nil {
		return nil, nil
	}
	if b.Zipped {
		zr, err := zip.NewReader(bytes.NewReader(b.Raw), int64(len(b.Raw)))
		if err != nil {
			return nil, err
		}
		var es []Entry
		for _, f := range zr.File {
			es = append(es, Entry{Name: f.Name, Zip: f})
		}
		return es, nil
	}
	var es []Entry
	err := filepath.WalkDir(b.Path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(b.Path, p)
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			name += "/"
		} else if !d.Type().IsRegular() {
			return nil
		}
		es = append(es, Entry{Name: name, File: p})
		return nil
	})
	return es, err
}

// PickText is a bundle's text among the files at its root: text.fountain,
// text.md, text.markdown, text.txt, else another text.* (.fountain first,
// then as those); skip, if set, leaves names out.
func PickText(names []string, skip func(string) bool) string {
	order := []string{".fountain", ".md", ".markdown", ".txt"}
	rank := func(n string) int {
		l := strings.ToLower(n)
		for i, e := range order {
			if l == "text"+e {
				return i
			}
		}
		if i := slices.Index(order, path.Ext(l)); i >= 0 {
			return len(order) + i
		}
		return 2 * len(order)
	}
	best := ""
	for _, n := range names {
		if !IsTextName(n) || skip != nil && skip(n) {
			continue
		}
		if best == "" || rank(n) < rank(best) || (rank(n) == rank(best) && n < best) {
			best = n
		}
	}
	return best
}

// IsTextName tells whether a name is a text file's (text.*).
func IsTextName(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "text.") && len(name) > len("text.")
}

// TextMarkdown tells whether a bundle's text is Markdown (TextKind).
func TextMarkdown(textName string, info []byte, bundlePath string) bool {
	return TextKind(textName, info, bundlePath) == Markdown
}

// TextKind is the kind of a bundle's text: as its name says (text.md,
// text.fountain), else as info.json's "type" does (public.plain.text,
// Highland's for text.txt: plain text), else as the bundle's type does
// (TextBundle's default, Markdown; Highland's and Accolade's, Fountain).
func TextKind(textName string, info []byte, bundlePath string) Kind {
	switch strings.ToLower(path.Ext(textName)) {
	case ".md", ".markdown":
		return Markdown
	case ".fountain", ".spmd":
		return Fountain
	}
	if t, ok := InfoString(info, "type"); ok {
		t = strings.ToLower(t)
		switch {
		case strings.Contains(t, "fountain"):
			return Fountain
		case strings.Contains(t, "markdown"):
			return Markdown
		case strings.Contains(t, "plain"):
			return Plain
		}
	}
	if t, _ := TypeOf(bundlePath); t.Screenplay {
		return Fountain
	}
	return Markdown
}

// NewInfo is info.json for a new bundle, made by creator (a reverse-DNS
// name; "" for none).
func NewInfo(markdown bool, creator string) []byte {
	t := FountainUTI
	if markdown {
		t = MarkdownUTI
	}
	s := "{\n  \"version\": 2,\n  \"type\": \"" + t + "\",\n  \"transient\": false"
	if creator != "" {
		c, _ := jsonString(creator)
		s += ",\n  \"creatorIdentifier\": " + c
	}
	return []byte(s + "\n}\n")
}

// SameFile tells whether two names are one file.
func SameFile(a, b string) bool {
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil {
			return os.SameFile(fa, fb)
		}
	}
	aa, _ := filepath.Abs(strings.TrimRight(a, `/\`))
	ab, _ := filepath.Abs(strings.TrimRight(b, `/\`))
	return aa == ab
}

// New is the files of a new bundle with text: info.json, the text file
// (text.fountain or text.md) and assets/.
func New(text []byte, markdown bool, creator string) []File {
	name := "text.fountain"
	if markdown {
		name = "text.md"
	}
	return []File{{"info.json", NewInfo(markdown, creator)}, {name, text}, {"assets/", nil}}
}

// SaveText writes text as the bundle's text to dst in form, all else b
// holds (nil: a new bundle, info.json made by creator) kept as it is.
func SaveText(b *Bundle, dst string, form Form, text []byte, markdown bool, creator string) (*Bundle, error) {
	if b == nil {
		return Write(nil, dst, form, New(text, markdown, creator), nil)
	}
	if b.Text == "" {
		return nil, fmt.Errorf("%s holds no text file to write", filepath.Base(b.Path))
	}
	var gone []string
	if t, _ := TypeOf(dst); t.Ext == ".highland" && slices.Contains(b.Files, highlandRevisions) {
		if old, err := b.ReadText(); err == nil && !bytes.Equal(old, text) {
			gone = append(gone, highlandRevisions)
		}
	}
	return Write(b, dst, form, []File{{b.Text, text}}, gone)
}

// highlandRevisions is Highland's cache of a text's revisions, which it
// builds again from the text's {{REVISION}} tags: left out when the text
// changes, as highwater-too (which reads Highland 2's files) does, so that
// it never disagrees with them.
const highlandRevisions = "revisions/current.json"

// Write writes a bundle to dst in form: b's entries (none for a new
// bundle) copied through, files replacing theirs or added after them,
// those gone (names relative to the bundle) left out. The bundle as
// written is returned.
//
// A zip keeps its layout: files at its root stay there, and its folder is
// named for the file it is written to when it is a Name.textbundle/ one
// (as Highland names it on every save) or the zip is written to another
// file; a new zip, or one from a folder, has its files in
// Name.textbundle/, as TextBundle's own writer and Highland do. (A
// .highland gets no entry for that folder itself, as Highland writes
// none, nor for any other folder with files in it.) A folder written to its own
// path changes only the files written; anywhere else it is made whole and
// put in place (over an empty file, or a bundle of either form).
func Write(b *Bundle, dst string, form Form, files []File, gone []string) (*Bundle, error) {
	if form == Auto {
		if b != nil && SameFile(b.Path, dst) {
			form = b.Form()
		} else {
			form = FormOf(dst)
		}
	}
	dst = strings.TrimRight(dst, `/\`)
	if form == Dir {
		if err := writeDir(b, dst, files, gone); err != nil {
			return nil, err
		}
		return ReadDir(dst)
	}
	base := strings.TrimSuffix(filepath.Base(dst), filepath.Ext(dst))
	prefix := base + ".textbundle/"
	if b != nil && b.Zipped && (b.Prefix == "" || SameFile(b.Path, dst) && !strings.EqualFold(path.Ext(strings.TrimSuffix(b.Prefix, "/")), ".textbundle")) {
		prefix = b.Prefix
	}
	t, _ := TypeOf(dst)
	data, err := zipBundle(b, prefix, files, gone, t.Ext != ".highland")
	if err != nil {
		return nil, err
	}
	if err := putFile(dst, data); err != nil {
		return nil, err
	}
	nb, err := ReadZip(dst, data)
	if st, serr := os.Stat(dst); err == nil && serr == nil && st.Size() == int64(len(data)) {
		nb.ModTime, nb.Size = st.ModTime(), st.Size()
	}
	return nb, err
}

// putFile writes a zip to dst: over a file atomically, over a bundle
// folder by putting it aside first.
func putFile(dst string, data []byte) error {
	st, err := os.Lstat(dst)
	if err != nil || !st.IsDir() {
		return WriteFileAtomic(dst, data)
	}
	if _, err := ReadDir(dst); err != nil || PickText(dirRoot(dst), nil) == "" {
		return fmt.Errorf("%s is a folder but no bundle: not replaced", filepath.Base(dst))
	}
	tmp := tempName(dst)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	defer os.Remove(tmp)
	return swap(tmp, dst)
}

// swap puts tmp in dst's place, dst removed once it is.
func swap(tmp, dst string) error {
	old := tmp + ".old"
	if err := os.Rename(dst, old); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(old, dst)
		return err
	}
	return os.RemoveAll(old)
}

func tempName(dst string) string {
	return filepath.Join(filepath.Dir(dst), "."+filepath.Base(dst)+"."+strconv.FormatInt(time.Now().UnixNano(), 36))
}

func dirRoot(dir string) []string {
	ents, _ := os.ReadDir(dir)
	var names []string
	for _, e := range ents {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// zipBundle is the zip of a bundle, its files in prefix.
// (dirEntry: with entries for folders, prefix's when it is new, and
// those from a folder; else only for empty ones, as Highland writes them.)
func zipBundle(b *Bundle, prefix string, files []File, gone []string, dirEntry bool) ([]byte, error) {
	es, err := b.Entries()
	if err != nil {
		return nil, err
	}
	oldPrefix := ""
	if b != nil && b.Zipped {
		oldPrefix = b.Prefix
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if b != nil && b.comment != "" {
		if err := zw.SetComment(b.comment); err != nil {
			return nil, err
		}
	}
	pending := slices.Clone(files)
	take := func(rel string) (File, bool) {
		for i, f := range pending {
			if f.Name == rel {
				pending = slices.Delete(pending, i, i+1)
				return f, true
			}
		}
		return File{}, false
	}
	now := time.Now()
	write := func(name string, data []byte, like *zip.FileHeader) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: now}
		if strings.HasSuffix(name, "/") {
			h.Method = zip.Store
			h.SetMode(fs.ModeDir | 0o755)
		} else {
			h.SetMode(0o644)
		}
		if like != nil {
			h.CreatorVersion, h.ExternalAttrs = like.CreatorVersion, like.ExternalAttrs
			if like.Method == zip.Store && !strings.HasSuffix(name, "/") {
				h.Method = zip.Store // stored as it was; any other method: deflated
			}
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if (b == nil || !b.Zipped) && prefix != "" && dirEntry {
		if err := write(prefix, nil, nil); err != nil {
			return nil, err
		}
	}
	for _, e := range es {
		if Yield != nil {
			Yield()
		}
		name := e.Name
		rel, inside := strings.CutPrefix(name, oldPrefix)
		if !b.Zipped {
			rel, inside = name, true
		}
		if inside {
			name = prefix + rel
		}
		if inside && slices.Contains(gone, rel) {
			continue
		}
		if inside && rel != "" {
			if f, ok := take(rel); ok {
				var like *zip.FileHeader
				if e.Zip != nil {
					like = &e.Zip.FileHeader
				}
				if err := write(name, f.Data, like); err != nil {
					return nil, err
				}
				continue
			}
		}
		switch {
		case e.Zip != nil && name == e.Name:
			if err := zw.Copy(e.Zip); err != nil {
				return nil, err
			}
		case e.Zip != nil:
			h := e.Zip.FileHeader
			h.Name = name
			r, err := e.Zip.OpenRaw()
			if err != nil {
				return nil, err
			}
			w, err := zw.CreateRaw(&h)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(w, r); err != nil {
				return nil, err
			}
		case strings.HasSuffix(name, "/"):
			if !dirEntry && slices.ContainsFunc(es, func(o Entry) bool { // a folder not empty: implied
				return len(o.Name) > len(e.Name) && strings.HasPrefix(o.Name, e.Name)
			}) {
				continue
			}
			if err := write(name, nil, nil); err != nil {
				return nil, err
			}
		default:
			data, err := os.ReadFile(e.File)
			if err != nil {
				return nil, err
			}
			if err := write(name, data, nil); err != nil {
				return nil, err
			}
		}
	}
	for _, f := range pending {
		if err := write(prefix+f.Name, f.Data, nil); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeDir writes a bundle folder.
func writeDir(b *Bundle, dst string, files []File, gone []string) error {
	if b != nil && !b.Zipped && SameFile(b.Path, dst) {
		for _, f := range files {
			if err := WriteFile(dst, f); err != nil {
				return err
			}
		}
		for _, g := range gone {
			if fs.ValidPath(g) {
				if err := os.Remove(filepath.Join(dst, filepath.FromSlash(g))); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
		}
		return nil
	}
	tmp := tempName(dst)
	if err := os.Mkdir(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp) // nothing left once renamed
	es, err := b.Entries()
	if err != nil {
		return err
	}
	oldPrefix := ""
	if b != nil && b.Zipped {
		oldPrefix = b.Prefix
	}
	for _, e := range es {
		rel, inside := strings.CutPrefix(e.Name, oldPrefix)
		if e.Zip == nil {
			rel, inside = e.Name, true
		}
		if !inside || rel == "" || IsJunk(e.Name) || slices.Contains(gone, rel) {
			continue
		}
		if slices.ContainsFunc(files, func(f File) bool { return f.Name == rel }) {
			continue
		}
		var data []byte
		switch {
		case strings.HasSuffix(rel, "/"):
		case e.Zip != nil:
			r, err := e.Zip.Open()
			if err != nil {
				return err
			}
			data, err = io.ReadAll(r)
			r.Close()
			if err != nil {
				return err
			}
		default:
			if data, err = os.ReadFile(e.File); err != nil {
				return err
			}
		}
		if err := WriteFile(tmp, File{rel, data}); err != nil {
			return err
		}
	}
	for _, f := range files {
		if err := WriteFile(tmp, f); err != nil {
			return err
		}
	}
	st, err := os.Lstat(dst)
	if err != nil {
		return os.Rename(tmp, dst)
	}
	switch {
	case st.Mode().IsRegular() && st.Size() == 0: // a save dialog's
		if err := os.Remove(dst); err != nil {
			return err
		}
		return os.Rename(tmp, dst)
	case st.IsDir():
		if PickText(dirRoot(dst), nil) == "" {
			return fmt.Errorf("%s is a folder but no bundle: not replaced", filepath.Base(dst))
		}
	case st.Mode().IsRegular():
		if data, err := os.ReadFile(dst); err != nil {
			return err
		} else if _, err := ReadZip(dst, data); err != nil {
			return fmt.Errorf("%s exists and is no bundle: not replaced", filepath.Base(dst))
		}
	default:
		return fmt.Errorf("%s exists and is no bundle: not replaced", filepath.Base(dst))
	}
	return swap(tmp, dst)
}

// WriteFile writes one file (or folder) into a bundle folder.
func WriteFile(dir string, f File) error {
	if !fs.ValidPath(strings.TrimSuffix(f.Name, "/")) {
		return fmt.Errorf("%q: no name for a file in a bundle", f.Name)
	}
	p := filepath.Join(dir, filepath.FromSlash(f.Name))
	if strings.HasSuffix(f.Name, "/") {
		return os.MkdirAll(p, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return WriteFileAtomic(p, f.Data)
}

// WriteFileAtomic writes a file through a temporary one renamed into
// place (through a symlink: to the file it points to), keeping its mode.
func WriteFileAtomic(p string, data []byte) error {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		p = real
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(p); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}
