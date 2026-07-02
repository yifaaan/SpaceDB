package planner

import (
	"spacedb/parser"
	"spacedb/types"
	"testing"
)

func TestBuildCreateTablePlan(t *testing.T) {
	stmt, err := parser.Parse(`
		CREATE TABLE users (
			id INT,
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

	// id 没有写 NOT NULL，因此默认可空，并补 NULL 默认值
	id := node.Schema.Columns[0]
	if !id.Nullable || id.Default == nil || id.Default.Kind != types.ValueNull {
		t.Fatalf("id column = %#v", id)
	}

	// name 明确 NOT NULL，则没有 DEFAULT
	name := node.Schema.Columns[1]
	if name.Nullable || name.Default != nil {
		t.Fatalf("name column = %#v", name)
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
}
