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

	// ON
	predicate, err := resolveJoinPredicate(j.Predicate, leftRows.Columns, rightRows.Columns)
	if err != nil {
		return nil, err
	}

	columns := slices.Concat(leftRows.Columns, rightRows.Columns)
	rows := make([]types.Row, 0, len(leftRows.Rows)*len(rightRows.Rows))
	for _, a := range leftRows.Rows {
		matched := false
		for _, b := range rightRows.Rows {
			isMatch := predicate == nil
			if predicate != nil {
				lv := a[predicate.leftIndex]
				rv := b[predicate.rightIndex]

				if lv.Kind != types.ValueNull && rv.Kind != types.ValueNull {
					cmp, err := lv.Compare(rv)
					if err != nil {
						return nil, fmt.Errorf("executor: comparing JOIN columns %q and %q: %w", predicate.leftName, predicate.rightName, err)
					}
					isMatch = cmp == 0
				}
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

type resolvedJoinPredicate struct {
	leftIndex  int
	rightIndex int
	leftName   string
	rightName  string
}

func resolveJoinPredicate(expression *parser.Expression, leftColumns []string, rightColumns []string) (*resolvedJoinPredicate, error) {
	if expression == nil {
		return nil, nil
	}

	if expression.Kind != parser.OperationExpression {
		return nil, fmt.Errorf("executor: JOIN predicate has expression kind %d", expression.Kind)
	}

	op, ok := expression.Value.(parser.Operation)
	if !ok || op.Kind != parser.OperationEqual {
		return nil, fmt.Errorf("executor: unsupported JOIN predicate %#v", expression.Value)
	}

	leftName, leftOK := op.Left.Value.(string)
	rightName, rightOK := op.Right.Value.(string)
	if op.Left.Kind != parser.ColumnReference || !leftOK || op.Right.Kind != parser.ColumnReference || !rightOK {
		return nil, fmt.Errorf("executor: JOIN operands must be column references")
	}

	leftIndex := slices.Index(leftColumns, leftName)
	if leftIndex < 0 {
		return nil, fmt.Errorf("executor: JOIN column %q does not exist in left input", leftName)
	}

	rightIndex := slices.Index(rightColumns, rightName)
	if rightIndex < 0 {
		return nil, fmt.Errorf("executor: JOIN column %q does not exist in right input", rightName)
	}

	return &resolvedJoinPredicate{
		leftIndex,
		rightIndex,
		leftName,
		rightName,
	}, nil
}
