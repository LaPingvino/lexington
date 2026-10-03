package fdx

import (
	"strings"
	"testing"
)

const parseSample = `<?xml version="1.0" encoding="UTF-8" standalone="no" ?>
<FinalDraft DocumentType="Script" Template="No" Version="5">
  <Content>
    <Paragraph Type="Scene Heading"><Text>INT. BARN - DAY</Text></Paragraph>
    <Paragraph Type="Action"><Text>Rain </Text><Text
      Style="Italic">hammers</Text><Text> the </Text><Text Style="Bold+Underline">roof</Text><Text>.</Text></Paragraph>
    <Paragraph Type="Character"><Text>JOHN</Text></Paragraph>
    <Paragraph Type="Dialogue"><Text>It's </Text><Text Style="Bold+Italic">coming</Text><Text>.</Text></Paragraph>
    <Paragraph Type="Lyrics"><Text>La la la</Text></Paragraph>
    <Paragraph><DualDialogue>
      <Paragraph Type="Character"><Text>BOB</Text></Paragraph>
      <Paragraph Type="Dialogue"><Text>Run!</Text></Paragraph>
      <Paragraph Type="Character"><Text>ALICE</Text></Paragraph>
      <Paragraph Type="Parenthetical"><Text>(shouting)</Text></Paragraph>
      <Paragraph Type="Dialogue"><Text>Where?</Text></Paragraph>
    </DualDialogue></Paragraph>
    <Paragraph Type="Action" Alignment="Center"><Text>THE END</Text></Paragraph>
    <Paragraph Type="Shot" StartsNewPage="Yes"><Text>ANGLE ON THE DOOR</Text></Paragraph>
  </Content>
  <TitlePage><Content>
    <Paragraph Alignment="Center"><Text>The Barn</Text></Paragraph>
    <Paragraph Alignment="Center"><Text>Part One</Text></Paragraph>
    <Paragraph Alignment="Center"></Paragraph>
    <Paragraph Alignment="Center"><Text>Written by</Text></Paragraph>
    <Paragraph Alignment="Center"><Text>Jane Smith</Text></Paragraph>
    <Paragraph></Paragraph>
    <Paragraph><Text>1 Writer's Lane</Text></Paragraph>
  </Content></TitlePage>
</FinalDraft>`

func TestParseFinalDraftFeatures(t *testing.T) {
	sp, err := ParseWithError(strings.NewReader(parseSample))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range sp {
		got = append(got, string(l.Type)+"="+l.Contents)
	}
	want := []string{
		"titlepage=", "Title=The Barn", "Title=Part One", "Credit=Written by", "Author=Jane Smith",
		"metasection=", "Contact=1 Writer's Lane", "newpage=",
		"scene=INT. BARN - DAY",
		"empty=", "action=Rain *hammers* the _**roof**_.",
		"empty=", "speaker=JOHN", "dialog=It's ***coming***.", "lyrics=La la la",
		"empty=", "dualspeaker_open=", "speaker=BOB", "dialog=Run!",
		"empty=", "dualspeaker_next=", "speaker=ALICE", "paren=(shouting)", "dialog=Where?",
		"empty=", "dualspeaker_close=",
		"empty=", "center=THE END",
		"newpage=", "action=ANGLE ON THE DOOR",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestParseReportsInvalidFiles(t *testing.T) {
	if _, err := ParseWithError(strings.NewReader("<FinalDraft><Content>")); err == nil {
		t.Error("truncated file: no error")
	}
	if _, err := ParseWithError(strings.NewReader("Title: not xml")); err == nil {
		t.Error("Fountain text: no error")
	}
	if sp := Parse(strings.NewReader("nope")); len(sp) != 0 {
		t.Errorf("Parse of invalid input = %v, want empty", sp)
	}
}
