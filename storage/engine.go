package storage

import "iter"

// Entry 是扫描返回的一条 KV 记录
type Entry struct {
	Key   []byte
	Value []byte
}

// Engine 最底层 KV 存储的接口。
//
// key 和 value 都使用字节切片
type Engine interface {
	Set(key, value []byte) error

	Get(key []byte) ([]byte, error)

	Delete(key []byte) error

	// Scan 返回 [start, end) 范围内的记录。
	// nil start 表示没有下界，nil end 表示没有上界。
	Scan(start, end []byte) iter.Seq2[Entry, error]

	// ScanPrefix 返回所有以 prefix 开头的记录。
	ScanPrefix(prefix []byte) iter.Seq2[Entry, error]
}
