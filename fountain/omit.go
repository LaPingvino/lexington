package fountain

import (
	"strings"

	"github.com/LaPingvino/lexington/lex"
)

// An omitted scene, as a locked shooting script keeps it: the heading
// becomes OMITTED with the scene's number, and the scene as it was goes
// into a boneyard right under it, which no Fountain app prints, so that
// it can come back:
//
//	.OMITTED #12#
//
//	/* Omitted 12
//	INT. HOUSE - DAY #12#
//
//	Anna waits.
//	*/
//
// A "*/" in the scene would close the boneyard: it is kept as "* /"
// (and read back as "*/"), as Accolade's notes block does.

const omittedOpen = "/* Omitted"

// OmitScene is a scene's text (its heading line up to the next heading)
// omitted; false if its heading has no number (a scene is omitted only
// once the numbers are locked) or is omitted already. The blank lines
// after the scene stay after the block.
func OmitScene(scene string) (string, bool) {
	body := strings.TrimRight(scene, "\n")
	trail := scene[len(body):]
	heading, _, _ := strings.Cut(body, "\n")
	_, n := lex.SceneNumber(heading)
	if _, omitted := lex.OmittedNumber(heading); n == "" || omitted || strings.TrimSpace(body) == "" {
		return scene, false
	}
	return "." + lex.Omitted + " #" + n + "#\n\n" + omittedOpen + " " + n + "\n" +
		strings.ReplaceAll(body, "*/", "* /") + "\n*/" + trail, true
}

// RestoreScene is an omitted scene (OmitScene's text, from its heading
// to the next) as it was, and anything after the block kept after it;
// false if it is not one, or its block is gone (then the scene can only
// be written again).
func RestoreScene(omitted string) (string, bool) {
	heading, rest, _ := strings.Cut(omitted, "\n")
	n, ok := lex.OmittedNumber(heading)
	if !ok {
		return omitted, false
	}
	rest = strings.TrimLeft(rest, "\n")
	first, inside, _ := strings.Cut(rest, "\n")
	if f := strings.TrimSpace(first); f != omittedOpen+" "+n && !(n == "" && f == omittedOpen) {
		return omitted, false
	}
	end := closingLine(inside)
	if end < 0 {
		return omitted, false
	}
	scene := strings.ReplaceAll(inside[:end], "* /", "*/")
	after := inside[end:]
	after = after[strings.Index(after, "*/")+2:]
	return strings.TrimSuffix(scene, "\n") + after, true
}

// closingLine is where the line that closes the boneyard starts ("*/"
// on its own), or -1.
func closingLine(s string) int {
	at := 0
	for {
		line, more, found := strings.Cut(s[at:], "\n")
		if strings.TrimSpace(line) == "*/" {
			return at
		}
		if !found {
			return -1
		}
		at = len(s) - len(more)
	}
}

// IsOmitted reports whether a scene's text is an omitted one that
// RestoreScene can bring back.
func IsOmitted(scene string) bool {
	_, ok := RestoreScene(scene)
	return ok
}

// SplitOmitted is a merged run of omitted scenes (".OMITTED #12-14#",
// the run's blocks under it: "/* Omitted 12 …", "/* Omitted 13 …") one
// OMITTED heading a block again, each as OmitScene writes it, so that
// each can be restored; false if the heading is no range or has no
// block under it. The blocks are read up to the first line that is none;
// from there on the text stays as it was.
func SplitOmitted(merged string) (string, bool) {
	heading, rest, _ := strings.Cut(merged, "\n")
	n, ok := lex.OmittedNumber(heading)
	if !ok {
		return merged, false
	}
	if _, _, isRange := lex.NumberRange(n); !isRange {
		return merged, false
	}
	var out strings.Builder
	blocks := 0
	for {
		lead := len(rest) - len(strings.TrimLeft(rest, "\n"))
		first, inside, _ := strings.Cut(rest[lead:], "\n")
		f := strings.TrimSpace(first)
		num, isBlock := strings.CutPrefix(f, omittedOpen+" ")
		end := -1
		if isBlock && num != "" {
			end = closingLine(inside)
		}
		if end < 0 {
			break
		}
		after := inside[end:]
		after = after[strings.Index(after, "*/")+2:]
		if blocks > 0 {
			out.WriteString(rest[:lead])
		}
		out.WriteString("." + lex.Omitted + " #" + num + "#\n\n" + first + "\n" + inside[:end] + "*/")
		blocks++
		rest = after
	}
	if blocks == 0 {
		return merged, false
	}
	return out.String() + rest, true
}
