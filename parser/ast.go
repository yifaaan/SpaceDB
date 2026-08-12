package parser

import (
	"spacedb/lexer"
	"spacedb/types"
)

// ExpressionKind 表示常量表达式的种类。
type ExpressionKind uint8

const (
	Null ExpressionKind = iota
	BooleanLiteral
	IntegerLiteral
	FloatLiteral
	StringLiteral

	// OperationExpression 由运算符连接的表达式
	// 当前只支持 ON left_column = right_column
	OperationExpression

	// FunctionExpression 表示函数调用
	FunctionExpression

	// ColumnReference 表示 SELECT 中引用某个表字段
	//
	// 如 SELECT name FROM users 中的 name
	ColumnReference
)

type OperationKind uint8

const (
	OperationEqual OperationKind = iota
	OperationGreaterThan
	OperationLessThan
)

// Operation 表示二元表达式, Expression.Value类型
//
//	WHERE score > 60
//
// 表示为：
//
//	Operation{
//	    Kind: OperationGreaterThan,
//	    Left: Expression{
//	        Kind:  ColumnReference,
//	        Value: "score",
//	    },
//	    Right: Expression{
//	        Kind:  IntegerLiteral,
//	        Value: int64(60),
//	    },
//	}
//
// 当前支持 =、>、<
type Operation struct {
	Kind  OperationKind
	Left  Expression
	Right Expression
}

// FunctionCall SQL 函数调用
//
//	COUNT(score)
//
// 表示为：
//
//	FunctionCall{
//	    Name:     "count",
//	    Argument: "score",
//	}
type FunctionCall struct {
	Name     string
	Argument string
}

// Expression SQL 表达式
//
// Value 的实际类型取决于 Kind：
//
//	Null             -> nil
//	BooleanLiteral   -> bool
//	IntegerLiteral   -> int64
//	FloatLiteral     -> float64
//	StringLiteral    -> string
//	ColumnReference  -> string，保存列名
//	OperationExpression -> Operation
//	FunctionExpression -> FunctionCall
type Expression struct {
	Kind  ExpressionKind
	Value any // nil / bool / int64 / float64 / string / string
}

// NullExpression 构造 NULL 常量表达式
func NullExpression() Expression {
	return Expression{Kind: Null}
}

// Column 描述 CREATE TABLE 中的一列
type Column struct {
	Name     string
	DataType types.DataType
	// nil   = 未写 NULL / NOT NULL 约束
	//
	// &true = 显式 NULL
	//
	// &false = 显式 NOT NULL。
	Nullable     *bool
	DefaultValue *Expression

	// PrimaryKey 表示该列是否带有 PRIMARY KEY 约束
	PrimaryKey bool
}

// CreateTableStatement CREATE TABLE 的 AST
type CreateTableStatement struct {
	Name    string
	Columns []Column
}

// InsertStatement INSERT 语句的 AST
type InsertStatement struct {
	TableName string
	Columns   []string
	Values    [][]Expression
}

type OrderDirection uint8

const (
	// OrderAscending 升序
	//
	// 没有显式指定 ASC 或 DESC 时，默认使用升序
	OrderAscending OrderDirection = iota

	// OrderDescending 降序
	OrderDescending
)

// OrderBy ORDER BY 中的一项排序
//
//	ORDER BY score DESC
//
// 表示：
//
//	OrderBy{
//	    Column:    "score",
//	    Direction: OrderDescending,
//	}
type OrderBy struct {
	Column    string
	Direction OrderDirection
}

// SelectItem 表示 SELECT 后的一项投影
//
// 如：
//
//	SELECT name AS username
//
// 表示为 Expression = name，Alias = username
type SelectItem struct {
	Expression Expression

	Alias *string
}

// FromItem 表示 SELECT 的数据来源：TABLE / JOINED TABLE
type FromItem interface {
	fromItem()
}

// TableFromItem
//
//	Select * FROM users
//
// 对应 TableFromItem{Name: "users"}
type TableFromItem struct {
	Name string
}

func (TableFromItem) fromItem() {}

// JoinType 表示连接类型
type JoinType uint8

const (
	JoinCross JoinType = iota
	JoinInner
	JoinLeft
	JoinRight
)

// JoinFromItem 连接
//
// 可递归嵌套
//
//	FROM t1 CROSS JOIN t2 CROSS JOIN t3
//
// 表示为左结合树：
//
//	Join
//	├── Join
//	│   ├── Table(t1)
//	│   └── Table(t2)
//	└── Table(t3)
type JoinFromItem struct {
	Left  FromItem
	Right FromItem
	Type  JoinType

	// Predicate Join 匹配条件
	//
	// CROSS JOIN 没有 ON 条件，为 nil；
	// INNER、LEFT、RIGHT JOIN 必须是非 nil
	Predicate *Expression
}

func (JoinFromItem) fromItem() {}

var _ FromItem = TableFromItem{}
var _ FromItem = JoinFromItem{}

// SelectStatement SELECT 语句的 AST
type SelectStatement struct {
	// TableFromItem / JoinFromItem
	From FromItem

	// 空切片表示 SELECT *
	SelectItems []SelectItem

	Where *Expression

	// GroupBy 保存 GROUP BY 后面的表达式
	GroupBy *Expression

	// Having 在聚合之后过滤聚合结果
	Having *Expression

	OrderBy []OrderBy

	Limit *Expression

	Offset *Expression
}

// UpdateStatement 表示 UPDATE 语句
type UpdateStatement struct {
	TableName string

	// Assignments 保存 SET 后的“列名 -> 常量表达式”。
	Assignments map[string]Expression

	// Filter 为 nil 表示没有 WHERE，即更新表中全部行。
	// 非 nil 时保存完整的比较表达式，例如 age > 18。
	Filter *Expression
}

// DeleteStatement 表示 DELETE FROM 语句
//
// Filter 为 nil 表示删除整张表；
// 非 nil 时只删除满足比较表达式的行
type DeleteStatement struct {
	TableName string
	Filter    *Expression
}

type Statement interface {
	statement()
}

func (CreateTableStatement) statement() {}
func (InsertStatement) statement()      {}
func (SelectStatement) statement()      {}
func (UpdateStatement) statement()      {}
func (DeleteStatement) statement()      {}

var _ Statement = CreateTableStatement{}
var _ Statement = InsertStatement{}
var _ Statement = SelectStatement{}
var _ Statement = UpdateStatement{}
var _ Statement = DeleteStatement{}

// keywordDataType 把类型关键字归一化为 DataType
// INT / INTEGER → Integer；BOOL / BOOLEAN → Boolean；
// FLOAT / DOUBLE → Float；STRING / TEXT / VARCHAR → String。
func keywordDataType(keyword lexer.Keyword) (types.DataType, bool) {
	switch keyword {
	case lexer.KeywordInt, lexer.KeywordInteger:
		return types.Integer, true
	case lexer.KeywordBoolean, lexer.KeywordBool:
		return types.Boolean, true
	case lexer.KeywordFloat, lexer.KeywordDouble:
		return types.Float, true
	case lexer.KeywordString, lexer.KeywordText, lexer.KeywordVarchar:
		return types.String, true
	default:
		return 0, false
	}
}
