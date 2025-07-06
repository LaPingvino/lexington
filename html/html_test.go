package html

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
)

// TestHTMLWrite checks if the HTML writer generates a valid HTML document
// containing the expected elements and content from a sample screenplay.
func TestHTMLWrite(t *testing.T) {
	// 1. Define a sample screenplay with various element types.
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeTitlePage},
		lex.Line{Type: "Title", Contents: "The Great Test"},
		lex.Line{Type: "Author", Contents: "A. Software Engineer"},
		lex.Line{Type: "metasection"},
		lex.Line{Type: lex.TypeScene, Contents: "INT. TEST SUITE - DAY"},
		lex.Line{Type: lex.TypeAction, Contents: "A simple action line."},
		lex.Line{Type: lex.TypeSpeaker, Contents: "TDD BOT"},
		lex.Line{Type: lex.TypeParen, Contents: "(smiling)"},
		lex.Line{Type: lex.TypeDialog, Contents: "Does this HTML look right?"},
		lex.Line{Type: lex.TypeTrans, Contents: "FADE TO BLACK."},
		lex.Line{Type: lex.TypeNewPage},
		lex.Line{Type: lex.TypeCenter, Contents: "Centered Text"},
	}

	// 2. Write the screenplay to an in-memory buffer.
	var buffer bytes.Buffer
	writer := &HTMLWriter{Elements: rules.Default}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("HTMLWriter.Write returned an unexpected error: %v", err)
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

	writer := &HTMLWriter{Elements: rules.Default}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("HTMLWriter.Write returned an unexpected error for an empty screenplay: %v", err)
	}

	htmlOutput := buffer.String()
	if !strings.Contains(htmlOutput, "<!DOCTYPE html>") {
		t.Error("Expected a basic HTML structure even for an empty screenplay, but didn't find a doctype.")
	}
}

// TestDualDialogueHTML tests that dual dialogue is rendered correctly with proper table structure
// and that dialogue margins are reset within dual dialogue blocks.
func TestDualDialogueHTML(t *testing.T) {
	// Create a screenplay with dual dialogue
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
		lex.Line{Type: lex.TypeDualOpen},
		lex.Line{Type: lex.TypeSpeaker, Contents: "ALICE"},
		lex.Line{Type: lex.TypeDialog, Contents: "I have something to tell you."},
		lex.Line{Type: lex.TypeDualNext},
		lex.Line{Type: lex.TypeSpeaker, Contents: "BOB"},
		lex.Line{Type: lex.TypeDialog, Contents: "I have something to tell you too."},
		lex.Line{Type: lex.TypeDualClose},
		lex.Line{Type: lex.TypeAction, Contents: "They both stop and look at each other."},
	}

	var buffer bytes.Buffer
	writer := &HTMLWriter{Elements: rules.Default}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("HTMLWriter.Write returned an unexpected error: %v", err)
	}

	htmlOutput := buffer.String()

	// Check that dual dialogue table structure is correct
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"Dual dialogue table", `<table class="dual-dialogue">`, true},
		{"Table row", `<tr>`, true},
		{"First table cell", `<td>`, true},
		{"Second table cell (after dualspeaker_next)", `</td><td>`, true},
		{"Table close", `</td></tr></table>`, true},
		{"Alice in first column",
			`<td><div class="speaker">ALICE</div><div class="dialogue">I have something to tell you.</div>`,
			true},
		{"Bob in second column",
			`<td><div class="speaker">BOB</div><div class="dialogue">I have something to tell you too.</div>`,
			true},
		{"No nested tables", `<table class="dual-dialogue"><tr><td><table`, false},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(htmlOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full HTML output:\n%s", htmlOutput)
				}
			}
		})
	}
}

// TestInlineMarkupProcessing tests that inline markup is correctly converted to HTML
func TestInlineMarkupProcessing(t *testing.T) {
	// Create a screenplay with various inline markup patterns
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeTitlePage},
		lex.Line{Type: "Title", Contents: "The **Bold** Title"},
		lex.Line{Type: "Author", Contents: "*Italic* Author"},
		lex.Line{Type: "metasection"},
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - ***BOLD ITALIC*** DAY"},
		lex.Line{Type: lex.TypeAction, Contents: "She walks to the _underlined_ door."},
		lex.Line{Type: lex.TypeSpeaker, Contents: "**BOLD** SPEAKER"},
		lex.Line{Type: lex.TypeDialog, Contents: "I have *italic* and **bold** and ***bold italic*** text."},
		lex.Line{Type: lex.TypeParen, Contents: "(*whispers* _quietly_)"},
		lex.Line{Type: lex.TypeTrans, Contents: "FADE TO ***BLACK***."},
		lex.Line{Type: lex.TypeCenter, Contents: "**THE** _END_"},
	}

	var buffer bytes.Buffer
	writer := &HTMLWriter{Elements: rules.Default}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("HTMLWriter.Write returned an unexpected error: %v", err)
	}

	htmlOutput := buffer.String()

	// Test cases for various inline markup patterns
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		// Basic bold markup
		{"Bold in title", "<h1>The <b>Bold</b> Title</h1>", true},
		{"Bold in speaker", `<div class="speaker"><b>BOLD</b> SPEAKER</div>`, true},

		// Basic italic markup
		{"Italic in author", "<p><i>Italic</i> Author</p>", true},
		{"Italic in dialog",
			`<div class="dialogue">I have <i>italic</i> and <b>bold</b> and <b><i>bold italic</i></b> text.</div>`,
			true},
		{"Italic in parenthetical", `<div class="parenthetical">(<i>whispers</i> <u>quietly</u>)</div>`, true},

		// Bold+italic markup
		{"Bold italic in scene", `<div class="scene-heading">INT. ROOM - <b><i>BOLD ITALIC</i></b> DAY</div>`, true},
		{"Bold italic in dialog",
			`<div class="dialogue">I have <i>italic</i> and <b>bold</b> and <b><i>bold italic</i></b> text.</div>`,
			true},
		{"Bold italic in transition", `<div class="transition">FADE TO <b><i>BLACK</i></b>.</div>`, true},

		// Underline markup
		{"Underline in action", `<div class="action">She walks to the <u>underlined</u> door.</div>`, true},
		{"Underline in parenthetical", `<div class="parenthetical">(<i>whispers</i> <u>quietly</u>)</div>`, true},
		{"Underline in center", `<div class="center"><b>THE</b> <u>END</u></div>`, true},

		// Make sure raw markup doesn't appear
		{"No raw bold markup", "**Bold**", false},
		{"No raw italic markup", "*italic*", false},
		{"No raw bold-italic markup", "***bold italic***", false},
		{"No raw underline markup", "_underlined_", false},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(htmlOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full HTML output:\n%s", htmlOutput)
				}
			}
		})
	}
}

// TestProcessInlineMarkupFunction tests the processInlineMarkup function directly
func TestProcessInlineMarkupFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"No markup", "Plain text", "Plain text"},
		{"Bold markup", "This is **bold** text", "This is <b>bold</b> text"},
		{"Italic markup", "This is *italic* text", "This is <i>italic</i> text"},
		{"Bold italic markup", "This is ***bold italic*** text", "This is <b><i>bold italic</i></b> text"},
		{"Underline markup", "This is _underlined_ text", "This is <u>underlined</u> text"},
		{"Mixed markup", "**Bold** and *italic* and _underlined_", "<b>Bold</b> and <i>italic</i> and <u>underlined</u>"},
		{"Multiple same markup", "**First** and **second** bold", "<b>First</b> and <b>second</b> bold"},
		{"Complex mixed",
			"***Bold italic*** with **bold** and *italic* and _underlined_",
			"<b><i>Bold italic</i></b> with <b>bold</b> and <i>italic</i> and <u>underlined</u>"},
		{"Empty markup", "**", "**"},
		{"Single asterisk", "*", "*"},
		{"Single underscore", "_", "_"},
		{"Nested asterisks", "****bold****", "<i><b><i>bold</i></b></i>"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := processInlineMarkup(test.input)
			if string(result) != test.expected {
				t.Errorf("processInlineMarkup(%q) = %q, want %q", test.input, string(result), test.expected)
			}
		})
	}
}

// TestInlineMarkupWithoutMarkupChars tests that text without markup characters is processed efficiently
func TestInlineMarkupWithoutMarkupChars(t *testing.T) {
	testText := "This is plain text without any markup characters"
	result := processInlineMarkup(testText)

	if string(result) != testText {
		t.Errorf("processInlineMarkup should return unchanged text when no markup characters present")
	}
}
