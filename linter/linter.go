package linter

import (
	"fmt"
	"strings"

	"github.com/lapingvino/lexington/internal"
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
	currentLineNum := 1 // Start at line 1
	inDualDialogueBlock := false

	for i, line := range screenplay {
		currentLineNum = l.updateLineNumber(line, currentLineNum)
		inDualDialogueBlock = l.checkDualDialogueIssues(line, currentLineNum, inDualDialogueBlock)
		l.checkBasicIssues(line, screenplay, i, currentLineNum)
	}
}

// updateLineNumber increments line number for content lines
func (l *Linter) updateLineNumber(line lex.Line, currentLineNum int) int {
	// These are structural and don't directly correspond to input lines
	if line.Type != lex.TypeTitlePage && line.Type != "metasection" {
		return currentLineNum + 1
	}
	return currentLineNum
}

// checkDualDialogueIssues handles dual dialogue validation
func (l *Linter) checkDualDialogueIssues(line lex.Line, currentLineNum int, inDualDialogueBlock bool) bool {
	switch line.Type {
	case lex.TypeDualOpen:
		if inDualDialogueBlock {
			l.addError(currentLineNum, internal.MsgNestedDualDialogue, line.Contents)
		}
		return true
	case lex.TypeDualClose:
		return false
	case lex.TypeSpeaker:
		if strings.HasSuffix(strings.TrimSpace(line.Contents), "^") && inDualDialogueBlock {
			l.addError(currentLineNum, internal.MsgTooManyDualSpeakers, line.Contents)
		}
	}
	return inDualDialogueBlock
}

// checkBasicIssues performs basic validation checks
func (l *Linter) checkBasicIssues(line lex.Line, screenplay lex.Screenplay, i, currentLineNum int) {
	// Check for empty speaker names
	if line.Type == lex.TypeSpeaker && strings.TrimSpace(line.Contents) == "" {
		l.addError(currentLineNum, internal.MsgEmptySpeaker, line.Contents)
	}

	// Check for parentheticals without preceding dialogue/speaker
	if line.Type == lex.TypeParen && i > 0 {
		prevType := screenplay[i-1].Type
		if prevType != lex.TypeSpeaker && prevType != lex.TypeDialog && prevType != lex.TypeParen {
			l.addError(currentLineNum, internal.MsgMisplacedParenthetical, line.Contents)
		}
	}
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
