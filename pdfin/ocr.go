package pdfin

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"image"
	"image/png"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// OCR recognises the text of a scanned page: an image in, hOCR out (the
// HTML with each word's box that Tesseract writes).
type OCR interface {
	HOCR(ctx context.Context, img image.Image) (string, error)
}

// Options for reading a PDF.
type Options struct {
	// OCR reads pages without text (a scanned script); without it such
	// a PDF is an error.
	OCR OCR
	// Progress, if set, is told how many of the pages are read.
	Progress func(done, pages int)
	// Skipped, if set, is told of a page that could not be read (a broken
	// stream, a scan in a format that cannot be decoded); the script is
	// read without it.
	Skipped func(page int, err error)
}

// Tesseract is the tesseract program as OCR, for -l Language ("eng"
// if empty).
type Tesseract struct {
	Path     string
	Language string
}

// InstalledTesseract is the tesseract program if it is installed.
func InstalledTesseract() (*Tesseract, bool) {
	path, err := exec.LookPath("tesseract")
	if err != nil {
		return nil, false
	}
	return &Tesseract{Path: path}, true
}

func (t *Tesseract) HOCR(ctx context.Context, img image.Image) (string, error) {
	data, err := pngOf(img)
	if err != nil {
		return "", err
	}
	lang := t.Language
	if lang == "" {
		lang = "eng"
	}
	cmd := exec.CommandContext(ctx, t.Path, "stdin", "stdout", "-l", lang, "hocr")
	cmd.Stdin = bytes.NewReader(data)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract: %v: %s", err, strings.TrimSpace(errs.String()))
	}
	return out.String(), nil
}

func encodePNG(w io.Writer, img image.Image) error { return png.Encode(w, img) }

var (
	hocrLine = regexp.MustCompile(`class=['"]ocr_(?:line|caption|header|textfloat)['"][^>]*title=['"]bbox (\d+) (\d+) (\d+) (\d+)`)
	hocrWord = regexp.MustCompile(`(?s)class=['"]ocrx_word['"][^>]*title=['"]bbox (\d+) (\d+) (\d+) (\d+)[^'"]*['"][^>]*>(.*?)</span>`)
	hocrTags = regexp.MustCompile(`<[^>]*>`)
)

type box struct {
	x0, y0, x1, y1 float64
	text           string
}

// hocrLines are the lines of an OCRed page, as if printed: each word's
// position in inches, words close together in one run. dpi is the
// image's pixels per inch.
func hocrLines(page int, hocr string, dpi float64) []line {
	// the words, each in the line whose box comes before it
	lineAt := hocrLine.FindAllStringSubmatchIndex(hocr, -1)
	var lines [][]box
	for _, m := range hocrWord.FindAllStringSubmatchIndex(hocr, -1) {
		n := 0
		for n < len(lineAt) && lineAt[n][0] < m[0] {
			n++
		}
		for len(lines) < n {
			lines = append(lines, nil)
		}
		text := strings.TrimSpace(html.UnescapeString(hocrTags.ReplaceAllString(hocr[m[10]:m[11]], "")))
		if text == "" {
			continue
		}
		num := func(i int) float64 { v, _ := strconv.Atoi(hocr[m[i]:m[i+1]]); return float64(v) / dpi }
		b := box{num(2), num(4), num(6), num(8), text}
		if n == 0 {
			lines = append(lines, nil)
			n = 1
		}
		lines[n-1] = append(lines[n-1], b)
	}
	var out []line
	for _, ws := range lines {
		if len(ws) == 0 {
			continue
		}
		sort.Slice(ws, func(i, j int) bool { return ws[i].x0 < ws[j].x0 })
		// a character's width, from the words: a gap of several is
		// another run (a scene number, the second column)
		chars, width := 0, 0.0
		for _, w := range ws {
			chars += len([]rune(w.text))
			width += w.x1 - w.x0
		}
		cw := width / float64(max(chars, 1))
		var runs []run
		bottom := 0.0
		for _, w := range ws {
			bottom = max(bottom, w.y1)
			if n := len(runs); n > 0 && w.x0-runs[n-1].End < 3*cw {
				runs[n-1].Text += " " + w.text
				runs[n-1].End = w.x1
				continue
			}
			runs = append(runs, run{X: w.x0, End: w.x1, Text: w.text})
		}
		out = append(out, line{Page: page, Y: bottom, Runs: runs})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Y < out[j].Y })
	// Tesseract makes lines of their own of what stands apart on one
	// line (scene numbers in the margins, dual dialogue's columns)
	var merged []line
	for _, l := range out {
		// (a margin's scene number can sit a little off the line)
		if n := len(merged); n > 0 && l.Y-merged[n-1].Y < 0.1 {
			m := &merged[n-1]
			m.Runs = append(m.Runs, l.Runs...)
			sort.SliceStable(m.Runs, func(i, j int) bool { return m.Runs[i].X < m.Runs[j].X })
			m.Y = max(m.Y, l.Y)
			continue
		}
		merged = append(merged, l)
	}
	return merged
}
