package storage

import (
	"bytes"
	"iter"
	"path/filepath"
	"slices"
	"testing"
)

func TestDiskEnginePointOperations(t *testing.T) {
	engine, err := NewDiskEngine(filepath.Join(t.TempDir(), "spacedb.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	if err := engine.Set([]byte("name"), []byte("alice")); err != nil {
		t.Fatal(err)
	}

	value, err := engine.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte("alice")) {
		t.Fatalf("value = %q, want alice", value)
	}

	// 覆盖旧值。
	if err := engine.Set([]byte("name"), []byte("bob")); err != nil {
		t.Fatal(err)
	}

	value, err = engine.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte("bob")) {
		t.Fatalf("value = %q, want bob", value)
	}

	if err := engine.Delete([]byte("name")); err != nil {
		t.Fatal(err)
	}

	value, err = engine.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("deleted value = %q, want nil", value)
	}
}

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

func TestDiskEngineScan(t *testing.T) {
	engine, err := NewDiskEngine(filepath.Join(t.TempDir(), "spacedb.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	for key, value := range map[string]string{
		"aa": "value-aa",
		"ab": "old-ab",
		"ba": "value-ba",
		"bb": "value-bb",
		"ca": "value-ca",
	} {
		if err := engine.Set([]byte(key), []byte(value)); err != nil {
			t.Fatal(err)
		}
	}

	// 覆盖后，扫描只能读到最新位置。
	if err := engine.Set([]byte("ab"), []byte("new-ab")); err != nil {
		t.Fatal(err)
	}

	// 删除后，B-tree 中不再存在 ba。
	if err := engine.Delete([]byte("ba")); err != nil {
		t.Fatal(err)
	}

	forward := collectEntries(t, engine.Scan([]byte("aa"), []byte("ca")))

	if want := []string{"aa", "ab", "bb"}; !slices.Equal(entryKeys(forward), want) {
		t.Fatalf("forward keys = %v, want %v", entryKeys(forward), want)
	}

	if !bytes.Equal(forward[1].Value, []byte("new-ab")) {
		t.Fatalf("ab value = %q, want new-ab", forward[1].Value)
	}

	reverse := collectEntries(t, engine.ScanReverse([]byte("aa"), []byte("ca")))

	if want := []string{"bb", "ab", "aa"}; !slices.Equal(entryKeys(reverse), want) {
		t.Fatalf("reverse keys = %v, want %v", entryKeys(reverse), want)
	}
}

func TestDiskEngineScanPrefix(t *testing.T) {
	engine, err := NewDiskEngine(filepath.Join(t.TempDir(), "spacedb.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	for _, key := range []string{"user:1", "user:2", "order:1"} {
		if err := engine.Set([]byte(key), []byte("value")); err != nil {
			t.Fatal(err)
		}
	}

	entries := collectEntries(t, engine.ScanPrefix([]byte("user:")))

	if want := []string{"user:1", "user:2"}; !slices.Equal(entryKeys(entries), want) {
		t.Fatalf("prefix keys = %v, want %v", entryKeys(entries), want)
	}
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
