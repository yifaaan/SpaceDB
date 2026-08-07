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
	t.shared.mu.Lock()
	defer t.shared.mu.Unlock()

	engine := t.shared.engine
	// 找到当前事务的写集合
	writeKeys := make([][]byte, 0)
	for entry, err := range engine.ScanPrefix(transactionWritePrefix(t.state.version)) {
		if err != nil {
			return fmt.Errorf("storage: scanning transaction %d writes: %w", t.state.version, err)
		}
		decoded, err := decodeMvccKey(entry.Key)
		if err != nil {
			return err
		}
		if decoded.kind != mvccKeyTxnWrite || decoded.version != t.state.version {
			return fmt.Errorf("storage: unexpected transaction-write key %x", entry.Key)
		}

		writeKeys = append(writeKeys, bytes.Clone(entry.Key))
	}
	for _, key := range writeKeys {
		if err := engine.Delete(key); err != nil {
			return fmt.Errorf("storage: deleting transaction-write key %x: %w", key, err)
		}
	}

	// 删除活跃事务标记
	if err := engine.Delete(activeTransactionKey(t.state.version).encode()); err != nil {
		return fmt.Errorf("storage: marking transaction %d committed: %w", t.state.version, err)
	}

	return nil
}

func (t *MVCCTransaction) Rollback() error {
	t.shared.mu.Lock()
	defer t.shared.mu.Unlock()

	engine := t.shared.engine
	// 找到当前事务的写集合和插入的用户key
	allKeys := make([][]byte, 0)
	for entry, err := range engine.ScanPrefix(transactionWritePrefix(t.state.version)) {
		if err != nil {
			return fmt.Errorf("storage: scanning transaction %d writes: %w", t.state.version, err)
		}
		decoded, err := decodeMvccKey(entry.Key)
		if err != nil {
			return err
		}
		if decoded.kind != mvccKeyTxnWrite || decoded.version != t.state.version {
			return fmt.Errorf("storage: unexpected transaction-write key %x", entry.Key)
		}

		allKeys = append(allKeys, bytes.Clone(entry.Key))
		allKeys = append(allKeys, versionedKey(decoded.rawKey, decoded.version).encode())
	}
	for _, key := range allKeys {
		if err := engine.Delete(key); err != nil {
			return fmt.Errorf("storage: deleting key %x: %w", key, err)
		}
	}

	// 删除活跃事务标记
	if err := engine.Delete(activeTransactionKey(t.state.version).encode()); err != nil {
		return fmt.Errorf("storage: marking transaction %d committed: %w", t.state.version, err)
	}

	return nil
}

func (t *MVCCTransaction) Set(key, value []byte) error {
	return t.write(key, value, false)
}

// Get 在事务中读取一个 KV
//
// 返回该 key 最新可见版本的 value；
// 最新可见版本是删除标记或 key 不存在时返回 nil。
func (t *MVCCTransaction) Get(key []byte) ([]byte, error) {
	t.shared.mu.RLock()
	defer t.shared.mu.RUnlock()

	engine := t.shared.engine

	// 扫描该 key 的所有版本，从新到旧
	from := versionedKey(key, 0).encode()
	maxKey := versionedKey(key, ^version(0)).encode()
	to := append(maxKey, 0)

	for entry, err := range engine.ScanReverse(from, to) {
		if err != nil {
			return nil, fmt.Errorf("storage: scanning versions for key %x: %w", key, err)
		}

		decoded, err := decodeMvccKey(entry.Key)
		if err != nil {
			return nil, err
		}

		if !t.state.isVisible(decoded.version) {
			continue
		}

		return decodeVersionedValue(entry.Value)
	}

	return nil, nil
}

// Delete 在事务中删除一个 KV
func (t *MVCCTransaction) Delete(key []byte) error {
	return t.write(key, nil, true)
}

// ScanPrefix 返回所有以 prefix 开头且最新可见版本不是删除标记的 KV
func (t *MVCCTransaction) ScanPrefix(prefix []byte) ([]Entry, error) {
	t.shared.mu.RLock()
	defer t.shared.mu.RUnlock()

	engine := t.shared.engine

	entries := make([]Entry, 0)

	// 当前正在聚合的 raw key 及其最新可见版本
	var currentKey []byte
	var currentValue []byte
	var currentVisible bool

	// flush 将 currentKey 的最新可见版本加入结果
	flush := func() {
		if currentVisible {
			entries = append(entries, Entry{
				Key:   currentKey,
				Value: currentValue,
			})
		}
	}

	for raw, err := range engine.ScanPrefix(versionedKeyPrefix(prefix)) {
		if err != nil {
			return nil, err
		}

		decoded, err := decodeMvccKey(raw.Key)
		if err != nil {
			return nil, err
		}

		if !bytes.Equal(decoded.rawKey, currentKey) {
			flush()
			currentKey = decoded.rawKey
			currentValue = nil
			currentVisible = false
		}

		if !t.state.isVisible(decoded.version) {
			continue
		}

		value, err := decodeVersionedValue(raw.Value)
		if err != nil {
			return nil, err
		}

		// 同一 raw key 的版本按升序排列，后出现的版本更新
		currentValue = value
		currentVisible = value != nil
	}

	flush()

	return entries, nil
}

// decodeVersionedValue 解码版本数据
//
// 第一个字节是类型标记：
//
//	0 = 删除
//	1 = 正常值，后续字节是实际 value
func decodeVersionedValue(encoded []byte) ([]byte, error) {
	if len(encoded) == 0 {
		return nil, fmt.Errorf("storage: truncated versioned value")
	}

	switch encoded[0] {
	case 0:
		return nil, nil
	case 1:
		return bytes.Clone(encoded[1:]), nil
	default:
		return nil, fmt.Errorf("storage: invalid versioned value tag %d", encoded[0])
	}
}

// write 更新/删除数据
func (t *MVCCTransaction) write(rawKey []byte, value []byte, deleted bool) error {
	t.shared.mu.Lock()
	defer t.shared.mu.Unlock()

	engine := t.shared.engine

	// 如果当前活跃事务集合为空，就从next+1开始
	fromVersion := t.state.version + 1
	for av := range t.state.activeVersions {
		fromVersion = min(fromVersion, av)
	}
	from := versionedKey(rawKey, fromVersion).encode()
	maxKey := versionedKey(rawKey, ^version(0)).encode()
	to := append(maxKey, 0)

	// 只检查范围内最新的版本(最后一个)
	//
	// 如果最新版本不可见，说明：
	//  1. 它来自当前事务开始时仍活跃的事务；或
	//  2. 它来自当前事务开始后创建的新事务。
	//
	// 两种情况都属于写冲突。
	for entry, err := range engine.ScanReverse(from, to) {
		if err != nil {
			return fmt.Errorf("storage: scanning versions for key %x: %w", rawKey, err)
		}
		key, err := decodeMvccKey(entry.Key)
		if err != nil {
			return err
		}
		if !t.state.isVisible(key.version) {
			return fmt.Errorf("storage: write conflict")
		}
		break
	}

	// 将当前key 加入当前事务的写集合
	// Commit 会删除该记录；
	// Rollback 会根据该记录找到并删除当前事务写入的数据版本。
	if err := engine.Set(transactionWriteKey(t.state.version, rawKey).encode(), nil); err != nil {
		return fmt.Errorf("storage: recording transaction write: %w", err)
	}

	// 写入用户value

	// 第一个字节区分删除标记和正常值：
	//
	//      0 = 删除
	//      1 = 正常值，后续字节是实际 value
	encodedValue := []byte{0}
	if !deleted {
		encodedValue[0] = 1
		encodedValue = append(encodedValue, value...)
	}

	if err := t.shared.engine.Set(versionedKey(rawKey, t.state.version).encode(), encodedValue); err != nil {
		return fmt.Errorf(
			"storage: writing version %d for key %x: %w", t.state.version, rawKey, err)
	}

	return nil
}
