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
	if f.Source == nil {
		return nil, fmt.Errorf("executor: filter source is nil")
	}

	sourceResult, err := f.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing filter source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf(
			"executor: filter source returned %T, want RowsResult",
			sourceResult,
		)
	}

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

		switch value.Kind {
		case types.ValueNull:
			// UNKNOWN 不满足 WHERE/HAVING。
			continue
		case types.ValueBoolean:
			if value.Boolean {
				rows = append(rows, row)
			}
		default:
			return nil, fmt.Errorf(
				"executor: filter predicate returned value kind %d at row %d, want boolean or NULL",
				value.Kind,
				rowIndex+1,
			)
		}
	}

	return RowsResult{
		Columns: rowsResult.Columns,
		Rows:    rows,
	}, nil
}
