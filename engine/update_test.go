package engine

import (
	"testing"

	"spacedb/executor"
	"spacedb/storage"
	"spacedb/types"
)

func TestSessionUpdate(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	if _, err := session.Execute(`
		CREATE TABLE users (
			id INT PRIMARY KEY NOT NULL,
			name STRING NOT NULL,
			score INT NOT NULL
		);
	`); err != nil {
		t.Fatal(err)
	}

	for _, sql := range []string{
		"INSERT INTO users VALUES (1, 'alice', 10);",
		"INSERT INTO users VALUES (2, 'bob', 20);",
		"INSERT INTO users VALUES (3, 'carol', 30);",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatal(err)
		}
	}

	result, err := session.Execute("UPDATE users SET name = 'anna', score = 11 WHERE id = 1;")
	if err != nil {
		t.Fatal(err)
	}
	updateResult, ok := result.(executor.UpdateResult)
	if !ok {
		t.Fatalf("result = %T, want executor.UpdateResult", result)
	}
	if updateResult.Count != 1 {
		t.Fatalf("updated count = %d, want 1", updateResult.Count)
	}

	// 修改主键时，旧 row key 应删除，新 row key 应保存同一行。
	result, err = session.Execute("UPDATE users SET id = 33 WHERE id = 3;")
	if err != nil {
		t.Fatal(err)
	}
	updateResult = result.(executor.UpdateResult)
	if updateResult.Count != 1 {
		t.Fatalf("primary-key update count = %d, want 1", updateResult.Count)
	}

	result, err = session.Execute("SELECT * FROM users;")
	if err != nil {
		t.Fatal(err)
	}
	rowsResult, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}
	if len(rowsResult.Rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rowsResult.Rows))
	}

	// 使用主键索引结果，避免测试依赖底层扫描顺序。
	rowsByID := make(map[int64]types.Row)
	for _, row := range rowsResult.Rows {
		rowsByID[row[0].Integer] = row
	}

	first := rowsByID[1]
	if first == nil || first[1].Str != "anna" || first[2].Integer != 11 {
		t.Fatalf("updated row 1 = %#v", first)
	}

	second := rowsByID[2]
	if second == nil || second[1].Str != "bob" {
		t.Fatalf("unchanged row 2 = %#v", second)
	}

	if _, exists := rowsByID[3]; exists {
		t.Fatal("old primary key 3 still exists")
	}

	moved := rowsByID[33]
	if moved == nil || moved[1].Str != "carol" {
		t.Fatalf("moved row 33 = %#v", moved)
	}
}

func TestSessionUpdateWithComparisonFilter(t *testing.T) {
	session := NewSession(NewKVEngine(storage.NewMemoryEngine()))

	if _, err := session.Execute(`
		CREATE TABLE scores (
			id INT PRIMARY KEY NOT NULL,
			score INT NULL,
			passed BOOL NOT NULL
		);
	`); err != nil {
		t.Fatal(err)
	}

	for _, sql := range []string{
		"INSERT INTO scores VALUES (1, 8, false);",
		"INSERT INTO scores VALUES (2, 18, false);",
		"INSERT INTO scores VALUES (3, 28, false);",
		"INSERT INTO scores VALUES (4, NULL, false);",
	} {
		if _, err := session.Execute(sql); err != nil {
			t.Fatal(err)
		}
	}

	result, err := session.Execute("UPDATE scores SET passed = true WHERE score > 10;")
	if err != nil {
		t.Fatal(err)
	}

	updateResult, ok := result.(executor.UpdateResult)
	if !ok {
		t.Fatalf("result = %T, want executor.UpdateResult", result)
	}
	if updateResult.Count != 2 {
		t.Fatalf("updated count = %d, want 2", updateResult.Count)
	}

	result, err = session.Execute("SELECT * FROM scores;")
	if err != nil {
		t.Fatal(err)
	}
	rowsResult, ok := result.(executor.RowsResult)
	if !ok {
		t.Fatalf("result = %T, want executor.RowsResult", result)
	}

	passedByID := make(map[int64]bool, len(rowsResult.Rows))
	for _, row := range rowsResult.Rows {
		passedByID[row[0].Integer] = row[2].Boolean
	}

	for _, id := range []int64{2, 3} {
		if !passedByID[id] {
			t.Fatalf("row %d was not updated", id)
		}
	}
	for _, id := range []int64{1, 4} {
		if passedByID[id] {
			t.Fatalf("row %d was unexpectedly updated", id)
		}
	}
}
