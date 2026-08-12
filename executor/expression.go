package executor

import (
	"fmt"
	"slices"

	"spacedb/parser"
	"spacedb/types"
)

// evaluateExpression 在给定行上下文中计算表达式。
//
// leftColumns/leftRow 是表达式默认读取的输入；rightColumns/rightRow
// 用于二元表达式的右操作数。Filter 会把同一行同时作为左右输入，Join
// 则分别传入连接两侧，因此即使两张表有同名列，也能保持 ON 左右语义。
func evaluateExpression(
	expression parser.Expression,
	leftColumns []string,
	leftRow types.Row,
	rightColumns []string,
	rightRow types.Row,
) (types.Value, error) {
	switch expression.Kind {
	case parser.Null:
		return types.Value{Kind: types.ValueNull}, nil

	case parser.BooleanLiteral:
		value, ok := expression.Value.(bool)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"executor: boolean expression contains %T",
				expression.Value,
			)
		}
		return types.Value{Kind: types.ValueBoolean, Boolean: value}, nil

	case parser.IntegerLiteral:
		value, ok := expression.Value.(int64)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"executor: integer expression contains %T",
				expression.Value,
			)
		}
		return types.Value{Kind: types.ValueInteger, Integer: value}, nil

	case parser.FloatLiteral:
		value, ok := expression.Value.(float64)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"executor: float expression contains %T",
				expression.Value,
			)
		}
		return types.Value{Kind: types.ValueFloat, Float: value}, nil

	case parser.StringLiteral:
		value, ok := expression.Value.(string)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"executor: string expression contains %T",
				expression.Value,
			)
		}
		return types.Value{Kind: types.ValueString, String: value}, nil

	case parser.ColumnReference:
		columnName, ok := expression.Value.(string)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"executor: column reference contains %T",
				expression.Value,
			)
		}

		columnIndex := slices.Index(leftColumns, columnName)
		if columnIndex < 0 {
			return types.Value{}, fmt.Errorf(
				"executor: column %q does not exist",
				columnName,
			)
		}
		if columnIndex >= len(leftRow) {
			return types.Value{}, fmt.Errorf(
				"executor: row does not contain column %q at index %d",
				columnName,
				columnIndex,
			)
		}
		return leftRow[columnIndex], nil

	case parser.OperationExpression:
		operation, ok := expression.Value.(parser.Operation)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"executor: operation expression contains %T",
				expression.Value,
			)
		}

		left, err := evaluateExpression(
			operation.Left,
			leftColumns,
			leftRow,
			rightColumns,
			rightRow,
		)
		if err != nil {
			return types.Value{}, fmt.Errorf("executor: evaluating left operand: %w", err)
		}

		// 交换上下文，让右操作数优先从右输入解析列名。Filter 的左右
		// 上下文本来相同；Join 则由此保持 ON 两侧分别绑定两张表。
		right, err := evaluateExpression(
			operation.Right,
			rightColumns,
			rightRow,
			leftColumns,
			leftRow,
		)
		if err != nil {
			return types.Value{}, fmt.Errorf("executor: evaluating right operand: %w", err)
		}

		return evaluateComparison(operation.Kind, left, right)

	default:
		return types.Value{}, fmt.Errorf(
			"executor: expression kind %d cannot be evaluated for a row",
			expression.Kind,
		)
	}
}

// evaluateComparison 实现 SQL 比较的三值逻辑。
//
// 任一操作数为 NULL 时结果是 NULL，而不是 true 或 false。Filter 只保留
// true，因此 WHERE/HAVING 中的未知结果会被丢弃。
func evaluateComparison(
	kind parser.OperationKind,
	left types.Value,
	right types.Value,
) (types.Value, error) {
	if left.Kind == types.ValueNull || right.Kind == types.ValueNull {
		return types.Value{Kind: types.ValueNull}, nil
	}

	comparison, err := left.Compare(right)
	if err != nil {
		return types.Value{}, fmt.Errorf("executor: comparing values: %w", err)
	}

	var result bool
	switch kind {
	case parser.OperationEqual:
		result = comparison == 0
	case parser.OperationGreaterThan:
		result = comparison > 0
	case parser.OperationLessThan:
		result = comparison < 0
	default:
		return types.Value{}, fmt.Errorf(
			"executor: unsupported comparison operation %d",
			kind,
		)
	}

	return types.Value{
		Kind:    types.ValueBoolean,
		Boolean: result,
	}, nil
}
