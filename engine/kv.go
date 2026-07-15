package engine

import (
	"spacedb/executor"
	"spacedb/schema"
	"spacedb/storage"
	"spacedb/types"
)

// KVEngine 对 storage.MVCC底层存储 的 SQL Engine 封装。
type KVEngine struct {
	storage *storage.MVCC
}

func NewKVEngine() *KVEngine {
	return &KVEngine{
		storage: storage.NewMVCC(storage.NewMemoryEngine()),
	}
}

func (e *KVEngine) Begin() (executor.Transaction, error) {
	txn, err := e.storage.Begin()
	if err != nil {
		return nil, err
	}
	return &KVTransaction{txn: txn}, nil
}

// KVTransaction 将底层 MVCCTransaction 适配成 executor.Transaction。
type KVTransaction struct {
	txn *storage.MVCCTransaction
}

func (t *KVTransaction) Commit() error {
	return nil
}

func (t *KVTransaction) Rollback() error {
	return nil
}

func (t *KVTransaction) CreateRow(string, types.Row) error {
	return nil
}

func (t *KVTransaction) ScanTable(string) ([]types.Row, error) {
	return nil, nil
}

func (t *KVTransaction) CreateTable(schema.Table) error {
	return nil
}

func (t *KVTransaction) GetTable(string) (*schema.Table, error) {
	return nil, nil
}

var _ executor.Transaction = (*KVTransaction)(nil)
var _ Engine = (*KVEngine)(nil)
