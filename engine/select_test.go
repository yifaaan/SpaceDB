package engine

import (
	"slices"
	"spacedb/executor"
	"spacedb/storage"
	"spacedb/types"
	"testing"
)

func TestTestSessionSelectAll(t *testing.T) {
	engine := NewKVEngine(storage.NewMemoryEngine())
	session := NewSession(engine)

	if _, err := session.Execute(`
		CREATE TABLE users (
			id INT PRIMARY KEY NOT NULL,
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
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	_, err := session.Execute("SELECT * FROM missing;")
	if err == nil {
		t.Fatal("expected missing-table error")
	}
}

func TestSessionSelectProjection(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	_, err := session.Execute(`
                CREATE TABLE users (
                        id INT PRIMARY KEY,
                        name STRING NOT NULL,
                        score INT NOT NULL
                );
        `)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := session.Execute(
		"INSERT INTO users VALUES " +
			"(1, 'alice', 80), (2, 'bob', 95);",
	); err != nil {
		t.Fatal(err)
	}

	result, err := session.Execute("SELECT name AS username, id, 100 AS fixed FROM users ORDER BY score DESC;")
	if err != nil {
		t.Fatal(err)
	}

	rows, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}

	wantColumns := []string{"username", "id", "fixed"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf("columns = %#v, want %#v", rows.Columns, wantColumns)
	}

	if len(rows.Rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(rows.Rows))
	}

	if rows.Rows[0][0].String != "bob" || rows.Rows[0][1].Integer != 2 || rows.Rows[0][2].Integer != 100 {
		t.Fatalf("first row = %#v", rows.Rows[0])
	}
}

func TestSessionCrossJoin(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	statements := []string{
		"CREATE TABLE t1 (a INT PRIMARY KEY);",
		"CREATE TABLE t2 (b INT PRIMARY KEY);",
		"CREATE TABLE t3 (c INT PRIMARY KEY);",
		"INSERT INTO t1 VALUES (1), (2);",
		"INSERT INTO t2 VALUES (10), (20);",
		"INSERT INTO t3 VALUES (100), (200);",
	}

	for _, sql := range statements {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	result, err := session.Execute("SELECT * FROM t1 CROSS JOIN t2 CROSS JOIN t3;")
	if err != nil {
		t.Fatal(err)
	}

	rows, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}

	if want := []string{"a", "b", "c"}; !slices.Equal(rows.Columns, want) {
		t.Fatalf("columns = %#v, want %#v", rows.Columns, want)
	}

	// 2 × 2 × 2
	if len(rows.Rows) != 8 {
		t.Fatalf("row count = %d, want 8", len(rows.Rows))
	}

	seen := make(map[[3]int64]bool)
	for _, row := range rows.Rows {
		if len(row) != 3 {
			t.Fatalf("row = %#v, want 3 values", row)
		}

		seen[[3]int64{
			row[0].Integer,
			row[1].Integer,
			row[2].Integer,
		}] = true
	}

	for _, a := range []int64{1, 2} {
		for _, b := range []int64{10, 20} {
			for _, c := range []int64{100, 200} {
				if !seen[[3]int64{a, b, c}] {
					t.Fatalf("missing combination (%d, %d, %d)", a, b, c)
				}
			}
		}
	}
}

func TestSessionConditionalJoins(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		"CREATE TABLE t1 (a INT PRIMARY KEY);",
		"CREATE TABLE t2 (b INT PRIMARY KEY);",
		"CREATE TABLE empty (c INT PRIMARY KEY);",
		"INSERT INTO t1 VALUES (1), (2), (3);",
		"INSERT INTO t2 VALUES (2), (3), (4);",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	tests := []struct {
		name      string
		sql       string
		columns   []string
		rowCount  int
		nullCount int
	}{
		{
			"inner",
			"SELECT * FROM t1 JOIN t2 ON a = b;",
			[]string{"a", "b"},
			2,
			0,
		},
		{
			"left",
			"SELECT * FROM t1 LEFT JOIN t2 ON a = b;",
			[]string{"a", "b"},
			3,
			1,
		},
		{
			"right",
			"SELECT * FROM t1 RIGHT JOIN t2 ON a = b;",
			[]string{"b", "a"},
			3,
			1,
		},
		{
			"empty right",
			"SELECT * FROM t1 LEFT JOIN empty ON a = c;",
			[]string{"a", "c"},
			3,
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.Execute(tt.sql)
			if err != nil {
				t.Fatal(err)
			}

			rows := result.(executor.RowsResult)
			if !slices.Equal(rows.Columns, tt.columns) {
				t.Fatalf("columns = %#v, want %#v", rows.Columns, tt.columns)
			}
			if len(rows.Rows) != tt.rowCount {
				t.Fatalf("row count = %d, want %d", len(rows.Rows), tt.rowCount)
			}

			nulls := 0
			for _, row := range rows.Rows {
				if row[len(row)-1].Kind == types.ValueNull {
					nulls++
				}
			}
			if nulls != tt.nullCount {
				t.Fatalf("NULL rows = %d, want %d", nulls, tt.nullCount)
			}
		})
	}
}
