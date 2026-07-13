package storage

import (
	"bytes"
	"testing"
)

func TestMemoryEnginePointOperations(t *testing.T) {
	engine := NewMemoryEngine()

	value, err := engine.Get([]byte("missing"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("missing value = %v, want nil", value)
	}

	if err := engine.Set([]byte("name"), []byte("alice")); err != nil {
		t.Fatal(err)
	}

	value, err = engine.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte("alice")) {
		t.Fatalf("value = %q, want alice", value)
	}

	if err := engine.Set([]byte("name"), []byte("bob")); err != nil {
		t.Fatal(err)
	}

	value, err = engine.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(value, []byte("bob")) {
		t.Fatalf("overwritten value = %q, want bob", value)
	}

	if err := engine.Delete([]byte("name")); err != nil {
		t.Fatal(err)
	}

	value, err = engine.Get([]byte("name"))
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("deleted value = %v, want nil", value)
	}
}

func TestMemoryEngineSupportsEmptyKeyAndValue(t *testing.T) {
	engine := NewMemoryEngine()

	if err := engine.Set([]byte{}, []byte{}); err != nil {
		t.Fatal(err)
	}

	value, err := engine.Get([]byte{})
	if err != nil {
		t.Fatal(err)
	}
	if value == nil || len(value) != 0 {
		t.Fatalf("empty value = %#v, want non-nil empty slice", value)
	}
}
