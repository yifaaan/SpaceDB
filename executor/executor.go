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

func (InsertExecutor) Execute(_ Transaction) (ResultSet, error) {
	return nil, ErrNotImplemented
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
