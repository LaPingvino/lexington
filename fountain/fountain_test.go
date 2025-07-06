package fountain

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lapingvino/lexington/lex"
)

// TestFountainRoundTrip ensures that parsing a Fountain file, writing it back,
// and parsing it again yields the same internal representation.
func TestFountainRoundTrip(t *testing.T) {
	// 1. Read and parse the original example.fountain file.
	// The scenes slice is passed for scene heading detection.
	scenes := []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}
	originalFile, err := os.Open("../testdata/input/fountain_example.fountain")
	if err != nil {
		t.Fatalf("Failed to open example.fountain: %v", err)
	}
	defer func() {
		if err := originalFile.Close(); err != nil {
			t.Logf("Error closing original file: %v", err)
		}
	}()

	originalScreenplay := Parse(scenes, originalFile)
	if len(originalScreenplay) == 0 {
		t.Fatal("Parsing the original file resulted in an empty screenplay.")
	}

	// 2. Write the parsed screenplay to an in-memory buffer.
	var buffer bytes.Buffer
	writer := &FountainWriter{SceneConfig: scenes}
	err = writer.Write(&buffer, originalScreenplay)
	if err != nil {
		t.Fatalf("FountainWriter.Write returned an unexpected error: %v", err)
	}
	if buffer.Len() == 0 {
		t.Fatal("Writing the screenplay to the buffer resulted in no data.")
	}

	// 3. Parse the content that was just written to the buffer.
	roundTripScreenplay := Parse(scenes, &buffer)
	if len(roundTripScreenplay) == 0 {
		t.Fatal("Parsing the round-tripped file resulted in an empty screenplay.")
	}

	// 4. Compare the original screenplay struct with the round-tripped one.
	if !reflect.DeepEqual(originalScreenplay, roundTripScreenplay) {
		t.Errorf("Round-tripped screenplay does not match the original.")
		// Provide detailed output for easier debugging.
		if len(originalScreenplay) != len(roundTripScreenplay) {
			t.Fatalf("Length mismatch: original %d, round-trip %d", len(originalScreenplay), len(roundTripScreenplay))
		}
		for i := 0; i < len(originalScreenplay); i++ {
			if !reflect.DeepEqual(originalScreenplay[i], roundTripScreenplay[i]) {
				t.Errorf("Line %d mismatch:\n  Original:  %+v\n  RoundTrip: %+v\n", i, originalScreenplay[i], roundTripScreenplay[i])
			}
		}
	}
}

// TestParse checks the output of parsing example.fountain against a known-good structure.
func TestParse(t *testing.T) {
	scenes := []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}
	file, err := os.Open("../testdata/input/fountain_example.fountain")
	if err != nil {
		t.Fatalf("Failed to open example.fountain: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Logf("Error closing file: %v", err)
		}
	}()

	screenplay := Parse(scenes, file)

	// Note: The parser produces an extra `empty` line at the very end
	// because of its "read-ahead" logic to terminate dialogue blocks.
	// This is expected behavior.
	// The fountain_example.fountain file starts with a scene heading and has no title page,
	// so no titlepage or newpage markers should be generated.
	expected := lex.Screenplay{
		lex.Line{Type: "scene", Contents: "INT. HOUSE - DAY"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "speaker", Contents: "MARY"},
		lex.Line{Type: "dialog", Contents: "I can't believe how easy it is to write in Fountain."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "speaker", Contents: "TOM"},
		lex.Line{Type: "paren", Contents: "(typing)"},
		lex.Line{Type: "dialog", Contents: "Look! I just made a parenthetical!"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "action", Contents: "SOMETHING HAPPENS!"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "action", Contents: "(what? I don't know...)"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "scene", Contents: "EXT. GARDEN"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "speaker", Contents: "TOM"},
		lex.Line{Type: "dialog", Contents: "What am I doing here now?"},
		lex.Line{Type: "dialog", Contents: "To be honest, I have absolutely no idea!"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "action", Contents: "And that means really no idea!"},
		lex.Line{Type: "empty", Contents: ""},
	}

	if !reflect.DeepEqual(screenplay, expected) {
		t.Errorf("Parsed screenplay does not match expected structure.")
		t.Logf("Got %d lines, Expected %d lines.", len(screenplay), len(expected))
		t.Logf("Got:\n%#v\n", screenplay)
		t.Logf("Expected:\n%#v\n", expected)
		limit := len(screenplay)
		if len(expected) < limit {
			limit = len(expected)
		}
		for i := 0; i < limit; i++ {
			if !reflect.DeepEqual(screenplay[i], expected[i]) {
				t.Errorf("First mismatch at line %d:\n  Got:      %+v\n  Expected: %+v\n", i, screenplay[i], expected[i])
				break
			}
		}
	}
}

// TestParseDualDialogue checks if the parser correctly handles dual dialogue syntax.
func TestParseDualDialogue(t *testing.T) {
	scenes := []string{"INT", "EXT"}
	fountainContent := `title: Test Scene

INT. ROOM - DAY

MARY
I am speaking.

TOM ^
At the same time.`
	reader := strings.NewReader(fountainContent)
	screenplay := Parse(scenes, reader)

	expected := lex.Screenplay{
		lex.Line{Type: "titlepage", Contents: ""},
		lex.Line{Type: "Title", Contents: "Test Scene"},
		lex.Line{Type: "newpage", Contents: ""},
		lex.Line{Type: "scene", Contents: "INT. ROOM - DAY"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "dualspeaker_open", Contents: ""},
		lex.Line{Type: "speaker", Contents: "MARY"},
		lex.Line{Type: "dialog", Contents: "I am speaking."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "dualspeaker_next", Contents: ""},
		lex.Line{Type: "speaker", Contents: "TOM"},
		lex.Line{Type: "dialog", Contents: "At the same time."},
		lex.Line{Type: "dualspeaker_close", Contents: ""},
		lex.Line{Type: "empty", Contents: ""},
	}

	if !reflect.DeepEqual(screenplay, expected) {
		t.Errorf("Parsed dual dialogue does not match expected structure.")
		t.Logf("Got %d lines, Expected %d lines.", len(screenplay), len(expected))
		t.Logf("Got:\n%#v\n", screenplay)
		t.Logf("Expected:\n%#v\n", expected)
		if len(screenplay) != len(expected) {
			t.Fatalf("Length mismatch: got %d, expected %d", len(screenplay), len(expected))
		}
		for i := 0; i < len(screenplay); i++ {
			if !reflect.DeepEqual(screenplay[i], expected[i]) {
				t.Errorf("First mismatch at line %d:\n  Got:      %+v\n  Expected: %+v\n", i, screenplay[i], expected[i])
				break
			}
		}
	}
}
