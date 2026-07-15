package storage

import (
	"bytes"
	"testing"
)

func TestMVCCTransactionsShareState(t *testing.T) {
	mvcc := NewMVCC(NewMemoryEngine())

	first, err := mvcc.Begin()
	if err != nil {
		t.Fatal(err)
	}

	second, err := mvcc.Begin()
	if err != nil {
		t.Fatal(err)
	}

	if err := first.Set([]byte("name"), []byte("alice")); err != nil {
		t.Fatal(err)
	}

	v, err := second.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(v, []byte("alice")) {
		t.Fatalf("value = %q, want alice", v)
	}

	entries, err := second.ScanPrefix([]byte("na"))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}

	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := second.Rollback(); err != nil {
		t.Fatal(err)
	}
}
