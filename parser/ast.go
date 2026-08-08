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
)

// Expression 常量表达式
type Expression struct {
	Kind  ExpressionKind
	Value any // nil / bool / int64 / float64 / string
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

// SelectStatement SELECT 语句的 AST
type SelectStatement struct {
	TableName string

	OrderBy []OrderBy
}

// EqualityFilter 表示 WHERE 条件：
//
//	WHERE column = constant
//
// 暂时不支持 AND、OR、大于、小于...
type EqualityFilter struct {
	Column string
	Value  Expression
}

// UpdateStatement 表示 UPDATE 语句
type UpdateStatement struct {
	TableName string

	// Assignments 保存 SET 后的“列名 -> 常量表达式”。
	Assignments map[string]Expression

	// Filter 为 nil 表示没有 WHERE，即更新表中全部行。
	Filter *EqualityFilter
}

// DeleteStatement 表示 DELETE FROM 语句
//
// Filter 为 nil 表示删除整张表；
// 非 nil 时只删除满足 column = literal 的行
type DeleteStatement struct {
	TableName string
	Filter    *EqualityFilter
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
