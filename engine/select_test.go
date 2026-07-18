package engine

import (
	"slices"
	"spacedb/executor"
	"spacedb/types"
	"testing"
)

func TestTestSessionSelectAll(t *testing.T) {
	engine := NewKVEngine()
	session := NewSession(engine)

	if _, err := session.Execute(`
		CREATE TABLE users (
			id INT NOT NULL,
			name STRING DEFAULT 'guest',
			active BOOL DEFAULT true
		);
	`); err != nil {
		t.Fatal(err)
	}

	if _, err := session.Execute("INSERT INTO users VALUES (1);"); err != nil {
		t.Fatal(err)
	}

	if _, err := session.Execute("INSERT INTO users (name, id) VALUES ('alice', 2);"); err != nil {
		t.Fatal(err)
	}

	result, err := session.Execute("SELECT * FROM users;")
	if err != nil {
		t.Fatal(err)
	}
	rows, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result= %T, want executor.RowsResult", rows)
	}

	wantColumns := []string{"id", "name", "active"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf("columns = %#v, want %#v", rows.Columns, wantColumns)
	}

	if len(rows.Rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows.Rows))
	}

	first := rows.Rows[0]
	if first[0].Integer != 1 ||
		first[1].String != "guest" ||
		first[2].Kind != types.ValueBoolean ||
		!first[2].Boolean {
		t.Fatalf("first row = %#v", first)
	}

	second := rows.Rows[1]
	if second[0].Integer != 2 ||
		second[1].String != "alice" {
		t.Fatalf("second row = %#v", second)
	}
}

func TestSessionSelectMissingTable(t *testing.T) {
	session := NewSession(NewKVEngine())

	_, err := session.Execute("SELECT * FROM missing;")
	if err == nil {
		t.Fatal("expected missing-table error")
	}
}
