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
