package planner

import (
	"fmt"
	"spacedb/parser"
	"spacedb/schema"
)

// Node 执行节点，Planner 的构建产物
type Node interface {
	node()
}

// CreateTableNode 表示“创建表”的执行节点
type CreateTableNode struct {
	Schema schema.Table
}

func (CreateTableNode) node() {}

// InsertNode 表示 INSERT INTO ... VALUES ... 的执行节点
type InsertNode struct {
	TableName string
	Columns   []string
	Values    [][]parser.Expression
}

func (InsertNode) node() {}

// ScanNode 表示对一张表进行完整扫描的执行节点
//
// 当前 只支持 SELECT * FROM table，
type ScanNode struct {
	TableName string
}

func (ScanNode) node() {}

// Plan 一个完整的执行计划
// 当前一个 SQL 语句对应一个根节点
type Plan struct {
	Node Node
}

// Build 将 Parser AST 转换为执行计划。
//
// 当前支持参考提交中的三种语句：
// CREATE TABLE、INSERT INTO ... VALUES 和 SELECT * FROM。
func Build(stmt parser.Statement) (Plan, error) {
	switch stmt := stmt.(type) {
	case parser.CreateTableStatement:
		table, err := tableFromCreateStatement(stmt)
		if err != nil {
			return Plan{}, fmt.Errorf("planner: building create-table plan: %w", err)
		}
		return Plan{Node: CreateTableNode{Schema: table}}, nil

	case parser.InsertStatement:
		columns := stmt.Columns

		if columns == nil {
			columns = []string{}
		}

		return Plan{
			Node: InsertNode{
				TableName: stmt.TableName,
				Columns:   columns,
				Values:    stmt.Values,
			},
		}, nil

	case parser.SelectStatement:
		return Plan{
			Node: ScanNode{
				TableName: stmt.TableName,
			},
		}, nil

	default:
		return Plan{}, fmt.Errorf(
			"planner: statement type %T is not implemented",
			stmt,
		)
	}
}
