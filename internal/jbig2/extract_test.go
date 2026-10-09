package jbig2

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/internal/pdf"
)

// TestExtractSamples writes the JBIG2 streams of a PDF as test samples. It
// only runs when JBIG2_EXTRACT names the PDF and JBIG2_OUT the directory:
//
//	JBIG2_EXTRACT=file.pdf JBIG2_OUT=dir go test -run ExtractSamples
//
// Each page's image is written as <name>-pNNN.jb2, its globals as
// <name>-globals-<hash>.jb2g, and <name>-pNNN.txt holds
// "width height globalsfile Decode=... ColorSpace=...".
func TestExtractSamples(t *testing.T) {
	file, out := os.Getenv("JBIG2_EXTRACT"), os.Getenv("JBIG2_OUT")
	if file == "" || out == "" {
		t.Skip("set JBIG2_EXTRACT and JBIG2_OUT to extract samples")
	}
	f, r, err := pdf.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	name := strings.TrimSuffix(filepath.Base(file), ".pdf")
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		xobjects := p.V.Key("Resources").Key("XObject")
		for _, key := range xobjects.Keys() {
			x := xobjects.Key(key)
			if x.Key("Subtype").Name() != "Image" || x.Key("Filter").Name() != "JBIG2Decode" {
				continue
			}
			data, err := io.ReadAll(x.RawReader())
			if err != nil {
				t.Fatal(err)
			}
			gname := "-"
			if g := x.Key("DecodeParms").Key("JBIG2Globals"); g.Kind() == pdf.Stream {
				gdata, err := io.ReadAll(g.Reader())
				if err != nil {
					t.Fatal(err)
				}
				gname = fmt.Sprintf("%s-globals-%x.jb2g", name, sha256.Sum256(gdata))
				if err := os.WriteFile(filepath.Join(out, gname), gdata, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			base := fmt.Sprintf("%s-p%03d", name, i)
			if err := os.WriteFile(filepath.Join(out, base+".jb2"), data, 0o644); err != nil {
				t.Fatal(err)
			}
			meta := fmt.Sprintf("%d %d %s Decode=%v ColorSpace=%v\n", x.Key("Width").Int64(), x.Key("Height").Int64(), gname, x.Key("Decode"), x.Key("ColorSpace"))
			if err := os.WriteFile(filepath.Join(out, base+".txt"), []byte(meta), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
