package engine

import "errors"

var (
	// ErrTableExists 目标表已经存在
	ErrTableExists = errors.New("engine: table already exists")

	// ErrInvalidTable 表结构不合法
	ErrInvalidTable = errors.New("engine: invalid table")
)
