package lex

import "strings"

// Runs of OMITTED scenes. The text keeps one OMITTED heading a scene
// (".OMITTED #12#", the scene in boneyard under it), so that each keeps
// its number and can come back. A shooting script may print a run of
// them as one line, "12-14 OMITTED 12-14": MergeOmittedRuns makes the
// rows so, for printing only (PDF, HTML, DOCX, ODT and the rest;
// cli.Options.MergeOmitted); FDX keeps one heading a number.
// ".OMITTED #12-14#" in the text is one scene, labelled "12-14".

// OmittedRunLabel is how a run of OMITTED scenes prints its numbers,
// its first and last label joined: a dash, or an en
// dash when a label has a dash of its own (I-1-A – I-1-C).
func OmittedRunLabel(first, last string) string {
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
		merged.Contents = WithSceneNumber(l.Contents, OmittedRunLabel(first, last))
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
