package planner

import (
	"fmt"
	"spacedb/parser"
	"spacedb/schema"
	"spacedb/types"
)

// tableFromCreateStatement 把 CREATE TABLE AST 转成可供执行器使用的表结构
func tableFromCreateStatement(stmt parser.CreateTableStatement) (schema.Table, error) {
	table := schema.Table{
		Name:    stmt.Name,
		Columns: make([]schema.Column, 0, len(stmt.Columns)),
	}

	for _, astColumn := range stmt.Columns {
		// 没写 NULL 或 NOT NULL 时，默认允许 NULL
		nullable := true
		if astColumn.Nullable != nil {
			nullable = *astColumn.Nullable
		}

		var defaultValue *types.Value

		switch {
		case astColumn.DefaultValue != nil:
			v, err := ValueFromExpression(*astColumn.DefaultValue)
			if err != nil {
				return schema.Table{}, fmt.Errorf("planner: converting default for column %q: %w", astColumn.Name, err)
			}
			defaultValue = &v
		case nullable:
			v := types.Value{Kind: types.ValueNull}
			defaultValue = &v
		}

		table.Columns = append(table.Columns, schema.Column{
			Name:       astColumn.Name,
			DataType:   astColumn.DataType,
			Nullable:   nullable,
			Default:    defaultValue,
			PrimaryKey: astColumn.PrimaryKey,
		})
	}
	return table, nil
}
