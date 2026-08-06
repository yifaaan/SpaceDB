package storage

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"
)

func newTestEngine(t *testing.T, name string) Engine {
	t.Helper()

	switch name {
	case "memory":
		return NewMemoryEngine()

	case "disk":
		engine, err := NewDiskEngine(filepath.Join(t.TempDir(), "spacedb.log"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = engine.Close() })
		return engine

	default:
		t.Fatalf("unknown engine %q", name)
		return nil
	}
}

// testPointOperations 测试点读：不存在、写入、覆盖、删除、空 key
func testPointOperations(t *testing.T, engine Engine) {
	t.Helper()

	// 获取一个不存在的 key
	value, err := engine.Get([]byte("not exist"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("missing value = %v, want nil", value)
	}

	// 获取一个存在的 key
	if err := engine.Set([]byte("aa"), []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	value, err = engine.Get([]byte("aa"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte{1, 2, 3, 4}) {
		t.Fatalf("value = %v, want [1 2 3 4]", value)
	}

	// 重复 Set 覆盖旧值
	if err := engine.Set([]byte("aa"), []byte{5, 6, 7, 8}); err != nil {
		t.Fatal(err)
	}
	value, err = engine.Get([]byte("aa"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte{5, 6, 7, 8}) {
		t.Fatalf("overwritten value = %v, want [5 6 7 8]", value)
	}

	// 删除之后再读取
	if err := engine.Delete([]byte("aa")); err != nil {
		t.Fatal(err)
	}
	value, err = engine.Get([]byte("aa"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("deleted value = %v, want nil", value)
	}

	// key、value 都为空的情况
	value, err = engine.Get([]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("empty key value = %v, want nil", value)
	}
	if err := engine.Set([]byte{}, []byte{}); err != nil {
		t.Fatal(err)
	}
	value, err = engine.Get([]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if value == nil || len(value) != 0 {
		t.Fatalf("empty value = %#v, want non-nil empty slice", value)
	}

	// 正常数据仍然可读
	if err := engine.Set([]byte("cc"), []byte{5, 6, 7, 8}); err != nil {
		t.Fatal(err)
	}
	value, err = engine.Get([]byte("cc"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte{5, 6, 7, 8}) {
		t.Fatalf("cc value = %v, want [5 6 7 8]", value)
	}
}

func testScanRange(t *testing.T, engine Engine) {
	t.Helper()

	for _, pair := range [][2]string{
		{"nnaes", "value1"},
		{"amhue", "value2"},
		{"meeae", "value3"},
		{"uujeh", "value4"},
		{"anehe", "value5"},
	} {
		if err := engine.Set([]byte(pair[0]), []byte(pair[1])); err != nil {
			t.Fatal(err)
		}
	}

	// 正向扫描 [a, e)，对应 Rust 的 next() 依次返回 amhue、anehe
	entries := collectEntries(t, engine.Scan([]byte("a"), []byte("e")))
	if want := []string{"amhue", "anehe"}; !slices.Equal(entryKeys(entries), want) {
		t.Fatalf("scan keys = %v, want %v", entryKeys(entries), want)
	}

	// 反向扫描 [b, z)，对应 Rust 的 next_back() 依次返回 uujeh、nnaes、meeae
	entries = collectEntries(t, engine.ScanReverse([]byte("b"), []byte("z")))
	if want := []string{"uujeh", "nnaes", "meeae"}; !slices.Equal(entryKeys(entries), want) {
		t.Fatalf("reverse keys = %v, want %v", entryKeys(entries), want)
	}
}

// testScanPrefix 测试前缀扫描
func testScanPrefix(t *testing.T, engine Engine) {
	t.Helper()

	for _, pair := range [][2]string{
		{"ccnaes", "value1"},
		{"camhue", "value2"},
		{"deeae", "value3"},
		{"eeujeh", "value4"},
		{"canehe", "value5"},
		{"aanehe", "value6"},
	} {
		if err := engine.Set([]byte(pair[0]), []byte(pair[1])); err != nil {
			t.Fatal(err)
		}
	}

	// 前缀 ca 只能命中 camhue、canehe
	entries := collectEntries(t, engine.ScanPrefix([]byte("ca")))
	if want := []string{"camhue", "canehe"}; !slices.Equal(entryKeys(entries), want) {
		t.Fatalf("prefix keys = %v, want %v", entryKeys(entries), want)
	}
}

// TestEngine 用同一套断言验证所有引擎实现
func TestEngine(t *testing.T) {
	for _, name := range []string{"memory", "disk"} {
		t.Run(name+"/point-operations", func(t *testing.T) {
			testPointOperations(t, newTestEngine(t, name))
		})
		t.Run(name+"/scan", func(t *testing.T) {
			testScanRange(t, newTestEngine(t, name))
		})
		t.Run(name+"/scan-prefix", func(t *testing.T) {
			testScanPrefix(t, newTestEngine(t, name))
		})
	}
}
