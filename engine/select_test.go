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
		first[1].Str != "guest" ||
		first[2].Kind != types.ValueBoolean ||
		!first[2].Boolean {
		t.Fatalf("first row = %#v", first)
	}

	second := rows.Rows[1]
	if second[0].Integer != 2 ||
		second[1].Str != "alice" {
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

	if rows.Rows[0][0].Str != "bob" || rows.Rows[0][1].Integer != 2 || rows.Rows[0][2].Integer != 100 {
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
	if values[2].Kind != types.ValueString || values[2].Str != "alpha" {
		t.Fatalf("min(label) = %#v, want alpha", values[2])
	}
	if values[3].Kind != types.ValueString || values[3].Str != "gamma" {
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

func TestSessionSumAggregate(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		`CREATE TABLE totals (
                        id INT PRIMARY KEY,
                        integer_amount INT NULL,
                        float_amount FLOAT NULL,
                        label STRING NULL
                );`,
		`CREATE TABLE empty_totals (
                        id INT PRIMARY KEY,
                        amount INT NULL
                );`,
		`INSERT INTO totals VALUES
                        (1, 10, 1.25, 'first'),
                        (2, NULL, 2.5, 'second'),
                        (3, 3, NULL, NULL);`,
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	result, err := session.Execute(`
                SELECT
                        sum(integer_amount) AS integer_total,
                        sum(float_amount) AS float_total
                FROM totals;
        `)
	if err != nil {
		t.Fatal(err)
	}

	rows, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}

	wantColumns := []string{"integer_total", "float_total"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf("columns = %#v, want %#v", rows.Columns, wantColumns)
	}

	if len(rows.Rows) != 1 || len(rows.Rows[0]) != 2 {
		t.Fatalf("rows = %#v, want one row with two values", rows.Rows)
	}

	values := rows.Rows[0]

	if values[0].Kind != types.ValueFloat || values[0].Float != 13 {
		t.Fatalf("sum(integer_amount) = %#v, want float 13", values[0])
	}

	if values[1].Kind != types.ValueFloat || values[1].Float != 3.75 {
		t.Fatalf("sum(float_amount) = %#v, want float 3.75", values[1])
	}

	result, err = session.Execute("SELECT sum(amount) FROM empty_totals;")
	if err != nil {
		t.Fatal(err)
	}

	emptyRows := result.(executor.RowsResult)
	if len(emptyRows.Rows) != 1 || len(emptyRows.Rows[0]) != 1 || emptyRows.Rows[0][0].Kind != types.ValueNull {
		t.Fatalf("empty SUM rows = %#v, want [[NULL]]", emptyRows.Rows)
	}

	if _, err := session.Execute("SELECT sum(label) FROM totals;"); err == nil {
		t.Fatal("SUM on string column succeeded, want error")
	}
}

func TestSessionAvgAggregate(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		`CREATE TABLE scores (
                        id INT PRIMARY KEY,
                        integer_score INT NULL,
                        float_score FLOAT NULL,
                        label STRING NULL
                );`,
		`CREATE TABLE empty_scores (
                        id INT PRIMARY KEY,
                        score INT NULL
                );`,
		`INSERT INTO scores VALUES
                        (1, 10, 1.5, 'first'),
                        (2, 20, NULL, 'second'),
                        (3, NULL, 4.5, 'third');`,
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	result, err := session.Execute(`
                SELECT
                        avg(integer_score) AS integer_average,
                        avg(float_score)
                FROM scores;
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

	wantColumns := []string{"integer_average", "avg"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf(
			"columns = %#v, want %#v",
			rows.Columns,
			wantColumns,
		)
	}

	if len(rows.Rows) != 1 || len(rows.Rows[0]) != 2 {
		t.Fatalf(
			"rows = %#v, want one row with two values",
			rows.Rows,
		)
	}

	values := rows.Rows[0]

	if values[0].Kind != types.ValueFloat ||
		values[0].Float != 15 {
		t.Fatalf(
			"avg(integer_score) = %#v, want float 15",
			values[0],
		)
	}

	if values[1].Kind != types.ValueFloat ||
		values[1].Float != 3 {
		t.Fatalf(
			"avg(float_score) = %#v, want float 3",
			values[1],
		)
	}
	result, err = session.Execute(
		"SELECT avg(score) FROM empty_scores;",
	)
	if err != nil {
		t.Fatal(err)
	}

	emptyRows := result.(executor.RowsResult)
	if len(emptyRows.Rows) != 1 ||
		len(emptyRows.Rows[0]) != 1 ||
		emptyRows.Rows[0][0].Kind != types.ValueNull {
		t.Fatalf(
			"empty AVG rows = %#v, want [[NULL]]",
			emptyRows.Rows,
		)
	}

	// AVG 与 SUM 一样，只允许数值列。
	if _, err := session.Execute(
		"SELECT avg(label) FROM scores;",
	); err == nil {
		t.Fatal("AVG on string column succeeded, want error")
	}
}

func TestSessionGroupBy(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		`CREATE TABLE measurements_by_label (
			id INT PRIMARY KEY,
			label STRING NULL,
			measurement FLOAT NULL
		);`,
		`CREATE TABLE empty_groups (
			id INT PRIMARY KEY,
			label STRING NULL
		);`,
		`INSERT INTO measurements_by_label VALUES
			(1, 'aa', 3.1),
			(2, 'bb', 5.3),
			(3, NULL, NULL),
			(4, NULL, 4.6),
			(5, 'bb', 5.8),
			(6, 'dd', 1.4);`,
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	result, err := session.Execute(`
		SELECT
			label,
			min(measurement),
			max(id),
			avg(measurement)
		FROM measurements_by_label
		GROUP BY label
		ORDER BY avg;
	`)
	if err != nil {
		t.Fatal(err)
	}

	rows, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}

	wantColumns := []string{"label", "min", "max", "avg"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf("columns = %#v, want %#v", rows.Columns, wantColumns)
	}

	if len(rows.Rows) != 4 {
		t.Fatalf("row count = %d, want 4", len(rows.Rows))
	}

	// ORDER BY avg 使用现有 Value.Compare 语义：NULL 小于非 NULL。
	// 本数据中 NULL 分组仍有 measurement=4.6，因此按平均值排列为：
	// dd(1.4)、aa(3.1)、NULL(4.6)、bb(5.55)。
	want := []struct {
		labelKind types.ValueKind
		label     string
		min       float64
		max       int64
		avg       float64
	}{
		{types.ValueString, "dd", 1.4, 6, 1.4},
		{types.ValueString, "aa", 3.1, 1, 3.1},
		{types.ValueNull, "", 4.6, 4, 4.6},
		{types.ValueString, "bb", 5.3, 5, 5.55},
	}

	for rowIndex, expected := range want {
		row := rows.Rows[rowIndex]
		if len(row) != 4 {
			t.Fatalf("row %d = %#v, want 4 values", rowIndex+1, row)
		}

		if row[0].Kind != expected.labelKind ||
			row[0].Str != expected.label ||
			row[1].Kind != types.ValueFloat ||
			row[1].Float != expected.min ||
			row[2].Kind != types.ValueInteger ||
			row[2].Integer != expected.max ||
			row[3].Kind != types.ValueFloat ||
			row[3].Float != expected.avg {
			t.Fatalf("row %d = %#v, want %#v", rowIndex+1, row, expected)
		}
	}

	// 没有聚合函数的 GROUP BY 仍会为每个不同键产生一行。
	result, err = session.Execute(`
		SELECT label
		FROM measurements_by_label
		GROUP BY label
		ORDER BY label;
	`)
	if err != nil {
		t.Fatal(err)
	}

	distinctRows := result.(executor.RowsResult)
	if len(distinctRows.Rows) != 4 {
		t.Fatalf(
			"distinct group row count = %d, want 4",
			len(distinctRows.Rows),
		)
	}
	if distinctRows.Rows[0][0].Kind != types.ValueNull ||
		distinctRows.Rows[1][0].Str != "aa" ||
		distinctRows.Rows[2][0].Str != "bb" ||
		distinctRows.Rows[3][0].Str != "dd" {
		t.Fatalf("distinct group rows = %#v", distinctRows.Rows)
	}

	// GROUP BY 空表没有任何分组，因此返回零行；这与无 GROUP BY 的
	// 聚合仍返回一行是两个不同的 SQL 语义。
	result, err = session.Execute(`
		SELECT label, count(id)
		FROM empty_groups
		GROUP BY label;
	`)
	if err != nil {
		t.Fatal(err)
	}

	emptyRows := result.(executor.RowsResult)
	if len(emptyRows.Rows) != 0 {
		t.Fatalf("empty grouped rows = %#v, want zero rows", emptyRows.Rows)
	}
}

func TestSessionWhereAndHavingFilters(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		`CREATE TABLE filter_values (
			id INT PRIMARY KEY,
			label STRING NULL,
			amount FLOAT NULL,
			active BOOL
		);`,
		`INSERT INTO filter_values VALUES
			(1, 'aa', 3.1, true),
			(2, 'bb', 5.3, true),
			(3, NULL, NULL, false),
			(4, NULL, 4.6, false),
			(5, 'bb', 5.8, true),
			(6, 'dd', 1.4, false);`,
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	// Boolean 的排序规则是 false < true，因此 active < true 只保留
	// active=false 的三行。
	result, err := session.Execute(`
		SELECT id
		FROM filter_values
		WHERE active < true
		ORDER BY id;
	`)
	if err != nil {
		t.Fatal(err)
	}

	rows := result.(executor.RowsResult)
	if len(rows.Rows) != 3 ||
		rows.Rows[0][0].Integer != 3 ||
		rows.Rows[1][0].Integer != 4 ||
		rows.Rows[2][0].Integer != 6 {
		t.Fatalf("boolean filter rows = %#v, want ids 3, 4, 6", rows.Rows)
	}

	// NULL > 4 的结果是 NULL，不是 false；Filter 只保留 true，故 id=3
	// 不会进入结果。
	result, err = session.Execute(`
		SELECT id
		FROM filter_values
		WHERE amount > 4
		ORDER BY id;
	`)
	if err != nil {
		t.Fatal(err)
	}

	rows = result.(executor.RowsResult)
	if len(rows.Rows) != 3 ||
		rows.Rows[0][0].Integer != 2 ||
		rows.Rows[1][0].Integer != 4 ||
		rows.Rows[2][0].Integer != 5 {
		t.Fatalf("numeric filter rows = %#v, want ids 2, 4, 5", rows.Rows)
	}

	// SQL 中 NULL = NULL 仍然是 UNKNOWN，因此不会匹配任何行。
	result, err = session.Execute(`
		SELECT id
		FROM filter_values
		WHERE label = NULL;
	`)
	if err != nil {
		t.Fatal(err)
	}
	rows = result.(executor.RowsResult)
	if len(rows.Rows) != 0 {
		t.Fatalf("NULL equality rows = %#v, want zero rows", rows.Rows)
	}

	// WHERE 先过滤原始行，再由 GROUP BY 聚合；HAVING 最后过滤聚合行。
	result, err = session.Execute(`
		SELECT label, sum(amount)
		FROM filter_values
		WHERE amount > 1
		GROUP BY label
		HAVING sum < 5
		ORDER BY sum;
	`)
	if err != nil {
		t.Fatal(err)
	}

	rows = result.(executor.RowsResult)
	wantColumns := []string{"label", "sum"}
	if !slices.Equal(rows.Columns, wantColumns) {
		t.Fatalf("filter aggregate columns = %#v, want %#v", rows.Columns, wantColumns)
	}
	if len(rows.Rows) != 3 {
		t.Fatalf("HAVING row count = %d, want 3", len(rows.Rows))
	}

	// ORDER BY sum: dd=1.4、aa=3.1、NULL=4.6；bb=11.1 被 HAVING 丢弃。
	if rows.Rows[0][0].Str != "dd" || rows.Rows[0][1].Float != 1.4 ||
		rows.Rows[1][0].Str != "aa" || rows.Rows[1][1].Float != 3.1 ||
		rows.Rows[2][0].Kind != types.ValueNull || rows.Rows[2][1].Float != 4.6 {
		t.Fatalf("HAVING rows = %#v", rows.Rows)
	}
}

func TestSessionWhereFiltersJoinedRows(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	for _, sql := range []string{
		"CREATE TABLE filter_left (a INT PRIMARY KEY);",
		"CREATE TABLE filter_right (b INT PRIMARY KEY);",
		"INSERT INTO filter_left VALUES (1), (2);",
		"INSERT INTO filter_right VALUES (2), (3);",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatalf("Execute(%q): %v", sql, err)
		}
	}

	result, err := session.Execute(`
		SELECT *
		FROM filter_left CROSS JOIN filter_right
		WHERE b > 2
		ORDER BY a;
	`)
	if err != nil {
		t.Fatal(err)
	}

	rows := result.(executor.RowsResult)
	if len(rows.Rows) != 2 ||
		rows.Rows[0][0].Integer != 1 || rows.Rows[0][1].Integer != 3 ||
		rows.Rows[1][0].Integer != 2 || rows.Rows[1][1].Integer != 3 {
		t.Fatalf("joined filter rows = %#v", rows.Rows)
	}
}
