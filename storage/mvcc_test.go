package storage

import (
	"bytes"
	"testing"
)

func TestMVCCTransactionSnapshotVisibility(t *testing.T) {
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

	// second 在 first 写入前已经开始，因此 first 的版本不在它的快照中。
	v, err := second.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Fatalf("value before first commit = %q, want nil", v)
	}

	entries, err := second.ScanPrefix([]byte("na"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entry count before first commit = %d, want 0", len(entries))
	}

	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}

	// first 后来提交也不会改变 second 在 Begin 时固定的快照。
	v, err = second.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if v != nil {
		t.Fatalf("value after first commit = %q, want nil", v)
	}

	if err := second.Rollback(); err != nil {
		t.Fatal(err)
	}

	// first 提交后才开始的事务能够看到其已提交版本。
	third, err := mvcc.Begin()
	if err != nil {
		t.Fatal(err)
	}

	v, err = third.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(v, []byte("alice")) {
		t.Fatalf("new transaction value = %q, want alice", v)
	}

	entries, err = third.ScanPrefix([]byte("na"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("new transaction entry count = %d, want 1", len(entries))
	}

	if err := third.Commit(); err != nil {
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

	encodedNext, err := engine.Get(nextVersionKey().encode())
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
