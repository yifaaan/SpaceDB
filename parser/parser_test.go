package parser

import (
	"spacedb/types"
	"strings"
	"testing"
)

func TestParserStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want func(t *testing.T, statement Statement)
	}{
		{
			name: "select",
			sql:  "  SeLeCt * FROM users; ",
			want: func(t *testing.T, statement Statement) {
				selectStatement, ok := statement.(SelectStatement)
				from, fromOk := selectStatement.From.(TableFromItem)
				if !ok || !fromOk || from.Name != "users" {
					t.Fatalf("statement = %#v", statement)
				}
			},
		},
		{
			name: "insert",
			sql:  "INSERT INTO users (id, name) VALUES (1, 'alice'), (2, 'bob');",
			want: func(t *testing.T, statement Statement) {
				insertStatement, ok := statement.(InsertStatement)
				if !ok || insertStatement.TableName != "users" || len(insertStatement.Columns) != 2 || len(insertStatement.Values) != 2 {
					t.Fatalf("statement = %#v", statement)
				}
				if insertStatement.Values[0][0].Value != int64(1) || insertStatement.Values[0][1].Value != "alice" {
					t.Fatalf("values = %#v", insertStatement.Values)
				}
			},
		},
		{
			name: "create table",
			sql:  "CREATE TABLE users (id INT NOT NULL, name STRING DEFAULT 'guest', active BOOL);",
			want: func(t *testing.T, statement Statement) {
				createStatement, ok := statement.(CreateTableStatement)
				if !ok || createStatement.Name != "users" || len(createStatement.Columns) != 3 {
					t.Fatalf("statement = %#v", statement)
				}
				if createStatement.Columns[0].Nullable == nil || *createStatement.Columns[0].Nullable {
					t.Fatalf("nullable = %#v", createStatement.Columns[0].Nullable)
				}
				if createStatement.Columns[1].DefaultValue == nil || createStatement.Columns[1].DefaultValue.Value != "guest" {
					t.Fatalf("default = %#v", createStatement.Columns[1].DefaultValue)
				}
			},
		},
		{
			name: "update",
			sql:  "UPDATE users SET name = 'alice', age = 20 WHERE id = 1;",
			want: func(t *testing.T, statement Statement) {
				updateStatement, ok := statement.(UpdateStatement)
				if !ok {
					t.Fatalf(
						"statement type = %T, want UpdateStatement",
						statement,
					)
				}

				if updateStatement.TableName != "users" {
					t.Fatalf(
						"table = %q, want users",
						updateStatement.TableName,
					)
				}

				if len(updateStatement.Assignments) != 2 {
					t.Fatalf(
						"assignments = %#v",
						updateStatement.Assignments,
					)
				}

				if got := updateStatement.Assignments["name"].Value; got != "alice" {
					t.Fatalf("name value = %#v, want alice", got)
				}

				if got := updateStatement.Assignments["age"].Value; got != int64(20) {
					t.Fatalf("age value = %#v, want 20", got)
				}

				if updateStatement.Filter == nil {
					t.Fatal("filter is nil")
				}
				if updateStatement.Filter.Column != "id" ||
					updateStatement.Filter.Value.Value != int64(1) {
					t.Fatalf("filter = %#v", updateStatement.Filter)
				}
			},
		},
		{
			name: "delete",
			sql:  "DELETE FROM users WHERE id = 1;",
			want: func(t *testing.T, statement Statement) {
				deleteStatement, ok := statement.(DeleteStatement)
				if !ok {
					t.Fatalf("statement type = %T, want DeleteStatement", statement)
				}

				if deleteStatement.TableName != "users" {
					t.Fatalf("table = %q, want users", deleteStatement.TableName)
				}

				if deleteStatement.Filter == nil {
					t.Fatalf("filter is nil")
				}

				if deleteStatement.Filter.Column != "id" || deleteStatement.Filter.Value.Value != int64(1) {
					t.Fatalf("filter = %#v", deleteStatement.Filter)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := Parse(tt.sql)
			if err != nil {
				t.Fatal(err)
			}
			tt.want(t, statement)
		})
	}
}

func TestParserErrors(t *testing.T) {
	tests := []string{
		"",
		"SELECT * FROM users",
		"SELECT * FROM users; SELECT * FROM other;",
		"SELECT FROM users;",
		"SELECT * FROM 123;",
		"CREATE TABLE users (id INT,);",
		"CREATE TABLE users (age INT DEFAULT);",
		"INSERT INTO users VALUES ();",
		"UPDATE users SET name = 'a', name = 'b';",
		"UPDATE users SET name = 'a' WHERE id;",

		"SELECT * FROM t1 JOIN t2;",
		"SELECT * FROM t1 LEFT t2 ON a = b;",
		"SELECT * FROM t1 RIGHT JOIN t2 a = b;",
		"SELECT * FROM t1 CROSS JOIN t2 ON a = b;",

		"SELECT count() FROM users;",
		"SELECT count(*) FROM users;",
		"SELECT sum(a, b) FROM users;",
	}
	for _, sql := range tests {
		t.Run(sql, func(t *testing.T) {
			_, err := Parse(sql)
			if err == nil {
				t.Fatal("Parse succeeded for invalid SQL")
			}
			if !strings.Contains(err.Error(), "offset") {
				t.Fatalf("error = %q, want offset", err)
			}
		})
	}
}

func TestParserDataTypeAliases(t *testing.T) {
	statement, err := Parse("CREATE TABLE flags (a INTEGER, b BOOLEAN, c DOUBLE, d VARCHAR);")
	if err != nil {
		t.Fatal(err)
	}
	columns := statement.(CreateTableStatement).Columns
	want := []types.DataType{types.Integer, types.Boolean, types.Float, types.String}
	for i := range want {
		if columns[i].DataType != want[i] {
			t.Errorf("column %d type = %v, want %v", i, columns[i].DataType, want[i])
		}
	}
}

func TestParserKeepsLexerErrors(t *testing.T) {
	_, err := Parse("SELECT * FROM @;")
	if err == nil || !strings.Contains(err.Error(), "lexer: unexpected character") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseSelectProjection(t *testing.T) {
	statement, err := Parse("SELECT id, name AS username, 100 AS fixed FROM users;")
	if err != nil {
		t.Fatal(err)
	}

	selectStatement, ok := statement.(SelectStatement)
	if !ok {
		t.Fatalf("statement type = %T, want SelectStatement", statement)
	}

	if len(selectStatement.SelectItems) != 3 {
		t.Fatalf("select items = %#v, want 3 items", selectStatement.SelectItems)
	}

	id := selectStatement.SelectItems[0]
	if id.Expression.Kind != ColumnReference || id.Expression.Value != "id" || id.Alias != nil {
		t.Fatalf("first select item = %#v", id)
	}

	name := selectStatement.SelectItems[1]
	if name.Expression.Kind != ColumnReference ||
		name.Expression.Value != "name" ||
		name.Alias == nil ||
		*name.Alias != "username" {
		t.Fatalf("second select item = %#v", name)
	}

	fixed := selectStatement.SelectItems[2]
	if fixed.Expression.Kind != IntegerLiteral ||
		fixed.Expression.Value != int64(100) ||
		fixed.Alias == nil ||
		*fixed.Alias != "fixed" {
		t.Fatalf("third select item = %#v", fixed)
	}
}

func TestParseConditionalJoins(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		joinType JoinType
		leftCol  string
		rightCol string
	}{
		{"inner", "SELECT * FROM t1 JOIN t2 ON a = b;", JoinInner, "a", "b"},
		{"left", "SELECT * FROM t1 LEFT JOIN t2 ON a = b;", JoinLeft, "a", "b"},
		// RIGHT JOIN 会交换表达式两侧。
		{"right", "SELECT * FROM t1 RIGHT JOIN t2 ON a = b;", JoinRight, "b", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := Parse(tt.sql)
			if err != nil {
				t.Fatal(err)
			}

			selectStatement, ok := statement.(SelectStatement)
			if !ok {
				t.Fatalf("statement = %T", statement)
			}

			join, ok := selectStatement.From.(JoinFromItem)
			if !ok {
				t.Fatalf("from = %T", selectStatement.From)
			}
			if join.Type != tt.joinType || join.Predicate == nil {
				t.Fatalf("join = %#v", join)
			}

			operation, ok := join.Predicate.Value.(Operation)
			if !ok || operation.Kind != OperationEqual {
				t.Fatalf("predicate = %#v", join.Predicate)
			}

			if operation.Left.Value != tt.leftCol ||
				operation.Right.Value != tt.rightCol {
				t.Fatalf("operation = %#v", operation)
			}
		})
	}
}

func TestParseAggregateFunctions(t *testing.T) {
	statement, err := Parse(`
                SELECT
                        count(a) AS total,
                        min(b),
                        max(c),
                        sum(c),
                        avg(c)
                FROM tbl1;
        `)
	if err != nil {
		t.Fatal(err)
	}

	selectStatement, ok := statement.(SelectStatement)
	if !ok {
		t.Fatalf("statement = %T, want SelectStatement", statement)
	}

	want := []FunctionCall{
		{Name: "count", Argument: "a"},
		{Name: "min", Argument: "b"},
		{Name: "max", Argument: "c"},
		{Name: "sum", Argument: "c"},
		{Name: "avg", Argument: "c"},
	}

	if len(selectStatement.SelectItems) != len(want) {
		t.Fatalf("select items = %d, want %d", len(selectStatement.SelectItems), len(want))
	}

	for i, expected := range want {
		expression := selectStatement.SelectItems[i].Expression
		if expression.Kind != FunctionExpression {
			t.Fatalf("item %d kind = %d, want FunctionExpression", i, expression.Kind)
		}

		call, ok := expression.Value.(FunctionCall)
		if !ok || call != expected {
			t.Fatalf("item %d = %#v, want %#v", i, expression.Value, expected)
		}
	}

	alias := selectStatement.SelectItems[0].Alias
	if alias == nil || *alias != "total" {
		t.Fatalf("COUNT alias = %#v, want total", alias)
	}
}

func TestParseGroupBy(t *testing.T) {
	statement, err := Parse(`
                SELECT b, min(c) AS lowest
                FROM t1
                GROUP BY b
                ORDER BY lowest;
        `)
	if err != nil {
		t.Fatal(err)
	}

	selectStatement, ok := statement.(SelectStatement)
	if !ok {
		t.Fatalf("statement = %T, want SelectStatement", statement)
	}

	if selectStatement.GroupBy == nil {
		t.Fatal("GroupBy = nil, want column b")
	}

	groupBy := *selectStatement.GroupBy
	if groupBy.Kind != ColumnReference {
		t.Fatalf("GroupBy kind = %d, want ColumnReference", groupBy.Kind)
	}

	columnName, ok := groupBy.Value.(string)
	if !ok || columnName != "b" {
		t.Fatalf("GroupBy value = %#v, want b", groupBy.Value)
	}

	if len(selectStatement.SelectItems) != 2 {
		t.Fatalf("select items = %d, want 2", len(selectStatement.SelectItems))
	}

	call, ok := selectStatement.SelectItems[1].Expression.Value.(FunctionCall)
	if !ok || call.Name != "min" || call.Argument != "c" {
		t.Fatalf("aggregate expression = %#v", selectStatement.SelectItems[1].Expression)
	}

	if len(selectStatement.OrderBy) != 1 || selectStatement.OrderBy[0].Column != "lowest" {
		t.Fatalf("OrderBy = %#v, want lowest", selectStatement.OrderBy)
	}
}

func TestParseGroupByRequiresBy(t *testing.T) {
	_, err := Parse(
		"SELECT b FROM t1 GROUP b;",
	)
	if err == nil {
		t.Fatal("Parse succeeded without BY")
	}

	if !strings.Contains(err.Error(), "expected keyword 'BY'") {
		t.Fatalf("error = %v, want missing-BY error", err)
	}
}

func TestParseSelectWhereAndHaving(t *testing.T) {
	statement, err := Parse(`
                SELECT category, sum(amount)
                FROM sales
                WHERE amount > 10
                GROUP BY category
                HAVING sum < 100
                ORDER BY sum;
        `)
	if err != nil {
		t.Fatal(err)
	}

	selectStatement, ok := statement.(SelectStatement)
	if !ok {
		t.Fatalf(
			"statement = %T, want SelectStatement",
			statement,
		)
	}

	if selectStatement.Where == nil {
		t.Fatal("Where = nil, want amount > 10")
	}

	whereOperation, ok := selectStatement.Where.Value.(Operation)
	if !ok {
		t.Fatalf(
			"Where value = %T, want Operation",
			selectStatement.Where.Value,
		)
	}

	if whereOperation.Kind != OperationGreaterThan ||
		whereOperation.Left.Kind != ColumnReference ||
		whereOperation.Left.Value != "amount" ||
		whereOperation.Right.Kind != IntegerLiteral ||
		whereOperation.Right.Value != int64(10) {
		t.Fatalf(
			"Where operation = %#v, want amount > 10",
			whereOperation,
		)
	}

	if selectStatement.GroupBy == nil ||
		selectStatement.GroupBy.Value != "category" {
		t.Fatalf(
			"GroupBy = %#v, want category",
			selectStatement.GroupBy,
		)
	}

	if selectStatement.Having == nil {
		t.Fatal("Having = nil, want sum < 100")
	}

	havingOperation, ok := selectStatement.Having.Value.(Operation)
	if !ok {
		t.Fatalf(
			"Having value = %T, want Operation",
			selectStatement.Having.Value,
		)
	}

	if havingOperation.Kind != OperationLessThan ||
		havingOperation.Left.Kind != ColumnReference ||
		havingOperation.Left.Value != "sum" ||
		havingOperation.Right.Kind != IntegerLiteral ||
		havingOperation.Right.Value != int64(100) {
		t.Fatalf(
			"Having operation = %#v, want sum < 100",
			havingOperation,
		)
	}

	if len(selectStatement.OrderBy) != 1 ||
		selectStatement.OrderBy[0].Column != "sum" {
		t.Fatalf(
			"OrderBy = %#v, want sum",
			selectStatement.OrderBy,
		)
	}
}

func TestParseComparisonRequiresOperator(t *testing.T) {
	_, err := Parse(
		"SELECT * FROM sales WHERE amount + 10;",
	)
	if err == nil {
		t.Fatal("Parse succeeded without a comparison operator")
	}

	if !strings.Contains(
		err.Error(),
		"expected comparison operator",
	) {
		t.Fatalf(
			"error = %v, want comparison-operator error",
			err,
		)
	}
}
