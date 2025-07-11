package fdx

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// TestFdxRoundTrip tests if parsing an FDX file, writing it back out,
// and parsing it again results in the same internal structure. This ensures
// that the write and parse functions are compatible and no data is lost.
func TestFdxRoundTrip(t *testing.T) {
	// 1. Read and parse the original example.fdx file.
	originalFile, err := os.Open("example.fdx")
	if err != nil {
		t.Fatalf("Failed to open example.fdx: %v", err)
	}
	defer func() {
		if closeErr := originalFile.Close(); closeErr != nil {
			t.Logf("Error closing original file: %v", closeErr)
		}
	}()

	originalScreenplay := Parse(originalFile)
	if len(originalScreenplay) == 0 {
		t.Fatal("Parsing the original file resulted in an empty screenplay.")
	}

	// 2. Write the parsed screenplay to an in-memory buffer.
	var buffer bytes.Buffer
	writer := &FDXWriter{}
	err = writer.Write(&buffer, originalScreenplay)
	if err != nil {
		t.Fatalf("FDXWriter.Write returned an unexpected error: %v", err)
	}
	if buffer.Len() == 0 {
		t.Fatal("Writing the screenplay to the buffer resulted in no data.")
	}

	// 3. Parse the content that was just written to the buffer.
	roundTripScreenplay := Parse(&buffer)
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
				t.Errorf("Line %d mismatch:\n  Original: %+v\n  RoundTrip: %+v\n", i, originalScreenplay[i], roundTripScreenplay[i])
			}
		}
	}
}

// TestParse specifically checks the output of parsing example.fdx against a known-good structure.
func TestParse(t *testing.T) {
	file, err := os.Open("example.fdx")
	if err != nil {
		t.Fatalf("Failed to open example.fdx: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Logf("Error closing file: %v", closeErr)
		}
	}()

	screenplay := Parse(file)

	expected := lex.Screenplay{
		lex.Line{Type: lex.TypeScene, Contents: "INT. HOUSE - DAY"},
		lex.Line{Type: lex.TypeSpeaker, Contents: "MARY"},
		lex.Line{Type: lex.TypeDialog, Contents: "I can't believe how easy it is to write in Fountain."},
		lex.Line{Type: lex.TypeSpeaker, Contents: "TOM"},
		lex.Line{Type: lex.TypeParen, Contents: "(typing)"},
		lex.Line{Type: lex.TypeDialog, Contents: "Look! I just made a parenthetical!"},
		lex.Line{Type: lex.TypeAction, Contents: "SOMETHING HAPPENS!"},
		lex.Line{Type: lex.TypeAction, Contents: "(what? I don't know...)"},
		lex.Line{Type: lex.TypeScene, Contents: "EXT. GARDEN"},
		lex.Line{Type: lex.TypeSpeaker, Contents: "TOM"},
		lex.Line{Type: lex.TypeDialog, Contents: "What am I doing here now?"},
		lex.Line{Type: lex.TypeDialog, Contents: "To be honest, I have absolutely no idea!"},
		lex.Line{Type: lex.TypeEmpty, Contents: ""},
		lex.Line{Type: lex.TypeAction, Contents: "And that means really no idea!"},
	}

	if !reflect.DeepEqual(screenplay, expected) {
		t.Errorf("Parsed screenplay does not match expected structure.")
		// To make debugging easier, print out the differences.
		t.Logf("Got:\n%#v\n", screenplay)
		t.Logf("Expected:\n%#v\n", expected)
		if len(screenplay) != len(expected) {
			t.Fatalf("Length mismatch: got %d, expected %d", len(screenplay), len(expected))
		}
		for i := 0; i < len(screenplay); i++ {
			if !reflect.DeepEqual(screenplay[i], expected[i]) {
				t.Errorf("Line %d mismatch:\n  Got: %+v\n  Expected: %+v\n", i, screenplay[i], expected[i])
			}
		}
	}
}
