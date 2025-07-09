package markdown

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/lex"
)

// TestMarkdownWrite checks if the Markdown writer generates the expected output
// containing the correct elements and formatting from a sample screenplay.
func TestMarkdownWrite(t *testing.T) {
	// Define a sample screenplay with various element types
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeTitlePage},
		lex.Line{Type: "Title", Contents: "The Great Test"},
		lex.Line{Type: "Author", Contents: "A. Software Engineer"},
		lex.Line{Type: "metasection"},
		lex.Line{Type: lex.TypeScene, Contents: "INT. TEST SUITE - DAY"},
		lex.Line{Type: lex.TypeAction, Contents: "A simple action line."},
		lex.Line{Type: lex.TypeSpeaker, Contents: "TDD BOT"},
		lex.Line{Type: lex.TypeParen, Contents: "(smiling)"},
		lex.Line{Type: lex.TypeDialog, Contents: "Does this Markdown look right?"},
		lex.Line{Type: lex.TypeAction, Contents: "Another action line."},
		lex.Line{Type: "trans", Contents: "FADE TO BLACK."},
		lex.Line{Type: lex.TypeNewPage},
		lex.Line{Type: lex.TypeCenter, Contents: "Centered Text"},
	}

	// Write the screenplay to an in-memory buffer
	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	if markdownOutput == "" {
		t.Fatal("Markdown output is empty.")
	}

	// Perform checks to validate the Markdown output
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"Title Element", "# The Great Test", true},
		{"Author Element", "**A. Software Engineer**", true},
		{"Meta Separator", "---", true},
		{"Scene Heading", "## INT. TEST SUITE - DAY", true},
		{"Action Line", "A simple action line.", true},
		{"Speaker in Dialogue Block", "> **TDD BOT**", true},
		{"Parenthetical in Dialogue Block", "> *(smiling)*", true},
		{"Dialog in Dialogue Block", "> Does this Markdown look right?", true},
		{"Action After Dialogue", "Another action line.", true},
		{"Transition", "**FADE TO BLACK.**", true},
		{"Page Break", "\\newpage", true},
		{"Centered Text", "<center>Centered Text</center>", true},
		{"Bogus Content", "This should not be in the output", false},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(markdownOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full Markdown output:\n%s", markdownOutput)
				}
			}
		})
	}
}

// TestDialogueBlockFormatting tests that dialogue blocks are properly formatted
// with speakers, dialog, and parentheticals grouped together in blockquotes.
func TestDialogueBlockFormatting(t *testing.T) {
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
		lex.Line{Type: lex.TypeAction, Contents: "First action line."},
		lex.Line{Type: lex.TypeSpeaker, Contents: "ALICE"},
		lex.Line{Type: lex.TypeDialog, Contents: "Hello there."},
		lex.Line{Type: lex.TypeParen, Contents: "(whispering)"},
		lex.Line{Type: lex.TypeDialog, Contents: "Can you hear me?"},
		lex.Line{Type: lex.TypeAction, Contents: "Second action line."},
		lex.Line{Type: lex.TypeSpeaker, Contents: "BOB"},
		lex.Line{Type: lex.TypeDialog, Contents: "Yes, I can hear you."},
		lex.Line{Type: lex.TypeAction, Contents: "Third action line."},
	}

	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	// Check that dialogue blocks are properly formatted
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"Scene heading not in blockquote", "## INT. ROOM - DAY", true},
		{"First action not in blockquote", "First action line.", true},
		{"Speaker in blockquote", "> **ALICE**", true},
		{"Dialog in blockquote", "> Hello there.", true},
		{"Parenthetical in blockquote", "> *(whispering)*", true},
		{"Second dialog in blockquote", "> Can you hear me?", true},
		{"Second action not in blockquote", "Second action line.", true},
		{"Second speaker in blockquote", "> **BOB**", true},
		{"Second dialog in blockquote", "> Yes, I can hear you.", true},
		{"Third action not in blockquote", "Third action line.", true},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(markdownOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full Markdown output:\n%s", markdownOutput)
				}
			}
		})
	}

	// Check that dialogue blocks are properly separated from action
	lines := strings.Split(markdownOutput, "\n")
	var foundActionAfterDialogue bool
	var foundDialogueBlock bool

	for i, line := range lines {
		if strings.Contains(line, "> **ALICE**") {
			foundDialogueBlock = true
		}
		if foundDialogueBlock && strings.Contains(line, "Second action line.") {
			// Check that there's a blank line before the action
			if i > 0 && strings.TrimSpace(lines[i-1]) == "" {
				foundActionAfterDialogue = true
			}
		}
	}

	if !foundActionAfterDialogue {
		t.Error("Expected action line to be properly separated from dialogue block")
	}
}

// TestDualDialogueMarkdown tests that dual dialogue is rendered correctly
func TestDualDialogueMarkdown(t *testing.T) {
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
		lex.Line{Type: lex.TypeDualOpen},
		lex.Line{Type: lex.TypeSpeaker, Contents: "ALICE"},
		lex.Line{Type: lex.TypeDialog, Contents: "I have something to tell you."},
		lex.Line{Type: lex.TypeParen, Contents: "(nervously)"},
		lex.Line{Type: lex.TypeDialog, Contents: "It's important."},
		lex.Line{Type: lex.TypeDualNext},
		lex.Line{Type: lex.TypeSpeaker, Contents: "BOB"},
		lex.Line{Type: lex.TypeDialog, Contents: "I have something to tell you too."},
		lex.Line{Type: lex.TypeParen, Contents: "(excitedly)"},
		lex.Line{Type: lex.TypeDialog, Contents: "You go first."},
		lex.Line{Type: lex.TypeDualClose},
		lex.Line{Type: lex.TypeAction, Contents: "They both stop and look at each other."},
	}

	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	// Check that dual dialogue HTML structure is correct
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"Dual dialogue open", `<div style="display: flex; justify-content: space-between;">`, true},
		{"First column open", `<div style="width: 48%;">`, true},
		{"Dual dialogue next", `</div>
<div style="width: 48%;">`, true},
		{"Dual dialogue close", `</div>
</div>`, true},
		{"Alice in first column", `> **ALICE**`, true},
		{"Bob after dual next", `> **BOB**`, true},
		{"Alice dialog with line break", `I have something to tell you.  `, true},
		{"Bob dialog with line break", `I have something to tell you too.  `, true},
		{"Alice parenthetical", `*(nervously)*  `, true},
		{"Bob parenthetical", `*(excitedly)*  `, true},
		{"Action after dual dialogue", "They both stop and look at each other.", true},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(markdownOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full Markdown output:\n%s", markdownOutput)
				}
			}
		})
	}
}

// TestInlineMarkupProcessing tests that inline markup is correctly converted to Markdown
func TestInlineMarkupProcessing(t *testing.T) {
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
		lex.Line{Type: "trans", Contents: "FADE TO ***BLACK***."},
		lex.Line{Type: lex.TypeCenter, Contents: "**THE** _END_"},
	}

	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	// Test cases for various inline markup patterns
	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		// Basic bold markup
		{"Bold in title", "# The **Bold** Title", true},
		{"Bold in speaker", "> ****BOLD** SPEAKER**", true},

		// Basic italic markup
		{"Italic in author", "***Italic* Author**", true},
		{"Italic in dialog", "> I have *italic* and **bold** and ***bold italic*** text.", true},

		// Bold+italic markup
		{"Bold italic in scene", "## INT. ROOM - ***BOLD ITALIC*** DAY", true},
		{"Bold italic in transition", "**FADE TO ***BLACK***.**", true},

		// Underline markup (converted to <u> tags)
		{"Underline in action", "She walks to the <u>underlined</u> door.", true},
		{"Underline in parenthetical", "> *(*whispers* <u>quietly</u>)*", true},
		{"Underline in center", "<center>**THE** <u>END</u></center>", true},

		// Make sure fountain markup is properly converted
		{"Fountain bold to markdown", "**bold**", true},
		{"Fountain italic to markdown", "*italic*", true},
		{"Fountain bold-italic to markdown", "***bold italic***", true},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(markdownOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full Markdown output:\n%s", markdownOutput)
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
		{"Bold markup", "This is **bold** text", "This is **bold** text"},
		{"Italic markup", "This is *italic* text", "This is *italic* text"},
		{"Bold italic markup", "This is ***bold italic*** text", "This is ***bold italic*** text"},
		{"Underline markup", "This is _underlined_ text", "This is <u>underlined</u> text"},
		{"Mixed markup", "**Bold** and *italic* and _underlined_", "**Bold** and *italic* and <u>underlined</u>"},
		{"Multiple same markup", "**First** and **second** bold", "**First** and **second** bold"},
		{"Complex mixed",
			"***Bold italic*** with **bold** and *italic* and _underlined_",
			"***Bold italic*** with **bold** and *italic* and <u>underlined</u>"},
		{"Empty markup", "**", "**"},
		{"Single asterisk", "*", "*"},
		{"Single underscore", "_", "_"},
		{"No markup chars", "Plain text without markup", "Plain text without markup"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := processInlineMarkup(test.input)
			if result != test.expected {
				t.Errorf("processInlineMarkup(%q) = %q, want %q", test.input, result, test.expected)
			}
		})
	}
}

// TestEmptyScreenplay ensures that writing an empty screenplay doesn't cause a panic
func TestEmptyScreenplay(t *testing.T) {
	var screenplay lex.Screenplay // Empty screenplay
	var buffer bytes.Buffer

	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error for an empty screenplay: %v", err)
	}

	markdownOutput := buffer.String()
	if markdownOutput != "" {
		t.Error("Expected empty output for empty screenplay, got:", markdownOutput)
	}
}

// TestTitlePageElements tests that title page elements are properly formatted
func TestTitlePageElements(t *testing.T) {
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeTitlePage},
		lex.Line{Type: "Title", Contents: "Test Title"},
		lex.Line{Type: "Credit", Contents: "Written by"},
		lex.Line{Type: "Author", Contents: "Test Author"},
		lex.Line{Type: "metasection"},
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
	}

	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"Title formatting", "# Test Title", true},
		{"Credit formatting", "*Written by*", true},
		{"Author formatting", "**Test Author**", true},
		{"Meta separator", "---", true},
		{"Scene after title page", "## INT. ROOM - DAY", true},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(markdownOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full Markdown output:\n%s", markdownOutput)
				}
			}
		})
	}
}

// TestSpecialElements tests sections, synopsis, and other special elements
func TestSpecialElements(t *testing.T) {
	screenplay := lex.Screenplay{
		lex.Line{Type: "section", Contents: "# Act I"},
		lex.Line{Type: "section", Contents: "## Chapter 1"},
		lex.Line{Type: "synopse", Contents: "= This is a synopsis line"},
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
		lex.Line{Type: lex.TypeAction, Contents: "Action line."},
		lex.Line{Type: lex.TypeLyrics, Contents: "♪ Musical lyrics here ♪"},
	}

	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	checks := []struct {
		name     string
		substr   string
		expected bool
	}{
		{"Section level 1", "### Act I", true},
		{"Section level 2", "#### Chapter 1", true},
		{"Synopsis formatting", "> This is a synopsis line", true},
		{"Lyrics in dialogue block", "> ♪ Musical lyrics here ♪", true},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			actual := strings.Contains(markdownOutput, check.substr)
			if actual != check.expected {
				t.Errorf("strings.Contains(%q) = %v, want %v", check.substr, actual, check.expected)
				if !check.expected {
					t.Logf("Full Markdown output:\n%s", markdownOutput)
				}
			}
		})
	}
}

// TestInlineMarkupWithoutMarkupChars tests that text without markup characters is processed efficiently
func TestInlineMarkupWithoutMarkupChars(t *testing.T) {
	testText := "This is plain text without any markup characters"
	result := processInlineMarkup(testText)

	if result != testText {
		t.Errorf("processInlineMarkup should return unchanged text when no markup characters present")
	}
}

// TestEmptyLinesAndSpacing tests that empty lines and spacing are handled correctly
func TestEmptyLinesAndSpacing(t *testing.T) {
	screenplay := lex.Screenplay{
		lex.Line{Type: lex.TypeScene, Contents: "INT. ROOM - DAY"},
		lex.Line{Type: lex.TypeAction, Contents: "Action line."},
		lex.Line{Type: lex.TypeEmpty},
		lex.Line{Type: lex.TypeSpeaker, Contents: "ALICE"},
		lex.Line{Type: lex.TypeDialog, Contents: "Hello."},
		lex.Line{Type: lex.TypeEmpty},
		lex.Line{Type: lex.TypeAction, Contents: "Another action."},
	}

	var buffer bytes.Buffer
	writer := &MarkdownWriter{}
	err := writer.Write(&buffer, screenplay)
	if err != nil {
		t.Fatalf("MarkdownWriter.Write returned an unexpected error: %v", err)
	}

	markdownOutput := buffer.String()

	// Check that empty lines create proper separation
	if !strings.Contains(markdownOutput, "Action line.\n\n\n") {
		t.Error("Expected empty line to create proper spacing after action")
	}

	if !strings.Contains(markdownOutput, "Another action.") {
		t.Error("Expected action line after dialogue block")
	}
}
