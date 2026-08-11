package executor

import (
	"fmt"
	"slices"
	"spacedb/parser"
	"spacedb/planner"
	"spacedb/schema"
	"spacedb/types"
)

// ResultSet 所有 SQL 执行结果的接口
type ResultSet interface {
	resultSet()
}

// CreateTableResult CREATE TABLE 的执行结果
type CreateTableResult struct {
	TableName string
}

func (CreateTableResult) resultSet() {}

// InsertResult INSERT 的执行结果
type InsertResult struct {
	Count int
}

func (InsertResult) resultSet() {}

// UpdateResult UPDATE 更新的行数
type UpdateResult struct {
	Count int
}

func (UpdateResult) resultSet() {}

// DeleteResult DELETE 删除的行数
type DeleteResult struct {
	Count int
}

func (DeleteResult) resultSet() {}

// RowsResult 查询返回的多行数据
type RowsResult struct {
	Columns []string
	Rows    []types.Row
}

func (RowsResult) resultSet() {}

// Executor 执行计划节点的统一执行接口
//
// 不直接依赖具体 KV 存储，只通过 Transaction 使用事务能力
type Executor interface {
	Execute(txn Transaction) (ResultSet, error)
}

// CreateTableExecutor 对应 planner.CreateTableNode
type CreateTableExecutor struct {
	Schema schema.Table
}

func (c CreateTableExecutor) Execute(txn Transaction) (ResultSet, error) {
	if err := txn.CreateTable(c.Schema); err != nil {
		return nil, err
	}

	return CreateTableResult{c.Schema.Name}, nil
}

// InsertExecutor 对应 planner.InsertNode
type InsertExecutor struct {
	TableName string
	Columns   []string
	Values    [][]parser.Expression
}

func (i InsertExecutor) Execute(txn Transaction) (ResultSet, error) {

	table, err := txn.GetTable(i.TableName)
	if err != nil {
		return nil, fmt.Errorf("executor: loading table %q: %w", i.TableName, err)
	}
	if table == nil {
		return nil, fmt.Errorf("executor: table %q does not exist", i.TableName)
	}

	inserted := 0
	for rowIdx, exps := range i.Values {
		// 把语法层表达式转换为运行时值
		row := make(types.Row, len(exps))
		for i, e := range exps {
			v, err := planner.ValueFromExpression(e)
			if err != nil {
				return nil, fmt.Errorf("executor: converting row %d value %d: %w", rowIdx+1, i+1, err)
			}
			row[i] = v
		}

		// 将部分列转换成完整列顺序
		fullRow, err := makeInsertRow(table, i.Columns, row)
		if err != nil {
			return nil, fmt.Errorf("executor: preparing row %d: %w", rowIdx+1, err)
		}
		if err := txn.CreateRow(i.TableName, fullRow); err != nil {
			return nil, fmt.Errorf("executor: inserting row %d: %w", rowIdx+1, err)
		}
		inserted++
	}

	return InsertResult{Count: inserted}, nil
}

// ScanExecutor 对应 planner.ScanNode
type ScanExecutor struct {
	TableName string
	Filter    *parser.EqualityFilter
}

func (s ScanExecutor) Execute(txn Transaction) (ResultSet, error) {
	table, err := txn.GetTable(s.TableName)
	if err != nil {
		return nil, fmt.Errorf("executor: loading table %q: %w", s.TableName, err)
	}
	if table == nil {
		return nil, fmt.Errorf("executor: table %q does not exist", s.TableName)
	}

	var filter *RowFilter
	if s.Filter != nil {
		value, err := planner.ValueFromExpression(s.Filter.Value)
		if err != nil {
			return nil, fmt.Errorf("executor: converting filter for column %q: %w", s.Filter.Column, err)
		}
		filter = &RowFilter{
			Column: s.Filter.Column,
			Value:  value,
		}
	}

	rows, err := txn.ScanTable(s.TableName, filter)
	if err != nil {
		return nil, fmt.Errorf("executor: scanning table %q: %w", s.TableName, err)
	}

	columns := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		columns = append(columns, column.Name)
	}

	return RowsResult{
		Columns: columns,
		Rows:    rows,
	}, nil
}

// OrderExecutor 对应 planner.OrderNode
type OrderExecutor struct {
	// Source 产生待排序的 RowsResult
	Source  Executor
	OrderBy []parser.OrderBy
}

func (o OrderExecutor) Execute(txn Transaction) (ResultSet, error) {
	// 先扫描数据
	sourceResult, err := o.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing ORDER BY source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: ORDER BY source returned %T, want RowsResult", sourceResult)
	}

	// 将 ORDER BY item的列名转成 idx
	columnIdxs := make([]int, len(o.OrderBy))
	for i, order := range o.OrderBy {
		targetIdx := slices.Index(rowsResult.Columns, order.Column)
		if targetIdx == -1 {
			return nil, fmt.Errorf("executor: ORDER BY column %q does not exist", order.Column)
		}

		columnIdxs[i] = targetIdx
	}

	rows := slices.Clone(rowsResult.Rows)

	var compareErr error

	slices.SortStableFunc(rows, func(a, b types.Row) int {
		if compareErr != nil {
			return 0
		}

		for i, order := range o.OrderBy {
			targetIdx := columnIdxs[i]

			cmp, err := a[targetIdx].Compare(b[targetIdx])
			if err != nil {
				compareErr = fmt.Errorf("executor: comparing ORDER BY column %q: %w", order.Column, err)
				return 0
			}
			if cmp == 0 {
				continue
			}

			if order.Direction == parser.OrderDescending {
				return -cmp
			}
			return cmp
		}
		return 0
	})

	if compareErr != nil {
		return nil, compareErr
	}

	return RowsResult{
		Columns: rowsResult.Columns,
		Rows:    rows,
	}, nil
}

type OffsetExecutor struct {
	Source Executor
	Offset int
}

func (o OffsetExecutor) Execute(txn Transaction) (ResultSet, error) {
	sourceResult, err := o.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing OFFSET source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: OFFSET source returned %T, want RowsResult", sourceResult)
	}

	start := o.Offset
	if start > len(rowsResult.Rows) {
		start = len(rowsResult.Rows)
	}

	return RowsResult{
		Columns: rowsResult.Columns,
		Rows:    rowsResult.Rows[start:],
	}, nil
}

type LimitExecutor struct {
	Source Executor
	Limit  int
}

func (l LimitExecutor) Execute(txn Transaction) (ResultSet, error) {
	sourceResult, err := l.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing LIMIT source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: LIMIT source returned %T, want RowsResult", sourceResult)
	}

	// LIMIT 大于结果总数时，返回全部结果
	end := l.Limit
	if end > len(rowsResult.Rows) {
		end = len(rowsResult.Rows)
	}

	return RowsResult{
		Columns: rowsResult.Columns,
		Rows:    rowsResult.Rows[:end],
	}, nil
}

type ProjectionExecutor struct {
	Source Executor
	Items  []parser.SelectItem
}

func (p ProjectionExecutor) Execute(txn Transaction) (ResultSet, error) {
	sourceResult, err := p.Source.Execute(txn)
	if err != nil {
		return nil, fmt.Errorf("executor: executing projection source: %w", err)
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: projection source returned %T, want RowsResult", sourceResult)
	}

	// 每个投影项提前解析一次。
	//
	// columnIndex >= 0 表示从原始 Row 中读取指定列
	// columnIndex == -1 表示每一行都使用 constant
	items := make([]struct {
		columnIndex int
		constant    types.Value
		outputName  string
	}, 0, len(p.Items))

	for _, item := range p.Items {
		outputName := "?column?"
		if item.Alias != nil {
			outputName = *item.Alias
		}

		if item.Expression.Kind == parser.ColumnReference {
			columnName, ok := item.Expression.Value.(string)
			if !ok {
				return nil, fmt.Errorf("executor: column reference contains %T", item.Expression.Value)
			}

			columnIndex := slices.Index(rowsResult.Columns, columnName)
			if columnIndex < 0 {
				return nil, fmt.Errorf("executor: projection column %q does not exist", columnName)
			}

			// 没有 AS 时，结果列名沿用原始列名
			if item.Alias == nil {
				outputName = columnName
			}

			items = append(items, struct {
				columnIndex int
				constant    types.Value
				outputName  string
			}{
				columnIndex: columnIndex,
				outputName:  outputName,
			})
			continue
		}

		// 非列引用表达式按照常量处理。
		//
		// 例如 SELECT 100 AS fixed_score FROM users
		value, err := planner.ValueFromExpression(item.Expression)
		if err != nil {
			return nil, fmt.Errorf("executor: converting projection expression: %w", err)
		}

		items = append(items, struct {
			columnIndex int
			constant    types.Value
			outputName  string
		}{
			columnIndex: -1,
			constant:    value,
			outputName:  outputName,
		})
	}

	columns := make([]string, len(items))
	for index, item := range items {
		columns[index] = item.outputName
	}

	rows := make([]types.Row, 0, len(rowsResult.Rows))
	for rowIndex, sourceRow := range rowsResult.Rows {
		projectedRow := make(types.Row, 0, len(items))

		for _, item := range items {
			if item.columnIndex == -1 {
				projectedRow = append(projectedRow, item.constant)
				continue
			}

			if item.columnIndex >= len(sourceRow) {
				return nil, fmt.Errorf(
					"executor: row %d does not contain projected column index %d", rowIndex+1, item.columnIndex)
			}

			projectedRow = append(
				projectedRow,
				sourceRow[item.columnIndex],
			)
		}

		rows = append(rows, projectedRow)
	}

	return RowsResult{
		Columns: columns,
		Rows:    rows,
	}, nil
}

// UpdateExecutor 对应 planner.UpdateNode。
type UpdateExecutor struct {
	TableName   string
	Source      Executor
	Assignments map[string]parser.Expression
}

func (u UpdateExecutor) Execute(txn Transaction) (ResultSet, error) {
	table, err := txn.GetTable(u.TableName)
	if err != nil {
		return nil, fmt.Errorf("executor: loading table %q: %w", u.TableName, err)
	}
	if table == nil {
		return nil, fmt.Errorf("executor: table %q does not exist", u.TableName)
	}

	// 列位置与运行时值只转换一次，避免每更新一行都重复处理。
	assignments := make([]struct {
		columnIndex int
		value       types.Value
	}, 0, len(u.Assignments))

	for columnName, expression := range u.Assignments {
		columnIndex, err := table.ColumnIndex(columnName)
		if err != nil {
			return nil, fmt.Errorf("executor: resolving update column %q: %w", columnName, err)
		}
		value, err := planner.ValueFromExpression(expression)
		if err != nil {
			return nil, fmt.Errorf("executor: converting update value for column %q: %w", columnName, err)
		}
		assignments = append(assignments, struct {
			columnIndex int
			value       types.Value
		}{
			columnIndex: columnIndex,
			value:       value,
		})
	}

	if u.Source == nil {
		return nil, fmt.Errorf("executor: UPDATE source is nil")
	}

	sourceResult, err := u.Source.Execute(txn)
	if err != nil {
		return nil, err
	}
	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: UPDATE source returned %T, want RowsResult", sourceResult)
	}

	updated := 0
	for rowIndex, row := range rowsResult.Rows {
		oldPrimaryKey, err := table.PrimaryKeyValue(row)
		if err != nil {
			return nil, fmt.Errorf("executor: reading primary key from row %d: %w", rowIndex+1, err)
		}

		updatedRow := slices.Clone(row)
		for _, assignment := range assignments {
			updatedRow[assignment.columnIndex] = assignment.value
		}

		if err := txn.UpdateRow(table, oldPrimaryKey, updatedRow); err != nil {
			return nil, fmt.Errorf("executor: updating row %d: %w", rowIndex+1, err)
		}
		updated++
	}

	return UpdateResult{Count: updated}, nil
}

// DeleteExecutor 对应 planner.DeleteNode
//
// Source 先扫描出符合 WHERE 条件的行
// 然后根据每一行的主键执行删除操作
type DeleteExecutor struct {
	TableName string
	Source    Executor
}

func (d DeleteExecutor) Execute(txn Transaction) (ResultSet, error) {
	table, err := txn.GetTable(d.TableName)
	if err != nil {
		return nil, fmt.Errorf("executor: loading table %q: %w", d.TableName, err)
	}
	if table == nil {
		return nil, fmt.Errorf("executor: table %q does not exist", d.TableName)
	}

	sourceResult, err := d.Source.Execute(txn)
	if err != nil {
		return nil, err
	}

	rowsResult, ok := sourceResult.(RowsResult)
	if !ok {
		return nil, fmt.Errorf("executor: DELETE source returned %T, want RowsResult", sourceResult)
	}

	deleted := 0

	for i, row := range rowsResult.Rows {
		pk, err := table.PrimaryKeyValue(row)
		if err != nil {
			return nil, fmt.Errorf("executor: reading primary key from row %d: %w", i+1, err)
		}

		if err := txn.DeleteRow(table, pk); err != nil {
			return nil, fmt.Errorf("executor: deleting raw %d: %w", i+1, err)
		}

		deleted++
	}
	return DeleteResult{Count: deleted}, nil
}

// Build 根据 plan 节点创建对应的 Executor
func Build(node planner.Node) (Executor, error) {
	switch node := node.(type) {
	case planner.CreateTableNode:
		return CreateTableExecutor{Schema: node.Schema}, nil

	case planner.InsertNode:
		return InsertExecutor{TableName: node.TableName, Columns: node.Columns, Values: node.Values}, nil

	case planner.ScanNode:
		return ScanExecutor{TableName: node.TableName, Filter: node.Filter}, nil

	case planner.UpdateNode:
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building UPDATE source: %w", err)
		}
		return UpdateExecutor{
			TableName:   node.TableName,
			Source:      source,
			Assignments: node.Assignments,
		}, nil

	case planner.DeleteNode:
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building DELETE source: %w", err)
		}
		return DeleteExecutor{
			TableName: node.TableName,
			Source:    source,
		}, nil

	case planner.OrderNode:
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building ORDER BY source: %w", err)
		}

		return OrderExecutor{
			Source:  source,
			OrderBy: node.OrderBy,
		}, nil

	case planner.OffsetNode:
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building OFFSET source: %w", err)
		}
		return OffsetExecutor{
			Source: source,
			Offset: node.Offset,
		}, nil

	case planner.LimitNode:
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building LIMIT source: %w", err)
		}

		return LimitExecutor{
			Source: source,
			Limit:  node.Limit,
		}, nil

	case planner.AggregateNode:
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building aggregate source: %w", err)
		}

		return AggregateExecutor{
			Source: source,
			Items:  node.Items,
		}, nil

	case planner.ProjectionNode:
		// Projection 的数据来源可能是 Scan、Order、Offset 或 Limit
		// 因此继续通过 Build 递归构造，不能直接假设它是 ScanNode
		source, err := Build(node.Source)
		if err != nil {
			return nil, fmt.Errorf("executor: building projection source: %w", err)
		}

		return ProjectionExecutor{
			Source: source,
			Items:  node.Items,
		}, nil

	case planner.NestedLoopJoinNode:
		left, err := Build(node.Left)
		if err != nil {
			return nil, fmt.Errorf("executor: building left join input: %w", err)
		}

		right, err := Build(node.Right)
		if err != nil {
			return nil, fmt.Errorf("executor: building right join input: %w", err)
		}

		return NestedLoopJoinExecutor{
			Left:      left,
			Right:     right,
			Predicate: node.Predicate,
			Outer:     node.Outer,
		}, nil

	default:
		return nil, fmt.Errorf("executor: unsupported plan node %T", node)
	}
}

// makeInsertRow 将 INSERT 提供的值整理为完整行
//
// 当 INSERT 没有指定列名时：
//
//	INSERT INTO users VALUES (1, 'alice')
//
// 值按照表字段顺序排列，缺少的尾部字段使用默认值。
//
// 当 INSERT 指定列名时：
//
//	INSERT INTO users (name, id) VALUES ('alice', 1)
//
// 函数会按照表定义的顺序重新排列字段
func makeInsertRow(table *schema.Table, columnNames []string, input types.Row) (types.Row, error) {
	// 没有指定列名，按定义的顺序
	if len(columnNames) == 0 {
		if len(input) > len(table.Columns) {
			return nil, fmt.Errorf("too many values: got %d, columns %d", len(input), len(table.Columns))
		}

		result := make(types.Row, 0, len(table.Columns))
		result = append(result, input...)
		for i := len(input); i < len(table.Columns); i++ {
			column := table.Columns[i]
			if column.Default == nil { // 必须赋值
				return nil, fmt.Errorf("no default value for column %q", column.Name)
			}
			result = append(result, *column.Default)
		}
		return result, nil
	}

	// 指定了列名
	if len(columnNames) != len(input) {
		return nil, fmt.Errorf("columns and values count mismatch: columns %d, values %d", len(columnNames), len(input))
	}
	positions := make(map[string]int, len(table.Columns))
	for i, column := range table.Columns {
		positions[column.Name] = i
	}

	result := make(types.Row, len(table.Columns))
	filled := make([]bool, len(table.Columns))

	for i, columnName := range columnNames {
		columnIdx, ok := positions[columnName]
		if !ok {
			return nil, fmt.Errorf("unknown column %q", columnName)
		}
		// 列名重复
		if filled[columnIdx] {
			return nil, fmt.Errorf("column %q specified more than once", columnName)
		}

		result[columnIdx] = input[i]
		filled[columnIdx] = true
	}

	// 没有显式提供的字段使用默认值
	for i, column := range table.Columns {
		if filled[i] {
			continue
		}
		if column.Default == nil {
			return nil, fmt.Errorf("no value given for column %q", column.Name)
		}
		result[i] = *column.Default
	}
	return result, nil
}
