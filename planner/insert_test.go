package planner

import (
	"testing"

	"spacedb/parser"
)

func TestBuildInsertPlan(t *testing.T) {
	stmt, err := parser.Parse("INSERT INTO users VALUES (1, 'alice', true);")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	node, ok := plan.Node.(InsertNode)
	if !ok {
		t.Fatalf("node = %T, want InsertNode", plan.Node)
	}

	if node.TableName != "users" {
		t.Fatalf("table name = %q, want users", node.TableName)
	}

	if node.Columns == nil || len(node.Columns) != 0 {
		t.Fatalf("columns = %#v, want empty non-nil slice", node.Columns)
	}

	if len(node.Values) != 1 || len(node.Values[0]) != 3 {
		t.Fatalf("values = %#v", node.Values)
	}

	if node.Values[0][0].Kind != parser.IntegerLiteral || node.Values[0][0].Value != int64(1) {
		t.Fatalf("first value = %#v", node.Values[0][0])
	}
}

func TestBuildInsertPlanWithColumnsAndRows(t *testing.T) {
	stmt, err := parser.Parse("INSERT INTO users (id, name) VALUES (1, 'alice'), (2, 'bob');")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	node := plan.Node.(InsertNode)

	if len(node.Columns) != 2 || node.Columns[0] != "id" || node.Columns[1] != "name" {
		t.Fatalf("columns = %#v", node.Columns)
	}

	if len(node.Values) != 2 {
		t.Fatalf("row count = %d, want 2", len(node.Values))
	}

	if node.Values[1][1].Value != "bob" {
		t.Fatalf("second row = %#v", node.Values[1])
	}
}
