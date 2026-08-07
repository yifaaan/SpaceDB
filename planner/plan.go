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

// ScanNode 表示表扫描。
//
// Filter 为 nil 表示扫描全部行；
// 非 nil 时只返回满足“列 = 常量”的行
type ScanNode struct {
	TableName string
	Filter    *parser.EqualityFilter
}

func (ScanNode) node() {}

// UpdateNode 表示 UPDATE 的执行计划
//
// Source 通常是一个 ScanNode，负责找出需要更新的行
// Assignments 保存每个目标列对应的新常量值
type UpdateNode struct {
	TableName   string
	Source      Node
	Assignments map[string]parser.Expression
}

func (UpdateNode) node() {}

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
				Filter:    nil,
			},
		}, nil

	case parser.UpdateStatement:
		source := ScanNode{
			TableName: stmt.TableName,
			Filter:    stmt.Filter,
		}

		return Plan{
			Node: UpdateNode{
				TableName:   stmt.TableName,
				Source:      source,
				Assignments: stmt.Assignments,
			},
		}, nil
	default:
		return Plan{}, fmt.Errorf(
			"planner: statement type %T is not implemented",
			stmt,
		)
	}
}
