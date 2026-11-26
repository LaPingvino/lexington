package osf

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LaPingvino/lexington/bundle"
)

// DocumentName is the OSF document's name in a .fadein.
const DocumentName = "document.xml"

// A .fadein is a zip holding the script as document.xml, as Fade In
// writes it; like a .highland it can be a folder too (Name.fadein/, the
// zip unzipped), read and written the same. Whatever else is in one is
// kept as it is when the script is written back into it.

// IsFadeIn tells whether a name is a .fadein's.
func IsFadeIn(name string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimRight(name, `/\`)), ".fadein")
}

// ReadFadeIn is the OSF document in the .fadein at p, a zip or a folder.
func ReadFadeIn(p string) ([]byte, error) {
	st, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		return os.ReadFile(filepath.Join(p, DocumentName))
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return FadeInDocument(data)
}

// FadeInDocument is the OSF document in a .fadein's zip.
func FadeInDocument(data []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a Fade In file (a zip): %w", err)
	}
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, DocumentName) {
			r, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer r.Close()
			return io.ReadAll(r)
		}
	}
	return nil, errors.New("not a Fade In file: no " + DocumentName + " in it")
}

// FadeInZip is a .fadein's zip with doc as its document: base's (a
// .fadein's zip, nil for none) other entries copied through as they are.
func FadeInZip(doc, base []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: DocumentName, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(doc); err != nil {
		return nil, err
	}
	if base != nil {
		if zr, err := zip.NewReader(bytes.NewReader(base), int64(len(base))); err == nil {
			for _, f := range zr.File {
				if strings.EqualFold(f.Name, DocumentName) {
					continue
				}
				if err := zw.Copy(f); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteFadeIn writes doc as the document of the .fadein at dst, in form
// (bundle.Auto: as it is on disk, else a zip, else a folder for a name
// ending in a slash); what else a .fadein there holds is kept.
func WriteFadeIn(dst string, form bundle.Form, doc []byte) error {
	if form == bundle.Auto {
		form = bundle.Zip
		if st, err := os.Stat(dst); err == nil && st.IsDir() || strings.HasSuffix(dst, "/") || strings.HasSuffix(dst, string(filepath.Separator)) {
			form = bundle.Dir
		}
	}
	dst = strings.TrimRight(dst, `/\`)
	st, statErr := os.Stat(dst)
	if form == bundle.Dir {
		if statErr == nil && !st.IsDir() {
			// a zipped one becomes a folder: its entries unzipped
			data, err := os.ReadFile(dst)
			if err != nil {
				return err
			}
			tmp := dst + ".lexington-tmp"
			if err := unzipTo(data, tmp); err != nil {
				return err
			}
			if err := bundle.WriteFileAtomic(filepath.Join(tmp, DocumentName), doc); err != nil {
				return err
			}
			if err := os.Remove(dst); err != nil {
				return err
			}
			return os.Rename(tmp, dst)
		}
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		return bundle.WriteFileAtomic(filepath.Join(dst, DocumentName), doc)
	}
	var base []byte
	if statErr == nil && st.IsDir() {
		// a folder becomes a zip: its files zipped
		var err error
		if base, err = zipDir(dst); err != nil {
			return err
		}
	} else if statErr == nil {
		base, _ = os.ReadFile(dst)
	}
	data, err := FadeInZip(doc, base)
	if err != nil {
		return err
	}
	if statErr == nil && st.IsDir() {
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
	}
	return bundle.WriteFileAtomic(dst, data)
}

func unzipTo(data []byte, dir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if !filepath.IsLocal(name) {
			continue
		}
		p := filepath.Join(dir, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			continue
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			return err
		}
	}
	return os.MkdirAll(dir, 0o755)
}

func zipDir(dir string) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = w.Write(body)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
