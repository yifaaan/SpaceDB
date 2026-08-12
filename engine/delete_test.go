package engine

import (
	"testing"

	"spacedb/executor"
	"spacedb/storage"
)

func TestSessionDelete(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	if _, err := session.Execute(`
		CREATE TABLE users (
			id INT PRIMARY KEY NOT NULL,
			name STRING NOT NULL
		);
	`); err != nil {
		t.Fatal(err)
	}

	for _, sql := range []string{
		"INSERT INTO users VALUES (1, 'alice');",
		"INSERT INTO users VALUES (2, 'bob');",
		"INSERT INTO users VALUES (3, 'carol');",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatal(err)
		}
	}

	result, err := session.Execute("DELETE FROM users WHERE id = 3;")
	if err != nil {
		t.Fatal(err)
	}

	deleteResult, ok := result.(executor.DeleteResult)
	if !ok {
		t.Fatalf("result = %T, want executor.DeleteResult", result)
	}
	if deleteResult.Count != 1 {
		t.Fatalf("deleted count = %d, want 1", deleteResult.Count)
	}

	result, err = session.Execute("DELETE FROM users WHERE id = 2;")
	if err != nil {
		t.Fatal(err)
	}

	deleteResult = result.(executor.DeleteResult)
	if deleteResult.Count != 1 {
		t.Fatalf("second deleted count = %d, want 1", deleteResult.Count)
	}

	result, err = session.Execute("SELECT * FROM users;")
	if err != nil {
		t.Fatal(err)
	}

	rowsResult, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}

	if len(rowsResult.Rows) != 1 {
		t.Fatalf("remaining row count = %d, want 1", len(rowsResult.Rows))
	}

	remaining := rowsResult.Rows[0]
	if remaining[0].Integer != 1 || remaining[1].String != "alice" {
		t.Fatalf("remaining row = %#v", remaining)
	}
}

func TestSessionDeleteWithComparisonFilter(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	if _, err := session.Execute(`
		CREATE TABLE scores (
			id INT PRIMARY KEY NOT NULL,
			score INT NULL
		);
	`); err != nil {
		t.Fatal(err)
	}

	for _, sql := range []string{
		"INSERT INTO scores VALUES (1, 8);",
		"INSERT INTO scores VALUES (2, 18);",
		"INSERT INTO scores VALUES (3, 28);",
		"INSERT INTO scores VALUES (4, NULL);",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatal(err)
		}
	}

	result, err := session.Execute("DELETE FROM scores WHERE score < 20;")
	if err != nil {
		t.Fatal(err)
	}
	deleteResult, ok := result.(executor.DeleteResult)
	if !ok {
		t.Fatalf("result = %T, want executor.DeleteResult", result)
	}
	if deleteResult.Count != 2 {
		t.Fatalf("deleted count = %d, want 2", deleteResult.Count)
	}

	// NULL = NULL 的结果是 UNKNOWN，Filter 不会把该行交给 DeleteExecutor。
	result, err = session.Execute("DELETE FROM scores WHERE score = NULL;")
	if err != nil {
		t.Fatal(err)
	}
	deleteResult, ok = result.(executor.DeleteResult)
	if !ok {
		t.Fatalf("result = %T, want executor.DeleteResult", result)
	}
	if count := deleteResult.Count; count != 0 {
		t.Fatalf("NULL comparison deleted count = %d, want 0", count)
	}

	result, err = session.Execute("SELECT * FROM scores;")
	if err != nil {
		t.Fatal(err)
	}
	rowsResult, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}
	if len(rowsResult.Rows) != 2 {
		t.Fatalf("remaining row count = %d, want 2", len(rowsResult.Rows))
	}

	remaining := make(map[int64]bool, len(rowsResult.Rows))
	for _, row := range rowsResult.Rows {
		remaining[row[0].Integer] = true
	}
	if !remaining[3] || !remaining[4] {
		t.Fatalf("remaining ids = %#v, want 3 and 4", remaining)
	}
}
