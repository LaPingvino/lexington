package sqlite

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testTables() []TableDef {
	var many [][]Value
	for i := 0; i < 3000; i++ {
		many = append(many, []Value{nil, fmt.Sprintf("name %05d é", 3000-i), int64(i * 1000003)})
	}
	big := strings.Repeat("Ünïcode text, long enough for overflow pages. ", 5000)
	return []TableDef{
		{Name: "vars", SQL: "CREATE TABLE vars ( variable TEXT PRIMARY KEY ON CONFLICT REPLACE, value TEXT NOT NULL )",
			Rows: [][]Value{{"b", "2"}, {"a", "1"}}, Unique: [][]string{{"variable"}}},
		{Name: "many", SQL: "CREATE TABLE many (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE NOT NULL, n INTEGER)",
			Rows: many, Unique: nil},
		{Name: "big", SQL: "CREATE TABLE big (id INTEGER PRIMARY KEY AUTOINCREMENT, text TEXT NOT NULL, f REAL, b BLOB)",
			Rows: [][]Value{{int64(7), big, 1.5, []byte{0, 1, 2}}, {nil, "", nil, nil}}},
	}
}

func TestWriteRead(t *testing.T) {
	data, err := Write(testTables())
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Rows("many")
	if err != nil || len(rows) != 3000 {
		t.Fatalf("%d rows of many, %v", len(rows), err)
	}
	if rows[2999]["name"] != "name 00001 é" || rows[2999]["id"] != int64(3000) || rows[10]["n"] != int64(10*1000003) {
		t.Errorf("row: %v %v", rows[2999], rows[10])
	}
	bigs, err := db.Rows("big")
	if err != nil || len(bigs) != 2 {
		t.Fatalf("%d rows of big, %v", len(bigs), err)
	}
	if bigs[0]["id"] != int64(7) || bigs[1]["id"] != int64(8) || bigs[0]["text"] != testTables()[2].Rows[0][1] || bigs[0]["f"] != 1.5 {
		t.Errorf("big: id %v, %v; text of %d", bigs[0]["id"], bigs[1]["id"], len(str(bigs[0]["text"])))
	}
	vars, _ := db.Rows("vars")
	if len(vars) != 2 || vars[1]["variable"] != "a" {
		t.Errorf("vars: %v", vars)
	}
}

// TestSQLite: the sqlite3 program finds the database sound, its indexes
// agreeing with their tables (when it is installed).
func TestSQLite(t *testing.T) {
	sqlite3, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("no sqlite3")
	}
	tables := testTables()
	tables[1].Unique = [][]string{{"name"}}
	tables[1].Rows = tables[1].Rows[:40] // one page of index
	data, err := Write(tables)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "t.db")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(sqlite3, p, "PRAGMA integrity_check; SELECT count(*) FROM many; SELECT seq FROM sqlite_sequence WHERE name='big'; SELECT value FROM vars WHERE variable='a'; SELECT length(text) FROM big WHERE id=7;").CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	want := fmt.Sprintf("ok\n40\n8\n1\n%d\n", len([]rune(str(tables[2].Rows[0][1]))))
	if string(out) != want {
		t.Errorf("sqlite3 says\n%s\nnot\n%s", out, want)
	}
	// and what it writes, read here
	if out, err := exec.Command(sqlite3, p, "INSERT INTO vars VALUES ('a', 'again'); INSERT INTO many (name) VALUES ('new');").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	data, _ = os.ReadFile(p)
	db, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	vars, _ := db.Rows("vars")
	many, _ := db.Rows("many")
	if len(vars) != 2 || len(many) != 41 {
		t.Errorf("after sqlite3's writes: %d vars, %d many", len(vars), len(many))
	}
}
