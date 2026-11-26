package pdfin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/pdf"
	"github.com/LaPingvino/lexington/rules"
)

var scenes = rules.DefaultConf().Scenes("en")

// fountainOf writes a screenplay as Fountain, without what a PDF cannot
// give back: sections, page breaks and the blank lines around them.
func fountainOf(t *testing.T, s lex.Screenplay) string {
	t.Helper()
	var b strings.Builder
	if err := (&fountain.FountainWriter{SceneConfig: scenes}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	out := regexp.MustCompile(`(?m)^(#.*|===)\n\n?`).ReplaceAllString(b.String(), "")
	return regexp.MustCompile(`\n{3,}`).ReplaceAllString(strings.TrimSpace(out), "\n\n")
}

// A script printed by Lexington reads back as the same script.
func TestRoundTrip(t *testing.T) {
	src, err := os.ReadFile("../examples/tv-episode.fountain")
	if err != nil {
		t.Fatal(err)
	}
	orig := fountain.Parse(scenes, strings.NewReader(string(src)))
	file := filepath.Join(t.TempDir(), "tv.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, orig); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := fountainOf(t, got), fountainOf(t, orig); a != b {
		t.Errorf("read back:\n%s\n\nwant:\n%s", a, b)
	}
}

func ln(page int, y float64, runs ...run) line { return line{Page: page, Y: y, Runs: runs} }
func at(x float64, text string) run {
	return run{X: x, End: x + 0.1*float64(len([]rune(text))), Text: text}
}

// What other programs print at page breaks: page numbers, (MORE), the
// name again with (CONT'D), scene numbers in both margins.
func TestPageBreaks(t *testing.T) {
	const step = 1.0 / 6
	page1 := []line{
		ln(1, 0.5, at(7.0, "1.")),
		ln(1, 1.0, at(1.0, "12"), at(1.5, "INT. HOUSE - DAY"), at(7.6, "12")),
		ln(1, 1.0+2*step, at(1.5, "Anna sits.")),
		ln(1, 1.0+4*step, at(3.7, "ANNA")),
		ln(1, 1.0+5*step, at(2.5, "I have been waiting")),
		ln(1, 1.0+6*step, at(2.5, "all morning")),
		ln(1, 1.0+7*step, at(3.7, "(MORE)")),
	}
	page2 := []line{
		ln(2, 0.5, at(7.0, "2.")),
		ln(2, 1.0, at(3.7, "ANNA (CONT'D)")),
		ln(2, 1.0+step, at(2.5, "for this biscuit.")),
		ln(2, 1.0+3*step, at(1.5, "She eats it.")),
	}
	s := classify([][]line{page1, page2}, 8.5, scenePrefix)
	var got []string
	for _, l := range s {
		got = append(got, string(l.Type)+": "+l.Contents)
	}
	want := []string{
		"scene: INT. HOUSE - DAY #12#",
		"empty: ",
		"action: Anna sits.",
		"empty: ",
		"speaker: ANNA",
		"dialog: I have been waiting all morning for this biscuit.",
		"empty: ",
		"action: She eats it.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A line with the same scene number in both margins is a scene heading,
// INT. or not, words or not, as shooting scripts number title cards and
// sung lines.
func TestNumberedLinesAreScenes(t *testing.T) {
	const step = 1.0 / 6
	page := []line{
		ln(1, 0.5, at(7.0, "1.")),
		ln(1, 1.0, at(0.75, "19"), at(1.5, "INT. PALACE - DAY"), at(7.18, "19")),
		ln(1, 1.0+2*step, at(0.75, "20"), at(7.18, "20")),
		ln(1, 1.0+4*step, at(1.5, "A grand room.")),
		ln(1, 1.0+6*step, at(0.75, "132"), at(1.5, "THE TROUPE OF PLAYERS"), at(7.08, "132")),
		ln(1, 1.0+8*step, at(3.5, "IN")),
	}
	var got []string
	for _, l := range classify([][]line{page}, 8.5, scenePrefix) {
		if l.Type == lex.TypeScene {
			got = append(got, l.Contents)
		}
	}
	if want := "INT. PALACE - DAY #19#|#20#|THE TROUPE OF PLAYERS #132#"; strings.Join(got, "|") != want {
		t.Errorf("scenes %q", got)
	}
}

func TestTitlePage(t *testing.T) {
	page := []line{
		ln(1, 4.0, at(3.75, "BISCUITS")),
		ln(1, 4.5, at(3.65, "Written by")),
		ln(1, 4.8, at(3.6, "Anna Smith")),
		ln(1, 9.0, at(1.5, "anna@example.com")),
		ln(1, 9.2, at(1.5, "Draft, May 2026")),
	}
	s, ok := titlePage(page, 8.5, scenePrefix)
	if !ok {
		t.Fatal("no title page")
	}
	var got []string
	for _, l := range s {
		got = append(got, string(l.Type)+"="+l.Contents)
	}
	want := "titlepage=|Title=BISCUITS|Credit=Written by|Author=Anna Smith|Contact=anna@example.com|Draft date=Draft, May 2026|newpage="
	if strings.Join(got, "|") != want {
		t.Errorf("got %s", strings.Join(got, "|"))
	}
	// a first page with a scene is no title page
	if _, ok := titlePage([]line{ln(1, 1, at(1.5, "INT. HOUSE - DAY"))}, 8.5, scenePrefix); ok {
		t.Error("scene page taken for the title page")
	}
}

func TestHOCRLines(t *testing.T) {
	// 100 dpi: a heading and the two columns of dual dialogue, which
	// Tesseract gives as separate lines on one baseline
	hocr := `<div class='ocr_page'>
<span class='ocr_line' title="bbox 150 100 400 112; baseline 0 0">
<span class='ocrx_word' title='bbox 150 100 180 112; x_wconf 95'>INT.</span>
<span class='ocrx_word' title='bbox 190 100 250 112; x_wconf 95'>HOUSE</span></span>
<span class='ocr_line' title="bbox 250 200 290 212"><span class='ocrx_word' title='bbox 250 200 290 212'>ANNA</span></span>
<span class='ocr_line' title="bbox 550 201 590 212"><span class='ocrx_word' title='bbox 550 201 590 212'>BRAM</span></span>
<span class='ocr_line' title="bbox 200 220 300 232"><span class='ocrx_word' title='bbox 200 220 230 232'>Tom &amp;</span><span class='ocrx_word' title='bbox 240 220 300 232'><strong>Jerry</strong></span></span>
</div>`
	ls := hocrLines(1, hocr, 100)
	var got []string
	for _, l := range ls {
		var rs []string
		for _, r := range l.Runs {
			rs = append(rs, r.Text)
		}
		got = append(got, strings.Join(rs, " | "))
	}
	if strings.Join(got, "\n") != "INT. HOUSE\nANNA | BRAM\nTom & Jerry" {
		t.Errorf("lines:\n%s", strings.Join(got, "\n"))
	}
	if ls[0].Runs[0].X != 1.5 || ls[1].Runs[1].X != 5.5 {
		t.Errorf("positions: %+v", ls)
	}
}

// A scanned script (images only) reads back with OCR, if tesseract is
// installed.
func TestScannedScript(t *testing.T) {
	ts, ok := InstalledTesseract()
	if !ok {
		t.Skip("no tesseract")
	}
	if _, err := ReadFile("testdata/scanned-tv-episode.pdf"); err == nil {
		t.Error("a scan without OCR should be an error")
	}
	got, err := ReadFileWith("testdata/scanned-tv-episode.pdf", Options{OCR: ts})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile("../examples/tv-episode.fountain")
	want := fountainOf(t, fountain.Parse(scenes, strings.NewReader(string(src))))
	// Tesseract recognises some scene numbers in the margins, not all
	noNumbers := regexp.MustCompile(` #\d+#`)
	want = noNumbers.ReplaceAllString(want, "")
	if a := noNumbers.ReplaceAllString(fountainOf(t, got), ""); a != want {
		t.Errorf("read back:\n%s\n\nwant:\n%s", a, want)
	}
}

// A cancelled read stops, with the context's error; until then every page
// is counted, so the count reaches the end.
func TestReadCancelAndProgress(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var counts []int
	_, err := ReadFileWith("testdata/scanned-tv-episode.pdf", Options{OCR: fakeOCR{}, Context: ctx, Progress: func(done, pages int) {
		counts = append(counts, done)
		cancel()
	}})
	if !errors.Is(err, context.Canceled) || len(counts) != 1 {
		t.Errorf("cancelled: %v after %v", err, counts)
	}
}

// fakeOCR reads nothing.
type fakeOCR struct{}

func (fakeOCR) HOCR(context.Context, image.Image) (string, error) { return "", nil }

// tightPDF is a one-page PDF with lines in Courier, set slightly tighter
// than the font's widths (as some Final Draft PDFs are).
func tightPDF(lines ...string) []byte {
	var content strings.Builder
	// each glyph moved 0.19 pt left of where its width puts it (a TJ
	// array's kerning, in thousandths of the font size)
	content.WriteString("BT /F1 12 Tf 14 TL 108 700 Td\n")
	for _, l := range lines {
		content.WriteString("[")
		for _, r := range l {
			fmt.Fprintf(&content, "(%c) 15.83 ", r)
		}
		content.WriteString("] TJ T*\n")
	}
	content.WriteString("ET")
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /FirstChar 32 /LastChar 126 /Widths [" +
			strings.Repeat("600 ", 95) + "] >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}

// Tight tracking does not split long lines' words: the end of the text
// was estimated from widths that added up past the glyphs.
func TestTightTrackingKeepsWords(t *testing.T) {
	text := "Deep blue sky overhead. Fat, scuddy clouds. Below them, black and"
	pdfData := tightPDF("INT. COW PASTURE - DAY", "", text)
	s, err := Read(bytes.NewReader(pdfData), int64(len(pdfData)))
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, l := range s {
		all = append(all, l.Contents)
	}
	if got := strings.Join(all, "|"); !strings.Contains(got, text) {
		t.Errorf("read %q", got)
	}
}

func TestScenePattern(t *testing.T) {
	ru := scenePattern([]string{"ИНТ", "НАТ", "ИНТ./НАТ"})
	for text, want := range map[string]bool{
		"ИНТ. КУХНЯ - ДЕНЬ": true, "ИНТ./НАТ. ДВОР": true, "INT. HOUSE": false, "ИНТЕРЕСНО": false,
	} {
		if ru.MatchString(text) != want {
			t.Errorf("%q: %v", text, !want)
		}
	}
	if scenePattern(nil) != scenePrefix {
		t.Error("no starts: English")
	}
}

// ScannedPages tells a scan from a printed script without reading either.
func TestScannedPages(t *testing.T) {
	scanned, pages, err := ScannedPages("testdata/scanned-tv-episode.pdf")
	if err != nil || pages == 0 || scanned != pages {
		t.Errorf("the scan: %d of %d scanned (%v)", scanned, pages, err)
	}
	src, _ := os.ReadFile("../examples/tv-episode.fountain")
	file := filepath.Join(t.TempDir(), "tv.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, fountain.Parse(scenes, strings.NewReader(string(src)))); err != nil {
		t.Fatal(err)
	}
	if scanned, pages, err := ScannedPages(file); err != nil || pages == 0 || scanned != 0 {
		t.Errorf("the printed one: %d of %d scanned (%v)", scanned, pages, err)
	}
}

// kinds is a screenplay as "type: contents" lines, without the blank ones.
func kinds(s lex.Screenplay) string {
	var out []string
	for _, l := range s {
		if l.Type != lex.TypeEmpty {
			out = append(out, string(l.Type)+": "+l.Contents)
		}
	}
	return strings.Join(out, "\n")
}

// A typescript's ways, read as a script: headings numbered with a word
// ("Scene 12 - int. kitchen -"), names at their speech's indent glued to
// their parentheticals, a stray mark of the scan between the lines.
func TestTypescriptLayout(t *testing.T) {
	const step = 1.0 / 6
	page := []line{
		ln(1, 1.0, at(1.5, "Scene 12 - int. kitchen, day -")),
		ln(1, 1.0+2*step, at(1.5, "Pia waits by the window, the kettle singing behind her, the")),
		ln(1, 1.0+3*step, at(1.5, "radio off.")),
		ln(1, 1.0+5*step, at(2.5, "THE GUARD(quietly)")),
		ln(1, 1.0+6*step, at(2.5, "Who goes there? Nobody comes here")),
		ln(1, 1.0+6.5*step, at(6.8, "•")),
		ln(1, 1.0+7*step, at(2.5, "at this hour.")),
		ln(1, 1.0+9*step, at(2.5, "VOICE OVER")),
		ln(1, 1.0+10*step, at(2.5, "\"Nobody ever did.\"")),
		ln(1, 1.0+12*step, at(1.5, "Sc. 13: the yard")),
		ln(1, 1.0+14*step, at(1.5, "Rain.")),
	}
	want := `scene: INT. KITCHEN, DAY #12#
action: Pia waits by the window, the kettle singing behind her, the radio off.
speaker: THE GUARD
paren: (quietly)
dialog: Who goes there? Nobody comes here at this hour.
speaker: VOICE OVER
dialog: "Nobody ever did."
scene: THE YARD #13#
action: Rain.`
	if got := kinds(classify([][]line{page}, 8.5, scenePrefix)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Names centred with a blank line before the speech, short speeches
// centred too (a translation's layout): cues and speeches all the same.
func TestCentredNamesWithBlankLine(t *testing.T) {
	const step = 1.0 / 6
	page := []line{
		ln(1, 1.0, at(1.5, "THEY WALK ON, TALKING ABOUT THE WEATHER AND THE STATE OF THE ROADS.")),
		ln(1, 1.0+2*step, at(3.9, "MIRA")),
		ln(1, 1.0+4*step, at(2.5, "Where do we go from here, now that")),
		ln(1, 1.0+5*step, at(2.5, "the road is closed?")),
		ln(1, 1.0+7*step, at(3.8, "TOMAS")),
		ln(1, 1.0+9*step, at(4.25, "Home.")), // centred between 1.5 and 7.5
		ln(1, 1.0+11*step, at(1.5, "3.")),
		ln(1, 1.0+12*step, at(1.5, "EXT – MARKET in OLD TOWN – DAY")),
		ln(1, 1.0+14*step, at(1.5, "CROWDS.")),
	}
	want := `action: THEY WALK ON, TALKING ABOUT THE WEATHER AND THE STATE OF THE ROADS.
speaker: MIRA
dialog: Where do we go from here, now that the road is closed?
speaker: TOMAS
dialog: Home.
scene: EXT – MARKET in OLD TOWN – DAY #3#
action: CROWDS.`
	if got := kinds(classify([][]line{page}, 8.5, scenePrefix)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A book's layout: a name above its speech at the same indent, the line
// break kept (not joined into the speech); words broken at line ends
// joined where the script has the whole word, compounds kept whole
// (both words in the script), left as printed elsewhere.
func TestHardBreaksAndHyphens(t *testing.T) {
	const step = 1.0 / 6
	page := []line{
		ln(1, 1.0, at(1.5, "The returned sol-")),
		ln(1, 1.0+step, at(1.5, "diers wait in line with their bas-")),
		ln(1, 1.0+2*step, at(1.5, "kets, as soldiers do, in the cold of the smoke-")),
		ln(1, 1.0+3*step, at(1.5, "filled morning. The smoke is filled with ash.")),
		ln(1, 1.0+5*step, at(1.5, "Doctor")),
		ln(1, 1.0+6*step, at(1.5, "That little girl needs bread and something warm to drink.")),
	}
	want := `action: The returned soldiers wait in line with their bas- kets, as soldiers do, in the cold of the smoke-filled morning. The smoke is filled with ash.
action: Doctor
action: That little girl needs bread and something warm to drink.`
	if got := kinds(classify([][]line{page}, 8.5, scenePrefix)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A centred name with its parenthetical under it, then a blank line and
// the speech.
func TestCentredNameParenthetical(t *testing.T) {
	const step = 1.0 / 6
	page := []line{
		ln(1, 1.0-2*step, at(1.5, "INT. HALL - DAY")),
		ln(1, 1.0, at(1.5, "THE DOOR OPENS SLOWLY AND THE WIND COMES IN FROM THE GARDEN.")),
		ln(1, 1.0+2*step, at(3.9, "MIRA")),
		ln(1, 1.0+3*step, at(1.5, "(SMILES)")), // at the margin, as one script sets them
		ln(1, 1.0+5*step, at(2.5, "Close it, please, it is cold.")),
	}
	want := `scene: INT. HALL - DAY
action: THE DOOR OPENS SLOWLY AND THE WIND COMES IN FROM THE GARDEN.
speaker: MIRA
paren: (SMILES)
dialog: Close it, please, it is cold.`
	if got := kinds(classify([][]line{page}, 8.5, scenePrefix)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A shot in capitals, its words going on after a dash on the next line,
// at the same indent: not a name and its speech.
func TestShotIsNoCue(t *testing.T) {
	const step = 1.0 / 6
	page := []line{
		ln(1, 1.0, at(1.5, "INT. HALL - DAY")),
		ln(1, 1.0+2*step, at(2.5, "A RED FLAG")),
		ln(1, 1.0+3*step, at(2.5, "—waving against a white sky.")),
	}
	for _, l := range classify([][]line{page}, 8.5, scenePrefix) {
		if l.Type == lex.TypeSpeaker {
			t.Errorf("a cue: %q", l.Contents)
		}
	}
}

// A title over several lines above an unusual credit, a "From a story
// by" line under the author.
func TestTitlePageMultiLine(t *testing.T) {
	page := []line{
		ln(1, 1.0, at(3.75, "MOON PIRATES")),
		ln(1, 1.2, at(3.8, "PART TWO")),
		ln(1, 1.4, at(3.55, "THE LOST HARBOUR")),
		ln(1, 2.0, at(3.25, "Written and Directed by")),
		ln(1, 2.6, at(3.65, "Anna Smith")),
		ln(1, 3.4, at(2.75, "From a story by Bo Berg")),
		ln(1, 9.2, at(5.0, "Third Draft, May 2026")),
	}
	s, ok := titlePage(page, 8.5, scenePrefix)
	if !ok {
		t.Fatal("no title page")
	}
	var got []string
	for _, l := range s {
		got = append(got, string(l.Type)+"="+l.Contents)
	}
	want := "titlepage=|Title=MOON PIRATES|Title=PART TWO|Title=THE LOST HARBOUR|Credit=Written and Directed by|" +
		"Author=Anna Smith|Source=From a story by Bo Berg|Draft date=Third Draft, May 2026|newpage="
	if strings.Join(got, "|") != want {
		t.Errorf("got  %s\nwant %s", strings.Join(got, "|"), want)
	}
}

// The script's end: FADE OUT: and THE END, numbered as scenes or not,
// are a transition and a centred line, never headings.
func TestTheEnd(t *testing.T) {
	const step = 0.1667
	page := []line{
		ln(1, 1.0, at(1.5, "Pia waves.")),
		ln(1, 1.0+2*step, at(6.3, "FADE OUT:")),
		ln(1, 1.0+4*step, at(1.0, "90"), at(1.5, "THE END"), at(7.5, "90")),
	}
	want := "action: Pia waves.\ntrans: FADE OUT:\ncenter: THE END"
	if got := kinds(classify([][]line{page}, 8.5, scenePrefix)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	page[2] = ln(1, 1.0+4*step, at(3.9, "THE END."))
	want = "action: Pia waves.\ntrans: FADE OUT:\ncenter: THE END"
	if got := kinds(classify([][]line{page}, 8.5, scenePrefix)); got != want {
		t.Errorf("unnumbered, got:\n%s", got)
	}
}

// MaxPages reads the first pages only, as many as it says.
func TestMaxPages(t *testing.T) {
	src, err := os.ReadFile("../examples/tv-episode.fountain")
	if err != nil {
		t.Fatal(err)
	}
	orig := fountain.Parse(scenes, strings.NewReader(string(src)))
	file := filepath.Join(t.TempDir(), "tv.pdf")
	if err := (&pdf.PDFWriter{OutputFile: file, Elements: rules.Default}).Write(nil, orig); err != nil {
		t.Fatal(err)
	}
	all, err := ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var read []int
	first, err := ReadFileWith(file, Options{MaxPages: 2, Progress: func(done, pages int) { read = append(read, done, pages) }})
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(read) != "[1 2 2 2]" {
		t.Errorf("progress %v, want two pages of two", read)
	}
	if len(first) == 0 || len(first) >= len(all) {
		t.Errorf("%d lines of the first two pages, %d of all", len(first), len(all))
	}
	if !strings.HasPrefix(fountainOf(t, all), fountainOf(t, first)[:200]) {
		t.Error("the first pages read otherwise than in the whole")
	}
}

// A script with its names at the margin, a colon after each, the speech
// a little in on the next line and the action further in (a stage play's
// layout, as some programs print a screenplay too): names and speeches
// read as such, the action as action, though its lines look centred.
func TestNamesAtTheMargin(t *testing.T) {
	const step = 1.0 / 6
	var page []line
	y := 1.0
	add := func(x float64, text string) {
		page = append(page, ln(1, y, run{X: x, End: x + 0.1*float64(len([]rune(text))), Text: text}))
		y += step
	}
	gap := func() { y += step }
	add(1.55, "INT. CIRCUS TENT - NIGHT")
	gap()
	action := "The ringmaster waits in the dark, a lamp in hand, and..."
	page = append(page, ln(1, y, run{X: 2.05, End: 7.0, Text: action}))
	y += step
	gap()
	for i, name := range []string{"EDWARD:", "SANDRA:", "EDWARD:", "AMOS:", "SANDRA:", "EDWARD:", "AMOS:", "SANDRA:"} {
		add(1.5, name)
		add(1.8, "A line of the speech, wrapped over")
		add(1.8, "two lines.")
		if i == 2 {
			gap()
			page = append(page, ln(1, y, run{X: 2.05, End: 7.0, Text: "He bows."}))
			y += step
		}
		gap()
	}
	s := classify([][]line{page}, 8.5, scenePrefix)
	// a few headings in capitals over paragraphs a little in are no names
	few := classify([][]line{page[:12]}, 8.5, scenePrefix)
	for _, l := range few {
		if l.Type == lex.TypeSpeaker {
			t.Errorf("a name in a few lines: %q", l.Contents)
		}
	}
	counts := map[lex.ElementType]int{}
	for _, l := range s {
		counts[l.Type]++
		if l.Type == lex.TypeSpeaker && strings.HasSuffix(l.Contents, ":") {
			t.Errorf("a name with its colon: %q", l.Contents)
		}
		if l.Type == lex.TypeDialog && l.Contents != "A line of the speech, wrapped over two lines." {
			t.Errorf("speech %q", l.Contents)
		}
	}
	if counts[lex.TypeSpeaker] != 8 || counts[lex.TypeDialog] != 8 || counts[lex.TypeAction] != 2 || counts[lex.TypeCenter] != 0 {
		t.Errorf("counts %v in\n%v", counts, s)
	}
}
