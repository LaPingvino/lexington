package sqlite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// TableDef is a table to write: its CREATE TABLE statement and rows, each
// its columns' values in order (an INTEGER PRIMARY KEY column's value is
// the rowid: an int64, or nil for the next one). Unique are the columns
// of its UNIQUE and non-integer PRIMARY KEY constraints, in the order the
// statement has them: each gets its index (sqlite_autoindex_Name_1…), as
// SQLite makes them.
type TableDef struct {
	Name   string
	SQL    string
	Rows   [][]Value
	Unique [][]string
}

// pageSize is the size of the pages written.
const pageSize = 4096

// writer builds a database's pages.
type writer struct {
	pages map[int][]byte
	next  int
}

func (w *writer) alloc() int {
	w.next++
	return w.next
}

// Write is a new database (UTF-8, pages of 4096 bytes) with the tables,
// in order; a table with AUTOINCREMENT makes sqlite_sequence, with its
// largest rowid.
func Write(tables []TableDef) ([]byte, error) {
	w := &writer{pages: map[int][]byte{}, next: 1} // page 1 is the schema's
	type index struct {
		name, table string
		cols        []int
		root        int
	}
	var seq [][]Value
	var schema [][]Value // sqlite_master's rows
	type planned struct {
		def     TableDef
		root    int
		alias   int // the rowid's column, -1 none
		indexes []index
	}
	var plan []planned
	hasSeq := false
	for _, t := range tables {
		if strings.Contains(strings.ToUpper(t.SQL), "AUTOINCREMENT") {
			hasSeq = true
		}
	}
	for _, t := range tables {
		cols := Columns(t.SQL)
		p := planned{def: t, root: w.alloc(), alias: -1}
		if a := rowidAlias(t.SQL); a != "" {
			for i, c := range cols {
				if strings.EqualFold(c, a) {
					p.alias = i
				}
			}
		}
		schema = append(schema, []Value{"table", t.Name, t.Name, int64(p.root), t.SQL})
		for k, u := range t.Unique {
			ix := index{name: fmt.Sprintf("sqlite_autoindex_%s_%d", t.Name, k+1), table: t.Name, root: w.alloc()}
			for _, c := range u {
				found := false
				for i, col := range cols {
					if strings.EqualFold(col, c) {
						ix.cols, found = append(ix.cols, i), true
					}
				}
				if !found {
					return nil, fmt.Errorf("no column %s in %s", c, t.Name)
				}
			}
			p.indexes = append(p.indexes, ix)
			schema = append(schema, []Value{"index", ix.name, t.Name, int64(ix.root), nil})
		}
		plan = append(plan, p)
	}
	seqRoot := 0
	if hasSeq {
		seqRoot = w.alloc()
		schema = append(schema, []Value{"table", "sqlite_sequence", "sqlite_sequence", int64(seqRoot), "CREATE TABLE sqlite_sequence(name,seq)"})
	}
	for _, p := range plan {
		var rows []tableRow
		last := int64(0)
		for _, r := range p.def.Rows {
			rec := append([]Value(nil), r...)
			id := last + 1
			if p.alias >= 0 && p.alias < len(rec) {
				if v, ok := rec[p.alias].(int64); ok {
					id = v
				}
				rec[p.alias] = nil
			}
			last = max(last, id)
			rows = append(rows, tableRow{id, rec})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].rowid < rows[j].rowid })
		if err := w.table(p.root, rows); err != nil {
			return nil, err
		}
		for _, ix := range p.indexes {
			if err := w.index(ix.root, rows, ix.cols); err != nil {
				return nil, err
			}
		}
		if hasSeq && strings.Contains(strings.ToUpper(p.def.SQL), "AUTOINCREMENT") && len(rows) > 0 {
			seq = append(seq, []Value{p.def.Name, last})
		}
	}
	if hasSeq {
		var rows []tableRow
		for i, r := range seq {
			rows = append(rows, tableRow{int64(i + 1), r})
		}
		if err := w.table(seqRoot, rows); err != nil {
			return nil, err
		}
	}
	var rows []tableRow
	for i, r := range schema {
		rows = append(rows, tableRow{int64(i + 1), r})
	}
	if err := w.table(1, rows); err != nil {
		return nil, err
	}
	out := make([]byte, 0, w.next*pageSize)
	for n := 1; n <= w.next; n++ {
		p := w.pages[n]
		if p == nil {
			p = make([]byte, pageSize)
		}
		out = append(out, p...)
	}
	header(out[:100], w.next)
	return out, nil
}

// header fills page 1's file header.
func header(h []byte, pages int) {
	copy(h, Magic)
	binary.BigEndian.PutUint16(h[16:], pageSize)
	h[18], h[19] = 1, 1 // the rollback journal's file format
	h[21], h[22], h[23] = 64, 32, 32
	binary.BigEndian.PutUint32(h[24:], 1) // change counter
	binary.BigEndian.PutUint32(h[28:], uint32(pages))
	binary.BigEndian.PutUint32(h[40:], 1) // schema cookie
	binary.BigEndian.PutUint32(h[44:], 4) // schema format
	binary.BigEndian.PutUint32(h[56:], 1) // UTF-8
	binary.BigEndian.PutUint32(h[92:], 1) // valid for change 1
	binary.BigEndian.PutUint32(h[96:], 3045000)
}

type tableRow struct {
	rowid int64
	rec   []Value
}

// cell is a b-tree cell and, for interior pages, the key it ends with.
type cell struct {
	data  []byte
	rowid int64
}

// table writes a table b-tree with root at page root.
func (w *writer) table(root int, rows []tableRow) error {
	var cells []cell
	usable := pageSize
	for _, r := range rows {
		payload := record(r.rec)
		var c []byte
		c = appendVarint(c, uint64(len(payload)))
		c = appendVarint(c, uint64(r.rowid))
		local := localSize(usable, len(payload))
		c = append(c, payload[:local]...)
		if local < len(payload) {
			c = binary.BigEndian.AppendUint32(c, uint32(w.overflow(payload[local:])))
		}
		cells = append(cells, cell{c, r.rowid})
	}
	return w.tree(root, cells, 0x0d)
}

// overflow writes a payload's rest on overflow pages: the first's number.
func (w *writer) overflow(rest []byte) int {
	first := 0
	var prev []byte
	for len(rest) > 0 {
		n := w.alloc()
		p := make([]byte, pageSize)
		k := copy(p[4:], rest)
		rest = rest[k:]
		w.pages[n] = p
		if prev != nil {
			binary.BigEndian.PutUint32(prev, uint32(n))
		} else {
			first = n
		}
		prev = p
	}
	return first
}

// tree writes cells as a b-tree of kind (0x0d table leaves) with its root
// at page root: leaves as full as they go, interior pages over them.
func (w *writer) tree(root int, cells []cell, kind byte) error {
	headerAt := func(n int) int {
		if n == 1 {
			return 100
		}
		return 0
	}
	// the leaves' groups: the root's page holds less on page 1
	fits := func(group []cell, at, hdr int) bool {
		size := at + hdr + 2*len(group)
		for _, c := range group {
			size += len(c.data)
		}
		return size <= pageSize
	}
	var groups [][]cell
	for _, c := range cells {
		if len(c.data)+8+2+100 > pageSize {
			return errors.New("a cell too big for a page")
		}
		if n := len(groups); n > 0 && fits(append(groups[n-1][:len(groups[n-1]):len(groups[n-1])], c), 0, 8) {
			groups[n-1] = append(groups[n-1], c)
			continue
		}
		groups = append(groups, []cell{c})
	}
	if len(groups) <= 1 && fits(cells, headerAt(root), 8) {
		w.pages[root] = page(cells, kind, headerAt(root), 0)
		return nil
	}
	type child struct {
		page  int
		rowid int64
	}
	var children []child
	for _, g := range groups {
		n := w.alloc()
		w.pages[n] = page(g, kind, 0, 0)
		children = append(children, child{n, g[len(g)-1].rowid})
	}
	for {
		// interior cells: a child and its largest rowid; the last child
		// the rightmost pointer
		var level []cell
		for _, c := range children {
			d := binary.BigEndian.AppendUint32(nil, uint32(c.page))
			level = append(level, cell{appendVarint(d, uint64(c.rowid)), c.rowid})
		}
		if fits(level[:len(level)-1], headerAt(root), 12) {
			w.pages[root] = page(level[:len(level)-1], 0x05, headerAt(root), children[len(children)-1].page)
			return nil
		}
		var next []child
		for len(level) > 0 {
			k := 1
			for k < len(level) && fits(level[:k+1], 0, 12) {
				k++
			}
			// k cells: k-1 in the page, the k-th its rightmost pointer
			n := w.alloc()
			w.pages[n] = page(level[:k-1], 0x05, 0, children[k-1].page)
			next = append(next, child{n, children[k-1].rowid})
			level, children = level[k:], children[k:]
		}
		children = next
	}
}

// page lays out a b-tree page: its header at at, the cell pointers after
// it, the cells from the end.
func page(cells []cell, kind byte, at int, right int) []byte {
	p := make([]byte, pageSize)
	p[at] = kind
	binary.BigEndian.PutUint16(p[at+3:], uint16(len(cells)))
	ptrs := at + 8
	if kind == 0x05 || kind == 0x02 {
		binary.BigEndian.PutUint32(p[at+8:], uint32(right))
		ptrs = at + 12
	}
	end := pageSize
	for i, c := range cells {
		end -= len(c.data)
		copy(p[end:], c.data)
		binary.BigEndian.PutUint16(p[ptrs+2*i:], uint16(end))
	}
	binary.BigEndian.PutUint16(p[at+5:], uint16(end))
	return p
}

// index writes an index b-tree of one leaf: the columns' values and the
// rowid, sorted as SQLite's BINARY collation sorts them.
func (w *writer) index(root int, rows []tableRow, cols []int) error {
	type entry struct {
		key []Value // the columns' values and the rowid
		rec []byte
	}
	var es []entry
	for _, r := range rows {
		var key []Value
		for _, c := range cols {
			key = append(key, r.rec[c])
		}
		key = append(key, r.rowid)
		es = append(es, entry{key, record(key)})
	}
	sort.SliceStable(es, func(i, j int) bool { return compare(es[i].key, es[j].key) < 0 })
	var cells []cell
	maxLocal := (pageSize-12)*64/255 - 23
	for _, e := range es {
		if len(e.rec) > maxLocal {
			return errors.New("an index key too long")
		}
		cells = append(cells, cell{data: append(appendVarint(nil, uint64(len(e.rec))), e.rec...)})
	}
	size := 8
	for _, c := range cells {
		size += 2 + len(c.data)
	}
	if size > pageSize {
		return errors.New("an index too big for one page")
	}
	w.pages[root] = page(cells, 0x0a, 0, 0)
	return nil
}

// compare orders values as SQLite does: NULL, numbers, text (by its
// bytes), blobs.
func compare(a, b []Value) int {
	rank := func(v Value) int {
		switch v.(type) {
		case nil:
			return 0
		case int64, float64:
			return 1
		case string:
			return 2
		}
		return 3
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		ra, rb := rank(a[i]), rank(b[i])
		if ra != rb {
			return ra - rb
		}
		switch x := a[i].(type) {
		case int64:
			y := b[i].(int64)
			if x != y {
				if x < y {
					return -1
				}
				return 1
			}
		case float64:
			if y := b[i].(float64); x != y {
				if x < y {
					return -1
				}
				return 1
			}
		case string:
			if c := strings.Compare(x, b[i].(string)); c != 0 {
				return c
			}
		case []byte:
			if c := bytes.Compare(x, b[i].([]byte)); c != 0 {
				return c
			}
		}
	}
	return len(a) - len(b)
}

// record encodes values as a record.
func record(vals []Value) []byte {
	var types []uint64
	var body []byte
	for _, v := range vals {
		switch v := v.(type) {
		case nil:
			types = append(types, 0)
		case int64:
			switch {
			case v == 0:
				types = append(types, 8)
			case v == 1:
				types = append(types, 9)
			case v >= math.MinInt8 && v <= math.MaxInt8:
				types, body = append(types, 1), append(body, byte(v))
			case v >= math.MinInt16 && v <= math.MaxInt16:
				types, body = append(types, 2), binary.BigEndian.AppendUint16(body, uint16(v))
			case v >= math.MinInt32 && v <= math.MaxInt32:
				types, body = append(types, 4), binary.BigEndian.AppendUint32(body, uint32(v))
			default:
				types, body = append(types, 6), binary.BigEndian.AppendUint64(body, uint64(v))
			}
		case float64:
			types, body = append(types, 7), binary.BigEndian.AppendUint64(body, math.Float64bits(v))
		case string:
			types, body = append(types, uint64(13+2*len(v))), append(body, v...)
		case []byte:
			types, body = append(types, uint64(12+2*len(v))), append(body, v...)
		default:
			panic(fmt.Sprintf("sqlite: no record value %T", v))
		}
	}
	var hdr []byte
	for _, t := range types {
		hdr = appendVarint(hdr, t)
	}
	// the header's size counts itself
	size := len(hdr) + 1
	if size > 127 {
		size = len(hdr) + 2
	}
	out := appendVarint(nil, uint64(size))
	return append(append(out, hdr...), body...)
}

// appendVarint appends SQLite's variable-length integer.
func appendVarint(b []byte, v uint64) []byte {
	if v > 0x00ffffffffffffff {
		var tmp [9]byte
		tmp[8] = byte(v)
		v >>= 8
		for i := 7; i >= 0; i-- {
			tmp[i] = byte(v&0x7f) | 0x80
			v >>= 7
		}
		return append(b, tmp[:]...)
	}
	var tmp [9]byte
	n := 0
	for {
		tmp[n] = byte(v & 0x7f)
		n++
		v >>= 7
		if v == 0 {
			break
		}
	}
	for i := n - 1; i >= 0; i-- {
		c := tmp[i]
		if i > 0 {
			c |= 0x80
		}
		b = append(b, c)
	}
	return b
}
