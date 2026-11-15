package rules

import (
	"fmt"
	"strings"
)

// KeyPagination is the switch of how the paginated formats (pdf, docx,
// odt, html when printed) break pages: a rule whose Style names what is
// on (ParsePagination). A set without it has all of it on; Hide turns
// it all off (each page then simply fills to its foot).
const KeyPagination ConfigKey = "pagination"

// Pagination is what keeps a page break from falling badly, as screenplay
// software does it.
type Pagination struct {
	// Keep: a scene heading, a printed section, a character name and a
	// parenthetical are never the last thing on a page, but go over with
	// (the first two lines of) what follows; a transition is never the
	// first thing on a page, but goes with the line before it; dual
	// dialogue is never split.
	Keep bool
	// Widows: a paragraph broken over two pages leaves at least two of
	// its lines on each.
	Widows bool
	// More: a speech broken over two pages breaks after a sentence, with
	// (MORE) below it and the name again, NAME (CONT'D), above the rest.
	More bool
}

// AllPagination is the default: all of it on.
var AllPagination = Pagination{Keep: true, Widows: true, More: true}

// ParsePagination reads a pagination as String writes it: the parts that
// are on joined with "+" ("keep+widows+more"), "all" (or "") for all of
// them, "none" (or "off") for none.
func ParsePagination(text string) (Pagination, error) {
	var p Pagination
	switch t := strings.ToLower(strings.TrimSpace(text)); t {
	case "", "all", "on":
		return AllPagination, nil
	case "none", "off":
		return p, nil
	}
	for _, part := range strings.Split(strings.ToLower(text), "+") {
		switch strings.TrimSpace(part) {
		case "keep":
			p.Keep = true
		case "widows":
			p.Widows = true
		case "more":
			p.More = true
		default:
			return Pagination{}, fmt.Errorf("no pagination %q: give all, none or keep, widows and more joined with +", text)
		}
	}
	return p, nil
}

// String is the pagination as ParsePagination reads it.
func (p Pagination) String() string {
	var parts []string
	for _, x := range []struct {
		on   bool
		name string
	}{{p.Keep, "keep"}, {p.Widows, "widows"}, {p.More, "more"}} {
		if x.on {
			parts = append(parts, x.name)
		}
	}
	switch len(parts) {
	case 0:
		return "none"
	case 3:
		return "all"
	}
	return strings.Join(parts, "+")
}

// Pagination is how the set breaks pages: all of it (AllPagination)
// without a pagination rule, none with a hidden one, otherwise what its
// Style names (all if it names nothing it knows).
func (s Set) Pagination() Pagination {
	f, ok := s[KeyPagination.String()]
	switch {
	case !ok:
		return AllPagination
	case f.Hide:
		return Pagination{}
	}
	p, err := ParsePagination(f.Style)
	if err != nil {
		return AllPagination
	}
	return p
}

// WithPagination is a copy of the set that breaks pages so.
func (s Set) WithPagination(p Pagination) Set {
	out := Set{}
	for k, f := range s {
		out[k] = f
	}
	out[KeyPagination.String()] = Format{Style: p.String()}
	return out
}
