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
	// v1.13.0's merged form, read as its scenes
	a, _ := fountain.OmitScene("INT. GONE - DAY #12#\n\nOld.\n\n")
	b, _ := fountain.OmitScene("INT. GONE - DAY #13#\n\nOld.\n\n")
	c, _ := fountain.OmitScene("INT. GONE - DAY #14#\n\nOld.\n\n")
	old := "INT. A - DAY #11#\n\nHi.\n\n.OMITTED #12-14#\n\n" + strings.TrimPrefix(a, ".OMITTED #12#\n\n") +
		strings.TrimPrefix(b, ".OMITTED #13#\n\n") + strings.TrimPrefix(c, ".OMITTED #14#\n\n") + "EXT. B - DAY #15#\n\nBye.\n"
	for name, src := range map[string]string{"now": text, "v1.13.0": old} {
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
