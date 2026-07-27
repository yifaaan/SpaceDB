package storage

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const (
	// 每条日志记录的头部固定为 8 字节
	//
	//	4 字节 key 长度 + 4 字节 value 长度
	diskLogHeaderSize = 8

	// value 长度使用 int32 保存
	// -1 表示这不是普通数据，而是一条删除记录
	diskLogTombstoneSize int32 = -1

	maxDiskLogValueSize = int64(1<<31 - 1)
	maxDiskLogEntrySize = uint64(1<<32 - 1)
)

// diskLog 追加写日志文件
type diskLog struct {
	file *os.File
}

func newDisLog(name string) (*diskLog, error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("storage: opening disk log %q: %w", name, err)
	}
	return &diskLog{file: file}, nil
}

// diskLogPosition 一条日志记录在文件中的位置
type diskLogPosition struct {
	EntryOffset int64  // 记录的位置
	EntrySize   uint32 // 记录大小
}

// writeEntry 将一条记录追加到日志文件末尾。
//
// 日志格式：
//
//	+-------------+---------------+----------+------------+
//	| key len(4)  | value len(4)  | key(N)   | value(M)   |
//	+-------------+---------------+----------+------------+
//
// 所有整数都使用大端序，保证磁盘格式与机器字节序无关。
//
// deleted 为 true 时，value length 写成 -1，并且不写入 value。
// 不能使用 value == nil 判断删除，因为数据库允许保存空 value。
func (d *diskLog) writeEntry(key []byte, value []byte, deleted bool) (diskLogPosition, error) {
	if int64(len(value)) > maxDiskLogValueSize {
		return diskLogPosition{}, fmt.Errorf("storage: value is too large: %d bytes", len(value))
	}

	valueSize := int32(len(value))
	if deleted {
		valueSize = diskLogTombstoneSize
	}

	// 被删除的记录，value 长度为零
	storedValueSize := len(value)
	if deleted {
		storedValueSize = 0
	}

	entrySize := uint64(diskLogHeaderSize) + uint64(len(key)) + uint64(storedValueSize)

	if entrySize > maxDiskLogEntrySize {
		return diskLogPosition{}, fmt.Errorf("storage: log entry is too large: %d bytes", entrySize)
	}

	// 文件末尾
	entryOffset, err := d.file.Seek(0, io.SeekEnd)
	if err != nil {
		return diskLogPosition{}, fmt.Errorf("storage: seeking to end of disk log: %w", err)
	}

	var header [diskLogHeaderSize]byte

	binary.BigEndian.PutUint32(header[:4], uint32(len(key)))
	binary.BigEndian.PutUint32(header[4:], uint32(valueSize))

	writer := bufio.NewWriterSize(d.file, int(entrySize))

	if _, err := writer.Write(header[:]); err != nil {
		return diskLogPosition{}, fmt.Errorf("storage: writing disk log header: %w", err)
	}
	if _, err := writer.Write(key); err != nil {
		return diskLogPosition{}, fmt.Errorf("storage: writing disk log key: %w", err)
	}
	if !deleted {
		if _, err := writer.Write(value); err != nil {
			return diskLogPosition{}, fmt.Errorf("storage: writing disk log value: %w", err)
		}
	}

	// flush
	if err := writer.Flush(); err != nil {
		return diskLogPosition{}, fmt.Errorf("storage: flushing disk log entry: %w", err)
	}

	return diskLogPosition{
		EntryOffset: entryOffset,
		EntrySize:   uint32(entrySize),
	}, nil
}

// readValue 根据 value 的文件偏移和长度读取数据
//
// 这里使用 ReadAt，而不是先 Seek 再 Read：
// ReadAt 不会改变文件的共享游标
func (d *diskLog) readValue(offset int64, size uint32) ([]byte, error) {
	value := make([]byte, int(size))

	if _, err := d.file.ReadAt(value, offset); err != nil {
		return nil, fmt.Errorf("storage: reading %d bytes at offset %d: %w", size, offset, err)
	}

	return value, nil
}

func (d *diskLog) close() error {
	return d.file.Close()
}

// readEntry 从 offset 处读取一条记录，返回 key, valueSize, err
func (d *diskLog) readEntry(offset int64) ([]byte, int32, error) {
	var header [diskLogHeaderSize]byte

	n, err := d.file.ReadAt(header[:], offset)
	if err != nil {
		return nil, 0, fmt.Errorf("storage: reading log header at offset %d: %w", offset, err)
	}
	keySize := binary.BigEndian.Uint32(header[:4])
	valueSize := int32(binary.BigEndian.Uint32(header[4:]))

	key := make([]byte, int(keySize))

	_, err = d.file.ReadAt(key, offset+int64(n))
	if err != nil {
		return nil, 0, fmt.Errorf("storage: reading log key at offset %d: %w", offset, err)
	}
	n += int(keySize)

	return key, valueSize, nil
}
