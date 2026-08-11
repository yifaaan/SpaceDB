package executor

import (
	"fmt"
	"slices"
	"spacedb/parser"
	"spacedb/types"
	"strings"
)

// AggregateExecutor 执行不带 GROUP BY 的全表聚合
//
// Source 产生普通的多行结果，例如：
//
//	Columns: ["id", "name"]
//	Rows: [
//	    [1, "alice"],
//	    [2, NULL],
//	    [3, "carol"],
//	]
//
// AggregateExecutor 会把全部输入行压缩成一行，例如：
//
//	SELECT count(name) FROM users;
//
// 返回：
//
//	Columns: ["count"]
//	Rows: [[2]]
type AggregateExecutor struct {
	Source Executor
	Items  []parser.SelectItem
}

func (a AggregateExecutor) Execute(txn Transaction) (ResultSet, error) {
	sourceResult, err := a.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing aggregate source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: aggregate source returned %T, want RowsResult", sourceResult)
	}

	// 一个聚合表达式产生一个输出列和一个输出值
	outputColumns := make([]string, 0, len(a.Items))
	outputRow := make(types.Row, 0, len(a.Items))

	for i, item := range a.Items {
		if item.Expression.Kind != parser.FunctionExpression {
			return nil, fmt.Errorf("executor: aggregate item %d is not a function expression", i+1)
		}

		call, ok := item.Expression.Value.(parser.FunctionCall)
		if !ok {
			return nil, fmt.Errorf("executor: aggregate item %d contains %T, want parser.FunctionCall", i+1, item.Expression.Value)
		}

		calculator, err := buildAggregateCalculator(call.Name)
		if err != nil {
			return nil, fmt.Errorf("executor: building aggregate function %q: %w", call.Name, err)
		}

		value, err := calculator.calculate(call.Argument, rowsResult.Columns, rowsResult.Rows)
		if err != nil {
			return nil, fmt.Errorf("executor: calculating %s(%s): %w", call.Name, call.Argument, err)
		}

		outputName := call.Name
		if item.Alias != nil {
			outputName = *item.Alias
		}

		outputColumns = append(outputColumns, outputName)
		outputRow = append(outputRow, value)
	}

	// 即使输入表没有任何行，聚合仍然必须返回一行
	//
	// 如空表上的 COUNT(id) 返回：
	//
	//      Rows: [[0]]
	return RowsResult{
		Columns: outputColumns,
		Rows:    []types.Row{outputRow},
	}, nil
}

// aggregateCalculator 描述一个聚合函数需要完成的计算

type aggregateCalculator interface {
	calculate(columnName string, columns []string, rows []types.Row) (types.Value, error)
}

// buildAggregateCalculator 根据 SQL 函数名选择具体计算器
func buildAggregateCalculator(name string) (aggregateCalculator, error) {
	switch strings.ToUpper(name) {
	case "COUNT":
		return countCalculator{}, nil

	case "MIN":
		return minCalculator{}, nil

	case "MAX":
		return maxCalculator{}, nil

	default:
		return nil, fmt.Errorf("aggregate function %q is not implemented", name)
	}
}

// countCalculator 实现 COUNT(column)
//
// SQL 语义：
//   - 只统计非 NULL 值；
//   - 空表返回 0；
//   - COUNT(*) 暂未实现。
type countCalculator struct{}

func (countCalculator) calculate(columnName string, columns []string, rows []types.Row) (types.Value, error) {
	columnIdx := slices.Index(columns, columnName)
	if columnIdx < 0 {
		return types.Value{}, fmt.Errorf("column %q does not exist", columnName)
	}

	var count int64
	for i, row := range rows {
		if columnIdx >= len(row) {
			return types.Value{}, fmt.Errorf("row %d does not contain column %q at index %d", i+1, columnName, columnIdx)
		}
		if row[columnIdx].Kind != types.ValueNull {
			count++
		}
	}
	return types.Value{
		Kind:    types.ValueInteger,
		Integer: count,
	}, nil
}

// minCalculator 返回指定列中最小的非 NULL 值
type minCalculator struct{}

func (minCalculator) calculate(columnName string, columns []string, rows []types.Row) (types.Value, error) {
	return calculateExtreme(columnName, columns, rows, true)
}

// maxCalculator 返回指定列中最大的非 NULL 值
type maxCalculator struct{}

func (maxCalculator) calculate(columnName string, columns []string, rows []types.Row) (types.Value, error) {
	return calculateExtreme(columnName, columns, rows, false)
}

func calculateExtreme(columnName string, columns []string, rows []types.Row, minimum bool) (types.Value, error) {
	columnIndex := slices.Index(columns, columnName)
	if columnIndex < 0 {
		return types.Value{}, fmt.Errorf("column %q does not exist", columnName)
	}

	var extreme types.Value
	found := false

	for rowIndex, row := range rows {
		if columnIndex >= len(row) {
			return types.Value{}, fmt.Errorf("row %d does not contain column %q at index %d", rowIndex+1, columnName, columnIndex)
		}

		current := row[columnIndex]

		if current.Kind == types.ValueNull {
			continue
		}

		if !found {
			extreme = current
			found = true
			continue
		}

		comparison, err := current.Compare(extreme)
		if err != nil {
			return types.Value{}, fmt.Errorf("comparing row %d column %q: %w", rowIndex+1, columnName, err)
		}

		replace := minimum && comparison < 0
		replace = replace || !minimum && comparison > 0

		if replace {
			extreme = current
		}
	}

	if !found {
		return types.Value{Kind: types.ValueNull}, nil
	}

	return extreme, nil
}
