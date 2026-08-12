package executor

import (
	"spacedb/schema"
	"spacedb/types"
)

// Transaction 描述 SQL 执行器需要的最小事务能力
type Transaction interface {
	Commit() error

	Rollback() error

	// CreateRow 向指定表写入一行
	CreateRow(tableName string, row types.Row) error

	// UpdateRow 使用旧主键定位原行，并写入更新后的行
	UpdateRow(table *schema.Table, oldPrimaryKey types.Value, row types.Row) error

	// DeleteRow 删除指定主键对应的行
	DeleteRow(table *schema.Table, primaryKey types.Value) error

	// ScanTable 返回指定表当前可见的行
	ScanTable(table *schema.Table) ([]types.Row, error)

	// CreateTable 创建表结构
	CreateTable(table schema.Table) error

	// GetTable 查询表结构
	GetTable(tableName string) (*schema.Table, error)
}
