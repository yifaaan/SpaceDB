package executor

import (
	"fmt"

	"spacedb/parser"
	"spacedb/types"
)

// FilterExecutor 对 Source 产生的每一行计算 Predicate。
//
// WHERE 和 HAVING 共用这个执行器：Planner 通过节点在计划树中的位置，
// 决定它过滤原始行还是聚合后的结果行。
type FilterExecutor struct {
	Source    Executor
	Predicate parser.Expression
}

func (f FilterExecutor) Execute(txn Transaction) (ResultSet, error) {
	sourceResult, err := f.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing filter source: %w", err)
	}

	rowsResult := sourceResult.(RowsResult)

	rows := make([]types.Row, 0, len(rowsResult.Rows))
	for rowIndex, row := range rowsResult.Rows {
		value, err := evaluateExpression(
			f.Predicate,
			rowsResult.Columns,
			row,
			rowsResult.Columns,
			row,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"executor: evaluating filter for row %d: %w",
				rowIndex+1,
				err,
			)
		}

		// UNKNOWN 不满足 WHERE/HAVING。
		if value.Kind == types.ValueBoolean && value.Boolean {
			rows = append(rows, row)
		}
	}

	return RowsResult{
		Columns: rowsResult.Columns,
		Rows:    rows,
	}, nil
}
