package schema

import (
	"fmt"
	"spacedb/types"
)

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
	Default    *types.Value
	PrimaryKey bool
}

// Validate 检查表结构是否满足当前数据库的基本约束。
//
// UPDATE 需要通过主键定位行，因此每张表必须且只能有一个主键。
func (t Table) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("schema: table name cannot be empty")
	}
	if len(t.Columns) == 0 {
		return fmt.Errorf("schema: table %q has no columns", t.Name)
	}

	primaryKeys := 0
	for _, column := range t.Columns {
		if !column.PrimaryKey {
			continue
		}

		primaryKeys++

		if column.Nullable {
			return fmt.Errorf("schema: primary key %q cannot be nullable in table %q", column.Name, t.Name)
		}
	}

	switch primaryKeys {
	case 0:
		return fmt.Errorf("schema: table %q has no primary key", t.Name)
	case 1:
		return nil
	default:
		return fmt.Errorf(
			"schema: table %q has multiple primary keys",
			t.Name,
		)
	}
}

// PrimaryKeyValue 从一行数据中取出主键值。
//
// row 的排列顺序必须与 Table.Columns 一致
func (t Table) PrimaryKeyValue(row types.Row) (types.Value, error) {
	if len(row) != len(t.Columns) {
		return types.Value{}, fmt.Errorf(
			"schema: row length mismatch: got %d, columns %d",
			len(row),
			len(t.Columns),
		)
	}

	for i, column := range t.Columns {
		if column.PrimaryKey {
			return row[i], nil
		}
	}

	return types.Value{}, fmt.Errorf(
		"schema: table %q has no primary key",
		t.Name,
	)
}

// ColumnIndex 返回指定列在 Row 中的位置
func (t Table) ColumnIndex(name string) (int, error) {
	for i, column := range t.Columns {
		if column.Name == name {
			return i, nil
		}
	}

	return 0, fmt.Errorf(
		"schema: column %q does not exist in table %q",
		name,
		t.Name,
	)
}
