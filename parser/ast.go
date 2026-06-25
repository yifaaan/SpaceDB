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

type Statement interface {
	statement()
}

func (CreateTableStatement) statement() {}
func (InsertStatement) statement()      {}
func (SelectStatement) statement()      {}

var _ Statement = CreateTableStatement{}
var _ Statement = InsertStatement{}
var _ Statement = SelectStatement{}

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
