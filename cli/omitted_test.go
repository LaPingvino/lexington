package cli

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
)

// A run of OMITTED scenes, one heading a number in the text, prints as
// one "12-14 OMITTED 12-14" line when asked, in every format but FDX
// (one heading a number).
func TestMergeOmittedRuns(t *testing.T) {
	dir := t.TempDir()
	text := "INT. A - DAY #11#\n\nHi.\n\n"
	for _, n := range []string{"12", "13", "14"} {
		o, _ := fountain.OmitScene("INT. GONE - DAY #" + n + "#\n\nOld.\n\n")
		text += o
	}
	text += "EXT. B - DAY #15#\n\nBye.\n"
	for name, src := range map[string]string{"now": text} {
		in := filepath.Join(dir, "s.fountain")
		if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		export := func(to string, flags ...string) string {
			t.Helper()
			out := filepath.Join(dir, "out."+to)
			args := append([]string{"-i", in, "-to", to, "-o", out, "-config", filepath.Join(dir, "none.toml")}, flags...)
			if got := Main(context.Background(), args); got != 0 {
				t.Fatalf("%s %s %q: exit %d", name, to, flags, got)
			}
			switch to {
			case "pdf":
				if _, err := exec.LookPath("pdftotext"); err != nil {
					t.Skip("no pdftotext")
				}
				b, err := exec.Command("pdftotext", "-layout", out, "-").Output()
				if err != nil {
					t.Fatal(err)
				}
				return string(b)
			case "docx", "odt":
				z, err := zip.OpenReader(out)
				if err != nil {
					t.Fatal(err)
				}
				defer z.Close()
				var all strings.Builder
				for _, f := range z.File {
					if f.Name == "word/document.xml" || f.Name == "content.xml" {
						r, _ := f.Open()
						b, _ := io.ReadAll(r)
						r.Close()
						all.Write(b)
					}
				}
				return all.String()
			}
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
		omitted := regexp.MustCompile(`OMITTED`)
		for _, to := range []string{"pdf", "html", "docx", "odt"} {
			got := export(to, "-mergeomitted")
			if n := len(omitted.FindAllString(got, -1)); n != 1 || !strings.Contains(got, "12-14") {
				t.Errorf("%s %s merged: %d OMITTED, 12-14 %v", name, to, n, strings.Contains(got, "12-14"))
			}
			got = export(to)
			if n := len(omitted.FindAllString(got, -1)); n != 3 || strings.Contains(got, "12-14") {
				t.Errorf("%s %s not merged: %d OMITTED, 12-14 %v", name, to, n, strings.Contains(got, "12-14"))
			}
		}
		pdf := export("pdf", "-mergeomitted")
		if !regexp.MustCompile(`(?m)^\s*12-14\s+OMITTED\s+12-14\s*$`).MatchString(pdf) {
			t.Errorf("%s: no 12-14 OMITTED 12-14 line in\n%s", name, pdf)
		}
		fdx := export("fdx", "-mergeomitted", "-lengths")
		for _, n := range []string{"12", "13", "14"} {
			if !strings.Contains(fdx, `Number="`+n+`"`) {
				t.Errorf("%s: FDX lacks heading %s", name, n)
			}
		}
		if strings.Contains(fdx, "12-14") {
			t.Errorf("%s: FDX has the run as one", name)
		}
		if n := strings.Count(fdx, "<SceneProperties"); n != 5 {
			t.Errorf("%s: FDX has %d SceneProperties, want 5", name, n)
		}
		if f := export("fountain", "-mergeomitted"); strings.Contains(f, "12-14") {
			t.Errorf("%s: Fountain has the run as one", name)
		}
	}
}

// A run of fixed labels prints as one too, joined by an en dash.
func TestMergeOmittedFixedLabels(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("no pdftotext")
	}
	dir := t.TempDir()
	text := "INT. A - DAY #I-1#\n\nHi.\n\n.OMITTED #I-1-A#\n\n.OMITTED #I-1-C#\n\nEXT. B - DAY #I-2#\n\nBye.\n"
	in, out := filepath.Join(dir, "s.fountain"), filepath.Join(dir, "s.pdf")
	if err := os.WriteFile(in, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Main(context.Background(), []string{"-i", in, "-o", out, "-mergeomitted", "-config", filepath.Join(dir, "none.toml")}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	b, err := exec.Command("pdftotext", "-layout", out, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^\s*I-1-A–I-1-C\s+OMITTED\s+I-1-A–I-1-C\s*$`).Match(b) {
		t.Errorf("no I-1-A–I-1-C OMITTED line in\n%s", b)
	}
}
