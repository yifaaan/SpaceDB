package engine

import (
	"spacedb/storage"
	"testing"
)

func TestKVEngineBeginsTransaction(t *testing.T) {
	engine := NewKVEngine()

	txn, err := engine.Begin()
	if err != nil {
		t.Fatal(err)
	}

	kvTxn, ok := txn.(*KVTransaction)
	if !ok {
		t.Fatalf("transaction = %T, want *KVTransaction", txn)
	}

	if kvTxn.txn == nil {
		t.Fatal("wrapped storage transaction is nil")
	}
}

func TestKVTransactionReturnsNotImplemented(t *testing.T) {
	engine := NewKVEngine()

	txn, err := engine.Begin()
	if err != nil {
		t.Fatal(err)
	}

	if err := txn.Commit(); err != storage.ErrNotImplemented {
		t.Fatalf("Commit error = %v, want storage.ErrNotImplemented", err)
	}
}
