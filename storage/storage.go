package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
)

// version 是 MVCC 使用的事务版本号
type version uint64

// MVCC 内部 key 使用独立命名空间，避免和当前 SQL 表、行数据的 key 冲突。
const (
	mvccMetadataPrefix          = "\x00spacedb\x00"
	mvccNextVersionKey          = mvccMetadataPrefix + "\x00"
	mvccActiveTransactionPrefix = mvccMetadataPrefix + "\x01"
)

// activeTransactionKey 为指定事务生成活跃标记 key
func activeTransactionKey(v version) []byte {
	key := make([]byte, len(mvccActiveTransactionPrefix)+8)
	copy(key, mvccActiveTransactionPrefix)
	binary.BigEndian.PutUint64(key[len(mvccActiveTransactionPrefix):], uint64(v))
	return key
}

// encodeVersion 把事务版本编码成固定 8 字节
func encodeVersion(v version) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, uint64(v))
	return encoded
}

// decodeVersion 解码固定长度的事务版本
func decodeVersion(encoded []byte) (version, error) {
	if len(encoded) != 8 {
		return 0, fmt.Errorf("storage: invalid version encoding: got %d bytes, want 8", len(encoded))
	}

	return version(binary.BigEndian.Uint64(encoded)), nil
}

// transactionState 是某个事务在 Begin 时得到的固定快照
type transactionState struct {
	// version 是当前事务自己的版本
	version version

	// activeVersions 是当前事务开始之前仍然活跃的事务
	//
	// 后面的可见性判断会使用这个集合：
	// 即使这些事务之后提交，它们写入的数据对当前事务仍然不可见
	activeVersions map[version]struct{}
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

	next := version(1)
	encodedNext, err := m.state.engine.Get([]byte(mvccNextVersionKey))
	if err != nil {
		return nil, fmt.Errorf("storage: loading next transaction version: %w", err)
	}
	if encodedNext != nil {
		next, err = decodeVersion(encodedNext)
		if err != nil {
			return nil, fmt.Errorf("storage: decoding next transaction version: %w", err)
		}
	}

	if next == ^version(0) {
		return nil, fmt.Errorf("storage: transaction version exhausted")
	}

	// 当前事务加入活跃集合前，先固定它开始时看到的活跃事务快照。
	activeVersions := make(map[version]struct{})
	activePrefix := []byte(mvccActiveTransactionPrefix)
	for entry, err := range m.state.engine.ScanPrefix(activePrefix) {
		if err != nil {
			return nil, fmt.Errorf("storage: scanning active transactions: %w", err)
		}
		if len(entry.Key) != len(activePrefix)+8 || !bytes.Equal(entry.Key[:len(activePrefix)], activePrefix) {
			return nil, fmt.Errorf("storage: invalid active transaction key %x", entry.Key)
		}

		activeVersion := version(binary.BigEndian.Uint64(entry.Key[len(activePrefix):]))
		activeVersions[activeVersion] = struct{}{}
	}

	// 当前事务使用 next，因此持久化 next+1，留给下一个事务
	if err := m.state.engine.Set([]byte(mvccNextVersionKey), encodeVersion(next+1)); err != nil {
		return nil, fmt.Errorf("storage: saving next transaction version: %w", err)
	}
	if err := m.state.engine.Set(activeTransactionKey(next), []byte{}); err != nil {
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
