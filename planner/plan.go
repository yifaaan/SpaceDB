package planner

import (
	"fmt"
	"spacedb/parser"
	"spacedb/schema"
	"spacedb/types"
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

// OrderNode ORDER BY 排序操作
//
// 如：
//
//	SELECT * FROM users ORDER BY score DESC;
//
// 生成：
//
//	OrderNode{
//	    Source: ScanNode{TableName: "users"},
//	    OrderBy: []parser.OrderBy{
//	        {Column: "score", Direction: parser.OrderDescending},
//	    },
//	}
type OrderNode struct {
	// Source 是被排序的数据源，通常是 ScanNode
	Source Node

	OrderBy []parser.OrderBy
}

func (OrderNode) node() {}

type LimitNode struct {
	// Source 执行查询、排序或 OFFSET
	Source Node
	Limit  int
}

func (LimitNode) node() {}

// OFFSET 必须在 LIMIT 之前执行
//
//	SELECT ... LIMIT 10 OFFSET 20
//
//	Scan -> Order -> Offset -> Limit
type OffsetNode struct {
	Source Node
	Offset int
}

func (OffsetNode) node() {}

// ProjectionNode 表示 SELECT 的列投影
//
// Source 负责产生完整的行数据，ProjectionNode 最后只保留
// SELECT 指定的列或常量表达式，并处理 AS 别名
//
// 如：
//
//	SELECT name AS username, score FROM users;
//
// Items 中会保存 name 和 score 两个投影项
type ProjectionNode struct {
	Source Node
	Items  []parser.SelectItem
}

func (ProjectionNode) node() {}

// NestedLoopJoinNode 表示嵌套循环 Join
//
// Left 和 Right 可以是 JoinNode，支持多表连接
type NestedLoopJoinNode struct {
	Left  Node
	Right Node
	// Predicate 为 nil 表示 CROSS JOIN
	Predicate *parser.Expression
	// Outer 为 true 表示保留左侧未匹配的行
	Outer bool
}

func (NestedLoopJoinNode) node() {}

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

// DeleteNode 表示 DELETE 的执行计划
//
// Source 是一个 ScanNode，负责找出待删除的行
type DeleteNode struct {
	TableName string
	Source    Node
}

func (DeleteNode) node() {}

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
		// SELECT 首先构造tabel/join数据
		node, err := buildFromItem(stmt.From)
		if err != nil {
			return Plan{}, fmt.Errorf("planner: building FROM clause: %w", err)
		}

		if len(stmt.OrderBy) != 0 {
			node = OrderNode{
				Source:  node,
				OrderBy: stmt.OrderBy,
			}
		}

		// OFFSET
		if stmt.Offset != nil {
			v, err := ValueFromExpression(*stmt.Offset)
			if err != nil {
				return Plan{}, fmt.Errorf("planner: converting OFFSET value: %w", err)
			}

			if v.Kind != types.ValueInteger || v.Integer < 0 {
				return Plan{}, fmt.Errorf("planner: OFFSET must be a non-negative integer")
			}

			if uint64(v.Integer) > uint64(^uint(0)>>1) {
				return Plan{}, fmt.Errorf("planner: OFFSET is too large")
			}

			node = OffsetNode{
				Source: node,
				Offset: int(v.Integer),
			}
		}

		// LIMIT
		if stmt.Limit != nil {
			v, err := ValueFromExpression(*stmt.Limit)
			if err != nil {
				return Plan{}, fmt.Errorf("planner: converting LIMIT value: %w", err)
			}

			if v.Kind != types.ValueInteger || v.Integer < 0 {
				return Plan{}, fmt.Errorf(
					"planner: LIMIT must be a non-negative integer",
				)
			}

			if uint64(v.Integer) > uint64(^uint(0)>>1) {
				return Plan{}, fmt.Errorf(
					"planner: LIMIT is too large",
				)
			}

			node = LimitNode{
				Source: node,
				Limit:  int(v.Integer),
			}
		}

		if len(stmt.SelectItems) != 0 {
			node = ProjectionNode{
				Source: node,
				Items:  stmt.SelectItems,
			}
		}

		return Plan{Node: node}, nil

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

	case parser.DeleteStatement:
		source := ScanNode{
			TableName: stmt.TableName,
			Filter:    stmt.Filter,
		}

		return Plan{
			Node: DeleteNode{
				TableName: stmt.TableName,
				Source:    source,
			},
		}, nil

	default:
		return Plan{}, fmt.Errorf(
			"planner: statement type %T is not implemented",
			stmt,
		)
	}
}

func buildFromItem(item parser.FromItem) (Node, error) {
	switch item := item.(type) {
	case parser.TableFromItem:
		return ScanNode{item.Name, nil}, nil

	case parser.JoinFromItem:
		if err := validateJoinItem(item); err != nil {
			return nil, err
		}

		leftItem := item.Left
		rightItem := item.Right

		// A RIGHT JOIN B 等价于：
		//
		// B LEFT JOIN A
		if item.Type == parser.JoinRight {
			leftItem, rightItem = rightItem, leftItem
		}

		left, err := buildFromItem(leftItem)
		if err != nil {
			return nil, fmt.Errorf("building left join input: %w", err)
		}

		right, err := buildFromItem(rightItem)
		if err != nil {
			return nil, fmt.Errorf("building right join input: %w", err)
		}

		return NestedLoopJoinNode{left, right, item.Predicate, item.Type == parser.JoinLeft || item.Type == parser.JoinRight}, nil

	default:
		return nil, fmt.Errorf("planner: unsupported FROM item %T", item)
	}
}

func validateJoinItem(item parser.JoinFromItem) error {
	switch item.Type {
	case parser.JoinCross:
		if item.Predicate != nil {
			return fmt.Errorf("planner: CROSS JOIN cannot have a predicate")
		}

	case parser.JoinInner, parser.JoinLeft, parser.JoinRight:
		if item.Predicate == nil {
			return fmt.Errorf("planner: join type %d requires a predicate", item.Type)
		}

	default:
		return fmt.Errorf("planner: unsupported join type %d", item.Type)
	}

	return nil
}
