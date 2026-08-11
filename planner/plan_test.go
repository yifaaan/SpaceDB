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

func TestBuildProjectionPlan(t *testing.T) {
	stmt, err := parser.Parse("SELECT name AS username, id FROM users ORDER BY score DESC LIMIT 10 OFFSET 2;")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	projection, ok := plan.Node.(ProjectionNode)
	if !ok {
		t.Fatalf("root node = %T, want ProjectionNode", plan.Node)
	}
	if len(projection.Items) != 2 {
		t.Fatalf("projection items = %#v", projection.Items)
	}

	if projection.Items[0].Alias == nil || *projection.Items[0].Alias != "username" {
		t.Fatalf("first projection item = %#v", projection.Items[0])
	}

	limit, ok := projection.Source.(LimitNode)
	if !ok {
		t.Fatalf("projection source = %T, want LimitNode", projection.Source)
	}

	offset, ok := limit.Source.(OffsetNode)
	if !ok {
		t.Fatalf("limit source = %T, want OffsetNode", limit.Source)
	}

	order, ok := offset.Source.(OrderNode)
	if !ok {
		t.Fatalf("offset source = %T, want OrderNode", offset.Source)
	}

	scan, ok := order.Source.(ScanNode)
	if !ok {
		t.Fatalf("order source = %T, want ScanNode", order.Source)
	}

	if scan.TableName != "users" {
		t.Fatalf("scan table = %q, want users", scan.TableName)
	}
}

func TestBuildCrossJoinPlan(t *testing.T) {
	stmt, err := parser.Parse("SELECT * FROM t1 CROSS JOIN t2 CROSS JOIN t3;")
	if err != nil {
		t.Fatal(err)
	}

	plan, err := Build(stmt)
	if err != nil {
		t.Fatal(err)
	}

	outer, ok := plan.Node.(NestedLoopJoinNode)
	if !ok {
		t.Fatalf("root = %T, want NestedLoopJoinNode", plan.Node)
	}
	if outer.Predicate != nil {
		t.Fatalf("CROSS JOIN predicate = %#v, want nil", outer.Predicate)
	}
	if outer.Outer {
		t.Fatal("CROSS JOIN must not be outer")
	}

	inner, ok := outer.Left.(NestedLoopJoinNode)
	if !ok {
		t.Fatalf("left = %T, want NestedLoopJoinNode", outer.Left)
	}

	left, ok := inner.Left.(ScanNode)
	if !ok || left.TableName != "t1" {
		t.Fatalf("inner left = %#v", inner.Left)
	}

	right, ok := inner.Right.(ScanNode)
	if !ok || right.TableName != "t2" {
		t.Fatalf("inner right = %#v", inner.Right)
	}

	last, ok := outer.Right.(ScanNode)
	if !ok || last.TableName != "t3" {
		t.Fatalf("outer right = %#v", outer.Right)
	}
}

func TestBuildConditionalJoinPlans(t *testing.T) {
	tests := []struct {
		name       string
		sql        string
		leftTable  string
		rightTable string
		outer      bool
	}{
		{
			"inner",
			"SELECT * FROM t1 JOIN t2 ON a = b;",
			"t1", "t2", false,
		},
		{
			"left",
			"SELECT * FROM t1 LEFT JOIN t2 ON a = b;",
			"t1", "t2", true,
		},
		{
			// RIGHT JOIN 被规范化成 t2 LEFT JOIN t1
			"right",
			"SELECT * FROM t1 RIGHT JOIN t2 ON a = b;",
			"t2", "t1", true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statement, err := parser.Parse(tt.sql)
			if err != nil {
				t.Fatal(err)
			}

			plan, err := Build(statement)
			if err != nil {
				t.Fatal(err)
			}

			join, ok := plan.Node.(NestedLoopJoinNode)
			if !ok {
				t.Fatalf("node = %T", plan.Node)
			}

			left, leftOK := join.Left.(ScanNode)
			right, rightOK := join.Right.(ScanNode)
			if !leftOK || !rightOK {
				t.Fatalf("inputs = (%T, %T), want ScanNode", join.Left, join.Right)
			}

			if left.TableName != tt.leftTable || right.TableName != tt.rightTable {
				t.Fatalf(
					"tables = (%q, %q), want (%q, %q)", left.TableName, right.TableName, tt.leftTable, tt.rightTable,
				)
			}

			if join.Outer != tt.outer || join.Predicate == nil {
				t.Fatalf("join = %#v", join)
			}
		})
	}
}
