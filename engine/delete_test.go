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
