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

// SelectStatement SELECT 语句的 AST
type SelectStatement struct {
	TableName string
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

type Statement interface {
	statement()
}

func (CreateTableStatement) statement() {}
func (InsertStatement) statement()      {}
func (SelectStatement) statement()      {}
func (UpdateStatement) statement()      {}

var _ Statement = CreateTableStatement{}
var _ Statement = InsertStatement{}
var _ Statement = SelectStatement{}
var _ Statement = UpdateStatement{}

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
