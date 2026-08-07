package storage

import (
	"bytes"
	"fmt"
	"sync"
)

// transactionState 是某个事务在 Begin 时得到的固定快照
type transactionState struct {
	// version 当前事务的版本
	version version

	// activeVersions 是当前事务开始之前仍然活跃的事务
	//
	// 后面的可见性判断会使用这个集合：
	// 即使这些事务之后提交，它们写入的数据对当前事务仍然不可见
	activeVersions map[version]struct{}
}

// isVisible 判断指定版本的数据对当前事务是否可见
//
//  1. 当前事务开始时仍活跃的事务版本不可见
//  2. 比当前事务更新的版本不可见
//  3. 其余版本可见，包括当前事务自己的版本
func (s transactionState) isVisible(v version) bool {
	if _, ok := s.activeVersions[v]; ok {
		return false
	}
	return v <= s.version
}

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

// Begin 原子地分配事务版本并获取活跃事务快照
func (m *MVCC) Begin() (*MVCCTransaction, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	// 获取事务序列号
	next := version(1)
	encodedNext, err := m.state.engine.Get([]byte(nextVersionKey().encode()))
	if err != nil {
		return nil, fmt.Errorf("storage: loading next transaction version: %w", err)
	}
	if encodedNext != nil {
		next, err = decodeVersion(encodedNext)
		if err != nil {
			return nil, fmt.Errorf("storage: decoding next transaction version: %w", err)
		}
	}

	// 构建当前活跃事务表
	activeVersions := make(map[version]struct{})
	activePrefix := activeTransactionPrefix()
	for entry, err := range m.state.engine.ScanPrefix(activePrefix) {
		if err != nil {
			return nil, fmt.Errorf("storage: scanning active transactions: %w", err)
		}
		key, err := decodeMvccKey(entry.Key)
		if err != nil {
			return nil, err
		}
		if key.kind != mvccKeyTxnActive {
			return nil, fmt.Errorf("storage: unexpected MVCC key kind %d", key.kind)
		}

		activeVersions[key.version] = struct{}{}
	}

	// 当前事务使用 next，持久化序列号 next+1
	if err := m.state.engine.Set(nextVersionKey().encode(), encodeVersion(next+1)); err != nil {
		return nil, fmt.Errorf("storage: saving next transaction version: %w", err)
	}
	if err := m.state.engine.Set(activeTransactionKey(next).encode(), nil); err != nil {
		return nil, fmt.Errorf("storage: marking transaction %d active: %w", next, err)
	}

	return &MVCCTransaction{
		shared: m.state,
		state: transactionState{
			version:        next,
			activeVersions: activeVersions,
		},
	}, nil
}

// MVCCTransaction 是 SQL 层使用的底层事务
type MVCCTransaction struct {
	// shared 指向所有事务共享的底层引擎和互斥锁
	shared *mvccState

	// state 是当前事务在 Begin 时生成的固定快照状态
	state transactionState
}

func (t *MVCCTransaction) Commit() error {
	return nil
}

func (t *MVCCTransaction) Rollback() error {
	return nil
}

func (t *MVCCTransaction) Set(key, value []byte) error {
	t.shared.mu.Lock()
	defer t.shared.mu.Unlock()

	return t.shared.engine.Set(key, value)
}

// Get 在事务中读取一个 KV
func (t *MVCCTransaction) Get(key []byte) ([]byte, error) {

	t.shared.mu.RLock()
	defer t.shared.mu.RUnlock()

	return t.shared.engine.Get(key)
}

// Delete 在事务中删除一个 KV
func (t *MVCCTransaction) Delete(key []byte) error {
	t.shared.mu.Lock()
	defer t.shared.mu.Unlock()

	return t.shared.engine.Delete(key)
}

func (t *MVCCTransaction) ScanPrefix(prefix []byte) ([]Entry, error) {
	t.shared.mu.RLock()
	defer t.shared.mu.RUnlock()

	entries := make([]Entry, 0)

	for entry, err := range t.shared.engine.ScanPrefix(prefix) {
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
