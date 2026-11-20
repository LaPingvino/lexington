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

// SplitOmitted rewrites the merged OMITTED heading Lexington v1.13.0
// wrote for a run of omitted scenes (".OMITTED #12-14#", the run's
// blocks under it: "/* Omitted 12 …", "/* Omitted 13 …"; lex.OmittedRange)
// as the text keeps them now: one OMITTED heading a block, each as
// OmitScene writes it, so that each can be restored. The run's first and
// last number get a heading even where their block is gone (a number
// between them without a block is lost: nothing says what it was).
// false if the heading is no such run. The blocks are read up to the
// first line that is none; from there on the text stays as it was.
func SplitOmitted(merged string) (string, bool) {
	heading, rest, _ := strings.Cut(merged, "\n")
	firstNo, lastNo, ok := lex.OmittedRange(heading)
	if !ok {
		return merged, false
	}
	head := func(n string) string { return "." + lex.Omitted + " #" + n + "#" }
	var out strings.Builder
	var nums []string
	for {
		lead := len(rest) - len(strings.TrimLeft(rest, "\n"))
		first, inside, _ := strings.Cut(rest[lead:], "\n")
		f := strings.TrimSpace(first)
		num, isBlock := strings.CutPrefix(f, omittedOpen+" ")
		end := -1
		if isBlock && num != "" && num != "scene" {
			end = closingLine(inside)
		}
		if end < 0 {
			break
		}
		after := inside[end:]
		after = after[strings.Index(after, "*/")+2:]
		if len(nums) == 0 && num != firstNo {
			out.WriteString(head(firstNo) + "\n\n")
		}
		if len(nums) > 0 {
			out.WriteString(rest[:lead])
		}
		out.WriteString(head(num) + "\n\n" + first + "\n" + inside[:end] + "*/")
		nums = append(nums, num)
		rest = after
	}
	switch {
	case len(nums) == 0:
		out.WriteString(head(firstNo) + "\n\n" + head(lastNo) + "\n")
	case nums[len(nums)-1] != lastNo:
		out.WriteString("\n\n" + head(lastNo))
	}
	return out.String() + rest, true
}
