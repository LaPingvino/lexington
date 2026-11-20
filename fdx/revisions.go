package fdx

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LaPingvino/lexington/lex"
)

// Final Draft's revisions, as a real file has them (docs: none; Final
// Draft's own default sets in an FDX it wrote):
//
//	<Revisions ActiveSet="1" Location="7.75" RevisionMode="Yes" RevisionsShown="Active"
//	    ShowAllMarks="No" ShowAllSets="No" ShowPageColor="No">
//	  <Revision Color="#000000000000" FullRevision="No" ID="1" Mark="*"
//	      Name="Blue Rev. (mm/dd/yy)" PageColor="#C6C6EDEDFEFE" Style=""/>
//	</Revisions>
//	<DeletedText><DeletedTextLocation Offset="844" RevisionID="1"/></DeletedText>
//
// and each <Text> run its RevisionID (0: the original). A deletion is a
// place only: its Offset counts the characters of the script's text runs
// with one for each paragraph's end (that is how the sample's offsets
// fall: within and at the start of the paragraphs its set changed). So
// the text that went cannot be had back: reading marks the paragraph
// the place is in (the one after a deleted paragraph), and writing writes
// no DeletedText (the offsets of other files are not verified).

// FdxRevisions is the <Revisions> element.
type FdxRevisions struct {
	ActiveSet      string        `xml:"ActiveSet,attr,omitempty"`
	Location       string        `xml:"Location,attr,omitempty"`
	RevisionMode   string        `xml:"RevisionMode,attr,omitempty"`
	RevisionsShown string        `xml:"RevisionsShown,attr,omitempty"`
	ShowAllMarks   string        `xml:"ShowAllMarks,attr,omitempty"`
	ShowAllSets    string        `xml:"ShowAllSets,attr,omitempty"`
	ShowPageColor  string        `xml:"ShowPageColor,attr,omitempty"`
	Revisions      []FdxRevision `xml:"Revision"`
}

// FdxRevision is a revision set.
type FdxRevision struct {
	Color        string `xml:"Color,attr"`
	FullRevision string `xml:"FullRevision,attr"`
	ID           string `xml:"ID,attr"`
	Mark         string `xml:"Mark,attr"`
	Name         string `xml:"Name,attr"`
	PageColor    string `xml:"PageColor,attr"`
	Style        string `xml:"Style,attr"`
}

// FdxDeletedText are the places text was deleted.
type FdxDeletedText struct {
	Locations []FdxDeletedTextLocation `xml:"DeletedTextLocation"`
}

// FdxDeletedTextLocation is one such place.
type FdxDeletedTextLocation struct {
	Offset     string `xml:"Offset,attr"`
	RevisionID string `xml:"RevisionID,attr"`
}

// revisionName is "Blue Rev. 20/11/26" (as Final Draft names its sets,
// "Blue Rev. (mm/dd/yy)"): the set's name and, when known, its date.
func revisionName(s lex.RevisionSet, layout string) string {
	if s.Date.IsZero() {
		return s.Name
	}
	if layout == "" {
		layout = lex.DefaultDateLayout
	}
	return s.Name + " Rev. " + s.Date.Format(layout)
}

// namedRevision reads a set's name back: "Blue Rev. 20/11/26" is Blue of
// that date (in layout, else as Final Draft's US date); Final Draft's
// own "Blue Rev. (mm/dd/yy)" is Blue, of no date. A name of another form
// is kept as it is.
var namedRevision = regexp.MustCompile(`^(.*?)\s+Rev\.?\s*\(?([^()]*)\)?$`)

func readRevisionName(name, layout string) lex.RevisionSet {
	s := lex.RevisionSet{Name: strings.TrimSpace(name)}
	m := namedRevision.FindStringSubmatch(s.Name)
	if m == nil || strings.TrimSpace(m[1]) == "" {
		return s
	}
	s.Name = strings.TrimSpace(m[1])
	for _, l := range []string{layout, lex.DefaultDateLayout, "01/02/06", "2006-01-02", "02.01.06", "01/02/2006"} {
		if l == "" {
			continue
		}
		if d, err := time.Parse(l, strings.TrimSpace(m[2])); err == nil {
			s.Date = d
			break
		}
	}
	return s
}

// paperHex is a colour 16 bits a channel.
func paperHex(c [3]uint8) string {
	return fmt.Sprintf("#%02X%02X%02X%02X%02X%02X", c[0], c[0], c[1], c[1], c[2], c[2])
}

// revisionIDs are the IDs the sets get: 1 on, in order, a first White
// (the original, Final Draft's 0) none.
func revisionIDs(r *lex.Revisions) map[string]string {
	ids := map[string]string{}
	if r == nil {
		return ids
	}
	n := 0
	for i, s := range r.Sets {
		if i == 0 && strings.EqualFold(s.Name, lex.RevisionColors[0]) {
			ids[s.Name] = "0"
			continue
		}
		n++
		ids[s.Name] = strconv.Itoa(n)
	}
	return ids
}

// buildRevisions is the <Revisions> of the sets (nil without any).
func buildRevisions(r *lex.Revisions, ids map[string]string) *FdxRevisions {
	if r == nil || len(r.Sets) == 0 {
		return nil
	}
	out := &FdxRevisions{Location: "7.75", RevisionMode: "No", RevisionsShown: "All", ShowAllMarks: "Yes",
		ShowAllSets: "Yes", ShowPageColor: "No"}
	if r.PageColors {
		out.ShowPageColor = "Yes"
	}
	for _, s := range r.Sets {
		id := ids[s.Name]
		if id == "0" {
			continue
		}
		paper := [3]uint8{255, 255, 255}
		if c, ok := lex.RevisionPaper(s.Name); ok {
			paper = c
		}
		out.Revisions = append(out.Revisions, FdxRevision{Color: "#000000000000", FullRevision: "No", ID: id,
			Mark: s.MarkOf(), Name: revisionName(s, r.DateLayout), PageColor: paperHex(paper)})
		out.ActiveSet = id
	}
	if len(out.Revisions) == 0 {
		return nil
	}
	return out
}

// readRevisions are the sets of a file's <Revisions> by their ID, those
// its runs or deletions use, and the sets in order (by ID).
type fileRevisions struct {
	byID map[string]lex.RevisionSet
	used map[string]bool
}

func newFileRevisions(r *FdxRevisions) fileRevisions {
	f := fileRevisions{byID: map[string]lex.RevisionSet{}, used: map[string]bool{}}
	if r == nil {
		return f
	}
	for _, rev := range r.Revisions {
		if rev.ID == "" || rev.ID == "0" {
			continue
		}
		s := readRevisionName(rev.Name, "")
		if rev.Mark != "" && rev.Mark != "*" {
			s.Mark = rev.Mark
		}
		f.byID[rev.ID] = s
	}
	return f
}

// of is the latest set of the IDs (by number), "" for none known.
func (f fileRevisions) of(ids ...string) string {
	best, name := -1, ""
	for _, id := range ids {
		s, ok := f.byID[id]
		n, err := strconv.Atoi(id)
		if !ok || err != nil || n <= best {
			continue
		}
		best, name = n, s.Name
		f.used[id] = true
	}
	return name
}

// sets are the sets the file's text uses, in the order of their IDs.
func (f fileRevisions) sets() []lex.RevisionSet {
	var ids []int
	for id := range f.used {
		n, _ := strconv.Atoi(id)
		ids = append(ids, n)
	}
	sort.Ints(ids)
	out := make([]lex.RevisionSet, 0, len(ids))
	for _, n := range ids {
		out = append(out, f.byID[strconv.Itoa(n)])
	}
	return out
}

// deletedAt are the paragraphs (in the order the reader meets them, a
// dual dialogue's own) a deletion is in, with the deletion's set.
func deletedAt(c FdxContent, d *FdxDeletedText) map[*FdxParagraph][]string {
	out := map[*FdxParagraph][]string{}
	if d == nil || len(d.Locations) == 0 {
		return out
	}
	type place struct {
		p          *FdxParagraph
		start, end int
	}
	var places []place
	off := 0
	var walk func(ps []FdxParagraph)
	walk = func(ps []FdxParagraph) {
		for i := range ps {
			p := &ps[i]
			if p.DualDialogue != nil {
				walk(p.DualDialogue.Paragraphs)
				continue
			}
			start := off
			for _, t := range p.Texts {
				off += len([]rune(t.Content))
			}
			places = append(places, place{p, start, off})
			off++ // the paragraph's end
		}
	}
	walk(c.Paragraphs)
	for _, l := range d.Locations {
		at, err := strconv.Atoi(l.Offset)
		if err != nil {
			continue
		}
		for _, pl := range places {
			if at >= pl.start && at <= pl.end {
				out[pl.p] = append(out[pl.p], l.RevisionID)
				break
			}
		}
	}
	return out
}

// ParseRevisions reads an FDX as ParseWithError does, with its revisions:
// each line's Revision the latest set of its runs (or of a deletion in
// it), and the sets the text uses, oldest first, by the file's own names
// ("Blue Rev. 20/11/26" read as Blue of that date).
func ParseRevisions(file io.Reader) (lex.Screenplay, []lex.RevisionSet, error) {
	s, revs, err := parse(file)
	if err != nil {
		return nil, nil, err
	}
	return s, revs.sets(), nil
}
