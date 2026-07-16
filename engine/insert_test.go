package engine

import (
	"testing"

	"spacedb/executor"
)

func TestSessionInsert(t *testing.T) {
	engine := NewKVEngine()
	session := NewSession(engine)

	_, err := session.Execute(`
		CREATE TABLE users (
			id INT NOT NULL,
			name STRING DEFAULT 'guest'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	result, err := session.Execute("INSERT INTO users VALUES (1);")
	if err != nil {
		t.Fatal(err)
	}

	insertResult, ok := result.(executor.InsertResult)
	if !ok {
		t.Fatalf("result = %T, want executor.InsertResult", result)
	}
	if insertResult.Count != 1 {
		t.Fatalf("insert count = %d, want 1", insertResult.Count)
	}

	// 测试显式列顺序与表结构顺序不同。
	result, err = session.Execute("INSERT INTO users (name, id) VALUES ('alice', 2);")
	if err != nil {
		t.Fatal(err)
	}

	insertResult = result.(executor.InsertResult)
	if insertResult.Count != 1 {
		t.Fatalf("insert count = %d, want 1", insertResult.Count)
	}
}

func TestSessionInsertTypeMismatch(t *testing.T) {
	session := NewSession(NewKVEngine())

	if _, err := session.Execute("CREATE TABLE users (id INT NOT NULL);"); err != nil {
		t.Fatal(err)
	}

	_, err := session.Execute("INSERT INTO users VALUES ('not-an-int');")
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestSessionInsertMissingRequiredValue(t *testing.T) {
	session := NewSession(NewKVEngine())

	if _, err := session.Execute("CREATE TABLE users (id INT NOT NULL, name STRING NOT NULL);"); err != nil {
		t.Fatal(err)
	}

	_, err := session.Execute("INSERT INTO users VALUES (1);")
	if err == nil {
		t.Fatal("expected missing required value error")
	}
}
