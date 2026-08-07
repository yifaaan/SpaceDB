package engine

import (
	"encoding/binary"
	"fmt"
	"math"
	"spacedb/executor"
	"spacedb/schema"
	"spacedb/storage"
	"spacedb/types"

	jsoniter "github.com/json-iterator/go"
)

const (
	// keyNamespaceTable 表示这条 KV 保存的是表元数据
	//
	// 例如 users 表的元数据 key：
	//
	//	0x01 | "users"
	keyNamespaceTable byte = 0x01

	// keyNamespaceRow 表示这条 KV 保存的是表中的一行数据记录
	//
	// 例如 users 表的行前缀：
	//
	//	0x02 | "users" | 0x00
	keyNamespaceRow byte = 0x02

	// keySeparator 用来标记表名结束
	keySeparator byte = 0x00
)

const (
	// 这些字节是主键值的类型标记。
	// 即使整数 1 和字符串 "1" 内容看起来相同，
	// 最终生成的 key 也不会相同。
	primaryKeyNull byte = iota
	primaryKeyBoolean
	primaryKeyInteger
	primaryKeyFloat
	primaryKeyString
)

// KVEngine 对 storage.MVCC底层存储 的 SQL Engine 封装。
type KVEngine struct {
	storage *storage.MVCC
}

func NewKVEngine(e storage.Engine) *KVEngine {
	return &KVEngine{
		storage: storage.NewMVCC(e),
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
	return t.txn.Commit()
}

func (t *KVTransaction) Rollback() error {
	return t.txn.Rollback()
}

// validateRow 检查一行数据是否符合表结构。
// CreateRow 和 UpdateRow 都需要执行相同校验。
func validateRow(table *schema.Table, row types.Row) error {
	if len(row) == 0 {
		return fmt.Errorf("engine: cannot store an empty row")
	}

	if len(row) != len(table.Columns) {
		return fmt.Errorf("engine: row length mismatch: got %d, columns %d", len(row), len(table.Columns))
	}

	for i, column := range table.Columns {
		value := row[i]
		if value.Kind == types.ValueNull {
			if !column.Nullable {
				return fmt.Errorf("engine: column %q cannot be NULL", column.Name)
			}
			continue
		}

		actualType, ok := value.DataType()
		if !ok {
			return fmt.Errorf("engine: invalid value kind %d for column %q", value.Kind, column.Name)
		}
		if actualType != column.DataType {
			return fmt.Errorf("engine: column %q type mismatch: want %d, got %d", column.Name, column.DataType, actualType)
		}
	}

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

	if err := validateRow(table, row); err != nil {
		return err
	}

	primaryKey, err := table.PrimaryKeyValue(row)
	if err != nil {
		return err
	}

	key, err := rowKey(tableName, primaryKey)
	if err != nil {
		return err
	}

	existing, err := t.txn.Get(key)
	if err != nil {
		return fmt.Errorf("engine: checking primary key for table %q: %w", tableName, err)
	}
	if existing != nil {
		return fmt.Errorf("engine: duplicate primary key in table %q", tableName)
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	encoded, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("engine: encoding row for table %q: %w", tableName, err)
	}

	return t.txn.Set(key, encoded)
}

func (t *KVTransaction) UpdateRow(table *schema.Table, oldPrimaryKey types.Value, row types.Row) error {
	if err := validateRow(table, row); err != nil {
		return err
	}

	newPrimaryKey, err := table.PrimaryKeyValue(row)
	if err != nil {
		return err
	}

	newKey, err := rowKey(table.Name, newPrimaryKey)
	if err != nil {
		return err
	}

	if oldPrimaryKey != newPrimaryKey {
		oldKey, err := rowKey(table.Name, oldPrimaryKey)
		if err != nil {
			return err
		}

		existing, err := t.txn.Get(newKey)
		if err != nil {
			return fmt.Errorf("engine: checking updated primary key: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("engine: duplicate primary key in table %q", table.Name)
		}

		if err := t.txn.Delete(oldKey); err != nil {
			return fmt.Errorf("engine: deleting old row key: %w", err)
		}
	}

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	encoded, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("engine: encoding updated row for table %q: %w", table.Name, err)
	}

	return t.txn.Set(newKey, encoded)
}

func (t *KVTransaction) ScanTable(tableName string, filter *executor.RowFilter) ([]types.Row, error) {
	table, err := t.GetTable(tableName)
	if err != nil {
		return nil, fmt.Errorf("engine: loading table %q: %w", tableName, err)
	}
	if table == nil {
		return nil, fmt.Errorf("engine: table %q does not exist", tableName)
	}

	filterColumn := 0
	if filter != nil {
		filterColumn, err = table.ColumnIndex(filter.Column)
		if err != nil {
			return nil, err
		}
	}

	prefix, err := rowPrefixKey(tableName)
	if err != nil {
		return nil, err
	}

	entries, err := t.txn.ScanPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("engine: scanning rows for table %q: %w", tableName, err)
	}

	rows := make([]types.Row, 0, len(entries))
	var json = jsoniter.ConfigCompatibleWithStandardLibrary

	for i, e := range entries {
		var row types.Row
		if err := json.Unmarshal(e.Value, &row); err != nil {
			return nil, fmt.Errorf("engine: decoding row %d from table %q: %w", i+1, tableName, err)
		}
		if len(row) != len(table.Columns) {
			return nil, fmt.Errorf("engine: stored row %d has %d values, want %d", i+1, len(row), len(table.Columns))
		}
		if filter != nil && row[filterColumn] != filter.Value {
			continue
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func (t *KVTransaction) CreateTable(table schema.Table) error {
	if err := table.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTable, err)
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

// tableKey 生成表元数据的 KV key。
//
// 编码格式：
//
//	0x01 | tableName
func tableKey(tableName string) ([]byte, error) {
	if tableName == "" {
		return nil, fmt.Errorf("engine: table name cannot be empty")
	}

	key := make([]byte, 0, 1+len(tableName))
	key = append(key, keyNamespaceTable)
	key = append(key, tableName...)

	return key, nil
}

// rowPrefixKey 生成某张表所有行共享的 key 前缀
//
// 编码格式：
//
//	0x02 | tableName | 0x00
//
// ScanTable 使用这个结果执行 ScanPrefix
func rowPrefixKey(tableName string) ([]byte, error) {
	if tableName == "" {
		return nil, fmt.Errorf("engine: table name cannot be empty")
	}

	prefix := make([]byte, 0, 2+len(tableName))
	prefix = append(prefix, keyNamespaceRow)
	prefix = append(prefix, tableName...)
	prefix = append(prefix, keySeparator)

	return prefix, nil
}

// rowKey 生成某一行的完整 KV key
//
// 编码格式：
//
//	0x02 | tableName | 0x00 | primaryKey
//
// 当前临时使用第一列作为 primaryKey
func rowKey(tableName string, primary types.Value) ([]byte, error) {
	prefix, err := rowPrefixKey(tableName)
	if err != nil {
		return nil, err
	}

	encodedPrimary, err := encodePrimaryKey(primary)
	if err != nil {
		return nil, fmt.Errorf(
			"engine: encoding primary key for table %q: %w",
			tableName,
			err,
		)
	}

	key := make([]byte, 0, len(prefix)+len(encodedPrimary))
	key = append(key, prefix...)
	key = append(key, encodedPrimary...)

	return key, nil
}

// encodePrimaryKey 将运行时 Value 编码成可以放进 KV key 的字节
func encodePrimaryKey(value types.Value) ([]byte, error) {
	switch value.Kind {
	case types.ValueNull:
		return []byte{primaryKeyNull}, nil

	case types.ValueBoolean:
		encoded := []byte{primaryKeyBoolean, 0}
		if value.Boolean {
			encoded[1] += 1
		}
		return encoded, nil

	case types.ValueInteger:
		encoded := make([]byte, 1+8)

		encoded[0] = primaryKeyInteger

		//	负数 < 0 < 正数(负数转换成 uint64后会排到正数后面，所以需要将符号位反转)
		ordered := uint64(value.Integer) ^ (1 << 63)

		binary.BigEndian.PutUint64(encoded[1:], ordered)

		return encoded, nil

	case types.ValueFloat:
		encoded := make([]byte, 1+8)
		encoded[0] = primaryKeyFloat

		bits := math.Float64bits(value.Float)

		if bits&(uint64(1)<<63) != 0 {
			bits = ^bits
		} else {
			bits ^= uint64(1) << 63
		}

		binary.BigEndian.PutUint64(encoded[1:], bits)
		return encoded, nil

	case types.ValueString:
		encoded := make([]byte, 0, 1+len(value.String))
		encoded = append(encoded, primaryKeyString)
		encoded = append(encoded, value.String...)
		return encoded, nil

	default:
		return nil, fmt.Errorf("unsupported value kind %d", value.Kind)
	}
}
