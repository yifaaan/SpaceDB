package planner

import (
	"spacedb/parser"
	"testing"
)

func TestBuildCreateTablePlan(t *testing.T) {
	stmt, err := parser.Parse(`
		CREATE TABLE users (
			id INT PRIMARY KEY,
			name STRING NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	node, ok := plan.Node.(CreateTableNode)
	if !ok {
		t.Fatalf("node = %T, want CreateTableNode", plan.Node)
	}

	if node.Schema.Name != "users" {
		t.Fatalf("table name = %q, want users", node.Schema.Name)
	}

	if len(node.Schema.Columns) != 2 {
		t.Fatalf("column count = %d, want 2", len(node.Schema.Columns))
	}

	// name 明确 NOT NULL，则没有 DEFAULT
	name := node.Schema.Columns[1]
	if name.Nullable || name.Default != nil {
		t.Fatalf("name column = %#v", name)
	}

	id := node.Schema.Columns[0]
	if !id.PrimaryKey {
		t.Fatalf("id primary key = false, want true")
	}
}

func TestBuildScanPlan(t *testing.T) {
	stmt, err := parser.Parse("SELECT * FROM users;")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	node, ok := plan.Node.(ScanNode)
	if !ok {
		t.Fatalf("node = %T, want ScanNode", plan.Node)
	}

	if node.TableName != "users" {
		t.Fatalf("table name = %q, want users", node.TableName)
	}

	if node.Filter != nil {
		t.Fatalf("SELECT scan filter = %#v, want nil", node.Filter)
	}
}

func TestBuildUpdatePlan(t *testing.T) {
	stmt, err := parser.Parse(
		"UPDATE users SET name = 'alice', age = 20 WHERE id = 1;",
	)
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	update, ok := plan.Node.(UpdateNode)
	if !ok {
		t.Fatalf("node = %T, want UpdateNode", plan.Node)
	}

	if update.TableName != "users" {
		t.Fatalf("table name = %q, want users", update.TableName)
	}

	if len(update.Assignments) != 2 {
		t.Fatalf("assignments = %#v", update.Assignments)
	}

	scan, ok := update.Source.(ScanNode)
	if !ok {
		t.Fatalf("source = %T, want ScanNode", update.Source)
	}

	if scan.TableName != "users" {
		t.Fatalf("scan table = %q, want users", scan.TableName)
	}

	if scan.Filter == nil {
		t.Fatal("scan filter is nil")
	}

	if scan.Filter.Column != "id" ||
		scan.Filter.Value.Value != int64(1) {
		t.Fatalf("scan filter = %#v", scan.Filter)
	}
}
