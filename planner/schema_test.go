package planner

import (
	"spacedb/parser"
	"spacedb/types"
	"testing"
)

func TestTableFromCreateStatement(t *testing.T) {
	stmt, err := parser.Parse(`
		CREATE TABLE users (
			id INT,
			age INT NOT NULL,
			score FLOAT DEFAULT 3.5,
			active BOOL NOT NULL DEFAULT true
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	create, ok := stmt.(parser.CreateTableStatement)
	if !ok {
		t.Fatalf("statement = %T, want parser.CreateTableStatement", stmt)
	}

	table, err := tableFromCreateStatement(create)

	if err != nil {
		t.Fatal(err)
	}

	if table.Name != "users" || len(table.Columns) != 4 {
		t.Fatalf("table = %#v", table)
	}

	id := table.Columns[0]
	if !id.Nullable || id.Default == nil || id.Default.Kind != types.ValueNull {
		t.Errorf("id column = %#v", id)
	}

	age := table.Columns[1]
	if age.Nullable || age.Default != nil {
		t.Errorf("age column = %#v", age)
	}

	score := table.Columns[2]
	if score.Default == nil ||
		score.Default.Kind != types.ValueFloat ||
		score.Default.Float != 3.5 {
		t.Errorf("score column = %#v", score)
	}

	active := table.Columns[3]
	if active.Nullable ||
		active.Default == nil ||
		active.Default.Kind != types.ValueBoolean ||
		!active.Default.Boolean {
		t.Errorf("active column = %#v", active)
	}
}
