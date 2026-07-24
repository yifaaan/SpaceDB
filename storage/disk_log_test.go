package storage

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskLogWriteAndReadValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spacedb.log")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	log := &diskLog{file: file}

	position, err := log.writeEntry([]byte("name"), []byte("alice"), false)
	if err != nil {
		t.Fatal(err)
	}

	if position.EntryOffset != 0 {
		t.Fatalf("entry offset = %d, want 0", position.EntryOffset)
	}

	// value 位于 8 字节头部和 4 字节 key 之后。
	if position.ValueOffset != 12 {
		t.Fatalf("value offset = %d, want 12", position.ValueOffset)
	}

	value, err := log.readValue(position.ValueOffset, position.ValueSize)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(value, []byte("alice")) {
		t.Fatalf("value = %q, want alice", value)
	}
}

func TestDiskLogDistinguishesEmptyValueAndDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spacedb.log")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	log := &diskLog{file: file}

	empty, err := log.writeEntry([]byte("empty"), []byte{}, false)
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := log.writeEntry([]byte("deleted"), nil, true)
	if err != nil {
		t.Fatal(err)
	}

	var emptyHeader [diskLogHeaderSize]byte
	if _, err := file.ReadAt(emptyHeader[:], empty.EntryOffset); err != nil {
		t.Fatal(err)
	}

	emptyValueSize := int32(binary.BigEndian.Uint32(emptyHeader[4:8]))
	if emptyValueSize != 0 {
		t.Fatalf("empty value size = %d, want 0", emptyValueSize)
	}

	var deletedHeader [diskLogHeaderSize]byte
	if _, err := file.ReadAt(deletedHeader[:], deleted.EntryOffset); err != nil {
		t.Fatal(err)
	}

	deletedValueSize := int32(
		binary.BigEndian.Uint32(deletedHeader[4:8]),
	)
	if deletedValueSize != diskLogTombstoneSize {
		t.Fatalf("deleted value size = %d, want %d", deletedValueSize, diskLogTombstoneSize)
	}
}

func TestDiskLogAppendsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spacedb.log")

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	log := &diskLog{file: file}

	first, err := log.writeEntry([]byte("key"), []byte("first"), false)
	if err != nil {
		t.Fatal(err)
	}

	second, err := log.writeEntry([]byte("key"), []byte("second"), false)
	if err != nil {
		t.Fatal(err)
	}

	// 第二条记录必须紧跟在第一条记录之后
	if second.EntryOffset !=
		first.EntryOffset+int64(first.EntrySize) {
		t.Fatalf("second offset = %d, want %d", second.EntryOffset, first.EntryOffset+int64(first.EntrySize))
	}
}
