// Package sqlite reads and writes SQLite database files, as far as the
// formats Lexington reads need it (KIT Scenarist's .kitsp): a table's rows
// read, and a new database written whole, its tables' and their UNIQUE
// indexes' b-trees built. It is SQLite's file format
// (sqlite.org/fileformat2.html) and nothing of its engine: no SQL is run,
// no journal is read, and what a database file holds is read as it is on
// disk.
package sqlite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
)

// Magic is how a SQLite database file begins.
const Magic = "SQLite format 3\x00"

// DB is a database file as read.
type DB struct {
	data     []byte
	pageSize int
	usable   int
	encoding int // 1 UTF-8, 2 UTF-16le, 3 UTF-16be
}

// Value is a column's value: nil, int64, float64, string or []byte.
type Value = any

// Table is a table of the schema.
type Table struct {
	Name     string
	RootPage int
	SQL      string
	Columns  []string
}

// Open reads a database file's header.
func Open(data []byte) (*DB, error) {
	if len(data) < 100 || !bytes.HasPrefix(data, []byte(Magic)) {
		return nil, errors.New("not a SQLite database")
	}
	ps := int(binary.BigEndian.Uint16(data[16:18]))
	if ps == 1 {
		ps = 65536
	}
	if ps < 512 || ps&(ps-1) != 0 {
		return nil, fmt.Errorf("a SQLite database with pages of %d bytes", ps)
	}
	db := &DB{data: data, pageSize: ps, usable: ps - int(data[20]), encoding: int(binary.BigEndian.Uint32(data[56:60]))}
	if db.encoding == 0 {
		db.encoding = 1
	}
	return db, nil
}

func (db *DB) page(n int) ([]byte, error) {
	start := (n - 1) * db.pageSize
	if n < 1 || start+db.pageSize > len(db.data) {
		return nil, fmt.Errorf("page %d is not in the file", n)
	}
	return db.data[start : start+db.pageSize], nil
}

// Tables are the schema's tables.
func (db *DB) Tables() ([]Table, error) {
	var out []Table
	err := db.walk(1, func(rowid int64, rec []Value) error {
		if len(rec) < 5 || str(rec[0]) != "table" {
			return nil
		}
		root, _ := rec[3].(int64)
		out = append(out, Table{Name: str(rec[1]), RootPage: int(root), SQL: str(rec[4]), Columns: Columns(str(rec[4]))})
		return nil
	})
	return out, err
}

func str(v Value) string {
	switch v := v.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return ""
}

// Rows are a table's rows, each a map of its columns' values; an
// INTEGER PRIMARY KEY column is the rowid.
func (db *DB) Rows(table string) ([]map[string]Value, error) {
	tables, err := db.Tables()
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		if !strings.EqualFold(t.Name, table) {
			continue
		}
		var out []map[string]Value
		alias := rowidAlias(t.SQL)
		err := db.walk(t.RootPage, func(rowid int64, rec []Value) error {
			row := map[string]Value{}
			for i, c := range t.Columns {
				if i < len(rec) {
					row[c] = rec[i]
				} else {
					row[c] = nil // added later (ALTER TABLE): its default, not kept here
				}
			}
			if alias != "" {
				row[alias] = rowid
			}
			out = append(out, row)
			return nil
		})
		return out, err
	}
	return nil, fmt.Errorf("no table %s", table)
}

// walk calls f on every row of the table b-tree at root, in rowid order.
func (db *DB) walk(root int, f func(rowid int64, rec []Value) error) error {
	seen := map[int]bool{}
	var visit func(n int) error
	visit = func(n int) error {
		if seen[n] {
			return fmt.Errorf("page %d twice in a b-tree", n)
		}
		seen[n] = true
		p, err := db.page(n)
		if err != nil {
			return err
		}
		h := 0
		if n == 1 {
			h = 100
		}
		kind := p[h]
		count := int(binary.BigEndian.Uint16(p[h+3:]))
		ptrs := h + 8
		if kind == 0x05 {
			ptrs = h + 12
		}
		for i := 0; i < count; i++ {
			off := int(binary.BigEndian.Uint16(p[ptrs+2*i:]))
			if off >= len(p) {
				return fmt.Errorf("a cell out of page %d", n)
			}
			switch kind {
			case 0x05: // interior: the child, then its largest rowid
				if err := visit(int(binary.BigEndian.Uint32(p[off:]))); err != nil {
					return err
				}
			case 0x0d:
				size, k := varint(p[off:])
				rowid, k2 := varint(p[off+k:])
				payload, err := db.payload(p, off+k+k2, int(size))
				if err != nil {
					return err
				}
				rec, err := db.record(payload)
				if err != nil {
					return err
				}
				if err := f(int64(rowid), rec); err != nil {
					return err
				}
			default:
				return fmt.Errorf("page %d is no table's (%#x)", n, kind)
			}
		}
		if kind == 0x05 {
			return visit(int(binary.BigEndian.Uint32(p[h+8:])))
		}
		return nil
	}
	return visit(root)
}

// localSize is how much of a payload of size p a table leaf keeps on its
// page.
func localSize(usable, p int) int {
	x := usable - 35
	if p <= x {
		return p
	}
	m := (usable-12)*32/255 - 23
	k := m + (p-m)%(usable-4)
	if k <= x {
		return k
	}
	return m
}

// payload is a cell's payload: on its page, and on its overflow pages.
func (db *DB) payload(p []byte, at, size int) ([]byte, error) {
	local := localSize(db.usable, size)
	if at+local > len(p) {
		return nil, errors.New("a cell past its page")
	}
	out := append([]byte(nil), p[at:at+local]...)
	if local == size {
		return out, nil
	}
	next := int(binary.BigEndian.Uint32(p[at+local:]))
	for len(out) < size {
		op, err := db.page(next)
		if err != nil {
			return nil, err
		}
		n := min(size-len(out), db.usable-4)
		out = append(out, op[4:4+n]...)
		next = int(binary.BigEndian.Uint32(op))
	}
	return out, nil
}

// record decodes a record: its header's serial types, then the values.
func (db *DB) record(b []byte) ([]Value, error) {
	hsize, k := varint(b)
	if int(hsize) > len(b) {
		return nil, errors.New("a record's header past it")
	}
	var types []uint64
	for at := k; at < int(hsize); {
		t, n := varint(b[at:])
		types = append(types, t)
		at += n
	}
	at := int(hsize)
	out := make([]Value, 0, len(types))
	for _, t := range types {
		n := serialSize(t)
		if at+n > len(b) {
			return nil, errors.New("a record's value past it")
		}
		v := b[at : at+n]
		at += n
		switch {
		case t == 0:
			out = append(out, nil)
		case t >= 1 && t <= 6:
			var x int64
			for _, c := range v {
				x = x<<8 | int64(c)
			}
			if shift := 64 - 8*len(v); shift > 0 {
				x = x << shift >> shift // the sign
			}
			out = append(out, x)
		case t == 7:
			out = append(out, math.Float64frombits(binary.BigEndian.Uint64(v)))
		case t == 8:
			out = append(out, int64(0))
		case t == 9:
			out = append(out, int64(1))
		case t >= 12 && t%2 == 0:
			out = append(out, append([]byte(nil), v...))
		case t >= 13:
			out = append(out, db.text(v))
		default:
			out = append(out, nil)
		}
	}
	return out, nil
}

func (db *DB) text(v []byte) string {
	if db.encoding == 1 {
		return string(v)
	}
	u := make([]uint16, len(v)/2)
	for i := range u {
		if db.encoding == 2 {
			u[i] = binary.LittleEndian.Uint16(v[2*i:])
		} else {
			u[i] = binary.BigEndian.Uint16(v[2*i:])
		}
	}
	return string(utf16.Decode(u))
}

func serialSize(t uint64) int {
	switch {
	case t <= 4:
		return []int{0, 1, 2, 3, 4}[t]
	case t == 5:
		return 6
	case t == 6 || t == 7:
		return 8
	case t >= 12:
		return int((t - 12) / 2)
	}
	return 0
}

// varint reads SQLite's variable-length integer: its value and length.
func varint(b []byte) (uint64, int) {
	var v uint64
	for i := 0; i < 9 && i < len(b); i++ {
		if i == 8 {
			return v<<8 | uint64(b[i]), 9
		}
		v = v<<7 | uint64(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return v, i + 1
		}
	}
	return v, len(b)
}

// Columns are the column names of a CREATE TABLE statement, in order.
func Columns(sql string) []string {
	open, end := strings.Index(sql, "("), strings.LastIndex(sql, ")")
	if open < 0 || end < open {
		return nil
	}
	var out []string
	for _, def := range splitTop(sql[open+1 : end]) {
		f := strings.Fields(def)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "PRIMARY", "UNIQUE", "CHECK", "FOREIGN", "CONSTRAINT":
			continue
		}
		out = append(out, strings.Trim(f[0], "`\"[]"))
	}
	return out
}

// splitTop splits at the commas outside parentheses.
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// rowidAlias is a table's INTEGER PRIMARY KEY column, "" for none.
func rowidAlias(sql string) string {
	open, end := strings.Index(sql, "("), strings.LastIndex(sql, ")")
	if open < 0 || end < open {
		return ""
	}
	for _, def := range splitTop(sql[open+1 : end]) {
		f := strings.Fields(strings.ToUpper(def))
		if len(f) >= 4 && f[1] == "INTEGER" && f[2] == "PRIMARY" && f[3] == "KEY" {
			return strings.Trim(strings.Fields(def)[0], "`\"[]")
		}
	}
	return ""
}
