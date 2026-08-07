package engine

import (
	"errors"
	"spacedb/executor"
	"spacedb/storage"
	"testing"
)

func TestSessionCreateTable(t *testing.T) {
	engine := NewKVEngine(storage.NewMemoryEngine())
	session := NewSession(engine)

	result, err := session.Execute("CREATE TABLE users (id INT PRIMARY KEY, name STRING);")

	if err != nil {
		t.Fatal(err)
	}

	createResult, ok := result.(executor.CreateTableResult)
	if !ok {
		t.Fatalf("result = %T, want CreateTableResult", result)
	}

	if createResult.TableName != "users" {
		t.Fatalf(
			"table name = %q, want users", createResult.TableName)
	}

	// 第二次创建同名表失败
	_, err = session.Execute(
		"CREATE TABLE users (id INT PRIMARY KEY);",
	)
	if !errors.Is(err, ErrTableExists) {
		t.Fatalf("error = %v, want ErrTableExists", err)
	}
}

func TestSessionCreateTablePersistsSchema(t *testing.T) {
	engine := NewKVEngine(storage.NewMemoryEngine())
	session := NewSession(engine)

	if _, err := session.Execute("CREATE TABLE users (id INT PRIMARY KEY, name STRING);"); err != nil {
		t.Fatal(err)
	}

	txn, err := engine.Begin()
	if err != nil {
		t.Fatal(err)
	}

	table, err := txn.GetTable("users")
	if err != nil {
		t.Fatal(err)
	}
	if table == nil {
		t.Fatal("table was not persisted")
	}

	if len(table.Columns) != 2 {
		t.Fatalf("column count = %d, want 2", len(table.Columns))
	}
}
