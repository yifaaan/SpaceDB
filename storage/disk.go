package storage

import (
	"bytes"
	"errors"
	"fmt"
	"iter"
	"os"
	"sync"

	"github.com/google/btree"
)

// ErrDiskEngineClosed 表示磁盘引擎已经关闭
var ErrDiskEngineClosed = errors.New("storage: disk engine closed")

// diskItem 是 B-tree 中保存的一项索引记录
//
// key 是用户传入的原始 KV key。
// logPosition 指向日志文件中最新 value 的位置。
type diskItem struct {
	key         []byte
	logPosition diskLogPosition
}

func lessDiskItem(a, b *diskItem) bool {
	return bytes.Compare(a.key, b.key) < 0
}

// DiskEngine 是基于追加日志的磁盘 KV 引擎
type DiskEngine struct {
	mu     sync.RWMutex
	keydir *btree.BTreeG[*diskItem]
	log    *diskLog
}

// NewDiskEngine 创建一个磁盘引擎
func NewDiskEngine(name string) (*DiskEngine, error) {
	file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("storage: opening disk log %q: %w", name, err)
	}

	return &DiskEngine{
		keydir: btree.NewG(400, lessDiskItem),
		log:    &diskLog{file: file},
	}, nil
}

// Close 关闭磁盘日志文件
func (d *DiskEngine) Close() error {
	if d == nil {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.log == nil || d.log.file == nil {
		return nil
	}

	err := d.log.file.Close()
	d.log = nil
	d.keydir = nil

	if err != nil {
		return fmt.Errorf("storage: closing disk engine: %w", err)
	}
	return nil
}

// Set 追加写入一条新记录，并更新内存索引
//
// 必须先写日志，再更新 keydir：
// 如果日志写失败，旧的 keydir 仍然指向旧值，
// 数据不会因为一次失败写入而丢失。
func (d *DiskEngine) Set(key, value []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureOpen(); err != nil {
		return err
	}

	// 先写日志
	position, err := d.log.writeEntry(key, value, false)
	if err != nil {
		return fmt.Errorf("storage: setting key %q: %w", key, err)
	}

	// 更新内存索引
	d.keydir.ReplaceOrInsert(&diskItem{
		key:         key,
		logPosition: position,
	})

	return nil
}

// Get 通过 B-tree 查找 value 的日志位置，
// 再从磁盘中读取真正的 value。
func (d *DiskEngine) Get(key []byte) ([]byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if err := d.ensureOpen(); err != nil {
		return nil, err
	}

	item, ok := d.keydir.Get(&diskItem{key: key})
	if !ok {
		return nil, nil
	}

	position := item.logPosition

	value, err := d.log.readValue(position.ValueOffset, position.ValueSize)
	if err != nil {
		return nil, fmt.Errorf("storage: getting key %q: %w", key, err)
	}

	return value, nil
}

// Delete 先追加写入表示删除的记录，再从 B-tree 中删除索引。
func (d *DiskEngine) Delete(key []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureOpen(); err != nil {
		return err
	}

	if _, err := d.log.writeEntry(key, nil, true); err != nil {
		return fmt.Errorf("storage: deleting key %q: %w", key, err)
	}

	d.keydir.Delete(&diskItem{key: key})

	return nil
}

// ensureOpen 检查磁盘引擎是否仍然可以使用。
//
// 调用它时，调用方必须已经持有 d.mu
func (d *DiskEngine) ensureOpen() error {
	if d == nil || d.log == nil || d.log.file == nil {
		return ErrDiskEngineClosed
	}
	return nil
}

// snapshotRange 从 B-tree 中复制指定范围的索引
//
// 扫描范围统一为 [start, end)：
//   - start 包含在结果中；
//   - end 不包含在结果中；
//   - nil start 表示没有下界；
//   - nil end 表示没有上界。
func (d *DiskEngine) snapshotRange(start, end []byte, reverse bool) ([]diskItem, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if err := d.ensureOpen(); err != nil {
		return nil, err
	}

	items := make([]diskItem, 0)

	if start != nil && end != nil && bytes.Compare(start, end) >= 0 {
		return items, nil
	}

	appendItem := func(item *diskItem) bool {
		items = append(items, diskItem{
			key:         bytes.Clone(item.key),
			logPosition: item.logPosition,
		})
		return true
	}

	if !reverse {
		switch {
		case start != nil && end != nil: // start <= key < end
			d.keydir.AscendRange(&diskItem{key: start}, &diskItem{key: end}, appendItem)

		case start != nil: // start <= key
			d.keydir.AscendGreaterOrEqual(&diskItem{key: start}, appendItem)

		case end != nil: // key < end
			d.keydir.AscendLessThan(&diskItem{key: end}, appendItem)

		default: // all
			d.keydir.Ascend(appendItem)
		}

		return items, nil
	}

	appendReverseItem := func(item *diskItem) bool {
		if end != nil && bytes.Compare(item.key, end) >= 0 {
			return true
		}

		// B-tree 当前正在从大到小遍历。
		// 一旦 key < start，后面的 key 只会更小，立即停止
		if start != nil && bytes.Compare(item.key, start) < 0 {
			return false
		}

		return appendItem(item)
	}

	if end != nil {
		d.keydir.DescendLessOrEqual(&diskItem{key: end}, appendReverseItem)
	} else {
		d.keydir.Descend(appendReverseItem)
	}
	return items, nil
}

// readScanItem 根据快照中的日志位置读取一条记录
func (d *DiskEngine) readScanItem(item diskItem) (Entry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if err := d.ensureOpen(); err != nil {
		return Entry{}, err
	}

	value, err := d.log.readValue(item.logPosition.ValueOffset, item.logPosition.ValueSize)
	if err != nil {
		return Entry{}, fmt.Errorf("storage: reading scanned key %q: %w", item.key, err)
	}

	return Entry{Key: item.key, Value: value}, nil
}

func (d *DiskEngine) scan(start, end []byte, reverse bool) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		items, err := d.snapshotRange(start, end, reverse)
		if err != nil {
			yield(Entry{}, err)
			return
		}

		for _, item := range items {
			entry, err := d.readScanItem(item)
			if err != nil {
				yield(Entry{}, err)
				return
			}

			if !yield(entry, nil) {
				return
			}
		}
	}
}

// Scan 按 key 从小到大返回 [start, end)。
func (d *DiskEngine) Scan(start, end []byte) iter.Seq2[Entry, error] {
	return d.scan(start, end, false)
}

// ScanReverse 按 key 从大到小返回 [start, end)。
func (d *DiskEngine) ScanReverse(start, end []byte) iter.Seq2[Entry, error] {
	return d.scan(start, end, true)
}

// ScanPrefix 返回所有以 prefix 开头的记录。
func (d *DiskEngine) ScanPrefix(prefix []byte) iter.Seq2[Entry, error] {
	return d.Scan(prefix, prefixEnd(prefix))
}

// ScanPrefixReverse 反向返回所有以 prefix 开头的记录。
func (d *DiskEngine) ScanPrefixReverse(prefix []byte) iter.Seq2[Entry, error] {
	return d.ScanReverse(prefix, prefixEnd(prefix))
}

var _ Engine = (*DiskEngine)(nil)
