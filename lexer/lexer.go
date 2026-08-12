package lexer

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Lexer struct {
	input  string
	offset int // 当前扫描位置（字节偏移）
}

func New(input string) *Lexer {
	return &Lexer{input: input}
}

// Lex 一次性切出输入的全部 Token（含 EndOfInput），出错立即返回
func Lex(input string) ([]Token, error) {
	return New(input).TokenizeAll()
}

// Next 扫描并返回下一个 Token。
func (l *Lexer) Next() (Token, error) {
	// 跳过空白
	l.skipWhitespace()
	if l.atEnd() {
		return Token{Kind: EndOfInput, Offset: l.offset}, nil
	}

	// 根据当前字符决定：
	// 单引号 → 字符串
	// 数字 → 数字
	// 字母/下划线 → 标识符或关键字
	// 其余 → 符号。
	switch {
	case l.peek() == '\'':
		return l.scanString()
	case isDigit(l.peek()):
		return l.scanNumber()
	case isIdentifierStart(l.peek()):
		return l.scanIdentifier(), nil
	default:
		return l.scanSymbol()
	}
}

// TokenizeAll 循环调用 Next 直到输入结束，返回完整的 Token 列表
func (l *Lexer) TokenizeAll() ([]Token, error) {
	tokens := make([]Token, 0, 8)
	for {
		token, err := l.Next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		if token.Kind == EndOfInput {
			return tokens, nil
		}
	}
}

func (l *Lexer) atEnd() bool {
	return l.offset >= len(l.input)
}

func (l *Lexer) peek() byte {
	if l.atEnd() {
		return 0
	}
	return l.input[l.offset]
}

// advance 消费并返回当前字符，把偏移向前推进一位
func (l *Lexer) advance() byte {
	value := l.input[l.offset]
	l.offset++
	return value
}

// skipWhitespace 跳过空白字符
func (l *Lexer) skipWhitespace() {
	remaining := l.input[l.offset:]
	trimmed := strings.TrimLeft(remaining, " \t\r\n\f\v")
	l.offset += len(remaining) - len(trimmed)
}

// scanIdentifier 扫描标识符或关键字
func (l *Lexer) scanIdentifier() Token {
	begin := l.offset
	// 首字符为字母或下划线，后续允许字母、数字、下划线；
	for !l.atEnd() && isIdentifierPart(l.peek()) {
		l.offset++
	}
	text := l.input[begin:l.offset]
	// 匹配关键字表
	if keyword, ok := KeywordFromIdentifier(text); ok {
		return Token{Kind: KeywordKind, Keyword: keyword, Offset: begin}
	}
	// 小写化的标识符
	return Token{Kind: Identifier, Value: strings.ToLower(text), Offset: begin}
}

// scanNumber 扫描数字字面量
func (l *Lexer) scanNumber() (Token, error) {
	begin := l.offset
	for !l.atEnd() && isDigit(l.peek()) {
		l.offset++
	}

	// 小数点后必须还有数字，否则是非法浮点（如 "1."）
	isFloat := !l.atEnd() && l.peek() == '.'
	if isFloat {
		l.offset++
		if l.atEnd() || !isDigit(l.peek()) {
			return Token{}, fmt.Errorf("lexer: digits required after decimal point at %s", DescribePosition(begin))
		}
		for !l.atEnd() && isDigit(l.peek()) {
			l.offset++
		}
	}

	text := l.input[begin:l.offset]
	if isFloat {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return Token{}, fmt.Errorf("lexer: invalid number literal %q at %s", text, DescribePosition(begin))
		}
		return Token{Kind: Number, Value: value, Offset: begin}, nil
	}

	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return Token{}, fmt.Errorf("lexer: integer literal out of range at %s", DescribePosition(begin))
		}
		return Token{}, fmt.Errorf("lexer: invalid integer literal %q at %s", text, DescribePosition(begin))
	}
	return Token{Kind: Number, Value: value, Offset: begin}, nil
}

// scanString 扫描单引号字符串字面量。不支持转义：遇到下一个单引号即结束，
func (l *Lexer) scanString() (Token, error) {
	begin := l.offset
	l.advance() // 跳过开头的单引号

	relativeEnd := strings.IndexByte(l.input[l.offset:], '\'')
	if relativeEnd < 0 {
		l.offset = len(l.input)
		return Token{}, fmt.Errorf("lexer: unterminated string literal at %s", DescribePosition(begin))
	}

	valueStart := l.offset
	l.offset += relativeEnd + 1 // 越过结尾单引号，指向字符串内容之后
	return Token{Kind: String, Value: l.input[valueStart : l.offset-1], Offset: begin}, nil
}

// scanSymbol 扫描单字符符号（括号、逗号、分号、四则运算符）
func (l *Lexer) scanSymbol() (Token, error) {
	begin := l.offset
	var kind Kind
	switch l.peek() {
	case '(':
		kind = OpenParen
	case ')':
		kind = CloseParen
	case ',':
		kind = Comma
	case ';':
		kind = Semicolon
	case '*':
		kind = Asterisk
	case '+':
		kind = Plus
	case '-':
		kind = Minus
	case '/':
		kind = Slash
	case '=':
		kind = Equal
	case '>':
		kind = GreaterThan
	case '<':
		kind = LessThan
	default:
		return Token{}, fmt.Errorf("lexer: unexpected character %q at %s", l.peek(), DescribePosition(begin))
	}
	l.offset++
	return Token{Kind: kind, Offset: begin}, nil
}

func isDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func isIdentifierStart(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value == '_'
}

func isIdentifierPart(value byte) bool {
	return isIdentifierStart(value) || isDigit(value)
}
