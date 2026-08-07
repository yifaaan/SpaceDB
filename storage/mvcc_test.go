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

func TestMVCCBeginAllocatesVersionAndCapturesActiveTransactions(t *testing.T) {
	engine := NewMemoryEngine()
	mvcc := NewMVCC(engine)

	first, err := mvcc.Begin()
	if err != nil {
		t.Fatal(err)
	}

	if first.state.version != 1 {
		t.Fatalf("first version = %d, want 1", first.state.version)
	}
	if len(first.state.activeVersions) != 0 {
		t.Fatalf(
			"first active versions = %v, want empty",
			first.state.activeVersions,
		)
	}

	second, err := mvcc.Begin()
	if err != nil {
		t.Fatal(err)
	}

	if second.state.version != 2 {
		t.Fatalf("second version = %d, want 2", second.state.version)
	}

	// 第二个事务开始时，第一个事务仍然活跃。
	if _, ok := second.state.activeVersions[1]; !ok {
		t.Fatal("second transaction did not capture active version 1")
	}

	// 当前事务不能出现在自己的活跃事务快照中。
	if _, ok := second.state.activeVersions[2]; ok {
		t.Fatal("second transaction contains itself in active versions")
	}

	encodedNext, err := engine.Get([]byte(mvccNextVersionKey))
	if err != nil {
		t.Fatal(err)
	}

	next, err := decodeVersion(encodedNext)
	if err != nil {
		t.Fatal(err)
	}
	if next != 3 {
		t.Fatalf("stored next version = %d, want 3", next)
	}
}
