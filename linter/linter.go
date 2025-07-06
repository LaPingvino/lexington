package linter

import (
	"fmt"
	"strings"

	"github.com/lapingvino/lexington/lex"
)

// LintError represents a single linting issue found in the screenplay.
type LintError struct {
	LineNum int    // The 1-based line number where the error occurred
	Message string // A descriptive message about the error
	Context string // The line content or relevant context
}

// Linter provides methods to lint a lex.Screenplay.
type Linter struct {
	Errors []LintError
}

// NewLinter creates and returns a new Linter instance.
func NewLinter() *Linter {
	return &Linter{}
}

// Lint performs linting checks on the given screenplay and stores any found errors.
// This function maps lex.Screenplay back to conceptual lines for easier error reporting.
func (l *Linter) Lint(screenplay lex.Screenplay) {
	// A simple line-tracking mechanism, assuming each lex.Line generally corresponds
	// to a source line, or that we can infer line numbers.
	// This is a simplification; a more robust linter might need access to the
	// original Fountain file content line by line.
	currentLineNum := 1 // Start at line 1

	inDualDialogueBlock := false

	for i, line := range screenplay {
		// Increment line number for most types. Adjust as needed if multiple lex.Lines
		// can originate from a single source line (e.g., continuations), or if
		// some lex.Lines are internal markers.
		if line.Type != "titlepage" && line.Type != "metasection" { // These are structural and don't directly correspond to input lines
			// Heuristic: increment line number unless it's a structural element
			// that doesn't consume a source line (e.g., internal markers).
			// This might need refinement based on how the lex parser maps.
			currentLineNum++
		}
		// Special handling for the very first line after titlepage elements, if any.
		// The `titlepage` and `metasection` tokens themselves don't correspond to a content line.
		// The actual title/author lines *do*, but are handled by the titlepage logic.
		// For now, let's assume `currentLineNum` increments based on actual content lines
		// from the original source.
		// For accurate line numbers, the parser should probably pass original line numbers to lex.Line.

		// Check for dual dialogue issues
		if line.Type == "dualspeaker_open" {
			if inDualDialogueBlock {
				l.addError(currentLineNum, "Nested dual dialogue block detected. Fountain specification allows only one dual dialogue block at a time.", line.Contents)
			}
			inDualDialogueBlock = true
		} else if line.Type == "dualspeaker_close" {
			inDualDialogueBlock = false
		} else if strings.HasSuffix(strings.TrimSpace(line.Contents), "^") && line.Type == "speaker" {
			// This is the problematic case for "ALICE ^" if it's treated as a new speaker
			// outside of an existing dual dialogue setup.
			if inDualDialogueBlock {
				// This implies a third speaker in a dual dialogue block, which is also an error.
				l.addError(currentLineNum, "More than two speakers in a dual dialogue block. Fountain specifies only two.", line.Contents)
			} else {
				// This indicates a dual dialogue speaker marker outside a dual block, implying structural issue.
				// This is actually handled by the parser, but we can lint for it to be explicit.
				// For now, the parser already tries to create a dual block.
				// This lint check is primarily for *additional* `^` speakers within an already established dual block.
			}
		}

		// Example: Check for empty speaker names
		if line.Type == "speaker" && strings.TrimSpace(line.Contents) == "" {
			l.addError(currentLineNum, "Empty speaker name detected.", line.Contents)
		}

		// Example: Check for parentheticals without preceding dialogue/speaker
		if line.Type == "paren" && i > 0 && !(screenplay[i-1].Type == "speaker" || screenplay[i-1].Type == "dialog" || screenplay[i-1].Type == "paren") {
			l.addError(currentLineNum, "Parenthetical without a preceding speaker or dialogue line. This might be interpreted as action.", line.Contents)
		}

		// Future checks can be added here:
		// - Unrecognized element types (if the parser outputs a generic "unknown" type)
		// - Formatting consistency (e.g., scene headings always start with INT./EXT.)
		// - Excessive empty lines
	}

	// Final check: if a dual dialogue block was opened but never closed.
	// This might be tricky as `dualspeaker_close` is an injected token.
	// The primary check is `More than two speakers` or `Nested`.
}

// addError appends a new LintError to the linter's error list.
func (l *Linter) addError(lineNum int, message, context string) {
	l.Errors = append(l.Errors, LintError{
		LineNum: lineNum,
		Message: message,
		Context: context,
	})
}

// HasErrors returns true if any linting errors were found.
func (l *Linter) HasErrors() bool {
	return len(l.Errors) > 0
}

// FormatErrors returns a string representation of all collected errors.
func (l *Linter) FormatErrors() string {
	if !l.HasErrors() {
		return "No linting errors found."
	}
	var sb strings.Builder
	sb.WriteString("Linting Errors:\n")
	for _, err := range l.Errors {
		sb.WriteString(fmt.Sprintf("  Line %d: %s\n    Context: \"%s\"\n", err.LineNum, err.Message, err.Context))
	}
	return sb.String()
}
