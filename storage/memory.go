package storage

import (
	"bytes"
	"errors"
	"iter"
	"slices"
)

// ErrNilMemoryEngine 表示对 nil 内存引擎执行操作。
var ErrNilMemoryEngine = errors.New("storage: nil memory engine")

// MemoryEngine 使用 Go map 保存 KV 数据
type MemoryEngine struct {
	data map[string][]byte
}

func NewMemoryEngine() *MemoryEngine {
	return &MemoryEngine{
		data: make(map[string][]byte),
	}
}

func (me *MemoryEngine) Set(key, value []byte) error {
	me.data[string(key)] = bytes.Clone(value)
	return nil
}

func (me *MemoryEngine) Get(key []byte) ([]byte, error) {
	v, ok := me.data[string(key)]
	if !ok {
		return nil, nil
	}

	return bytes.Clone(v), nil
}

func (m *MemoryEngine) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

// Scan 返回 [start, end) 范围内的记录。
// nil start 表示没有下界，nil end 表示没有上界。
func (me *MemoryEngine) Scan(start, end []byte) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		entries := make([]Entry, 0, len(me.data))

		for k, v := range me.data {
			kBytes := []byte(k)

			if start != nil && bytes.Compare(kBytes, start) < 0 {
				continue
			}

			if end != nil && bytes.Compare(kBytes, end) >= 0 {
				continue
			}

			entries = append(entries, Entry{Key: kBytes, Value: bytes.Clone(v)})
		}

		slices.SortFunc(entries, func(a, b Entry) int {
			return bytes.Compare(a.Key, b.Key)
		})

		for _, entry := range entries {
			if !yield(entry, nil) {
				return
			}
		}
	}
}

func (me *MemoryEngine) ScanPrefix(prefix []byte) iter.Seq2[Entry, error] {
	return me.Scan(prefix, prefixEnd(prefix))
}

// prefixEnd 计算所有指定前缀 key 的排除上界
//
// 例如：
//
//	"ca" -> "cb"
//	{0x12, 0xff} -> {0x13}
//
// 全部是 0xff 时不存在有限上界，因此返回 nil
func prefixEnd(prefix []byte) []byte {
	end := bytes.Clone(prefix)

	for i := len(end) - 1; i >= 0; i-- {
		if end[i] == 0xff {
			continue
		}
		end[i]++
		return end[:i+1]
	}
	return nil
}

var _ Engine = (*MemoryEngine)(nil)
