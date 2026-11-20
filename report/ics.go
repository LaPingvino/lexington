package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The shoot days as iCalendar (RFC 5545), an event a day, for any
// calendar: the summary "The Shed: shoot day 2 of 3", the location the
// day's notes, the description the call and the scenes in shooting order
// with their lengths, and the day's total.
//
// A day with a call time is an event from the call for ShootDayHours (a
// floating local time: the call is where the unit is, whatever the
// calendar's zone); one without is a whole-day event (VALUE=DATE, the
// end the day after). A day without a date is left out.

// ShootDayHours is how long a shoot day's event lasts from its call:
// twelve hours, the usual day (a choice; the call sheet has the wrap).
const ShootDayHours = 12

// ICSOptions are a calendar's own lines.
type ICSOptions struct {
	// Title is the script's title, before each event's summary.
	Title string
	// UIDDomain ends each event's UID: "<day id>@<domain>".
	UIDDomain string
	// Now is the calendar's DTSTAMP (time.Now if zero).
	Now time.Time
}

// WriteICS writes the schedule's dated days as an iCalendar file, and
// tells how many days it left out for want of a date.
func WriteICS(w io.Writer, r Report, s Schedule, o ICSOptions) (skipped int, err error) {
	now := o.Now
	if now.IsZero() {
		now = time.Now()
	}
	domain := o.UIDDomain
	if domain == "" {
		domain = "lexington"
	}
	var b strings.Builder
	line := func(name, value string) { b.WriteString(foldICS(name + ":" + value)) }
	line("BEGIN", "VCALENDAR")
	line("VERSION", "2.0")
	line("PRODID", "-//LaPingvino//Lexington schedule//EN")
	line("CALSCALE", "GREGORIAN")
	line("METHOD", "PUBLISH")
	if o.Title != "" {
		line("X-WR-CALNAME", escapeICS(o.Title+": shoot days"))
	}
	scenes := map[int]Scene{}
	for _, sc := range r.Scenes {
		scenes[sc.Order] = sc
	}
	for i, d := range s.Days {
		if d.Date.IsZero() {
			skipped++
			continue
		}
		id := d.ID
		if id == "" {
			id = "day-" + strconv.Itoa(i+1)
		}
		line("BEGIN", "VEVENT")
		line("UID", escapeICS(id+"@"+domain))
		line("DTSTAMP", now.UTC().Format("20060102T150405Z"))
		date := time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), 0, 0, 0, 0, time.UTC)
		if h, m, ok := ParseCall(d.Call); ok {
			start := date.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
			line("DTSTART", start.Format("20060102T150405"))
			line("DTEND", start.Add(ShootDayHours*time.Hour).Format("20060102T150405"))
		} else {
			line("DTSTART;VALUE=DATE", date.Format("20060102"))
			line("DTEND;VALUE=DATE", date.AddDate(0, 0, 1).Format("20060102"))
		}
		summary := fmt.Sprintf("Shoot day %d of %d", i+1, len(s.Days))
		if o.Title != "" {
			summary = o.Title + ": shoot day " + strconv.Itoa(i+1) + " of " + strconv.Itoa(len(s.Days))
		}
		line("SUMMARY", escapeICS(summary))
		if d.Notes != "" {
			line("LOCATION", escapeICS(d.Notes))
		}
		var desc []string
		if d.Call != "" {
			desc = append(desc, "Call "+d.Call)
		}
		if len(d.Scenes) > 0 {
			desc = append(desc, "Scenes:")
		}
		for _, ord := range d.Scenes {
			sc, ok := scenes[ord]
			if !ok {
				continue
			}
			desc = append(desc, fmt.Sprintf("%s %s (%s)", sc.ID(), sc.Heading, sc.Eighths))
		}
		desc = append(desc, EndOfDay(i, r.DayLength(d)))
		line("DESCRIPTION", escapeICS(strings.Join(desc, "\n")))
		line("END", "VEVENT")
	}
	line("END", "VCALENDAR")
	_, err = io.WriteString(w, b.String())
	return skipped, err
}

// ParseCall reads a call time, "7:00" or "07:30" (24 hours).
func ParseCall(s string) (h, m int, ok bool) {
	hs, ms, found := strings.Cut(strings.TrimSpace(s), ":")
	if !found || len(ms) != 2 || len(hs) < 1 || len(hs) > 2 {
		return 0, 0, false
	}
	h, err1 := strconv.Atoi(hs)
	m, err2 := strconv.Atoi(ms)
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// escapeICS escapes a TEXT value (RFC 5545 3.3.11): backslash, semicolon,
// comma, and line ends as \n.
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`, "\r", `\n`).Replace(s)
}

// foldICS is a content line ended with CRLF and folded (RFC 5545 3.1): no
// line longer than 75 octets, a fold never inside a UTF-8 character, each
// continuation starting with a space.
func foldICS(l string) string {
	var b strings.Builder
	limit := 75
	for len(l) > limit {
		cut := limit
		for cut > 0 && !utf8.RuneStart(l[cut]) {
			cut--
		}
		b.WriteString(l[:cut])
		b.WriteString("\r\n ")
		l = l[cut:]
		limit = 74 // the space counts
	}
	b.WriteString(l)
	b.WriteString("\r\n")
	return b.String()
}
