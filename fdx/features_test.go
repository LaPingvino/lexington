package fdx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// body parses an FDX document's Content (with the settings given) and
// returns its lines, blank lines left out.
func body(t *testing.T, settings, content string) []string {
	t.Helper()
	doc := `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>
<FinalDraft DocumentType="Script" Template="No" Version="4"><Content>` + content + `</Content>` + settings + `</FinalDraft>`
	s, err := ParseWithError(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range s {
		if l.Type != lex.TypeEmpty {
			out = append(out, string(l.Type)+"="+l.Contents)
		}
	}
	return out
}

func check(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

const caps = `<ElementSettings Type="Scene Heading"><FontSpec Style="AllCaps"/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="Character"><FontSpec Style="AllCaps"/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="Transition"><FontSpec Style="AllCaps"/><ParagraphSpec Alignment="Right"/></ElementSettings>
<ElementSettings Type="Dialogue"><FontSpec Style=""/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="Action"><FontSpec Style=""/><ParagraphSpec Alignment="Left"/></ElementSettings>`

// Capitals as Final Draft shows them: of a type, and of a run; scene
// numbers from the heading's Number.
func TestAllCapsAndNumbers(t *testing.T) {
	check(t, body(t, caps, `
<Paragraph Number="12A" Type="Scene Heading"><Text>int.  will’s bedroom - night</Text></Paragraph>
<Paragraph Number="No" Type="Scene Heading"><Text>ext. road</Text></Paragraph>
<Paragraph Number="3" Type="Action"><Text>The </Text><Text Style="AllCaps">dog</Text><Text> barks.</Text></Paragraph>
<Paragraph Type="Character"><Text>edward (v.O.)</Text></Paragraph>
<Paragraph Type="Dialogue"><Text>Hi.</Text></Paragraph>
<Paragraph Type="Transition"><Text Style="AllCaps">flashcuts</Text><Text> to:</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Bold+Underline+AllCaps">the end</Text></Paragraph>`),
		"scene=INT.  WILL’S BEDROOM - NIGHT #12A#", "scene=EXT. ROAD", "action=The DOG barks.",
		"speaker=EDWARD (V.O.)", "dialog=Hi.", "trans=FLASHCUTS TO:", "action=_**THE END**_")
	// without ElementSettings, Final Draft's own
	check(t, body(t, "", `<Paragraph Type="Character"><Text>edward</Text></Paragraph><Paragraph Type="Dialogue"><Text>Hi.</Text></Paragraph>`),
		"speaker=EDWARD", "dialog=Hi.")
}

// Markers that are text are escaped where Fountain would read them as
// emphasis, and only there; markers open and close where the style
// changes; emphasis Fountain cannot write (part of a word underlined) is
// left out, not printed as markers.
func TestEscapedMarkers(t *testing.T) {
	check(t, body(t, caps, `
<Paragraph Type="Action"><Text Style="Underline">H</Text><Text>ello.</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Underline">a </Text><Text Style="Bold+Underline">b</Text><Text Style="Underline"> c</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Italic">She </Text><Text Style="Italic">sings</Text><Text Style="Italic"> softly.</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Underline">one</Text><Text>  </Text><Text Style="Underline">two</Text></Paragraph>
<Paragraph Type="Action"><Text Style="Bold">foo</Text><Text Style="Italic">bar</Text></Paragraph>`),
		"action=Hello.", "action=_a **b** c_", "action=*She sings softly.*", "action=_one  two_", "action=**foo***bar*")
	check(t, body(t, caps, `
<Paragraph Type="Action"><Text>Steel enters the code on the keypad: </Text><Text Style="Bold">*9765*</Text></Paragraph>
<Paragraph Type="Action"><Text>snake_case and 2 * 3 * 4</Text></Paragraph>
<Paragraph Type="Action"><Text>He dialed *69 and then 23*, and then hung up.</Text></Paragraph>`),
		`action=Steel enters the code on the keypad: **\*9765\***`,
		"action=snake_case and 2 * 3 * 4",
		`action=He dialed \*69 and then 23\*, and then hung up.`)
}

// Script notes in a paragraph (as Final Draft files of Versions 2 and 4
// have them; before the text, the note goes after it) and
// hidden text are Fountain notes; a blank line of a note is two spaces,
// an empty note is no note.
func TestScriptNotesAndHiddenText(t *testing.T) {
	note := func(paras ...string) string {
		var b strings.Builder
		b.WriteString(`<ScriptNote Color="#000000000000" Name="" Type="">`)
		for _, p := range paras {
			b.WriteString(`<Paragraph Alignment="Left"><Text Style="">` + p + `</Text></Paragraph>`)
		}
		b.WriteString(`</ScriptNote>`)
		return b.String()
	}
	check(t, body(t, caps, `
<Paragraph Type="Dialogue">`+note("Louder?")+`<Text>The bus is late again.</Text></Paragraph>
<Paragraph Type="Action"><Text>Rain </Text>`+note("Or snow?", "", "Snow.")+`<Text>falls.</Text></Paragraph>
<Paragraph Type="Action">`+note("")+`<Text>Done.</Text></Paragraph>
<Paragraph Type="Action"><Text>Visible </Text><Text Style="Italic+HiddenText">hidden</Text><Text> text.</Text></Paragraph>
<Paragraph Type="Character">`+note("Who is this?")+`</Paragraph>
<Paragraph Type="General">`+note("A dual dialogue note.")+`<DualDialogue>
  <Paragraph Type="Character"><Text>BOB</Text></Paragraph><Paragraph Type="Dialogue"><Text>Run!</Text></Paragraph>
  <Paragraph Type="Character"><Text>ALICE</Text></Paragraph><Paragraph Type="Dialogue"><Text>Where?</Text></Paragraph>
</DualDialogue><Text></Text></Paragraph>`),
		"dialog=The bus is late again. [[Louder?]]",
		"action=Rain[[Or snow?\n  \nSnow.]] falls.",
		"action=Done.",
		"action=Visible [[hidden]] text.",
		"note=[[Who is this?]]",
		"note=[[A dual dialogue note.]]",
		"dualspeaker_open=", "speaker=BOB", "dialog=Run!", "dualspeaker_next=", "speaker=ALICE", "dialog=Where?",
		"dualspeaker_close=")
	// printed, the notes are not there
	s, _ := ParseWithError(strings.NewReader(`<FinalDraft Version="4"><Content><Paragraph Type="Action"><Text>Rain </Text>` +
		note("Or snow?", "", "Snow.") + `<Text>falls.</Text></Paragraph></Content></FinalDraft>`))
	if p := lex.Printable(s, false); p[len(p)-1].Contents != "Rain falls." {
		t.Errorf("printed: %q", p)
	}
}

// Dual dialogue in a General paragraph with an empty text after it (Final
// Draft) and in one without a type (Fade In).
func TestDualDialogueWrappers(t *testing.T) {
	dual := `<DualDialogue><Paragraph Type="Character"><Text>BOB</Text></Paragraph><Paragraph Type="Dialogue"><Text>Run!</Text></Paragraph>` +
		`<Paragraph Type="Character"><Text>ALICE</Text></Paragraph><Paragraph Type="Dialogue"><Text>Where?</Text></Paragraph></DualDialogue>`
	want := []string{"action=Before.", "dualspeaker_open=", "speaker=BOB", "dialog=Run!", "dualspeaker_next=", "speaker=ALICE",
		"dialog=Where?", "dualspeaker_close=", "action=After."}
	for _, wrapper := range []string{`<Paragraph Type="General">` + dual + `<Text></Text></Paragraph>`, `<Paragraph>` + dual + `</Paragraph>`} {
		check(t, body(t, caps, `<Paragraph Type="Action"><Text>Before.</Text></Paragraph>`+wrapper+`<Paragraph Type="Action"><Text>After.</Text></Paragraph>`), want...)
	}
}

// Final Draft 13's types: hidden ones (its outline, summaries, notes) as
// Fountain's hidden elements; act breaks centred, a new act on a new
// page, with their type's bold and underlining; a scene's summary (Final
// Draft's SceneProperties) as its synopsis.
func TestFD13Types(t *testing.T) {
	settings := caps + `
<ElementSettings Type="Outline 1"><FontSpec Style="Bold+HiddenText"/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="Outline 2"><FontSpec Style="Bold+HiddenText"/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="Summary"><FontSpec Style="HiddenText"/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="Note"><FontSpec Style="Italic+HiddenText"/><ParagraphSpec Alignment="Left"/></ElementSettings>
<ElementSettings Type="New Act"><FontSpec Style="Bold+Underline+AllCaps"/><ParagraphSpec Alignment="Center" StartsNewPage="Yes"/></ElementSettings>
<ElementSettings Type="End of Act"><FontSpec Style="Bold+Underline+AllCaps"/><ParagraphSpec Alignment="Center"/></ElementSettings>
<ElementSettings Type="Sequence"><FontSpec Style="Bold+Underline+AllCaps"/><ParagraphSpec Alignment="Left"/></ElementSettings>`
	content := `
<Paragraph Type="Outline 1" id="a1"><Text>Act one</Text></Paragraph>
<Paragraph Type="Outline 2" id="a2"><Text>The kitchen</Text></Paragraph>
<Paragraph Type="Summary" id="a3"><Text>Anna cooks.</Text></Paragraph>
<Paragraph Type="Note" id="a4"><Text>Too long?</Text></Paragraph>
<Paragraph Type="Scene Heading" Number="1" id="a5"><SceneProperties Length="1/8" Page="1" Title=""><Summary><Paragraph Alignment="Left"><Text>The fire starts.</Text></Paragraph></Summary><SceneArcBeats/></SceneProperties><Text>int. kitchen - day</Text></Paragraph>
<Paragraph Type="Action" id="a6"><Text>Smoke.</Text></Paragraph>
<Paragraph Type="End of Act" id="a7"><Text>end of act one</Text></Paragraph>
<Paragraph Type="New Act" id="a8"><Text>act two</Text></Paragraph>
<Paragraph Type="Sequence" id="a9"><Text>the chase</Text></Paragraph>`
	check(t, body(t, settings, content),
		"section=# Act one", "section=## The kitchen", "synopse=Anna cooks.", "note=[[Too long?]]",
		"scene=INT. KITCHEN - DAY #1#", "synopse=The fire starts.", "action=Smoke.",
		"center=_**END OF ACT ONE**_", "newpage=", "center=_**ACT TWO**_", "action=_**THE CHASE**_")
	// nothing hidden is printed
	doc := `<FinalDraft Version="6"><Content>` + content + `</Content>` + settings + `</FinalDraft>`
	s, _ := ParseWithError(strings.NewReader(doc))
	var b bytes.Buffer
	if err := (&FDXWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"Act one", "The kitchen", "Anna cooks", "The fire starts"} {
		if strings.Contains(b.String(), hidden) {
			t.Errorf("%q written as script text:\n%s", hidden, b.String())
		}
	}
}

// Lexington's FDX: the scene number as Number, settings for its types
// (in capitals where Final Draft has them), notes as script notes, and
// emphasis as styles; it reads back as it was.
func TestWriteFDXFeatures(t *testing.T) {
	s := lex.Screenplay{
		{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY #12A# [[COLOR BLUE]]"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeAction, Contents: "Rain [[or snow]] falls."},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeNote, Contents: "[[A note\n  \nover lines]]"},
		{Type: lex.TypeEmpty},
		{Type: lex.TypeCenter, Contents: "_**THE END**_"},
	}
	var b bytes.Buffer
	if err := (&FDXWriter{}).Write(&b, s); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`<Paragraph Type="Scene Heading" Number="12A">`,
		`<ElementSettings Type="Character">`,
		`Style="AllCaps"`,
		`<Text Style="Bold+Underline">THE END</Text>`,
		`<ScriptNote Color="#000000000000" Name="" Type="">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lacks %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[[") || strings.Contains(out, "#12A#") {
		t.Errorf("Fountain markup written:\n%s", out)
	}
	back, err := ParseWithError(&b)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range back {
		if l.Type != lex.TypeEmpty {
			got = append(got, string(l.Type)+"="+l.Contents)
		}
	}
	// a heading as it was; the other notes at the end of their
	// paragraph, the line of notes on the next one
	check(t, got, "scene=INT. HOUSE - DAY #12A# [[COLOR BLUE]]", "action=Rain falls. [[or snow]]",
		"center=_**THE END**_ [[A note\n  \nover lines]]")
	if c := lex.NoteColor(got[0]); c != "blue" {
		t.Errorf("color %q", c)
	}
}
