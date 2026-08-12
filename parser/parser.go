package parser

import (
	"errors"
	"fmt"

	"spacedb/lexer"
)

type Parser struct {
	input  string
	tokens []lexer.Token // 词法分析结果
	index  int           // 当前 Token 下标
}

func New(input string) *Parser {
	return &Parser{input: input}
}

func Parse(input string) (Statement, error) {
	return New(input).Parse()
}

// Parse 完成一次完整的解析，分四步：
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

// peek 返回当前 Token，只读不推进
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

// expect 消费下一个 Token 并断言其种类匹配，不匹配则返回带位置的错误。
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
	case lexer.KeywordUpdate:
		return p.parseUpdate()
	case lexer.KeywordDelete:
		return p.parseDelete()
	default:
		return nil, errors.New("parser: unsupported statement")
	}
}

// parseSelect 解析 SELECT 语句
func (p *Parser) parseSelect() (Statement, error) {
	if err := p.expectKeyword(lexer.KeywordSelect); err != nil {
		return nil, err
	}

	selectItems := make([]SelectItem, 0, 4)

	if p.peek().Kind == lexer.Asterisk {
		_ = p.next()
	} else {
		for {
			exp, err := p.parseExpression()
			if err != nil {
				return nil, fmt.Errorf("parser: parsing SELECT expression: %w", err)
			}

			var alias *string

			current := p.peek()
			if current.Kind == lexer.KeywordKind && current.Keyword == lexer.KeywordAs {
				_ = p.next()

				aliasName, err := p.expectIdentifier()
				if err != nil {
					return nil, fmt.Errorf("parser: parsing SELECT alias: %w", err)
				}
				alias = &aliasName
			}

			selectItems = append(selectItems, SelectItem{
				Expression: exp,
				Alias:      alias,
			})

			if p.peek().Kind != lexer.Comma {
				break
			}

			if err := p.expect(lexer.Comma); err != nil {
				return nil, err
			}
		}
	}

	from, err := p.parseFromClause()
	if err != nil {
		return nil, fmt.Errorf("parser: parsing FROM clause: %w", err)
	}

	whereClause, err := p.parseOptionalComparison(
		lexer.KeywordWhere,
	)
	if err != nil {
		return nil, err
	}

	// GROUP BY 必须出现在 FROM/JOIN 之后、ORDER BY 之前
	//
	// SELECT b, min(c)
	// FROM t1
	// GROUP BY b
	// ORDER BY min;
	var groupBy *Expression
	if current := p.peek(); current.Kind == lexer.KeywordKind && current.Keyword == lexer.KeywordGroup {
		_ = p.next()

		// BY
		if err := p.expectKeyword(lexer.KeywordBy); err != nil {
			return nil, fmt.Errorf("parser: parsing GROUP BY clause: %w", err)
		}

		expression, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("parser: parsing GROUP BY expression: %w", err)
		}

		groupBy = &expression
	}

	// HAVING 在聚合完成后过滤结果行。
	having, err := p.parseOptionalComparison(
		lexer.KeywordHaving,
	)
	if err != nil {
		return nil, err
	}

	// ORDER BY
	orderBy := make([]OrderBy, 0, 4)
	current := p.peek()
	if current.Kind == lexer.KeywordKind && current.Keyword == lexer.KeywordOrder {
		_ = p.next()
		// By
		if err := p.expectKeyword(lexer.KeywordBy); err != nil {
			return nil, err
		}

		for {
			columnName, err := p.expectIdentifier()
			if err != nil {
				return nil, err
			}

			direction := OrderAscending

			current := p.peek()
			if current.Kind == lexer.KeywordKind {
				switch current.Keyword {
				case lexer.KeywordAsc:
					_ = p.next()
					direction = OrderAscending
				case lexer.KeywordDesc:
					_ = p.next()
					direction = OrderDescending
				}
			}

			orderBy = append(orderBy, OrderBy{
				Column:    columnName,
				Direction: direction,
			})

			// comma
			if p.peek().Kind != lexer.Comma {
				break
			}
			if err := p.expect(lexer.Comma); err != nil {
				return nil, err
			}
		}
	}

	// LIMIT
	var limit *Expression
	current = p.peek()
	if current.Kind == lexer.KeywordKind && current.Keyword == lexer.KeywordLimit {
		_ = p.next()

		exp, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("parser: parsing LIMIT value: %w", err)
		}

		limit = &exp
	}

	// OFFSET
	var offset *Expression
	current = p.peek()
	if current.Kind == lexer.KeywordKind && current.Keyword == lexer.KeywordOffset {
		_ = p.next()

		exp, err := p.parseExpression()
		if err != nil {
			return nil, fmt.Errorf("parser: parsing OFFSET value: %w", err)
		}

		offset = &exp
	}

	return SelectStatement{
		From:        from,
		SelectItems: selectItems,
		Where:       whereClause,
		GroupBy:     groupBy,
		Having:      having,
		OrderBy:     orderBy,
		Limit:       limit,
		Offset:      offset,
	}, nil
}

// parseCreateTable 解析 CREATE TABLE：
//
//	CREATE TABLE <表名> ( <列> [, <列> ...] )
//
// 列之间用逗号分隔，最后一列之后要求右括号。
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

// parseInsert 解析 INSERT 语句：
//
//	INSERT INTO <表名> [ (列1, 列2 ...) ] VALUES (值...), (值...) ...
//
// 可选的列列表用 nil / 非 nil 区分；
// VALUES 后面允许一组或多组值，每组值是一行、以逗号分隔。
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

	// 可选的显式列列表：insert into t (a, b) values ...
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

	// VALUES 之后是一组或多组括号包裹的值
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

// parseUpdate 解析 UPDATE 语句
//
//	UPDATE <表名>
//	SET <列名> = <常量> [, <列名> = <常量> ...]
//	[WHERE <比较表达式>]
func (p *Parser) parseUpdate() (Statement, error) {
	if err := p.expectKeyword(lexer.KeywordUpdate); err != nil {
		return nil, err
	}

	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}

	if err := p.expectKeyword(lexer.KeywordSet); err != nil {
		return nil, err
	}

	assignments := make(map[string]Expression)

	// SET 后至少要求一组“列名 = 常量”。
	for {
		columnOffset := p.peek().Offset

		columnName, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}

		if err := p.expect(lexer.Equal); err != nil {
			return nil, err
		}

		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		if _, exists := assignments[columnName]; exists {
			return nil, fmt.Errorf("parser: duplicate update column %q at %s", columnName, lexer.DescribePosition(columnOffset))
		}

		assignments[columnName] = value

		if p.peek().Kind != lexer.Comma {
			break
		}
		if err := p.expect(lexer.Comma); err != nil {
			return nil, err
		}
	}

	// WHERE
	filter, err := p.parseOptionalComparison(lexer.KeywordWhere)
	if err != nil {
		return nil, err
	}

	return UpdateStatement{
		TableName:   tableName,
		Assignments: assignments,
		Filter:      filter,
	}, nil
}

// parseDelete 解析 DELETE 语句
//
//	`DELETE FROM <table> [WHERE <comparison expression>]`
func (p *Parser) parseDelete() (Statement, error) {
	if err := p.expectKeyword(lexer.KeywordDelete); err != nil {
		return nil, err
	}

	if err := p.expectKeyword(lexer.KeywordFrom); err != nil {
		return nil, err
	}

	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}

	filter, err := p.parseOptionalComparison(lexer.KeywordWhere)
	if err != nil {
		return nil, err
	}

	return DeleteStatement{
		TableName: tableName,
		Filter:    filter,
	}, nil
}

// parseColumn 解析一列：<列名> <类型> [约束 ...]。
// 约束是 NULL / NOT NULL / DEFAULT <常量表达式> 的任意组合，循环解析直到遇到非关键字
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

	// 约束循环：NULL → 可空；NOT NULL → 不可空；DEFAULT expr → 默认值
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

		case lexer.KeywordPrimary:
			_ = p.next()

			if err := p.expectKeyword(lexer.KeywordKey); err != nil {
				return Column{}, err
			}

			column.PrimaryKey = true
		default:
			return Column{}, fmt.Errorf("parser: unexpected column constraint '%s' at %s", next.Keyword, lexer.DescribePosition(next.Offset))
		}
	}
	return column, nil
}

// parseOptionalComparison 解析一个可选的 WHERE 或 HAVING 子句
//
// 如果当前 Token 不是指定关键字，则返回 nil，不消费任何 Token
func (p *Parser) parseOptionalComparison(keyword lexer.Keyword) (*Expression, error) {
	current := p.peek()
	if current.Kind != lexer.KeywordKind || current.Keyword != keyword {
		return nil, nil
	}

	if err := p.expectKeyword(keyword); err != nil {
		return nil, err
	}

	expression, err := p.parseComparisonExpression()
	if err != nil {
		return nil, fmt.Errorf("parser: parsing %s predicate: %w", keyword, err)
	}

	return &expression, nil
}

// parseComparisonExpression 解析一个二元比较表达式
func (p *Parser) parseComparisonExpression() (Expression, error) {
	left, err := p.parseExpression()
	if err != nil {
		return Expression{}, fmt.Errorf("parsing left comparison operand: %w", err)
	}

	operatorToken := p.next()

	var operationKind OperationKind
	switch operatorToken.Kind {
	case lexer.Equal:
		operationKind = OperationEqual
	case lexer.GreaterThan:
		operationKind = OperationGreaterThan
	case lexer.LessThan:
		operationKind = OperationLessThan
	default:
		return Expression{}, fmt.Errorf("expected comparison operator, got %s at %s", lexer.DescribeToken(operatorToken), lexer.DescribePosition(operatorToken.Offset))
	}

	right, err := p.parseExpression()
	if err != nil {
		return Expression{}, fmt.Errorf("parsing right comparison operand: %w", err)
	}

	return Expression{
		Kind: OperationExpression,
		Value: Operation{
			Kind:  operationKind,
			Left:  left,
			Right: right,
		},
	}, nil
}

// parseExpression 解析常量表达式，目前支持四类字面量：
// 字符串、数字（词法阶段已区分 int64 / float64）、TRUE / FALSE、NULL。
// 其余 Token 一律报错
func (p *Parser) parseExpression() (Expression, error) {
	token := p.next()
	switch token.Kind {
	case lexer.Identifier:
		name, ok := token.Value.(string)
		if !ok {
			return Expression{}, fmt.Errorf("parser: invalid identifier payload at %s", lexer.DescribePosition(token.Offset))
		}

		// 标识符后没有左括号，表示普通列引用
		// SELECT score FROM users
		if p.peek().Kind != lexer.OpenParen {
			return Expression{
				Kind:  ColumnReference,
				Value: name,
			}, nil
		}

		// 标识符跟左括号，表示函数调用
		// SELECT count(score) FROM users
		if err := p.expect(lexer.OpenParen); err != nil {
			return Expression{}, err
		}

		argument, err := p.expectIdentifier()
		if err != nil {
			return Expression{}, fmt.Errorf("parser: parsing argument of function %q: %w", name, err)
		}

		if err := p.expect(lexer.CloseParen); err != nil {
			return Expression{}, fmt.Errorf("parser: closing function %q: %w", name, err)
		}

		return Expression{
			Kind: FunctionExpression,
			Value: FunctionCall{
				Name:     name,
				Argument: argument,
			},
		}, nil

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
		return Expression{}, fmt.Errorf("parser: expected expression, got %s at %s", lexer.DescribeToken(token), lexer.DescribePosition(token.Offset))
	}
}

func (p *Parser) parseFromClause() (FromItem, error) {
	if err := p.expectKeyword(lexer.KeywordFrom); err != nil {
		return nil, err
	}

	tableName, err := p.expectIdentifier()
	if err != nil {
		return nil, err
	}

	var item FromItem = TableFromItem{Name: tableName}

	for {
		joinType, found, err := p.parseJoinType()
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}

		rightName, err := p.expectIdentifier()
		if err != nil {
			return nil, err
		}

		var predicate *Expression
		if joinType != JoinCross {
			if err := p.expectKeyword(lexer.KeywordOn); err != nil {
				return nil, err
			}

			left, err := p.parseExpression()
			if err != nil {
				return nil, fmt.Errorf("parser: parsing left JOIN operand: %w", err)
			}
			if err := p.expect(lexer.Equal); err != nil {
				return nil, err
			}

			right, err := p.parseExpression()
			if err != nil {
				return nil, fmt.Errorf("parser: parsing right JOIN operand: %w", err)
			}

			// Planner 会将 RIGHT JOIN 的左右数据交换,这里也将条件交换，保持对应
			if joinType == JoinRight {
				left, right = right, left
			}

			cond := Expression{
				Kind: OperationExpression,
				Value: Operation{
					Kind:  OperationEqual,
					Left:  left,
					Right: right,
				},
			}
			predicate = &cond
		}

		// 左结合
		item = JoinFromItem{
			Left:      item,
			Right:     TableFromItem{Name: rightName},
			Type:      joinType,
			Predicate: predicate,
		}
	}

	return item, nil
}

// parseJoinType 尝试消费一个 Join 操作符
//
// found == false 表示当前位置不是 Join，调用者应结束 FROM 解析。
// err != nil 表示看到了 LEFT、RIGHT 或 CROSS，但后面缺少 JOIN
func (p *Parser) parseJoinType() (joinType JoinType, found bool, err error) {
	current := p.peek()
	if current.Kind != lexer.KeywordKind {
		return 0, false, nil
	}

	switch current.Keyword {
	case lexer.KeywordCross:
		_ = p.next()
		if err := p.expectKeyword(lexer.KeywordJoin); err != nil {
			return 0, false, err
		}
		return JoinCross, true, nil

	case lexer.KeywordJoin:
		_ = p.next()
		return JoinInner, true, nil

	case lexer.KeywordLeft:
		_ = p.next()
		if err := p.expectKeyword(lexer.KeywordJoin); err != nil {
			return 0, false, err
		}
		return JoinLeft, true, nil

	case lexer.KeywordRight:
		_ = p.next()
		if err := p.expectKeyword(lexer.KeywordJoin); err != nil {
			return 0, false, err
		}
		return JoinRight, true, nil

	default:
		return 0, false, nil
	}

}
