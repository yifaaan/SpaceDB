package storage

import (
	"bytes"
	"errors"
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

var _ Engine = (*MemoryEngine)(nil)
