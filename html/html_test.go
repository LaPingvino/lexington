package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lapingvino/lexington/lex"
)

// TestHTMLWrite checks if the HTML writer generates a valid HTML document
// containing the expected elements and content from a sample screenplay.
func TestHTMLWrite(t *testing.T) {
	// 1. Define a sample screenplay with various element types.
	screenplay := lex.Screenplay{
		lex.Line{Type: "titlepage"},
		lex.Line{Type: "Title", Contents: "The Great Test"},
		lex.Line{Type: "Author", Contents: "A. Software Engineer"},
		lex.Line{Type: "metasection"},
		lex.Line{Type: "scene", Contents: "INT. TEST SUITE - DAY"},
		lex.Line{Type: "action", Contents: "A simple action line."},
		lex.Line{Type: "speaker", Contents: "TDD BOT"},
		lex.Line{Type: "paren", Contents: "(smiling)"},
		lex.Line{Type: "dialog", Contents: "Does this HTML look right?"},
		lex.Line{Type: "trans", Contents: "FADE TO BLACK."},
		lex.Line{Type: "newpage"},
		lex.Line{Type: "center", Contents: "Centered Text"},
	}

	// 2. Write the screenplay to an in-memory buffer.
	var buffer bytes.Buffer
	err := Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("Write function returned an unexpected error: %v", err)
	}

	htmlOutput := buffer.String()

	if htmlOutput == "" {
		t.Fatal("HTML output is empty.")
	}

	// 3. Perform a series of checks to validate the HTML output.
	// This is not an exhaustive check, but it covers the key structural elements.
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"HTML Doctype", "<!DOCTYPE html>", true},
		{"Title Element", "<h1>The Great Test</h1>", true},
		{"Author Element", "<p>A. Software Engineer</p>", true},
		{"Scene Heading", `<div class="scene-heading">INT. TEST SUITE - DAY</div>`, true},
		{"Action", `<div class="action">A simple action line.</div>`, true},
		{"Speaker", `<div class="speaker">TDD BOT</div>`, true},
		{"Parenthetical", `<div class="parenthetical">(smiling)</div>`, true},
		{"Dialogue", `<div class="dialogue">Does this HTML look right?</div>`, true},
		{"Transition", `<div class="transition">FADE TO BLACK.</div>`, true},
		{"Page Break", `<div class="newpage"></div>`, true},
		{"Centered", `<div class="center">Centered Text</div>`, true},
		{"Bogus Content", "This should not be in the output", false},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(htmlOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
			}
		})
	}
}

// TestEmptyScreenplay ensures that writing an empty screenplay doesn't cause a panic.
func TestEmptyScreenplay(t *testing.T) {
	var screenplay lex.Screenplay // Empty screenplay
	var buffer bytes.Buffer

	err := Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("Write function returned an unexpected error for an empty screenplay: %v", err)
	}

	htmlOutput := buffer.String()
	if !strings.Contains(htmlOutput, "<!DOCTYPE html>") {
		t.Error("Expected a basic HTML structure even for an empty screenplay, but didn't find a doctype.")
	}
}
