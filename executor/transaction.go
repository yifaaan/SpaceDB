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

	// ScanTable 返回指定表当前可见的所有行
	ScanTable(tableName string) ([]types.Row, error)

	// CreateTable 创建表结构
	CreateTable(table schema.Table) error

	// GetTable 查询表结构
	GetTable(tableName string) (*schema.Table, error)
}
