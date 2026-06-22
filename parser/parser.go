package parser

import (
	"errors"
	"fmt"

	"spacedb/lexer"
)

type Parser struct {
	input  string
	tokens []lexer.Token
	index  int
}

func New(input string) *Parser {
	return &Parser{input: input}
}

func Parse(input string) (Statement, error) {
	return New(input).Parse()
}

func (p *Parser) Parse() (Statement, error) {
	tokens, err := lexer.Lex(p.input)
	if err != nil {
		return nil, err
	}
	p.tokens = tokens
	p.index = 0

	statement, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	if err := p.expect(lexer.Semicolon); err != nil {
		return nil, err
	}
	if err := p.expect(lexer.EndOfInput); err != nil {
		return nil, err
	}
	return statement, nil
}

func (p *Parser) peek() lexer.Token {
	return p.tokens[p.index]
}

func (p *Parser) next() lexer.Token {
	token := p.peek()
	if token.Kind != lexer.EndOfInput {
		p.index++
	}
	return token
}

func (p *Parser) expect(kind lexer.Kind) error {
	token := p.next()
	if token.Kind != kind {
		return fmt.Errorf("parser: expected %s, got %s at %s", kind, lexer.DescribeToken(token), lexer.DescribePosition(token.Offset))
	}
	return nil
}

func (p *Parser) expectKeyword(keyword lexer.Keyword) error {
	token := p.next()
	if token.Kind != lexer.KeywordKind || token.Keyword != keyword {
		return fmt.Errorf("parser: expected keyword '%s', got %s at %s", keyword, lexer.DescribeToken(token), lexer.DescribePosition(token.Offset))
	}
	return nil
}

func (p *Parser) expectIdentifier() (string, error) {
	token := p.next()
	if token.Kind != lexer.Identifier {
		return "", fmt.Errorf("parser: expected identifier, got %s at %s", lexer.DescribeToken(token), lexer.DescribePosition(token.Offset))
	}
	return token.Value.(string), nil
}

func (p *Parser) parseStatement() (Statement, error) {
	current := p.peek()
	if current.Kind == lexer.EndOfInput {
		return nil, fmt.Errorf("parser: expected statement, got end of input at %s", lexer.DescribePosition(current.Offset))
	}
	if current.Kind != lexer.KeywordKind {
		return nil, fmt.Errorf("parser: statement must begin with a keyword, got %s at %s", lexer.DescribeToken(current), lexer.DescribePosition(current.Offset))
	}

	switch current.Keyword {
	case lexer.KeywordSelect:
		return p.parseSelect()
	case lexer.KeywordCreate:
		return p.parseCreateTable()
	case lexer.KeywordInsert:
		return p.parseInsert()
	default:
		return nil, errors.New("parser: unsupported statement")
	}
}

func (p *Parser) parseSelect() (Statement, error) {
	if err := p.expectKeyword(lexer.KeywordSelect); err != nil {
		return nil, err
	}
	if err := p.expect(lexer.Asterisk); err != nil {
		return nil, err
	}
	if err := p.expectKeyword(lexer.KeywordFrom); err != nil {
		return nil, err
	}
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}
	return SelectStatement{TableName: tableName}, nil
}

func (p *Parser) parseCreateTable() (Statement, error) {
	if err := p.expectKeyword(lexer.KeywordCreate); err != nil {
		return nil, err
	}
	if err := p.expectKeyword(lexer.KeywordTable); err != nil {
		return nil, err
	}
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}
	if err := p.expect(lexer.OpenParen); err != nil {
		return nil, err
	}

	columns := make([]Column, 0, 1)
	for {
		column, err := p.parseColumn()
		if err != nil {
			return nil, err
		}
		columns = append(columns, column)
		if p.peek().Kind != lexer.Comma {
			break
		}
		if err := p.expect(lexer.Comma); err != nil {
			return nil, err
		}
	}
	if err := p.expect(lexer.CloseParen); err != nil {
		return nil, err
	}
	return CreateTableStatement{Name: tableName, Columns: columns}, nil
}

func (p *Parser) parseInsert() (Statement, error) {
	if err := p.expectKeyword(lexer.KeywordInsert); err != nil {
		return nil, err
	}
	if err := p.expectKeyword(lexer.KeywordInto); err != nil {
		return nil, err
	}
	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}

	var columns []string
	if p.peek().Kind == lexer.OpenParen {
		if err := p.expect(lexer.OpenParen); err != nil {
			return nil, err
		}
		first, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}
		columns = []string{first}
		for p.peek().Kind != lexer.CloseParen {
			if err := p.expect(lexer.Comma); err != nil {
				return nil, err
			}
			column, err := p.expectIdentifier()
			if err != nil {
				return nil, err
			}
			columns = append(columns, column)
		}
		if err := p.expect(lexer.CloseParen); err != nil {
			return nil, err
		}
	}

	if err := p.expectKeyword(lexer.KeywordValues); err != nil {
		return nil, err
	}
	values := make([][]Expression, 0, 1)
	for {
		if err := p.expect(lexer.OpenParen); err != nil {
			return nil, err
		}
		first, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		row := []Expression{first}
		for p.peek().Kind != lexer.CloseParen {
			if err := p.expect(lexer.Comma); err != nil {
				return nil, err
			}
			expression, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			row = append(row, expression)
		}
		if err := p.expect(lexer.CloseParen); err != nil {
			return nil, err
		}
		values = append(values, row)
		if p.peek().Kind != lexer.Comma {
			break
		}
		if err := p.expect(lexer.Comma); err != nil {
			return nil, err
		}
	}
	return InsertStatement{TableName: tableName, Columns: columns, Values: values}, nil
}

func (p *Parser) parseColumn() (Column, error) {
	name, err := p.expectIdentifier()
	if err != nil {
		return Column{}, err
	}
	token := p.next()
	if token.Kind != lexer.KeywordKind {
		return Column{}, fmt.Errorf("parser: expected column data type, got %s at %s", lexer.DescribeToken(token), lexer.DescribePosition(token.Offset))
	}
	dataType, ok := keywordDataType(token.Keyword)
	if !ok {
		return Column{}, fmt.Errorf("parser: unexpected column data type '%s' at %s", token.Keyword, lexer.DescribePosition(token.Offset))
	}

	column := Column{Name: name, DataType: dataType}
	for p.peek().Kind == lexer.KeywordKind {
		next := p.peek()
		switch next.Keyword {
		case lexer.KeywordNull:
			_ = p.next()
			value := true
			column.Nullable = &value
		case lexer.KeywordNot:
			_ = p.next()
			if err := p.expectKeyword(lexer.KeywordNull); err != nil {
				return Column{}, err
			}
			value := false
			column.Nullable = &value
		case lexer.KeywordDefault:
			_ = p.next()
			expression, err := p.parseExpression()
			if err != nil {
				return Column{}, err
			}
			column.DefaultValue = &expression
		default:
			return Column{}, fmt.Errorf("parser: unexpected column constraint '%s' at %s", next.Keyword, lexer.DescribePosition(next.Offset))
		}
	}
	return column, nil
}

func (p *Parser) parseExpression() (Expression, error) {
	token := p.next()
	switch token.Kind {
	case lexer.String:
		return Expression{Kind: StringLiteral, Value: token.Value.(string)}, nil
	case lexer.Number:
		switch value := token.Value.(type) {
		case int64:
			return Expression{Kind: IntegerLiteral, Value: value}, nil
		case float64:
			return Expression{Kind: FloatLiteral, Value: value}, nil
		default:
			return Expression{}, fmt.Errorf("parser: invalid number payload at %s", lexer.DescribePosition(token.Offset))
		}
	case lexer.KeywordKind:
		switch token.Keyword {
		case lexer.KeywordTrue:
			return Expression{Kind: BooleanLiteral, Value: true}, nil
		case lexer.KeywordFalse:
			return Expression{Kind: BooleanLiteral, Value: false}, nil
		case lexer.KeywordNull:
			return NullExpression(), nil
		default:
			return Expression{}, fmt.Errorf("parser: keyword '%s' is not an expression at %s", token.Keyword, lexer.DescribePosition(token.Offset))
		}
	default:
		return Expression{}, fmt.Errorf("parser: expected constant expression, got %s at %s", lexer.DescribeToken(token), lexer.DescribePosition(token.Offset))
	}
}
