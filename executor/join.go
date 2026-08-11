package executor

import (
	"fmt"
	"slices"
	"spacedb/types"
)

// NestedLoopJoinExecutor 嵌套循环计算 CROSS JOIN
//
// 对左侧的每一行，依次拼接右侧的每一行：
//
//	leftRows  = [L1, L2]
//	rightRows = [R1, R2, R3]
//
// 输出顺序：
//
//	L1+R1, L1+R2, L1+R3,
//	L2+R1, L2+R2, L2+R3
type NestedLoopJoinExecutor struct {
	Left  Executor
	Right Executor
}

func (j NestedLoopJoinExecutor) Execute(txn Transaction) (ResultSet, error) {
	leftResult, err := j.Left.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing left join input: %w", err)
	}
	leftRows, ok := leftResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: left join input returned %T, want RowsResult", leftResult)
	}
	rightResult, err := j.Right.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing right join input: %w", err)
	}
	rightRows, ok := rightResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: right join input returned %T, want RowsResult", rightResult)
	}

	columns := slices.Concat(leftRows.Columns, rightRows.Columns)
	rows := make([]types.Row, 0, len(leftRows.Rows)*len(rightRows.Rows))
	for _, a := range leftRows.Rows {
		for _, b := range rightRows.Rows {
			c := slices.Concat(a, b)
			rows = append(rows, c)
		}
	}

	return RowsResult{columns, rows}, nil
}
