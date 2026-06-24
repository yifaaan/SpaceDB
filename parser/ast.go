package parser

import "spacedb/lexer"

type DataType uint8

const (
	Boolean DataType = iota
	Integer
	Float
	String
)

type ExpressionKind uint8

const (
	Null ExpressionKind = iota
	BooleanLiteral
	IntegerLiteral
	FloatLiteral
	StringLiteral
)

type Expression struct {
	Kind  ExpressionKind
	Value any // nil/bool/int64/float64/string
}

func NullExpression() Expression {
	return Expression{Kind: Null}
}

type Column struct {
	Name         string
	DataType     DataType
	Nullable     *bool
	DefaultValue *Expression
}

type CreateTableStatement struct {
	Name    string
	Columns []Column
}

type InsertStatement struct {
	TableName string
	Columns   []string
	Values    [][]Expression
}

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

func keywordDataType(keyword lexer.Keyword) (DataType, bool) {
	switch keyword {
	case lexer.KeywordInt, lexer.KeywordInteger:
		return Integer, true
	case lexer.KeywordBoolean, lexer.KeywordBool:
		return Boolean, true
	case lexer.KeywordFloat, lexer.KeywordDouble:
		return Float, true
	case lexer.KeywordString, lexer.KeywordText, lexer.KeywordVarchar:
		return String, true
	default:
		return 0, false
	}
}
