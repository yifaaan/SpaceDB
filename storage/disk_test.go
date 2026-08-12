package storage

import (
	"bytes"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func collectEntries(t *testing.T, sequence iter.Seq2[Entry, error]) []Entry {
	t.Helper()

	var entries []Entry
	for entry, err := range sequence {
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func entryKeys(entries []Entry) []string {
	keys := make([]string, len(entries))
	for i, entry := range entries {
		keys[i] = string(entry.Key)
	}
	return keys
}

func TestDiskEngineRebuildsKeydir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spacedb.log")

	engine, err := NewDiskEngine(path)
	if err != nil {
		t.Fatal(err)
	}

	// 同一个 key 写入两次，恢复时应由后面的日志记录覆盖前面的旧位置。
	if err := engine.Set([]byte("name"), []byte("alice")); err != nil {
		t.Fatal(err)
	}
	if err := engine.Set([]byte("name"), []byte("bob")); err != nil {
		t.Fatal(err)
	}

	// 删除会追加墓碑。恢复时墓碑必须从 keydir 删除对应 key，
	// 同时继续推进 offset，读取它后面的记录。
	if err := engine.Set([]byte("deleted"), []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := engine.Delete([]byte("deleted")); err != nil {
		t.Fatal(err)
	}
	if err := engine.Set([]byte("after"), []byte("tombstone")); err != nil {
		t.Fatal(err)
	}

	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewDiskEngine(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })

	value, err := reopened.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte("bob")) {
		t.Fatalf("reopened name = %q, want bob", value)
	}

	value, err = reopened.Get([]byte("deleted"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("reopened deleted value = %q, want nil", value)
	}

	entries := collectEntries(t, reopened.Scan(nil, nil))
	if want := []string{"after", "name"}; !slices.Equal(entryKeys(entries), want) {
		t.Fatalf("reopened keys = %v, want %v", entryKeys(entries), want)
	}
}

func TestNewDiskEngineCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spacedb.log")

	engine, err := NewDiskEngine(path)
	if err != nil {
		t.Fatal(err)
	}

	// name 的旧值不应出现在压缩后的日志中
	if err := engine.Set([]byte("name"), []byte("alice")); err != nil {
		t.Fatal(err)
	}
	if err := engine.Set([]byte("name"), []byte("bob")); err != nil {
		t.Fatal(err)
	}

	// deleted 的普通记录和删除墓碑都不应保留
	if err := engine.Set([]byte("deleted"), []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := engine.Delete([]byte("deleted")); err != nil {
		t.Fatal(err)
	}

	if err := engine.Set([]byte("active"), []byte("yes")); err != nil {
		t.Fatal(err)
	}

	// 压缩前全量扫描，只应看到每个 key 的最新值
	entriesBefore := collectEntries(t, engine.Scan(nil, nil))

	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	compacted, err := NewDiskEngineCompact(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compacted.Close() })

	// 最新值必须保留。
	value, err := compacted.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte("bob")) {
		t.Fatalf("name = %q, want bob", value)
	}

	// 已删除的 key 不能复活。
	value, err = compacted.Get([]byte("deleted"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("deleted = %q, want nil", value)
	}

	// 压缩前后的全量扫描必须完全一致
	entriesEqual := func(a, b []Entry) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if !bytes.Equal(a[i].Key, b[i].Key) || !bytes.Equal(a[i].Value, b[i].Value) {
				return false
			}
		}
		return true
	}
	entriesAfter := collectEntries(t, compacted.Scan(nil, nil))
	if !entriesEqual(entriesBefore, entriesAfter) {
		t.Fatalf("scan after compact = %v, want %v", entriesAfter, entriesBefore)
	}
	if want := []string{"active", "name"}; !slices.Equal(entryKeys(entriesAfter), want) {
		t.Fatalf("compacted keys = %v, want %v", entryKeys(entriesAfter), want)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Size() >= before.Size() {
		t.Fatalf("compacted size = %d, original size = %d", after.Size(), before.Size())
	}
}
