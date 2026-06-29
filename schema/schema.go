package schema

import "spacedb/types"

// Table 是经过 Planner 归一化后的表结构
type Table struct {
	Name    string
	Columns []Column
}

type Column struct {
	Name     string
	DataType types.DataType
	Nullable bool
	// Default 为 nil 表示没有默认值；非 nil 且 Kind 为 ValueNull
	// 表示该列的默认值明确是 SQL NULL。
	Default *types.Value
}
