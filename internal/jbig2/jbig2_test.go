package jbig2

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sample is a JBIG2 stream saved from a PDF by TestExtractSamples.
type sample struct {
	name          string // e.g. alien-1979-screenplay-p002
	data, globals []byte
	width, height int
}

func loadSample(t *testing.T, dir, base string) sample {
	t.Helper()
	meta, err := os.ReadFile(filepath.Join(dir, base+".txt"))
	if os.IsNotExist(err) {
		t.Skipf("sample %s is missing", base)
	}
	if err != nil {
		t.Fatal(err)
	}
	f := strings.Fields(string(meta))
	if len(f) < 3 {
		t.Fatalf("%s.txt: bad metadata %q", base, meta)
	}
	s := sample{name: base}
	s.width, _ = strconv.Atoi(f[0])
	s.height, _ = strconv.Atoi(f[1])
	if s.data, err = os.ReadFile(filepath.Join(dir, base+".jb2")); err != nil {
		t.Fatal(err)
	}
	if f[2] != "-" {
		if s.globals, err = os.ReadFile(filepath.Join(dir, f[2])); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

// hashImage hashes the ink of an image: 1 bit per pixel, row by row.
func hashImage(img *image.Gray) string {
	h := sha256.New()
	b := img.Bounds()
	fmt.Fprintf(h, "%dx%d\n", b.Dx(), b.Dy())
	row := make([]byte, (b.Dx()+7)/8)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		clear(row)
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.GrayAt(x, y).Y < 128 {
				row[(x-b.Min.X)/8] |= 0x80 >> ((x - b.Min.X) % 8)
			}
		}
		h.Write(row)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// compareWithPNG compares img with a bi-level PNG made by pdfimages. The
// polarity of the PNG depends on the PDF's colour space, so both are
// tried; it returns the number of differing pixels for the better one.
func compareWithPNG(t *testing.T, img *image.Gray, pngFile string) (diff int, inverted bool, first []image.Point) {
	t.Helper()
	f, err := os.Open(pngFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ref, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if ref.Bounds().Size() != img.Bounds().Size() {
		t.Fatalf("%s: size %v, decoded %v", pngFile, ref.Bounds().Size(), img.Bounds().Size())
	}
	same, opposite := 0, 0
	var firstSame, firstOpp []image.Point
	rb := ref.Bounds()
	for y := 0; y < rb.Dy(); y++ {
		for x := 0; x < rb.Dx(); x++ {
			r, _, _, _ := ref.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			refBlack := r < 0x8000
			black := img.GrayAt(x, y).Y < 128
			if refBlack != black {
				same++
				if len(firstSame) < 5 {
					firstSame = append(firstSame, image.Pt(x, y))
				}
			} else {
				opposite++
				if len(firstOpp) < 5 {
					firstOpp = append(firstOpp, image.Pt(x, y))
				}
			}
		}
	}
	if opposite < same {
		return opposite, true, firstOpp
	}
	return same, false, firstSame
}

// pdfimagesPage renders the images of one page of file with poppler's
// pdfimages and returns the PNG of the first one.
func pdfimagesPage(t *testing.T, file string, page int) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("pdfimages", "-png", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), file, filepath.Join(dir, "img"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pdfimages: %v: %s", err, out)
	}
	return filepath.Join(dir, "img-000.png")
}

// samplePDF returns the source PDF of the samples called name, for the
// comparison with poppler: name.pdf in the directory JBIG2_PDF_DIR, or ""
// if it is not set.
func samplePDF(name string) string {
	dir := os.Getenv("JBIG2_PDF_DIR")
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name+".pdf")
}

// TestSamples decodes the real samples (page streams of scanned PDFs, not
// in the repository) in the directory JBIG2_SAMPLES, as written by
// TestExtractSamples, and checks them against their stored hashes (which
// were verified against poppler), and against poppler itself when
// pdfimages and the source PDFs are available: set JBIG2_PDF_DIR to the
// directory holding alien-1979-screenplay.pdf and
// 12-angry-men-1957-screenplay.pdf.
func TestSamples(t *testing.T) {
	dir := os.Getenv("JBIG2_SAMPLES")
	if dir == "" {
		t.Skip("set JBIG2_SAMPLES to a directory of real samples")
	}
	metas, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
	if len(metas) == 0 {
		t.Skip("no samples in JBIG2_SAMPLES")
	}
	_, popplerErr := exec.LookPath("pdfimages")
	for _, meta := range metas {
		base := strings.TrimSuffix(filepath.Base(meta), ".txt")
		t.Run(base, func(t *testing.T) {
			s := loadSample(t, dir, base)
			start := time.Now()
			img, err := Decode(s.data, s.globals, s.width, s.height)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("decoded %dx%d in %v", s.width, s.height, time.Since(start))
			got := hashImage(img)
			if want, err := os.ReadFile(filepath.Join(dir, base+".sha256")); err == nil {
				if strings.TrimSpace(string(want)) != got {
					t.Errorf("hash %s, want %s", got, strings.TrimSpace(string(want)))
				}
			} else {
				t.Logf("no stored hash; got %s", got)
			}

			i := strings.LastIndex(base, "-p")
			pdfFile := samplePDF(base[:i])
			page, _ := strconv.Atoi(base[i+2:])
			if popplerErr != nil || pdfFile == "" {
				return
			}
			if _, err := os.Stat(pdfFile); err != nil {
				return
			}
			diff, inverted, first := compareWithPNG(t, img, pdfimagesPage(t, pdfFile, page))
			if diff != 0 {
				t.Errorf("%d pixels differ from poppler (inverted %v), first at %v", diff, inverted, first)
			}
		})
	}
}

// TestBulk compares every sample extracted into JBIG2_BULK (by
// TestExtractSamples) with the PNGs pdfimages -p wrote into JBIG2_GT
// (named <prefix>-NNN-000.png, the prefix being the first word of the
// sample name).
func TestBulk(t *testing.T) {
	dir, gt := os.Getenv("JBIG2_BULK"), os.Getenv("JBIG2_GT")
	if dir == "" || gt == "" {
		t.Skip("set JBIG2_BULK and JBIG2_GT")
	}
	metas, _ := filepath.Glob(filepath.Join(dir, "*.txt"))
	total, failed := 0, 0
	for _, meta := range metas {
		base := strings.TrimSuffix(filepath.Base(meta), ".txt")
		s := loadSample(t, dir, base)
		i := strings.LastIndex(base, "-p")
		page, _ := strconv.Atoi(base[i+2:])
		prefix := strings.SplitN(base, "-", 2)[0]
		if prefix == "12" {
			prefix = "angry"
		}
		matches, _ := filepath.Glob(filepath.Join(gt, fmt.Sprintf("%s-%03d-*.png", prefix, page)))
		if len(matches) == 0 {
			t.Errorf("%s: no ground truth", base)
			continue
		}
		total++
		img, err := Decode(s.data, s.globals, s.width, s.height)
		if err != nil {
			failed++
			t.Errorf("%s: %v", base, err)
			continue
		}
		diff, inverted, first := compareWithPNG(t, img, matches[0])
		if diff != 0 {
			failed++
			t.Errorf("%s: %d pixels differ (inverted %v), first at %v", base, diff, inverted, first)
		} else {
			t.Logf("%s: identical (inverted %v) %s", base, inverted, hashImage(img))
		}
	}
	t.Logf("%d pages compared, %d failed", total, failed)
}

// pageImage converts a decoded page to an image, ink black.
func pageImage(p *page) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, p.width, p.height))
	for i, v := range p.buf {
		if v == 0 {
			img.Pix[i] = 255
		}
	}
	return img
}

// TestStandalone decodes standalone JBIG2 test files that all encode the
// same 399x400 image, testdata/bitmap.png, using many JBIG2 features. The
// files come from SerenityOS (Tests/LibGfx/test-inputs/jbig2, BSD-2-Clause
// licence); a few are in testdata/standalone, the whole set is used when
// JBIG2_SERENITY names a directory holding them.
func TestStandalone(t *testing.T) {
	files, _ := filepath.Glob("testdata/standalone/*.jbig2")
	if dir := os.Getenv("JBIG2_SERENITY"); dir != "" {
		more, _ := filepath.Glob(filepath.Join(dir, "*.jbig2"))
		files = append(files, more...)
	}
	if len(files) == 0 {
		t.Skip("no standalone test files")
	}
	f, err := os.Open("testdata/bitmap.png")
	if err != nil {
		t.Skip(err)
	}
	ref, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, file := range files {
		name := filepath.Base(file)
		if name == "annex-h.jbig2" {
			continue // see TestAnnexH
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p, err := decodeFile(data, 1)
		if err != nil {
			failed++
			t.Errorf("%s: %v", name, err)
			continue
		}
		img := pageImage(p)
		// Sequentially organised files also go through Decode, as the
		// segments of a PDF stream: without the file header.
		if data[8]&1 != 0 {
			start := 9
			if data[8]&2 == 0 {
				start += 4
			}
			emb, err := Decode(data[start:], nil, 399, 400)
			if err != nil {
				failed++
				t.Errorf("%s: Decode: %v", name, err)
				continue
			}
			if !equalImages(emb, img) {
				failed++
				t.Errorf("%s: Decode differs from decodeFile", name)
			}
		}
		if img.Bounds() != ref.Bounds() {
			failed++
			t.Errorf("%s: size %v, want %v", name, img.Bounds(), ref.Bounds())
			continue
		}
		diff := 0
		var first image.Point
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				r, _, _, _ := ref.At(x, y).RGBA()
				if (r < 0x8000) != (img.GrayAt(x, y).Y < 128) {
					if diff == 0 {
						first = image.Pt(x, y)
					}
					diff++
				}
			}
		}
		if diff != 0 {
			failed++
			t.Errorf("%s: %d pixels differ, first at %v", name, diff, first)
		}
	}
	t.Logf("%d files, %d failed", len(files), failed)
}

func equalImages(a, b *image.Gray) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.GrayAt(x, y) != b.GrayAt(x, y) {
				return false
			}
		}
	}
	return true
}

// TestAnnexH decodes the example data stream of T.88 Annex H.1: three
// pages, the first two the same image, the third a part of it.
func TestAnnexH(t *testing.T) {
	data, err := os.ReadFile("testdata/standalone/annex-h.jbig2")
	if err != nil {
		t.Skip(err)
	}
	var pages [3]*image.Gray
	for i := range pages {
		p, err := decodeFile(data, uint32(i+1))
		if err != nil {
			t.Fatalf("page %d: %v", i+1, err)
		}
		pages[i] = pageImage(p)
	}
	if got := pages[0].Bounds().Size(); got != image.Pt(64, 56) {
		t.Fatalf("page 1 size %v", got)
	}
	if !equalImages(pages[0], pages[1]) {
		t.Errorf("pages 1 and 2 differ")
	}
	if got := pages[2].Bounds().Size(); got != image.Pt(37, 8) {
		t.Fatalf("page 3 size %v", got)
	}
	for y := 0; y < 8; y++ {
		for x := 0; x < 37; x++ {
			if pages[2].GrayAt(x, y) != pages[1].GrayAt(x+4, y+1) {
				t.Fatalf("page 3 differs from page 2 at %d,%d", x, y)
			}
		}
	}
}

// splitGlobals moves the symbol dictionaries of a PDF JBIG2 stream into a
// globals stream, as PDF producers that share dictionaries do.
func splitGlobals(t *testing.T, data []byte) (page, globals []byte) {
	t.Helper()
	for pos := 0; pos < len(data); {
		h := readSegmentHeader(data, pos)
		end := h.headerEnd + h.length
		if h.typ == 0 {
			globals = append(globals, data[pos:end]...)
		} else {
			page = append(page, data[pos:end]...)
		}
		pos = end
	}
	return page, globals
}

// embedded returns a standalone test file as a PDF JBIG2 stream: without
// the file header (sequentially organised files only).
func embedded(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata/standalone", name))
	if os.IsNotExist(err) {
		t.Skipf("%s is missing", name)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 13 || data[8]&1 == 0 {
		t.Fatalf("%s: not sequentially organised", name)
	}
	if data[8]&2 == 0 {
		return data[13:]
	}
	return data[9:]
}

// reference returns testdata/bitmap.png, the image of the standalone
// files, as an image.Gray.
func reference(t *testing.T) *image.Gray {
	t.Helper()
	f, err := os.Open("testdata/bitmap.png")
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	ref, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	g := image.NewGray(ref.Bounds())
	for y := 0; y < g.Rect.Dy(); y++ {
		for x := 0; x < g.Rect.Dx(); x++ {
			if r, _, _, _ := ref.At(x, y).RGBA(); r >= 0x8000 {
				g.Pix[y*g.Stride+x] = 255
			}
		}
	}
	return g
}

// TestGlobals checks that symbol dictionaries in the JBIG2Globals stream
// are used by the page stream: the symbol dictionaries of conformance
// files are moved into a globals stream, as PDF producers that share
// dictionaries do.
func TestGlobals(t *testing.T) {
	ref := reference(t)
	for _, name := range []string{
		"bitmap-symbol.jbig2",
		"bitmap-symbol-symhuff-texthuff.jbig2",
		"bitmap-symbol-symbolrefine-textrefine.jbig2",
	} {
		data := embedded(t, name)
		page, globals := splitGlobals(t, data)
		if len(globals) == 0 {
			t.Fatalf("%s: no symbol dictionary", name)
		}
		img, err := Decode(page, globals, 399, 400)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !equalImages(img, ref) {
			t.Errorf("%s: decoding with globals differs", name)
		}
		if _, err := Decode(page, nil, 399, 400); err == nil {
			t.Errorf("%s: no error without the globals", name)
		}
	}
}

// TestSizes checks that the page is cropped or padded to the PDF size.
func TestSizes(t *testing.T) {
	data := embedded(t, "bitmap-symbol.jbig2")
	full, err := Decode(data, nil, 399, 400)
	if err != nil {
		t.Fatal(err)
	}
	if !equalImages(full, reference(t)) {
		t.Fatal("image differs from testdata/bitmap.png")
	}
	small, err := Decode(data, nil, 100, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !equalImages(small, full.SubImage(image.Rect(0, 0, 100, 50)).(*image.Gray)) {
		t.Error("cropped image differs")
	}
	big, err := Decode(data, nil, 399+10, 400+10)
	if err != nil {
		t.Fatal(err)
	}
	if !equalImages(big.SubImage(image.Rect(0, 0, 399, 400)).(*image.Gray), full) {
		t.Error("padded image differs")
	}
	if big.GrayAt(399+5, 5).Y != 255 || big.GrayAt(5, 400+5).Y != 255 {
		t.Error("padding is not white")
	}
	if _, err := Decode(data, nil, 0, 10); err == nil {
		t.Error("no error for an empty image")
	}
}

// TestCorrupt checks that corrupt data gives errors, not panics or hangs.
func TestCorrupt(t *testing.T) {
	if _, err := Decode(nil, nil, 10, 10); err == nil {
		t.Error("no error for empty data")
	}
	if _, err := Decode([]byte("not a jbig2 stream at all"), nil, 10, 10); err == nil {
		t.Error("no error for garbage")
	}
	for _, name := range []string{
		"bitmap-symbol.jbig2",
		"bitmap-symbol-symhuff-texthuff.jbig2",
		"bitmap-mmr.jbig2",
		"bitmap-tpgdon.jbig2",
		"bitmap-halftone.jbig2",
		"bitmap-refine.jbig2",
	} {
		data := append([]byte(nil), embedded(t, name)...)
		// truncated streams
		for n := 0; n < len(data); n += len(data)/23 + 1 {
			Decode(data[:n], nil, 399, 400)
		}
		// changed bytes
		for i := 0; i < len(data); i += len(data)/29 + 1 {
			for _, x := range []byte{0x01, 0x5a} {
				data[i] ^= x
				Decode(data, nil, 399, 400)
				data[i] ^= x
			}
		}
	}
}

// FuzzDecode looks for panics and hangs on corrupt data.
func FuzzDecode(f *testing.F) {
	files, _ := filepath.Glob("testdata/standalone/*.jbig2")
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil || len(data) < 13 || data[8]&1 == 0 {
			continue // missing, or random access organisation
		}
		if data[8]&2 == 0 {
			f.Add(data[13:])
		} else {
			f.Add(data[9:])
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		Decode(data, nil, 399, 400)
	})
}
