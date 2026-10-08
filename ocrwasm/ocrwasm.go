// Package ocrwasm is OCR without anything to install: Tesseract compiled
// to WebAssembly (github.com/danlock/gogosseract, run by wazero), with
// the English model of tessdata_fast built in. It is slower than an
// installed tesseract, so programs use it when there is none: Lexington's
// pdfin for scanned scripts, Recuerdo for word lists in pictures.
//
// It is a module of its own, so that Lexington itself does not need
// wazero and the model.
package ocrwasm

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"io"
	"sync"

	"github.com/danlock/gogosseract"
)

// The English model of github.com/tesseract-ocr/tessdata_fast (Apache 2.0).
//
//go:embed eng.traineddata
var english []byte

// OCR is a WebAssembly Tesseract. Its methods may be called from several
// goroutines; it reads one image at a time.
type OCR struct {
	// Language is the model's language, "eng" for the built-in one;
	// TrainingData its .traineddata (nil: the built-in English one).
	Language     string
	TrainingData []byte

	mu   sync.Mutex
	tess *gogosseract.Tesseract
}

// New is the built-in English OCR.
func New() *OCR { return &OCR{} }

func (o *OCR) start(ctx context.Context) error {
	if o.tess != nil {
		return nil
	}
	lang, data := o.Language, o.TrainingData
	if data == nil {
		lang, data = "eng", english
	}
	cfg := gogosseract.Config{Language: lang, TrainingData: bytes.NewReader(data)}
	cfg.Stdout, cfg.Stderr = io.Discard, io.Discard // Tesseract's chatter
	tess, err := gogosseract.New(ctx, cfg)
	if err != nil {
		return fmt.Errorf("starting the OCR: %v", err)
	}
	o.tess = tess
	return nil
}

func (o *OCR) load(ctx context.Context, img image.Image) error {
	if err := o.start(ctx); err != nil {
		return err
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return err
	}
	return o.tess.LoadImage(ctx, &b, gogosseract.LoadImageOptions{})
}

// HOCR is the image's text as hOCR: each word with its box.
func (o *OCR) HOCR(ctx context.Context, img image.Image) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.load(ctx, img); err != nil {
		return "", err
	}
	return o.tess.GetHOCR(ctx, nil)
}

// Text is the image's text.
func (o *OCR) Text(ctx context.Context, img image.Image) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.load(ctx, img); err != nil {
		return "", err
	}
	return o.tess.GetText(ctx, nil)
}

// Close frees the WebAssembly Tesseract.
func (o *OCR) Close(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.tess == nil {
		return nil
	}
	err := o.tess.Close(ctx)
	o.tess = nil
	return err
}
