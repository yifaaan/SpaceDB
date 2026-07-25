package storage

import (
	"bytes"
	"iter"
	"slices"
)

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

func (me *MemoryEngine) scan(start, end []byte, reverse bool) iter.Seq2[Entry, error] {
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

			entries = append(entries, Entry{
				Key:   bytes.Clone(kBytes),
				Value: bytes.Clone(v),
			})
		}

		slices.SortFunc(entries, func(a, b Entry) int {
			return bytes.Compare(a.Key, b.Key)
		})

		if reverse {
			slices.Reverse(entries)
		}

		for _, entry := range entries {
			if !yield(entry, nil) {
				return
			}
		}
	}
}

// Scan 返回 [start, end) 范围内的记录。
// nil start 表示没有下界，nil end 表示没有上界。
func (me *MemoryEngine) Scan(start, end []byte) iter.Seq2[Entry, error] {
	return me.scan(start, end, false)
}

func (me *MemoryEngine) ScanReverse(start, end []byte) iter.Seq2[Entry, error] {
	return me.scan(start, end, true)
}

// ScanPrefix 返回所有以 prefix 开头的记录。
func (me *MemoryEngine) ScanPrefix(prefix []byte) iter.Seq2[Entry, error] {
	return me.Scan(prefix, prefixEnd(prefix))
}

func (me *MemoryEngine) ScanPrefixReverse(prefix []byte) iter.Seq2[Entry, error] {
	return me.ScanReverse(prefix, prefixEnd(prefix))
}

var _ Engine = (*MemoryEngine)(nil)

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
