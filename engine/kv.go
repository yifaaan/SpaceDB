package engine

import (
	"fmt"
	"spacedb/executor"
	"spacedb/schema"
	"spacedb/storage"
	"spacedb/types"
	"strings"

	jsoniter "github.com/json-iterator/go"
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

func (t *KVTransaction) CreateRow(tableName string, row types.Row) error {
	table, err := t.GetTable(tableName)
	if err != nil {
		return fmt.Errorf("engine: loading table %q: %w", tableName, err)
	}
	if table == nil {
		return fmt.Errorf("engine: table %q does not exist", tableName)
	}

	if len(row) == 0 {
		return fmt.Errorf("engine: cannot insert an empty row")
	}

	if len(row) != len(table.Columns) {
		return fmt.Errorf("engine: row length mismatch: got %d, columns %d", len(row), len(table.Columns))
	}

	// 存储层再次校验类型
	for i, column := range table.Columns {
		v := row[i]
		if v.Kind == types.ValueNull {
			if !column.Nullable {
				return fmt.Errorf("engine: column %q cannot be NULL", column.Name)
			}
			continue
		}

		actualType, ok := v.DataType()
		if !ok {
			return fmt.Errorf("engine: invalid value kind %d for column %q", v.Kind, column.Name)
		}
		if actualType != column.DataType {
			return fmt.Errorf("engine: column %q type mismatch: want %d, got %d", column.Name, column.DataType, actualType)
		}
	}

	key, err := rowKey(tableName, row[0])
	if err != nil {
		return err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	encoded, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("engine: encoding row for table %q: %w", tableName, err)
	}

	return t.txn.Set(key, encoded)
}

func (t *KVTransaction) ScanTable(string) ([]types.Row, error) {
	return nil, nil
}

func (t *KVTransaction) CreateTable(table schema.Table) error {
	// 判断表的有效性
	if table.Name == "" || len(table.Columns) == 0 {
		return fmt.Errorf("%w: table must have name and columns", ErrInvalidTable)
	}
	// 判断表是否存在
	exist, err := t.GetTable(table.Name)
	if err != nil {
		return err
	}
	if exist != nil {
		return fmt.Errorf("%w: %s", ErrTableExists, table.Name)
	}

	// 序列化
	key, err := tableKey(table.Name)
	if err != nil {
		return err
	}
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	encoded, err := json.Marshal(&table)
	if err != nil {
		return fmt.Errorf("engine: encoding table %q: %w", table.Name, err)
	}

	return t.txn.Set(key, encoded)

}

func (t *KVTransaction) GetTable(tableName string) (*schema.Table, error) {
	key, err := tableKey(tableName)
	if err != nil {
		return nil, err
	}

	encoded, err := t.txn.Get(key)
	if err != nil {
		return nil, err
	}
	if encoded == nil {
		return nil, nil
	}

	var table schema.Table
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	if err := json.Unmarshal(encoded, &table); err != nil {
		return nil, fmt.Errorf("engine: decoding table %q: %w", tableName, err)
	}
	return &table, nil
}

var _ executor.Transaction = (*KVTransaction)(nil)
var _ Engine = (*KVEngine)(nil)

// tableKey 为表结构生成专用的 KV key,
// 使用 table/ 前缀，避免和行数据 key 冲突
func tableKey(tableName string) ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	key := strings.Clone(tableName)
	key += "/"
	encoded, err := json.Marshal(key)
	if err != nil {
		return nil, fmt.Errorf("engine: encoding table name %q: %w", tableName, err)
	}
	return encoded, nil
}

// rowPrefixKey 返回某张表的行数据前缀
//
// 表结构使用：
//
//	"users/"
//
// 行数据使用：
//
//	"users/row/" + 主键编码
func rowPrefixKey(tableName string) ([]byte, error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	prefix := tableName + "/row/"

	encoded, err := json.Marshal(prefix)
	if err != nil {
		return nil, fmt.Errorf("engine: encoding row prefix for table %q: %w", tableName, err)
	}
	return encoded, nil
}

// rowKey 使用表名、row 前缀和第一列值生成行键（TODO:将第一列值换成主键）
func rowKey(tableName string, primary types.Value) ([]byte, error) {
	prefix, err := rowPrefixKey(tableName)
	if err != nil {
		return nil, err
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	encodedPrimary, err := json.Marshal(primary)
	if err != nil {
		return nil, fmt.Errorf("engine: encoding primary value for table %q: %w", tableName, err)
	}

	key := make([]byte, 0, len(prefix)+len(encodedPrimary))
	key = append(key, prefix...)
	key = append(key, encodedPrimary...)
	return key, nil
}
