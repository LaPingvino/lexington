package lex

import "strings"

// Runs of OMITTED scenes. The text keeps one OMITTED heading a scene
// (".OMITTED #12#", the scene in boneyard under it), so that each keeps
// its number and can come back. A shooting script may print a run of
// them as one line, "12-14 OMITTED 12-14": MergeOmittedRuns makes the
// rows so, for printing only (PDF, HTML, DOCX, ODT and the rest;
// cli.Options.MergeOmitted); FDX keeps one heading a number.
//
// Lexington v1.13.0 wrote the run itself into the text as
// ".OMITTED #12-14#"; SplitOmittedRanges (Fountain's reader runs it) reads
// that back as its scenes.

// omittedRunJoin joins a run's first and last label: a dash, or an en
// dash when a label has a dash of its own (I-1-A – I-1-C).
func omittedRunJoin(first, last string) string {
	if strings.Contains(first, "-") || strings.Contains(last, "-") {
		return first + "–" + last
	}
	return first + "-" + last
}

// between tells whether a row may stand between two OMITTED headings of
// one run: a blank line or boneyard (the omitted scenes' old text).
func betweenOmitted(l Line) bool {
	return l.Type == TypeEmpty || l.Type == TypeBoneyard || (l.Type == TypeAction && strings.TrimSpace(l.Contents) == "")
}

// omittedNo is the number of an OMITTED heading row ("" none or not
// one), and whether it is one.
func omittedNo(l Line) (string, bool) {
	if l.Type != TypeScene {
		return "", false
	}
	return OmittedNumber(l.Contents)
}

// MergeOmittedRuns is the screenplay with each run of two or more
// numbered OMITTED headings, nothing but blank lines and boneyard
// between them, printed as one: the first heading, numbered with the
// run's first and last label in the script's order ("12-14"), the other
// headings (and the blank lines before them) left out.
func MergeOmittedRuns(s Screenplay) Screenplay {
	out, _ := mergeOmittedRuns(s)
	return out
}

// OmittedRunSizes are, for each scene heading of MergeOmittedRuns(s) in
// order, how many of s's headings it stands for (1, or a run's length):
// to give measures of the merged script back to s's headings.
func OmittedRunSizes(s Screenplay) []int {
	_, sizes := mergeOmittedRuns(s)
	return sizes
}

func mergeOmittedRuns(s Screenplay) (Screenplay, []int) {
	out := make(Screenplay, 0, len(s))
	var sizes []int
	for i := 0; i < len(s); i++ {
		l := s[i]
		if l.Type != TypeScene {
			out = append(out, l)
			continue
		}
		first, ok := omittedNo(l)
		if !ok || first == "" {
			out = append(out, l)
			sizes = append(sizes, 1)
			continue
		}
		// the run's headings
		heads := []int{i}
		for j := i + 1; j < len(s); j++ {
			if n, ok := omittedNo(s[j]); ok && n != "" {
				heads = append(heads, j)
				continue
			}
			if !betweenOmitted(s[j]) {
				break
			}
		}
		if len(heads) < 2 {
			out = append(out, l)
			sizes = append(sizes, 1)
			continue
		}
		last, _ := omittedNo(s[heads[len(heads)-1]])
		merged := l
		merged.Contents = WithSceneNumber(l.Contents, omittedRunJoin(first, last))
		for _, h := range heads {
			if merged.Revision == "" {
				merged.Revision = s[h].Revision
			}
		}
		out = append(out, merged)
		sizes = append(sizes, len(heads))
		end := heads[len(heads)-1]
		for j := i + 1; j <= end; j++ {
			if s[j].Type == TypeScene {
				// the blank lines before a heading left out go with it
				for len(out) > 0 && out[len(out)-1].Type != TypeBoneyard && out[len(out)-1].Type != TypeScene && betweenOmitted(out[len(out)-1]) {
					out = out[:len(out)-1]
				}
				continue
			}
			out = append(out, s[j])
		}
		i = end
	}
	return out, sizes
}

// omittedBlockOpen is how an omitted scene's boneyard block starts:
// "/* Omitted 12" (fountain.OmitScene).
const omittedBlockOpen = "/* Omitted "

// SplitOmittedRanges reads v1.13.0's merged OMITTED headings
// (OmittedRange: ".OMITTED #12-14#") as their scenes: one OMITTED
// heading for each "/* Omitted 13" block under it, placed before its
// block, and the run's first and last number kept even where their
// block is gone. A number between them without a block is lost: nothing
// says what it was. Other rows are as they were.
func SplitOmittedRanges(s Screenplay) Screenplay {
	var out Screenplay
	for i := 0; i < len(s); i++ {
		l := s[i]
		first, last, ok := "", "", false
		if l.Type == TypeScene {
			first, last, ok = OmittedRange(l.Contents)
		}
		if !ok {
			if out != nil {
				out = append(out, l)
			}
			continue
		}
		if out == nil {
			out = append(make(Screenplay, 0, len(s)+4), s[:i]...)
		}
		heading := func(n string, at Line) Line {
			h := l
			h.Contents = WithSceneNumber(l.Contents, n)
			if at.Source != 0 {
				h.Source = at.Source
			}
			return h
		}
		end := i + 1
		for end < len(s) && betweenOmitted(s[end]) {
			end++
		}
		var nums []string
		var at []int
		for j := i + 1; j < end; j++ {
			if s[j].Type != TypeBoneyard {
				continue
			}
			if n, isBlock := strings.CutPrefix(strings.TrimSpace(s[j].Contents), omittedBlockOpen); isBlock {
				if n = strings.TrimSpace(n); n != "" && n != "scene" {
					nums, at = append(nums, n), append(at, j)
				}
			}
		}
		k := 0
		if len(nums) == 0 || nums[0] != first {
			out = append(out, heading(first, l))
		} else {
			out = append(out, heading(first, l))
			k = 1 // the first block's heading is the one written
		}
		for j := i + 1; j < end; j++ {
			if k < len(at) && at[k] == j {
				if nums[k] != first || k > 0 {
					out = append(out, heading(nums[k], s[j]))
				}
				k++
			}
			out = append(out, s[j])
		}
		if len(nums) == 0 || nums[len(nums)-1] != last {
			out = append(out, heading(last, l))
		}
		i = end - 1
	}
	if out == nil {
		return s
	}
	return out
}
