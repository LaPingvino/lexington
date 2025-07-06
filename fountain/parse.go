package fountain

import (
	"bufio"
	"io"
	"strings"

	"github.com/lapingvino/lexington/internal"
	"github.com/lapingvino/lexington/lex"
)

// Scene contains all the prefixes the scene detection looks for.
// This can be changed with the toml configuration in the rules package.
var Scene = []string{"INT", "EXT", "EST", "INT./EXT", "INT/EXT", "EXT/INT", "EXT./INT", "I/E"}

// last safely retrieves a pointer to a lex.Line from the screenplay,
// returning a pointer to an "empty" line if the index is out of bounds.
func last(out *lex.Screenplay, i int) *lex.Line {
	if len(*out) >= i {
		return &(*out)[len(*out)-i]
	} else {
		line := lex.Line{Type: "empty"} // Return an empty line if out is not long enough
		return &line
	}
}

// CheckScene determines if a row is a scene heading.
func CheckScene(row string) (bool, string, string) {
	upperRow := strings.ToUpper(row)

	// Check if any scene prefix matches
	_, found := internal.Find(Scene, func(prefix string) bool {
		return strings.HasPrefix(upperRow, prefix+" ") || strings.HasPrefix(upperRow, prefix+".")
	})

	if found {
		return true, "scene", upperRow
	}

	// Check for forced scene (starts with .)
	if strings.HasPrefix(row, ".") && !strings.HasPrefix(row, "..") {
		return true, "scene", row[1:]
	}

	return false, "scene", upperRow
}

// CheckCrow determines if a row is a transition or a centered text.
func CheckCrow(row string) (bool, string, string) {
	var crow bool
	var el string
	row = strings.ToUpper(row)
	if strings.HasPrefix(row, ">") || strings.HasSuffix(row, " TO:") {
		crow = true
		el = "trans"
	}
	if strings.HasPrefix(row, ">") && strings.HasSuffix(row, "<") {
		el = "center"
	}
	return crow, el, strings.Trim(row, ">< ")
}

// CheckEqual determines if a row is a synopsis or a page break.
func CheckEqual(row string) (bool, string, string) {
	var equal bool
	var el string
	if strings.HasPrefix(row, "=") {
		equal = true
		el = "synopse"
	}
	if len(row) >= 3 && strings.Trim(row, "=") == "" {
		el = "newpage"
	}
	return equal, el, strings.TrimLeft(row, "= ")
}

// CheckSection determines if a row is a section heading.
func CheckSection(row string) (bool, string, string) {
	var section bool
	if strings.HasPrefix(row, "#") {
		section = true
	}
	return section, "section", row
}

// CheckForce determines if a row is a forced type (e.g., @speaker, ~lyrics, !action).
func CheckForce(row string) (bool, string, string) {
	var force = true
	var ftype string
	if len(row) < 1 {
		return false, "", ""
	}
	switch row[0] {
	case '@':
		ftype = "speaker"
	case '~':
		ftype = "lyrics"
	case '!':
		ftype = "action"
	default:
		force = false
	}
	if force {
		row = row[1:]
	}
	return force, ftype, row
}

// Parse converts a Fountain file into the internal lex.Screenplay format.
func Parse(scenes []string, file io.Reader) (out lex.Screenplay) {
	Scene = scenes
	var err error
	var titlepage bool = true
	var inDialogueContext bool = false // True if the *previous* line allows for dialog/paren
	var inDualDialogue bool = false    // True if we are currently inside a dual dialogue block
	var titletag string
	var toParse []string
	var s string
	var consecutiveEmptyLines int = 0
	var hasTitlePageContent bool = false // Track if we actually have title page content

	// Read all lines into a buffer
	// Note: ReadString('\n') includes the newline character if found.
	f := bufio.NewReader(file)
	for err == nil {
		s, err = f.ReadString('\n')
		if err != io.EOF || len(s) > 0 {
			toParse = append(toParse, s)
		}
	}
	// Add a sentinel empty line to trigger final closing logic, especially for dual dialogue.
	// This line will be processed last but not necessarily appended to `out`.
	toParse = append(toParse, "")

	for i, row := range toParse {
		originalRow := row
		row = strings.TrimRight(originalRow, "\n\r")
		trimmedSpaceRow := strings.TrimSpace(row)

		var currentLine lex.Line
		var isCurrentLineDualSpeakerCandidate bool = false

		// --- Title Page Handling ---
		if titlepage {
			// A title page ends if:
			// 1. A line without a colon is encountered, AND it's not purely an empty line,
			//    OR if it's the very first line of the file and it doesn't have a colon
			//    (implies the file starts directly with screenplay body).
			// 2. Three or more consecutive empty lines are encountered.

			isKeyValLine := strings.Contains(row, ":") && !strings.HasPrefix(row, "   ")

			// Check for consecutive empty lines
			if trimmedSpaceRow == "" {
				consecutiveEmptyLines++
			} else {
				consecutiveEmptyLines = 0 // Reset counter if non-empty line
			}

			// Condition to exit title page mode
			if (!isKeyValLine && trimmedSpaceRow != "") || (consecutiveEmptyLines >= 2) {
				titlepage = false
				// Only add newpage marker if we actually had title page content
				if hasTitlePageContent {
					out = append(out, lex.Line{Type: lex.TypeNewPage})
				}
				if trimmedSpaceRow == "" {
					// If the transition was triggered by empty lines, and the current line is also empty,
					// this empty line acts as part of the page break and should not be processed further
					// by the screenplay body parser as a standalone empty line.
					continue
				}
				// If transition by a non-key-value, non-empty line (e.g., a scene heading),
				// this line needs to fall through and be parsed by the screenplay body logic.
			}

			// If still in title page mode after checks, parse as a title page line.
			if titlepage {
				if titletag == "" && trimmedSpaceRow != "" { // Only add titlepage marker if content exists
					out = append(out, lex.Line{Type: lex.TypeTitlePage})
					hasTitlePageContent = true
				}

				if isKeyValLine {
					// This is a title page key-value pair
					split := strings.SplitN(row, ":", 2)
					currentMetaTag := split[0]
					switch strings.ToLower(currentMetaTag) {
					case "title":
						titletag = "Title" // Capitalize for HTML template match
					case "credit":
						titletag = "Credit" // Capitalize for HTML template match
					case "author", "authors":
						titletag = "Author" // Capitalize for HTML template match
					default:
						// If previous tag was one of the special ones, add a metasection break.
						if titletag == "Title" || titletag == "Credit" || titletag == "Author" {
							out = append(out, lex.Line{Type: "metasection"})
						}
						titletag = currentMetaTag // For other custom tags, keep original case
						titletag = currentMetaTag
					}
					currentLine.Type = titletag
					currentLine.Contents = strings.TrimSpace(split[1])
					hasTitlePageContent = true
				} else {
					// Continuation of a multi-line title page entry
					currentLine.Type = titletag
					currentLine.Contents = trimmedSpaceRow
					hasTitlePageContent = true
				}

				if currentLine.Contents == "" { // Don't append purely empty content lines that aren't structural (like the `titlepage` itself)
					continue
				}
				out = append(out, currentLine)
				continue // Line was handled by title page, move to next row
			}
		}

		// --- Normal Screenplay Body Parsing (if titlepage is false) ---
		if trimmedSpaceRow == "" {
			currentLine.Type = "empty"
			currentLine.Contents = ""
		} else {
			foundExplicitType := false

			// Check forced types (@speaker, ~lyrics, !action)
			if check, ftype, contents := CheckForce(originalRow); check {
				currentLine.Type = ftype
				currentLine.Contents = strings.TrimSpace(contents) // Ensure content is fully trimmed
				foundExplicitType = true
				if ftype == "speaker" {
					// A forced speaker: @SPEAKER
					// Check if it's a dual dialogue speaker by looking at the original row's suffix
					if strings.HasSuffix(currentLine.Contents, "^") { // Check trimmed content for ^
						isCurrentLineDualSpeakerCandidate = true
						currentLine.Contents = strings.TrimRight(currentLine.Contents, " ^") // Trim " ^" from content (already trimmed)
					}
				}
			}

			// Check other structural types (Scene, Transition, Newpage, Section)
			if !foundExplicitType {
				checkfuncs := []func(string) (bool, string, string){
					CheckScene,
					CheckCrow,
					CheckEqual,
					CheckSection,
				}
				for _, checkfunc := range checkfuncs {
					if check, element, contents := checkfunc(row); check {
						currentLine.Type = element
						currentLine.Contents = strings.TrimSpace(contents) // Ensure content is fully trimmed
						foundExplicitType = true
						break
					}
				}
			}

			// If no explicit type found yet, determine based on inferred types (speaker, paren, dialog, action)
			if !foundExplicitType {
				charcheck := strings.Split(row, "(")
				if len(charcheck) > 0 && strings.ToUpper(charcheck[0]) == charcheck[0] && strings.TrimSpace(charcheck[0]) != "" {
					// Looks like a speaker name (all caps, not empty before parenthesis)
					currentLine.Type = lex.TypeSpeaker
					currentLine.Contents = trimmedSpaceRow
					// Check if it's a dual dialogue speaker by looking at the original row's suffix
					if strings.HasSuffix(currentLine.Contents, "^") { // Check currentLine.Contents for ^
						isCurrentLineDualSpeakerCandidate = true
						currentLine.Contents = strings.TrimRight(currentLine.Contents, " ^") // Trim " ^" from content (already trimmed)
					}
				} else if len(row) > 1 && row[0] == '(' && row[len(row)-1] == ')' {
					// It's a parenthetical
					if inDialogueContext {
						currentLine.Type = "paren"
					} else {
						// Standalone parenthetical like "(what? I don't know...)" is an action.
						currentLine.Type = "action"
					}
					currentLine.Contents = trimmedSpaceRow
				} else if inDialogueContext {
					// Dialogue following a dialogue element (based on inDialogueContext)
					currentLine.Type = "dialog"
					currentLine.Contents = trimmedSpaceRow
				} else {
					// Default to action if no other type matches and not in dialogue context
					currentLine.Type = "action"
					currentLine.Contents = trimmedSpaceRow
				}
			}
		}

		// 2. Dual Dialogue Closing Logic (This happens *after* currentLine is determined, but *before* appending it)
		// A dual dialogue block is closed if `inDualDialogue` is true AND the `currentLine` is not
		// a speaker, dialogue, parenthetical, or an empty line (which can separate dialogue blocks).
		if inDualDialogue {
			shouldCloseDual := false
			switch currentLine.Type {
			case lex.TypeScene, lex.TypeAction, lex.TypeTrans, lex.TypeCenter, "section", "synopse", lex.TypeNewPage, lex.TypeTitlePage, "metasection":
				shouldCloseDual = true
			case lex.TypeSpeaker:
				if !isCurrentLineDualSpeakerCandidate {
					shouldCloseDual = true // A regular speaker ends a dual dialogue
				}
			case lex.TypeEmpty:
				// If this is the *last* line (the sentinel empty line), and we are in dual dialogue,
				// it means the dual dialogue should close here.
				if i == len(toParse)-1 {
					shouldCloseDual = true
				}
				// Otherwise, an empty line *within* a dual dialogue block does not close it.
			}

			if shouldCloseDual {
				out = append(out, lex.Line{Type: lex.TypeDualClose})
				inDualDialogue = false
			}
		}

		// 3. Dual Dialogue Opening/Next Logic (This also happens *before* appending the current line)
		if isCurrentLineDualSpeakerCandidate {
			if !inDualDialogue { // This is the *first* dual speaker in a new dual block
				// Insert `dualspeaker_open` marker *before* the previous speaker's block.
				// This involves looking back for the preceding speaker's entire block (speaker + dialog/paren lines)
				// and inserting `dualspeaker_open` before it.
				// We need to find the empty line before the previous speaker.
				foundOpenInsertPoint := false
				for j := len(out) - 1; j >= 0; j-- {
					if out[j].Type == lex.TypeSpeaker { // Found the previous speaker (e.g., "MARY" in the test case)
						// Go back until an empty line *before* that speaker or the start of the screenplay
						for k := j; k >= 0; k-- { // Start from speaker line
							if out[k].Type == lex.TypeEmpty {
								// Insert dualspeaker_open after this empty line.
								out = append(out[:k+1], append([]lex.Line{{Type: lex.TypeDualOpen}}, out[k+1:]...)...)
								foundOpenInsertPoint = true
								break
							} else if k == 0 { // If we reach the very beginning and no empty line
								out = append([]lex.Line{{Type: lex.TypeDualOpen}}, out...)
								foundOpenInsertPoint = true
								break
							}
						}
						if foundOpenInsertPoint {
							break // Break from outer loop too, found insertion point for open
						}
					}
				}
				inDualDialogue = true // Mark that we are now inside a dual dialogue block
				// Append `dualspeaker_next` immediately before the current dual speaker's line.
				out = append(out, lex.Line{Type: lex.TypeDualNext})
			} else {
				// If already in dual dialogue and another '^' speaker is encountered,
				// close the current dual dialogue block and treat this as a regular speaker
				// to avoid breaking HTML structure with more than two columns.
				out = append(out, lex.Line{Type: lex.TypeDualClose})
				inDualDialogue = false
				isCurrentLineDualSpeakerCandidate = false // Reset for current line
				currentLine.Type = lex.TypeSpeaker
				currentLine.Contents = strings.TrimRight(trimmedSpaceRow, " ^") // Trim " ^" from content for display
			}
		}

		// Append the determined line for the current row
		// The last empty sentinel line should generally not be appended unless it's a specific requirement.
		// Given TestParseDualDialogue expects `dualspeaker_close` then `empty`, we need to append the empty.
		// However, for the general case, if it's the very last sentinel line and it triggered dual dialogue closure,
		// we should probably not append it as a regular empty line.
		if !(i == len(toParse)-1 && trimmedSpaceRow == "" && inDualDialogue) { // Don't append the final sentinel empty line if it just closed dual dialogue
			out = append(out, currentLine)
		} else if i == len(toParse)-1 && trimmedSpaceRow == "" && !inDualDialogue {
			// If it's the final sentinel and no dual dialogue was open, append it if needed by tests.
			// TestParse expects a final empty line.
			out = append(out, currentLine)
		}

		// 4. Update `inDialogueContext` for the next iteration.
		// This determines if subsequent lines can be automatically interpreted as `paren` or `dialog`.
		// It's crucial this happens *after* `currentLine` is appended, using its final type.
		if currentLine.IsDialogueElement() {
			inDialogueContext = true
		} else if currentLine.Type == lex.TypeEmpty {
			// If an empty line is encountered immediately after a speaker (and not in dual dialogue),
			// the speaker should be reverted to an action according to Fountain rules.
			// This indicates the end of a dialogue context.
			if len(out) >= 2 && out[len(out)-2].Type == lex.TypeSpeaker && !inDualDialogue {
				out[len(out)-2].Type = lex.TypeAction // Revert last speaker to action if followed only by empty line
				inDialogueContext = false             // Context breaks
			} else {
				// An empty line not immediately after a speaker also breaks the `inDialogueContext`
				// for auto-detection, unless it's within a dual dialogue block.
				inDialogueContext = false
			}
		} else {
			// Any other non-dialogue line breaks the context.
			inDialogueContext = false
		}
	}

	// This final closing is now handled within the loop for the sentinel line.
	// if inDualDialogue {
	// 	out = append(out, lex.Line{Type: "dualspeaker_close"})
	// }

	return out
}
