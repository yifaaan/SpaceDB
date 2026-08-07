package executor

import (
	"spacedb/schema"
	"spacedb/types"
)

// RowFilter 是 Executor 传给 Engine 的运行时等值过滤条件。
// Parser 表达式应在 Executor 中转换成 Value，Engine 不依赖 Parser AST。
type RowFilter struct {
	Column string
	Value  types.Value
}

// Transaction 描述 SQL 执行器需要的最小事务能力
type Transaction interface {
	Commit() error

	Rollback() error

	// CreateRow 向指定表写入一行
	CreateRow(tableName string, row types.Row) error

	// UpdateRow 使用旧主键定位原行，并写入更新后的行。
	UpdateRow(table *schema.Table, oldPrimaryKey types.Value, row types.Row) error

	// ScanTable 返回指定表当前可见且满足可选过滤条件的行。
	ScanTable(tableName string, filter *RowFilter) ([]types.Row, error)

	// CreateTable 创建表结构
	CreateTable(table schema.Table) error

	// GetTable 查询表结构
	GetTable(tableName string) (*schema.Table, error)
}
