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
				if !ok || selectStatement.TableName != "users" {
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
