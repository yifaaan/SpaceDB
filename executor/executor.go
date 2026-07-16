package executor

import (
	"errors"
	"fmt"
	"spacedb/parser"
	"spacedb/planner"
	"spacedb/schema"
	"spacedb/types"
)

var ErrNotImplemented = errors.New("executor: execution not implemented")

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

func (cte CreateTableExecutor) Execute(txn Transaction) (ResultSet, error) {
	if err := txn.CreateTable(cte.Schema); err != nil {
		return nil, err
	}

	return CreateTableResult{cte.Schema.Name}, nil
}

// InsertExecutor 对应 planner.InsertNode
type InsertExecutor struct {
	TableName string
	Columns   []string
	Values    [][]parser.Expression
}

func (ie InsertExecutor) Execute(txn Transaction) (ResultSet, error) {

	table, err := txn.GetTable(ie.TableName)
	if err != nil {
		return nil, fmt.Errorf("executor: loading table %q: %w", ie.TableName, err)
	}
	if table == nil {
		return nil, fmt.Errorf("executor: table %q does not exist", ie.TableName)
	}

	inserted := 0
	for rowIdx, exps := range ie.Values {
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
		fullRow, err := makeInsertRow(table, ie.Columns, row)
		if err != nil {
			return nil, fmt.Errorf("executor: preparing row %d: %w", rowIdx+1, err)
		}
		if err := txn.CreateRow(ie.TableName, fullRow); err != nil {
			return nil, fmt.Errorf("executor: inserting row %d: %w", rowIdx+1, err)
		}
		inserted++
	}

	return InsertResult{Count: inserted}, nil
}

// ScanExecutor 对应 planner.ScanNode
type ScanExecutor struct {
	TableName string
}

func (ScanExecutor) Execute(_ Transaction) (ResultSet, error) {
	return nil, ErrNotImplemented
}

// Build 根据 plan 节点创建对应的 Executor
func Build(node planner.Node) (Executor, error) {
	switch node := node.(type) {
	case planner.CreateTableNode:
		return CreateTableExecutor{Schema: node.Schema}, nil

	case planner.InsertNode:
		return InsertExecutor{TableName: node.TableName, Columns: node.Columns, Values: node.Values}, nil

	case planner.ScanNode:
		return ScanExecutor{TableName: node.TableName}, nil

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
