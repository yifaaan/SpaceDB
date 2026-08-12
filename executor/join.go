package executor

import (
	"fmt"
	"slices"
	"spacedb/parser"
	"spacedb/types"
)

// NestedLoopJoinExecutor 嵌套循环计算 CROSS JOIN
type NestedLoopJoinExecutor struct {
	Left  Executor
	Right Executor

	//	Predicate == nil：CROSS JOIN，所有组合都匹配
	//	Predicate != nil：只保留 ON 条件为真的组合
	Predicate *parser.Expression

	// Outer == true：左侧未匹配行必须补 NULL 后输出
	Outer bool
}

func (j NestedLoopJoinExecutor) Execute(txn Transaction) (ResultSet, error) {
	leftResult, err := j.Left.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing left join input: %w", err)
	}
	leftRows := leftResult.(RowsResult)
	rightResult, err := j.Right.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing right join input: %w", err)
	}
	rightRows := rightResult.(RowsResult)

	columns := slices.Concat(leftRows.Columns, rightRows.Columns)
	rows := make([]types.Row, 0, len(leftRows.Rows)*len(rightRows.Rows))
	for _, a := range leftRows.Rows {
		matched := false
		for _, b := range rightRows.Rows {
			isMatch := j.Predicate == nil
			if j.Predicate != nil {
				value, err := evaluateExpression(
					*j.Predicate,
					leftRows.Columns,
					a,
					rightRows.Columns,
					b,
				)
				if err != nil {
					return nil, fmt.Errorf("executor: evaluating JOIN predicate: %w", err)
				}

				isMatch = value.Kind == types.ValueBoolean && value.Boolean
			}
			if !isMatch {
				continue
			}
			c := slices.Concat(a, b)
			rows = append(rows, c)
			matched = true
		}
		// 外连接 不匹配的设置为 NULL
		if j.Outer && !matched {
			row := slices.Clone(a)
			for range len(rightRows.Columns) {
				row = append(row, types.Value{Kind: types.ValueNull})
			}
			rows = append(rows, row)
		}
	}

	return RowsResult{columns, rows}, nil
}
