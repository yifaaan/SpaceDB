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

func TestSessionCountAggregate(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		"CREATE TABLE metrics (id INT PRIMARY KEY, note STRING NULL);",
		"CREATE TABLE empty_metrics (id INT PRIMARY KEY);",
		"INSERT INTO metrics VALUES (1, 'x'), (2, NULL), (3, 'z');",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	tests := []struct {
		name       string
		sql        string
		wantColumn string
		wantCount  int64
	}{
		{
			name:       "count every non-null primary key",
			sql:        "SELECT count(id) AS total FROM metrics;",
			wantColumn: "total",
			wantCount:  3,
		},
		{
			name:       "ignore null values",
			sql:        "SELECT count(note) FROM metrics;",
			wantColumn: "count",
			wantCount:  2,
		},
		{
			name:       "empty table still produces one row",
			sql:        "SELECT count(id) FROM empty_metrics;",
			wantColumn: "count",
			wantCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := session.Execute(tt.sql)
			if err != nil {
				t.Fatal(err)
			}

			rows, ok := result.(executor.RowsResult)
			if !ok {
				t.Fatalf(
					"result = %T, want executor.RowsResult",
					result,
				)
			}

			if !slices.Equal(rows.Columns, []string{tt.wantColumn}) {
				t.Fatalf(
					"columns = %#v, want [%q]",
					rows.Columns,
					tt.wantColumn,
				)
			}

			if len(rows.Rows) != 1 {
				t.Fatalf(
					"row count = %d, want 1",
					len(rows.Rows),
				)
			}

			if len(rows.Rows[0]) != 1 {
				t.Fatalf(
					"value count = %d, want 1",
					len(rows.Rows[0]),
				)
			}

			value := rows.Rows[0][0]
			if value.Kind != types.ValueInteger ||
				value.Integer != tt.wantCount {
				t.Fatalf(
					"value = %#v, want integer %d",
					value,
					tt.wantCount,
				)
			}
		})
	}
}

func TestSessionMinMaxAggregate(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		`CREATE TABLE measurements (
                        id INT PRIMARY KEY,
                        score FLOAT NULL,
                        label STRING NULL
                );`,
		`CREATE TABLE empty_measurements (
                        id INT PRIMARY KEY,
                        score FLOAT NULL
                );`,
		`INSERT INTO measurements VALUES
                        (1, 8.5, 'beta'),
                        (2, NULL, 'alpha'),
                        (3, 2.25, NULL),
                        (4, 10.75, 'gamma');`,
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	result, err := session.Execute(`
                SELECT
                        min(score) AS lowest,
                        max(score) AS highest,
                        min(label),
                        max(label)
                FROM measurements;
        `)
	if err != nil {
		t.Fatal(err)
	}

	rows, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf(
			"result = %T, want executor.RowsResult",
			result,
		)
	}

	wantColumns := []string{"lowest", "highest", "min", "max"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf(
			"columns = %#v, want %#v",
			rows.Columns,
			wantColumns,
		)
	}

	if len(rows.Rows) != 1 || len(rows.Rows[0]) != 4 {
		t.Fatalf("rows = %#v, want one row with four values", rows.Rows)
	}

	values := rows.Rows[0]

	if values[0].Kind != types.ValueFloat || values[0].Float != 2.25 {
		t.Fatalf("min(score) = %#v, want float 2.25", values[0])
	}
	if values[1].Kind != types.ValueFloat || values[1].Float != 10.75 {
		t.Fatalf("max(score) = %#v, want float 10.75", values[1])
	}
	if values[2].Kind != types.ValueString || values[2].String != "alpha" {
		t.Fatalf("min(label) = %#v, want alpha", values[2])
	}
	if values[3].Kind != types.ValueString || values[3].String != "gamma" {
		t.Fatalf("max(label) = %#v, want gamma", values[3])
	}

	result, err = session.Execute(`
                SELECT min(score), max(score)
                FROM empty_measurements;
        `)
	if err != nil {
		t.Fatal(err)
	}

	emptyRows := result.(executor.RowsResult)
	if len(emptyRows.Rows) != 1 || len(emptyRows.Rows[0]) != 2 {
		t.Fatalf(
			"empty aggregate rows = %#v, want one row with two values",
			emptyRows.Rows,
		)
	}

	for i, value := range emptyRows.Rows[0] {
		if value.Kind != types.ValueNull {
			t.Fatalf(
				"empty aggregate value %d = %#v, want NULL",
				i,
				value,
			)
		}
	}
}
