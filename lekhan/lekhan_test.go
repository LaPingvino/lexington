package lekhan

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type vector struct {
	clusters []string
	cells    []int
	note     string
}

var escape = regexp.MustCompile(`<(ZWJ|ZWNJ|U\+[0-9A-Fa-f]{4,6})>`)

func unescape(s string) string {
	return escape.ReplaceAllStringFunc(s, func(m string) string {
		switch m {
		case "<ZWJ>":
			return "\u200d"
		case "<ZWNJ>":
			return "\u200c"
		}
		n, err := strconv.ParseInt(m[3:len(m)-1], 16, 32)
		if err != nil {
			return m
		}
		return string(rune(n))
	})
}

// vectors reads testdata/vectors.txt, a copy of Courier Badi's
// docs/lekhan-vectors.txt.
func vectors(t *testing.T) []vector {
	f, err := os.Open("testdata/vectors.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []vector
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			t.Fatalf("bad line %q", line)
		}
		v := vector{clusters: strings.Split(unescape(parts[0]), "|")}
		if len(parts) > 2 {
			v.note = parts[2]
		}
		for _, f := range strings.Fields(parts[1]) {
			n, err := strconv.Atoi(f)
			if err != nil {
				t.Fatalf("bad cells in %q", line)
			}
			v.cells = append(v.cells, n)
		}
		if len(v.cells) != len(v.clusters) {
			t.Fatalf("%q: %d clusters, %d counts", line, len(v.clusters), len(v.cells))
		}
		out = append(out, v)
	}
	return out
}

func TestVectors(t *testing.T) {
	vs := vectors(t)
	if len(vs) < 50 {
		t.Fatalf("only %d vectors", len(vs))
	}
	for _, v := range vs {
		text := strings.Join(v.clusters, "")
		rs := []rune(text)
		var got []string
		var cells []int
		for i := 0; i < len(rs); {
			j := ClusterEnd(rs, i)
			got = append(got, string(rs[i:j]))
			cells = append(cells, ClusterCells(rs[i:j]))
			i = j
		}
		if strings.Join(got, "|") != strings.Join(v.clusters, "|") {
			t.Errorf("%s (%s): clusters %q, want %q", text, v.note, got, v.clusters)
			continue
		}
		total := 0
		for k, c := range cells {
			if c != v.cells[k] {
				t.Errorf("%s (%s): cluster %q takes %d cells, want %d", text, v.note, got[k], c, v.cells[k])
			}
			total += c
		}
		share := Cells(rs)
		sum := 0
		for i, c := range share {
			sum += c
			if Share(rs, i) != c {
				t.Errorf("%s: Share(%d) %d, Cells %d", text, i, Share(rs, i), c)
			}
		}
		if sum != total || Width(text) != total {
			t.Errorf("%s: shares add to %d, Width %d, clusters %d", text, sum, Width(text), total)
		}
	}
}

// A conjunct's second cell is its second consonant's: typing it in grows
// the cluster from one cell to two.
func TestShares(t *testing.T) {
	for text, want := range map[string][]int{
		"क्ष":    {1, 0, 1},
		"कि":     {1, 0},
		"स्क्री": {1, 0, 1, 0, 0, 0},
		"क्":     {1, 0},
		"आ":      {1},
		"ा":      {1},
		" ं":     {1, 1},
	} {
		got := Cells([]rune(text))
		if len(got) != len(want) {
			t.Fatalf("%s: %v", text, got)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: shares %v, want %v", text, got, want)
				break
			}
		}
	}
}

func TestJoins(t *testing.T) {
	if !Joins('क', 'ि') || !Joins('्', 'ष') || Joins('्', 'अ') || Joins('क', 'ख') || Joins('\n', 'ा') {
		t.Error("Joins")
	}
	// the virama joins only after a consonant (GB9c)
	if rs := []rune("अ्क"); Boundary(rs, 2) == false {
		t.Error("a virama after a vowel joins")
	}
	if rs := []rune("क्\u200cष"); !Boundary(rs, 3) {
		t.Error("ZWNJ")
	}
}

// A lam-alef is one cluster of one cell, also with vowel marks on the
// lam; a tatweel or a joiner between, alef wasla, keep the two apart.
func TestLamAlef(t *testing.T) {
	for text, want := range map[string]int{
		"لا": 1, "لآ": 1, "لأ": 1, "لإ": 1, "لاَ": 1, "إلا": 2, "بلا": 2, "السلام": 5, "لأن": 2,
		"لَا": 1, "لّا": 1, "لٰا": 1, "لّٰا": 1, "فَلَّا": 2, "السَّلَامُ": 5,
		"لَـا": 3, "لـا": 3, "ل‍ا": 2, "ل‌ا": 2, "لٱ": 2, "ڵا": 2, "َا": 1,
	} {
		if got := Width(text); got != want {
			t.Errorf("Width(%q) = %d, want %d", text, got, want)
		}
	}
	rs := []rune("فَلَّا ب")
	for i, want := range []bool{true, false, true, false, false, false, true, true} {
		if got := Boundary(rs, i); got != want {
			t.Errorf("%q: Boundary(%d) = %v, want %v", string(rs), i, got, want)
		}
	}
	if ClusterEnd(rs, 2) != 6 || ClusterCells(rs[2:6]) != 1 {
		t.Errorf("لَّا: ends at %d, %d cells", ClusterEnd(rs, 2), ClusterCells(rs[2:6]))
	}
	if !Joins('ل', 'ا') || Joins('َ', 'ا') || Joins('ـ', 'ا') || Joins('ل', 'ٱ') || Joins('ب', 'ب') {
		t.Error("Joins")
	}
	if rs := []rune("بَا"); !Boundary(rs, 2) {
		t.Error("an alef after a vowelled beh joins")
	}
}
