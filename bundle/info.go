package bundle

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
)

// info.json is edited as text: its members are found and only the value
// asked for is replaced (or added, or removed), so the other keys keep
// their order and their bytes (TextBundle's own samples have a trailing
// comma, which encoding/json refuses).

type jsonMember struct {
	key                        string
	keyStart, valStart, valEnd int
}

// jsonObjectMembers are a JSON object's members, and where it closes.
func jsonObjectMembers(b []byte) (ms []jsonMember, open, close int, ok bool) {
	i := skipJSONSpace(b, 0)
	if i >= len(b) || b[i] != '{' {
		return nil, 0, 0, false
	}
	open = i
	i++
	for {
		i = skipJSONSpace(b, i)
		if i >= len(b) {
			return nil, 0, 0, false
		}
		switch b[i] {
		case '}':
			return ms, open, i, true
		case ',':
			i++
			continue
		case '"':
		default:
			return nil, 0, 0, false
		}
		ke := scanJSONString(b, i)
		if ke < 0 {
			return nil, 0, 0, false
		}
		var key string
		if json.Unmarshal(b[i:ke], &key) != nil {
			return nil, 0, 0, false
		}
		j := skipJSONSpace(b, ke)
		if j >= len(b) || b[j] != ':' {
			return nil, 0, 0, false
		}
		vs := skipJSONSpace(b, j+1)
		ve := scanJSONValue(b, vs)
		if ve < 0 {
			return nil, 0, 0, false
		}
		ms = append(ms, jsonMember{key, i, vs, ve})
		i = ve
	}
}

func skipJSONSpace(b []byte, i int) int {
	for i < len(b) && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i++
	}
	return i
}

// scanJSONString is the end of the string starting at i, -1 if none.
func scanJSONString(b []byte, i int) int {
	for j := i + 1; j < len(b); j++ {
		switch b[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return -1
}

// scanJSONValue is the end of the value starting at i, -1 if none.
func scanJSONValue(b []byte, i int) int {
	if i >= len(b) {
		return -1
	}
	switch b[i] {
	case '"':
		return scanJSONString(b, i)
	case '{', '[':
		depth := 0
		for j := i; j < len(b); j++ {
			switch b[j] {
			case '"':
				if j = scanJSONString(b, j); j < 0 {
					return -1
				}
				j--
			case '{', '[':
				depth++
			case '}', ']':
				if depth--; depth == 0 {
					return j + 1
				}
			}
		}
		return -1
	}
	j := i
	for j < len(b) && !strings.ContainsRune(",}] \t\r\n", rune(b[j])) {
		j++
	}
	if j == i {
		return -1
	}
	return j
}

// InfoValue is a top-level key's value in info.json, as it is written.
func InfoValue(info []byte, key string) ([]byte, bool) {
	ms, _, _, ok := jsonObjectMembers(info)
	if !ok {
		return nil, false
	}
	for _, m := range ms {
		if m.key == key {
			return info[m.valStart:m.valEnd], true
		}
	}
	return nil, false
}

// SetInfoValue is info.json with key's value val (nil: the key removed),
// all else as it was; ok is false if info.json cannot be read.
func SetInfoValue(info []byte, key string, val []byte) ([]byte, bool) {
	ms, open, close, ok := jsonObjectMembers(info)
	if !ok {
		return info, false
	}
	splice := func(from, to int, with string) []byte {
		return slices.Concat(info[:from], []byte(with), info[to:])
	}
	for i, m := range ms {
		if m.key != key {
			continue
		}
		if val != nil {
			return splice(m.valStart, m.valEnd, string(val)), true
		}
		if i > 0 { // from the end of the one before
			return splice(ms[i-1].valEnd, m.valEnd, ""), true
		}
		if i+1 < len(ms) {
			return splice(m.keyStart, ms[i+1].keyStart, ""), true
		}
		return splice(open+1, close, "\n"), true
	}
	if val == nil {
		return info, true
	}
	k, _ := json.Marshal(key)
	member := string(k) + ": " + string(val)
	if len(ms) == 0 {
		return splice(open+1, close, "\n  "+member+"\n"), true
	}
	// laid out as the first member is
	sep := " "
	if ws := info[open+1 : ms[0].keyStart]; bytes.ContainsRune(ws, '\n') {
		sep = "\n" + string(ws[bytes.LastIndexByte(ws, '\n')+1:])
	}
	last := ms[len(ms)-1].valEnd
	if j := skipJSONSpace(info, last); j < close && info[j] == ',' { // a trailing comma
		return splice(j+1, j+1, sep+member+","), true
	}
	return splice(last, last, ","+sep+member), true
}

// InfoString is a top-level key's value in info.json when it is a string.
func InfoString(info []byte, key string) (string, bool) {
	raw, ok := InfoValue(info, key)
	if !ok {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

// InfoReadable tells whether info.json can be read as an object (a
// trailing comma allowed), and whether it is strict JSON too.
func InfoReadable(info []byte) (readable, strict bool) {
	_, _, _, ok := jsonObjectMembers(info)
	return ok, ok && json.Valid(info)
}

func jsonString(s string) (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}
