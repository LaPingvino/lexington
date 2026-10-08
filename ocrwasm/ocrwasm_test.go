package ocrwasm

import (
	"context"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
)

// The built-in Tesseract reads a page of a script.
func TestHOCR(t *testing.T) {
	f, err := os.Open("testdata/page.png")
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	o := New()
	defer o.Close(ctx)
	start := time.Now()
	hocr, err := o.HOCR(ctx, img)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("one page in %v", time.Since(start))
	for _, want := range []string{"ocrx_word", "KITCHEN", "biscuit"} {
		if !strings.Contains(hocr, want) {
			t.Errorf("no %q in the hOCR", want)
		}
	}
	text, err := o.Text(ctx, img)
	if err != nil || !strings.Contains(text, "Just one left") {
		t.Errorf("text %q, %v", text, err)
	}
}
