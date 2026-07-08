package engine

import (
	"spacedb/executor"
	"spacedb/schema"
	"spacedb/types"
	"testing"
)

type fakeEngine struct {
	txn    *fakeTransaction
	begins int
}

func (e *fakeEngine) Begin() (executor.Transaction, error) {
	e.begins++
	return e.txn, nil
}

type fakeTransaction struct {
	committed  bool
	rolledBack bool
}

func (t *fakeTransaction) Commit() error {
	t.committed = true
	return nil
}

func (t *fakeTransaction) Rollback() error {
	t.rolledBack = true
	return nil
}

func (*fakeTransaction) CreateRow(string, types.Row) error {
	return nil
}

func (*fakeTransaction) ScanTable(string) ([]types.Row, error) {
	return nil, nil
}

func (*fakeTransaction) CreateTable(schema.Table) error {
	return nil
}

func (*fakeTransaction) GetTable(string) (*schema.Table, error) {
	return nil, nil
}

var _ executor.Transaction = (*fakeTransaction)(nil)

func TestSessionRollsBackWhenExecutorFails(t *testing.T) {
	txn := &fakeTransaction{}
	eng := &fakeEngine{txn: txn}
	session := NewSession(eng)

	_, err := session.Execute("SELECT * FROM users;")
	if err == nil {
		t.Fatal("Execute succeeded, want executor error")
	}

	if !txn.rolledBack {
		t.Fatal("transaction was not rolled back")
	}
	if txn.committed {
		t.Fatal("failed execution must not commit")
	}
}

func TestSessionDoesNotBeginTransactionWhenParsingFails(t *testing.T) {
	eng := &fakeEngine{txn: &fakeTransaction{}}
	session := NewSession(eng)

	_, err := session.Execute("SELECT FROM users;")
	if err == nil {
		t.Fatal("Execute succeeded for invalid SQL")
	}

	if eng.begins != 0 {
		t.Fatalf("begin count = %d, want 0", eng.begins)
	}
}
