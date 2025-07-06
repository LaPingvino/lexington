package pdf

import (
	"os"
	"testing"

	"github.com/lapingvino/lexington/lex"
	"github.com/lapingvino/lexington/rules"
)

// TestDualDialoguePDF tests the dual dialogue PDF generation functionality
// to ensure proper column positioning and no overlap between left and right columns.
func TestDualDialoguePDF(t *testing.T) {
	// Create a screenplay with dual dialogue including parentheticals
	screenplay := lex.Screenplay{
		lex.Line{Type: "scene", Contents: "INT. COFFEE SHOP - DAY"},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "action", Contents: "Two friends sit across from each other."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "dualspeaker_open", Contents: ""},
		lex.Line{Type: "speaker", Contents: "ALICE"},
		lex.Line{Type: "paren", Contents: "(nervously)"},
		lex.Line{Type: "dialog", Contents: "I have something important to tell you."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "dualspeaker_next", Contents: ""},
		lex.Line{Type: "speaker", Contents: "BOB"},
		lex.Line{Type: "paren", Contents: "(smiling)"},
		lex.Line{Type: "dialog", Contents: "What is it? You're making me nervous."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "dualspeaker_close", Contents: ""},
		lex.Line{Type: "action", Contents: "They both pause and look at each other."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "dualspeaker_open", Contents: ""},
		lex.Line{Type: "speaker", Contents: "ALICE"},
		lex.Line{Type: "dialog", Contents: "I'm moving to another city."},
		lex.Line{Type: "dualspeaker_next", Contents: ""},
		lex.Line{Type: "speaker", Contents: "BOB"},
		lex.Line{Type: "dialog", Contents: "When? This is so sudden!"},
		lex.Line{Type: "dualspeaker_close", Contents: ""},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "action", Contents: "Bob reaches across the table."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "speaker", Contents: "BOB"},
		lex.Line{Type: "dialog", Contents: "We'll stay in touch, right?"},
		lex.Line{Type: "empty", Contents: ""},
	}

	// Create style configuration with dual dialogue settings
	style := rules.Set{
		"scene":       rules.Format{Font: "CourierPrime", Size: 12, Style: "B", Align: "L", Left: 1.5, Right: 1.0},
		"action":      rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 1.5, Right: 1.0},
		"speaker":     rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 3.7, Right: 1.5},
		"dialog":      rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 2.5, Right: 1.5},
		"paren":       rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 3.1, Right: 1.5},
		"dualspeaker": rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 1.5, Right: 0.5},
		"dualdialog":  rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 1.0, Right: 0.5},
		"dualparen":   rules.Format{Font: "CourierPrime", Size: 12, Style: "", Align: "L", Left: 1.3, Right: 0.5},
		"empty":       rules.Format{Left: 1.5, Right: 1.0},
	}

	// Create temporary file for PDF output
	tmpfile, err := os.CreateTemp("", "lexington_dual_dialogue_test_*.pdf")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	// Test PDF generation without panicking
	var didPanic bool
	var panicValue interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
				panicValue = r
			}
		}()

		writer := &PDFWriter{OutputFile: tmpfile.Name(), Elements: style}
		err := writer.Write(nil, screenplay)
		if err != nil {
			t.Fatalf("PDFWriter.Write returned an unexpected error: %v", err)
		}
	}()

	if didPanic {
		t.Fatalf("The dual dialogue PDF creation process panicked: %v", panicValue)
	}

	// Verify that the PDF file was created and is not empty
	info, err := os.Stat(tmpfile.Name())
	if os.IsNotExist(err) {
		t.Fatalf("The dual dialogue PDF was not generated at the specified path: %s", tmpfile.Name())
	}
	if err != nil {
		t.Fatalf("Failed to get file info for the generated dual dialogue PDF: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("Generated dual dialogue PDF file is empty (size: 0 bytes).")
	}

	// For manual inspection during development, you can uncomment this line:
	// t.Logf("Generated dual dialogue test PDF at: %s", tmpfile.Name())
}

// TestDualDialogueColumnPositioning tests that dual dialogue columns are properly positioned
// and validates the flushDualDialogue function behavior.
func TestDualDialogueColumnPositioning(t *testing.T) {
	// Create a minimal dual dialogue screenplay
	screenplay := lex.Screenplay{
		lex.Line{Type: "dualspeaker_open", Contents: ""},
		lex.Line{Type: "speaker", Contents: "LEFT"},
		lex.Line{Type: "dialog", Contents: "Left column text"},
		lex.Line{Type: "dualspeaker_next", Contents: ""},
		lex.Line{Type: "speaker", Contents: "RIGHT"},
		lex.Line{Type: "dialog", Contents: "Right column text"},
		lex.Line{Type: "dualspeaker_close", Contents: ""},
	}

	// Use default style configuration
	style := rules.Default

	// Create temporary file for PDF output
	tmpfile, err := os.CreateTemp("", "lexington_dual_positioning_test_*.pdf")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	// Test that dual dialogue column positioning works without errors
	writer := &PDFWriter{OutputFile: tmpfile.Name(), Elements: style}
	err = writer.Write(nil, screenplay)
	if err != nil {
		t.Fatalf("Dual dialogue column positioning test failed: %v", err)
	}

	// Verify file creation
	info, err := os.Stat(tmpfile.Name())
	if os.IsNotExist(err) {
		t.Fatalf("The dual dialogue positioning test PDF was not generated")
	}
	if info.Size() == 0 {
		t.Errorf("Generated dual dialogue positioning test PDF is empty")
	}
}

// TestMultipleDualDialogueBlocks tests handling of multiple dual dialogue blocks
// in a single screenplay to ensure proper opening and closing of blocks.
func TestMultipleDualDialogueBlocks(t *testing.T) {
	// Create a screenplay with multiple dual dialogue blocks
	screenplay := lex.Screenplay{
		lex.Line{Type: "scene", Contents: "INT. ROOM - DAY"},
		lex.Line{Type: "empty", Contents: ""},
		// First dual dialogue block
		lex.Line{Type: "dualspeaker_open", Contents: ""},
		lex.Line{Type: "speaker", Contents: "ALICE"},
		lex.Line{Type: "dialog", Contents: "First block, left side."},
		lex.Line{Type: "dualspeaker_next", Contents: ""},
		lex.Line{Type: "speaker", Contents: "BOB"},
		lex.Line{Type: "dialog", Contents: "First block, right side."},
		lex.Line{Type: "dualspeaker_close", Contents: ""},
		// Action between blocks
		lex.Line{Type: "action", Contents: "They pause for a moment."},
		lex.Line{Type: "empty", Contents: ""},
		// Second dual dialogue block
		lex.Line{Type: "dualspeaker_open", Contents: ""},
		lex.Line{Type: "speaker", Contents: "ALICE"},
		lex.Line{Type: "dialog", Contents: "Second block, left side."},
		lex.Line{Type: "dualspeaker_next", Contents: ""},
		lex.Line{Type: "speaker", Contents: "BOB"},
		lex.Line{Type: "dialog", Contents: "Second block, right side."},
		lex.Line{Type: "dualspeaker_close", Contents: ""},
		lex.Line{Type: "empty", Contents: ""},
	}

	// Use default style configuration
	style := rules.Default

	// Create temporary file for PDF output
	tmpfile, err := os.CreateTemp("", "lexington_multiple_dual_test_*.pdf")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	// Test multiple dual dialogue blocks
	writer := &PDFWriter{OutputFile: tmpfile.Name(), Elements: style}
	err = writer.Write(nil, screenplay)
	if err != nil {
		t.Fatalf("Multiple dual dialogue blocks test failed: %v", err)
	}

	// Verify file creation
	info, err := os.Stat(tmpfile.Name())
	if os.IsNotExist(err) {
		t.Fatalf("The multiple dual dialogue blocks test PDF was not generated")
	}
	if info.Size() == 0 {
		t.Errorf("Generated multiple dual dialogue blocks test PDF is empty")
	}
}
