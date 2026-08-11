package executor

import (
	"fmt"
	"slices"
	"spacedb/parser"
	"spacedb/types"
	"strings"
)

// AggregateExecutor 执行全表聚合或单列 GROUP BY 聚合。
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
	Source  Executor
	Items   []parser.SelectItem
	GroupBy *parser.Expression
}

func (a AggregateExecutor) Execute(txn Transaction) (ResultSet, error) {
	if a.Source == nil {
		return nil, fmt.Errorf("executor: aggregate source is nil")
	}

	sourceResult, err := a.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing aggregate source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: aggregate source returned %T, want RowsResult", sourceResult)
	}

	outputColumns, err := aggregateOutputColumns(a.Items)
	if err != nil {
		return nil, err
	}

	if a.GroupBy == nil {
		// 没有 GROUP BY 时，全部输入行属于同一个隐式分组。即使输入为空，
		// 也必须计算一次，使 COUNT 返回 0，MIN/MAX/SUM/AVG 返回 NULL。
		outputRow, err := a.calculateRow(nil, "", rowsResult.Columns, rowsResult.Rows)
		if err != nil {
			return nil, err
		}

		return RowsResult{
			Columns: outputColumns,
			Rows:    []types.Row{outputRow},
		}, nil
	}

	groupColumn, groupColumnIndex, err := resolveGroupBy(*a.GroupBy, rowsResult.Columns)
	if err != nil {
		return nil, err
	}

	// map 只负责把分组键映射到 groups 下标；groups 切片保存分组首次出现
	// 的顺序。直接遍历 map 会让无 ORDER BY 的结果顺序随机变化。
	type rowGroup struct {
		key  types.Value
		rows []types.Row
	}

	groups := make([]rowGroup, 0)
	groupIndexes := make(map[types.Value]int)

	for rowIndex, row := range rowsResult.Rows {
		if groupColumnIndex >= len(row) {
			return nil, fmt.Errorf(
				"executor: row %d does not contain GROUP BY column %q at index %d",
				rowIndex+1,
				groupColumn,
				groupColumnIndex,
			)
		}

		key := row[groupColumnIndex]
		if groupIndex, ok := groupIndexes[key]; ok {
			groups[groupIndex].rows = append(groups[groupIndex].rows, row)
			continue
		}

		groupIndexes[key] = len(groups)
		groups = append(groups, rowGroup{
			key:  key,
			rows: []types.Row{row},
		})
	}

	outputRows := make([]types.Row, 0, len(groups))
	for groupIndex := range groups {
		group := &groups[groupIndex]
		outputRow, err := a.calculateRow(
			&group.key,
			groupColumn,
			rowsResult.Columns,
			group.rows,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"executor: calculating GROUP BY value %#v: %w",
				group.key,
				err,
			)
		}
		outputRows = append(outputRows, outputRow)
	}

	return RowsResult{
		Columns: outputColumns,
		Rows:    outputRows,
	}, nil
}

// aggregateOutputColumns 按 SELECT 项提前确定结果列名。列名只与表达式和
// 别名有关，不需要在每个分组中重复计算。
func aggregateOutputColumns(items []parser.SelectItem) ([]string, error) {
	columns := make([]string, 0, len(items))

	for itemIndex, item := range items {
		var outputName string

		switch item.Expression.Kind {
		case parser.FunctionExpression:
			call, ok := item.Expression.Value.(parser.FunctionCall)
			if !ok {
				return nil, fmt.Errorf(
					"executor: aggregate item %d contains %T, want parser.FunctionCall",
					itemIndex+1,
					item.Expression.Value,
				)
			}
			outputName = call.Name

		case parser.ColumnReference:
			columnName, ok := item.Expression.Value.(string)
			if !ok {
				return nil, fmt.Errorf(
					"executor: aggregate item %d column reference contains %T",
					itemIndex+1,
					item.Expression.Value,
				)
			}
			outputName = columnName

		default:
			return nil, fmt.Errorf(
				"executor: aggregate item %d has unsupported expression kind %d",
				itemIndex+1,
				item.Expression.Kind,
			)
		}

		if item.Alias != nil {
			outputName = *item.Alias
		}
		columns = append(columns, outputName)
	}

	return columns, nil
}

// resolveGroupBy 验证分组表达式并把分组列名解析成输入行下标。
func resolveGroupBy(expression parser.Expression, columns []string) (string, int, error) {
	if expression.Kind != parser.ColumnReference {
		return "", 0, fmt.Errorf("executor: GROUP BY expression must be a column reference")
	}

	columnName, ok := expression.Value.(string)
	if !ok {
		return "", 0, fmt.Errorf(
			"executor: GROUP BY column reference contains %T",
			expression.Value,
		)
	}

	columnIndex := slices.Index(columns, columnName)
	if columnIndex < 0 {
		return "", 0, fmt.Errorf(
			"executor: GROUP BY column %q does not exist",
			columnName,
		)
	}

	return columnName, columnIndex, nil
}

// calculateRow 计算一个分组对应的结果行。函数表达式在组内聚合；普通列
// 只能引用 GROUP BY 列，因此每组只需输出一次 groupValue。
func (a AggregateExecutor) calculateRow(
	groupValue *types.Value,
	groupColumn string,
	columns []string,
	rows []types.Row,
) (types.Row, error) {
	outputRow := make(types.Row, 0, len(a.Items))

	for itemIndex, item := range a.Items {
		switch item.Expression.Kind {
		case parser.FunctionExpression:
			call, ok := item.Expression.Value.(parser.FunctionCall)
			if !ok {
				return nil, fmt.Errorf(
					"aggregate item %d contains %T, want parser.FunctionCall",
					itemIndex+1,
					item.Expression.Value,
				)
			}

			calculator, err := buildAggregateCalculator(call.Name)
			if err != nil {
				return nil, fmt.Errorf("building aggregate function %q: %w", call.Name, err)
			}

			value, err := calculator.calculate(call.Argument, columns, rows)
			if err != nil {
				return nil, fmt.Errorf("calculating %s(%s): %w", call.Name, call.Argument, err)
			}
			outputRow = append(outputRow, value)

		case parser.ColumnReference:
			if groupValue == nil {
				return nil, fmt.Errorf(
					"aggregate item %d is a column reference without GROUP BY",
					itemIndex+1,
				)
			}

			columnName, ok := item.Expression.Value.(string)
			if !ok {
				return nil, fmt.Errorf(
					"aggregate item %d column reference contains %T",
					itemIndex+1,
					item.Expression.Value,
				)
			}
			if columnName != groupColumn {
				return nil, fmt.Errorf(
					"column %q must appear in GROUP BY or be used in an aggregate function",
					columnName,
				)
			}

			outputRow = append(outputRow, *groupValue)

		default:
			return nil, fmt.Errorf(
				"aggregate item %d has unsupported expression kind %d",
				itemIndex+1,
				item.Expression.Kind,
			)
		}
	}

	return outputRow, nil
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

	case "SUM":
		return sumCalculator{}, nil

	case "AVG":
		return avgCalculator{}, nil

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

// sumCalculator 实现 SUM(column)
type sumCalculator struct{}

func (sumCalculator) calculate(columnName string, columns []string, rows []types.Row) (types.Value, error) {
	columnIndex := slices.Index(columns, columnName)
	if columnIndex < 0 {
		return types.Value{}, fmt.Errorf("column %q does not exist", columnName)
	}

	var sum float64

	found := false

	for rowIndex, row := range rows {
		if columnIndex >= len(row) {
			return types.Value{}, fmt.Errorf("row %d does not contain column %q at index %d", rowIndex+1, columnName, columnIndex)
		}

		value := row[columnIndex]

		switch value.Kind {
		case types.ValueNull:
			// NULL
			continue

		case types.ValueInteger:
			// 把整数转换成 f64
			sum += float64(value.Integer)
			found = true

		case types.ValueFloat:
			sum += value.Float
			found = true

		default:
			return types.Value{}, fmt.Errorf(
				"SUM requires a numeric column, but column %q contains value kind %d at row %d", columnName, value.Kind, rowIndex+1)
		}
	}

	if !found {
		return types.Value{Kind: types.ValueNull}, nil
	}

	return types.Value{
		Kind:  types.ValueFloat,
		Float: sum,
	}, nil
}

// avgCalculator 实现 AVG(column)。
type avgCalculator struct{}

func (avgCalculator) calculate(columnName string, columns []string, rows []types.Row) (types.Value, error) {
	sumValue, err := (sumCalculator{}).calculate(columnName, columns, rows)
	if err != nil {
		return types.Value{}, fmt.Errorf("calculating AVG sum: %w", err)
	}

	countValue, err := (countCalculator{}).calculate(columnName, columns, rows)
	if err != nil {
		return types.Value{}, fmt.Errorf("calculating AVG count: %w", err)
	}

	if sumValue.Kind == types.ValueNull {
		return types.Value{Kind: types.ValueNull}, nil
	}
	if sumValue.Kind != types.ValueFloat {
		return types.Value{}, fmt.Errorf("AVG sum returned value kind %d, want float", sumValue.Kind)
	}

	if countValue.Kind != types.ValueInteger {
		return types.Value{}, fmt.Errorf("AVG count returned value kind %d, want integer", countValue.Kind)
	}

	if countValue.Integer == 0 {
		return types.Value{Kind: types.ValueNull}, nil
	}

	return types.Value{
		Kind:  types.ValueFloat,
		Float: sumValue.Float / float64(countValue.Integer),
	}, nil
}
