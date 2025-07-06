package lex

import (
	"bytes"
	"reflect"
	"testing"
)

// TestLexRoundTrip checks if writing a screenplay to the lex format and
// parsing it back results in the identical structure.
func TestLexRoundTrip(t *testing.T) {
	// 1. Define the original screenplay structure to be tested.
	originalScreenplay := Screenplay{
		Line{Type: "scene", Contents: "INT. HOUSE - DAY"},
		Line{Type: "action", Contents: "An example action."},
		Line{Type: "speaker", Contents: "MARY"},
		Line{Type: "dialog", Contents: "Hello, world."},
		Line{Type: "empty", Contents: ""},
		Line{Type: "paren", Contents: "(nervously)"},
		Line{Type: "trans", Contents: "CUT TO:"},
	}

	// 2. Write the screenplay to an in-memory buffer.
	var buffer bytes.Buffer
	writer := &LexWriter{}
	if err := writer.Write(&buffer, originalScreenplay); err != nil {
		t.Fatalf("Error writing screenplay: %v", err)
	}

	// Check if the writer produced any output.
	if buffer.Len() == 0 {
		t.Fatal("LexWriter.Write did not produce any output.")
	}

	// 3. Parse the buffer content back into a new screenplay structure.
	roundTripScreenplay := Parse(&buffer)

	// 4. Compare the original and round-tripped screenplays.
	if !reflect.DeepEqual(originalScreenplay, roundTripScreenplay) {
		t.Errorf("Round-tripped screenplay does not match the original.")
		// Provide detailed output for easier debugging.
		if len(originalScreenplay) != len(roundTripScreenplay) {
			t.Fatalf("Length mismatch: original %d, round-trip %d", len(originalScreenplay), len(roundTripScreenplay))
		}
		for i := 0; i < len(originalScreenplay); i++ {
			if !reflect.DeepEqual(originalScreenplay[i], roundTripScreenplay[i]) {
				t.Errorf("Line %d mismatch:\n  Original:  %+v\n  RoundTrip: %+v\n", i,
					originalScreenplay[i], roundTripScreenplay[i])
			}
		}
	}
}

// TestParse specifically checks the parsing of a simple lex format string.
func TestParse(t *testing.T) {
	lexData := `scene: INT. HOUSE - DAY
action: An example action.
speaker: MARY
dialog: Hello, world.
`
	reader := bytes.NewBufferString(lexData)
	screenplay := Parse(reader)

	expected := Screenplay{
		Line{Type: "scene", Contents: "INT. HOUSE - DAY"},
		Line{Type: "action", Contents: "An example action."},
		Line{Type: "speaker", Contents: "MARY"},
		Line{Type: "dialog", Contents: "Hello, world."},
	}

	if !reflect.DeepEqual(screenplay, expected) {
		t.Errorf("Parsed lex data does not match expected structure.")
		t.Logf("Got:\n%#v\n", screenplay)
		t.Logf("Expected:\n%#v\n", expected)
	}
}

// TestWrite checks if the writer produces the correct format.
func TestWrite(t *testing.T) {
	screenplay := Screenplay{
		Line{Type: "scene", Contents: "INT. HOUSE - DAY"},
		Line{Type: "speaker", Contents: "TOM"},
	}

	var buffer bytes.Buffer
	writer := &LexWriter{}
	if err := writer.Write(&buffer, screenplay); err != nil {
		t.Fatalf("Error writing screenplay: %v", err)
	}

	expected := `scene: INT. HOUSE - DAY
speaker: TOM
`

	if buffer.String() != expected {
		t.Errorf("Written lex data does not match expected format.")
		t.Logf("Got:\n%s\n", buffer.String())
		t.Logf("Expected:\n%s\n", expected)
	}
}
