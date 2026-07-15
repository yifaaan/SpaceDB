package storage

import (
	"bytes"
	"sync"
)

// mvccState 是 MVCC 和所有事务共享的状态
type mvccState struct {
	mu     sync.RWMutex
	engine Engine
}

// MVCC 底层多版本并发控制存储,对底层 Engine 的事务封装
type MVCC struct {
	state *mvccState
}

// NewMVCC 使用指定的底层 KV 引擎创建 MVCC
func NewMVCC(engine Engine) *MVCC {
	return &MVCC{
		state: &mvccState{
			engine: engine,
		},
	}
}

// Begin 创建一个与当前 MVCC 共享状态的新事务
func (m *MVCC) Begin() (*MVCCTransaction, error) {
	return &MVCCTransaction{
		state: m.state,
	}, nil
}

// MVCCTransaction 是 SQL 层使用的底层事务
type MVCCTransaction struct {
	state *mvccState
}

func (t *MVCCTransaction) Commit() error {
	return nil
}

func (t *MVCCTransaction) Rollback() error {
	return nil
}

func (t *MVCCTransaction) Set(key, value []byte) error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()

	return t.state.engine.Set(key, value)
}

// Get 在事务中读取一个 KV
func (t *MVCCTransaction) Get(key []byte) ([]byte, error) {

	t.state.mu.RLock()
	defer t.state.mu.RUnlock()

	return t.state.engine.Get(key)
}

// Delete 在事务中删除一个 KV
func (t *MVCCTransaction) Delete(key []byte) error {
	t.state.mu.Lock()
	defer t.state.mu.Unlock()

	return t.state.engine.Delete(key)
}

func (t *MVCCTransaction) ScanPrefix(prefix []byte) ([]Entry, error) {
	t.state.mu.RLock()
	defer t.state.mu.RUnlock()

	entries := make([]Entry, 0)

	for entry, err := range t.state.engine.ScanPrefix(prefix) {
		if err != nil {
			return nil, err
		}

		entries = append(entries, Entry{
			Key:   bytes.Clone(entry.Key),
			Value: bytes.Clone(entry.Value),
		})
	}
	return entries, nil
}
