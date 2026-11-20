package report

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// shootSchedule is a schedule of shoot.fountain: scenes 3 and 1 on day 1
// (a call, notes with a comma and a semicolon), 1A on day 2 (no call),
// an undated day off, and 4 on the last.
func shootSchedule() Schedule {
	d := func(day int) time.Time { return time.Date(2026, 12, day, 0, 0, 0, 0, time.UTC) }
	return Schedule{Days: []Day{
		{ID: "d1", Date: d(1), Call: "07:00", Notes: "Kovač house, Ljubljana; the kitchen", Scenes: []int{4, 1}},
		{ID: "d2", Date: d(2), Scenes: []int{2}},
		{ID: "d3"},
		{ID: "d4", Date: d(4), Call: "6:30", Scenes: []int{5}},
	}}
}

func TestStripColours(t *testing.T) {
	r := shoot(t)
	strips := r.Strips(shootBreakdown(), StripOptions{})
	want := []struct{ id, colour, ie, dn, cast string }{
		{"1", "white", "INT", "D", "1 ANNA,2 TOM,3 CARL"},
		{"1A", "green", "EXT", "N", "1 ANNA,2 TOM"},
		{"2", "white", "INT", "D", ""}, // omitted: no strip of its own to colour
		{"3", "blue", "INT", "N", "1 ANNA,2 TOM"},
		{"4", "yellow", "EXT", "D", "3 CARL"},
	}
	for i, w := range want {
		s := strips[i]
		c, _ := s.Colour()
		if s.ID() != w.id || c != w.colour || s.IE() != w.ie || s.DN() != w.dn || strings.Join(s.Cast, ",") != w.cast {
			t.Errorf("strip %d: %s %s %s %s %v, want %+v", i, s.ID(), c, s.IE(), s.DN(), s.Cast, w)
		}
	}
	for _, c := range []struct {
		time         string
		night, known bool
	}{{"NIGHT", true, true}, {"LATE EVENING", true, true}, {"DUSK", true, true}, {"DAWN", false, true},
		{"MORNING", false, true}, {"CONTINUOUS", false, false}, {"MOMENTS LATER", false, false}, {"", false, false},
		{"NACHT", true, true}, {"DAG", false, true}, {"NOKTO", true, true}, {"НОЧЬ", true, true}, {"夜", true, true}} {
		if n, k := dayOrNight(c.time); n != c.night || k != c.known {
			t.Errorf("%q: night %v known %v", c.time, n, k)
		}
	}
	for _, c := range []struct {
		lang, place string
		ext         bool
	}{{"", "INT", false}, {"", "EXT", true}, {"", "INT./EXT", true}, {"", "I/E", true}, {"", "EST", true},
		{"", "", false}, {"nl", "BIN", false}, {"nl", "BUI", true}, {"it", "EST", true}, {"eo", "EKST", true}} {
		if got := exteriorOf(c.lang, c.place); got != c.ext {
			t.Errorf("%s %q: exterior %v", c.lang, c.place, got)
		}
	}
}

// A time that names no day or night takes the scene before's.
func TestStripContinuous(t *testing.T) {
	r := Report{Scenes: []Scene{
		{Order: 1, Number: "1", Place: "EXT", Location: "ROAD", Time: "NIGHT"},
		{Order: 2, Number: "2", Place: "INT", Location: "CAR", Time: "CONTINUOUS"},
		{Order: 3, Number: "3", Place: "INT", Location: "CAR", Time: "DAY"},
		{Order: 4, Number: "4", Place: "INT", Location: "CAR", Time: "LATER"},
	}}
	s := r.Strips(Breakdown{}, StripOptions{})
	if !s[1].Night || s[1].DayNightFrom != "before" || s[3].Night || s[0].DayNightFrom != "" {
		t.Errorf("strips %+v", s)
	}
}

func TestDOOD(t *testing.T) {
	r := shoot(t)
	rows := r.DOOD(shootBreakdown(), shootSchedule())
	got := map[string]string{}
	for _, row := range rows {
		got[row.Name] = strings.Join(row.Codes, "|")
	}
	want := map[string]string{"ANNA": "SW|WF||", "TOM": "SW|WF||", "CARL": "SW|H|H|WF"}
	for name, codes := range want {
		if got[name] != codes {
			t.Errorf("%s: %q, want %q", name, got[name], codes)
		}
	}
	if len(rows) != 3 || rows[2].Name != "CARL" || rows[2].Work != 2 || rows[2].Hold != 2 || rows[2].Start != 1 || rows[2].Finish != 4 {
		t.Errorf("rows %+v", rows)
	}
	one := r.DOOD(Breakdown{}, Schedule{Days: []Day{{Scenes: []int{5}}, {Scenes: []int{2}}}})
	if len(one) != 2 || one[0].Codes[1] != CodeStartWorkFinish || one[0].Codes[0] != "" {
		t.Errorf("one day: %+v", one)
	}
	if r.DayLength(shootSchedule().Days[0]) != 3 {
		t.Errorf("day 1's length %v", r.DayLength(shootSchedule().Days[0]))
	}
	if EndOfDay(0, 38) != "End of Day 1 — 4 6/8 pages" || EndOfDay(1, 3) != "End of Day 2 — 3/8 page" {
		t.Errorf("%q %q", EndOfDay(0, 38), EndOfDay(1, 3))
	}
}

// The one-liner and DOOD CSVs and the .ics are stable (PRODUCTION.md §14);
// -update rewrites them.
func TestScheduleGolden(t *testing.T) {
	r := shoot(t)
	b, s := shootBreakdown(), shootSchedule()
	files := map[string][]byte{}
	for name, tab := range map[string]Table{"oneliner.csv": r.OneLinerTable(b, s, StripOptions{}), "dood.csv": r.DOODTable(b, s)} {
		var buf bytes.Buffer
		if err := tab.WriteCSV(&buf); err != nil {
			t.Fatal(err)
		}
		files[name] = buf.Bytes()
	}
	var ics bytes.Buffer
	skipped, err := WriteICS(&ics, r, s, ICSOptions{Title: "The Shed", UIDDomain: "test.example", Now: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)})
	if err != nil || skipped != 1 {
		t.Fatalf("ics: %v, %d skipped", err, skipped)
	}
	files["shoot.ics"] = ics.Bytes()
	for name, data := range files {
		golden := filepath.Join("testdata", name)
		if *update {
			if err := os.WriteFile(golden, data, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Errorf("%s:\n%s\nwant\n%s", name, data, want)
		}
	}
}

// The .ics keeps RFC 5545's basics: CRLF, lines of at most 75 octets
// folded with a space and never inside a character, the required
// properties, escaped text, whole days as dates.
func TestICSValid(t *testing.T) {
	r := shoot(t)
	s := shootSchedule()
	s.Days[0].Notes = strings.Repeat("Kovač house, ", 12) + "back door; parking\nbehind"
	var b bytes.Buffer
	if _, err := WriteICS(&b, r, s, ICSOptions{Title: "The Shed", Now: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	raw := b.String()
	if !strings.HasSuffix(raw, "\r\n") || strings.Contains(strings.ReplaceAll(raw, "\r\n", ""), "\n") {
		t.Fatal("not CRLF throughout")
	}
	physical := strings.Split(strings.TrimSuffix(raw, "\r\n"), "\r\n")
	for _, l := range physical {
		if len(l) > 75 {
			t.Errorf("line of %d octets: %q", len(l), l)
		}
		if !utf8Valid(l) {
			t.Errorf("a fold inside a character: %q", l)
		}
	}
	// unfolded
	var lines []string
	for _, l := range physical {
		if strings.HasPrefix(l, " ") {
			lines[len(lines)-1] += l[1:]
			continue
		}
		lines = append(lines, l)
	}
	if lines[0] != "BEGIN:VCALENDAR" || lines[len(lines)-1] != "END:VCALENDAR" {
		t.Errorf("calendar %q … %q", lines[0], lines[len(lines)-1])
	}
	has := func(want string) bool {
		for _, l := range lines {
			if l == want {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"VERSION:2.0", "PRODID:-//LaPingvino//Lexington schedule//EN", "DTSTART:20261201T070000",
		"DTEND:20261201T190000", "DTSTART;VALUE=DATE:20261202", "DTEND;VALUE=DATE:20261203", "DTSTART:20261204T063000",
		"DTSTAMP:20261120T120000Z", "SUMMARY:The Shed: shoot day 1 of 4",
		"LOCATION:" + strings.Repeat(`Kovač house\, `, 12) + `back door\; parking\nbehind`} {
		if !has(want) {
			t.Errorf("no %q in\n%s", want, strings.Join(lines, "\n"))
		}
	}
	events, uids := 0, map[string]bool{}
	var inEvent []string
	for _, l := range lines {
		switch {
		case l == "BEGIN:VEVENT":
			events++
			inEvent = nil
		case l == "END:VEVENT":
			for _, p := range []string{"UID:", "DTSTAMP:", "DTSTART"} {
				found := false
				for _, x := range inEvent {
					found = found || strings.HasPrefix(x, p)
				}
				if !found {
					t.Errorf("an event without %s", p)
				}
			}
		case strings.HasPrefix(l, "UID:"):
			uids[l] = true
		}
		inEvent = append(inEvent, l)
	}
	if events != 3 || len(uids) != 3 { // the undated day is left out
		t.Errorf("%d events, %d uids", events, len(uids))
	}
	if f := foldICS("X:" + strings.Repeat("č", 60)); strings.Contains(f, "\xc4\r\n") {
		t.Errorf("folded inside č: %q", f)
	}
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "\uFFFD") == s }

func TestSchedulePDFs(t *testing.T) {
	r := shoot(t)
	b, s := shootBreakdown(), shootSchedule()
	for name, write := range map[string]func(*bytes.Buffer) error{
		"oneliner": func(w *bytes.Buffer) error { return WriteOneLinerPDF(w, "The Shed", r, b, s, StripOptions{}, "letter") },
		"dood":     func(w *bytes.Buffer) error { return WriteDOODPDF(w, "The Shed", r, b, s, "a4") },
		"empty":    func(w *bytes.Buffer) error { return WriteDOODPDF(w, "The Shed", r, b, Schedule{}, "") },
	} {
		var buf bytes.Buffer
		if err := write(&buf); err != nil || !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) {
			t.Errorf("%s: %v", name, err)
		}
		if dir := os.Getenv("SCHEDULE_SHOT"); dir != "" {
			_ = os.WriteFile(filepath.Join(dir, name+".pdf"), buf.Bytes(), 0o644)
		}
	}
	// many days go over pages across
	var many Schedule
	for i := 0; i < 30; i++ {
		many.Days = append(many.Days, Day{Scenes: []int{1 + i%5}})
	}
	var buf bytes.Buffer
	if err := WriteDOODPDF(&buf, "x", r, b, many, ""); err != nil || len(regexp.MustCompile(`/Type /Page[^s]`).FindAll(buf.Bytes(), -1)) < 2 {
		t.Errorf("30 days: %v, one page", err)
	}
}
