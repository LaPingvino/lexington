package pdf

import (
	"os"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// TestCreatePDF is a smoke test for the PDF creation process.
// It generates a PDF from a sample screenplay and checks if the process
// completes without error and creates a non-empty file.
// This test doesn't validate the visual content of the PDF.
func TestCreatePDF(t *testing.T) {
	// 1. Create a sample screenplay structure.
	screenplay := lex.Screenplay{
		lex.Line{Type: "scene", Contents: "INT. A TEST ENVIRONMENT - DAY"},
		lex.Line{Type: "action", Contents: "A software engineer writes a test for PDF generation."},
		lex.Line{Type: "empty", Contents: ""},
		lex.Line{Type: "speaker", Contents: "ENGINEER"},
		lex.Line{Type: "paren", Contents: "(confidently)"},
		lex.Line{Type: "dialog", Contents: "This should produce a valid, non-empty PDF file."},
		lex.Line{Type: "trans", Contents: "FADE OUT."},
	}

	// 2. Define a minimal style configuration for the test.
	// This mimics the structure that would be loaded from a configuration file.
	style := rules.Set{
		"scene":     rules.Format{Font: "Courier", Size: 12, Style: "B", Align: "L", Left: 1.5, Right: 1.0},
		"action":    rules.Format{Font: "Courier", Size: 12, Style: "", Align: "L", Left: 1.5, Right: 1.0},
		"speaker":   rules.Format{Font: "Courier", Size: 12, Style: "", Align: "L", Left: 3.7, Right: 1.5},
		"dialog":    rules.Format{Font: "Courier", Size: 12, Style: "", Align: "L", Left: 2.5, Right: 1.5},
		"paren":     rules.Format{Font: "Courier", Size: 12, Style: "I", Align: "L", Left: 3.1, Right: 1.5},
		"trans":     rules.Format{Font: "Courier", Size: 12, Style: "B", Align: "R", Left: 1.5, Right: 1.0},
		"pagebreak": rules.Format{},
		"empty":     rules.Format{Left: 1.5, Right: 1.0},
	}

	// 3. Create a temporary file to write the PDF to.
	tmpfile, err := os.CreateTemp("", "lexington_test_*.pdf")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	// The Create function will open the file, so we close it here.
	if closeErr := tmpfile.Close(); closeErr != nil {
		t.Fatalf("Failed to close temporary file: %v", closeErr)
	}
	// 4. Defer cleanup of the temporary file.
	defer func() {
		if removeErr := os.Remove(tmpfile.Name()); removeErr != nil {
			t.Logf("Failed to remove temporary file: %v", removeErr)
		}
	}()

	// 5. Run the PDF creation function, recovering from any potential panics.
	// The current implementation of Create() logs errors instead of returning them,
	// so a panic is a possible failure mode we need to catch.
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
		writeErr := writer.Write(nil, screenplay)
		if writeErr != nil {
			t.Fatalf("PDFWriter.Write returned an unexpected error: %v", writeErr)
		}
	}()

	if didPanic {
		t.Fatalf("The PDF creation process panicked: %v", panicValue)
	}

	// 6. Check if the generated file exists and has a size greater than zero.
	info, err := os.Stat(tmpfile.Name())
	if os.IsNotExist(err) {
		t.Fatalf("The Create function did not generate a file at the specified path: %s", tmpfile.Name())
	}
	if err != nil {
		t.Fatalf("Failed to get file info for the generated PDF: %v", err)
	}

	if info.Size() == 0 {
		t.Errorf("Generated PDF file is empty (size: 0 bytes).")
	}

	// For manual inspection, you could print the temp file name.
	// t.Logf("Generated test PDF at: %s", tmpfile.Name())
}
