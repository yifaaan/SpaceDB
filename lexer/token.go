package lexer

import (
	"fmt"
	"strings"
)

type Keyword string

const (
	KeywordCreate  Keyword = "CREATE"
	KeywordTable   Keyword = "TABLE"
	KeywordInt     Keyword = "INT"
	KeywordInteger Keyword = "INTEGER"
	KeywordBoolean Keyword = "BOOLEAN"
	KeywordBool    Keyword = "BOOL"
	KeywordString  Keyword = "STRING"
	KeywordText    Keyword = "TEXT"
	KeywordVarchar Keyword = "VARCHAR"
	KeywordFloat   Keyword = "FLOAT"
	KeywordDouble  Keyword = "DOUBLE"
	KeywordSelect  Keyword = "SELECT"
	KeywordFrom    Keyword = "FROM"
	KeywordInsert  Keyword = "INSERT"
	KeywordInto    Keyword = "INTO"
	KeywordValues  Keyword = "VALUES"
	KeywordTrue    Keyword = "TRUE"
	KeywordFalse   Keyword = "FALSE"
	KeywordDefault Keyword = "DEFAULT"
	KeywordNot     Keyword = "NOT"
	KeywordNull    Keyword = "NULL"
	KeywordPrimary Keyword = "PRIMARY"
	KeywordKey     Keyword = "KEY"
)

type Kind uint8

const (
	EndOfInput Kind = iota
	KeywordKind
	Identifier
	String
	Number
	OpenParen
	CloseParen
	Comma
	Semicolon
	Asterisk
	Plus
	Minus
	Slash
)

type Token struct {
	Kind    Kind
	Keyword Keyword
	Value   any
	Offset  int
}

func KeywordFromIdentifier(identifier string) (Keyword, bool) {
	keyword := Keyword(strings.ToUpper(identifier))
	switch keyword {
	case KeywordCreate, KeywordTable, KeywordInt, KeywordInteger,
		KeywordBoolean, KeywordBool, KeywordString, KeywordText,
		KeywordVarchar, KeywordFloat, KeywordDouble, KeywordSelect,
		KeywordFrom, KeywordInsert, KeywordInto, KeywordValues,
		KeywordTrue, KeywordFalse, KeywordDefault, KeywordNot,
		KeywordNull, KeywordPrimary, KeywordKey:
		return keyword, true
	default:
		return "", false
	}
}

func (k Kind) String() string {
	switch k {
	case EndOfInput:
		return "end of input"
	case KeywordKind:
		return "keyword"
	case Identifier:
		return "identifier"
	case String:
		return "string literal"
	case Number:
		return "number literal"
	case OpenParen:
		return "'('"
	case CloseParen:
		return "')'"
	case Comma:
		return "','"
	case Semicolon:
		return "';'"
	case Asterisk:
		return "'*'"
	case Plus:
		return "'+'"
	case Minus:
		return "'-'"
	case Slash:
		return "'/'"
	default:
		return "unknown token"
	}
}

func DescribePosition(offset int) string {
	return fmt.Sprintf("offset %d", offset)
}

func DescribeToken(token Token) string {
	switch token.Kind {
	case KeywordKind:
		return fmt.Sprintf("keyword '%s'", token.Keyword)
	case Identifier, String:
		return fmt.Sprintf("%s '%s'", token.Kind, token.Value)
	case Number:
		return fmt.Sprintf("number literal %v", token.Value)
	default:
		return token.Kind.String()
	}
}
