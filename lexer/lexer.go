package lexer

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Lexer struct {
	input  string
	offset int
}

func New(input string) *Lexer {
	return &Lexer{input: input}
}

func Lex(input string) ([]Token, error) {
	return New(input).TokenizeAll()
}

func (l *Lexer) Next() (Token, error) {
	l.skipWhitespace()
	if l.atEnd() {
		return Token{Kind: EndOfInput, Offset: l.offset}, nil
	}

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

func (l *Lexer) advance() byte {
	value := l.input[l.offset]
	l.offset++
	return value
}

func (l *Lexer) skipWhitespace() {
	remaining := l.input[l.offset:]
	trimmed := strings.TrimLeft(remaining, " \t\r\n\f\v")
	l.offset += len(remaining) - len(trimmed)
}

func (l *Lexer) scanIdentifier() Token {
	begin := l.offset
	for !l.atEnd() && isIdentifierPart(l.peek()) {
		l.offset++
	}
	text := l.input[begin:l.offset]
	if keyword, ok := KeywordFromIdentifier(text); ok {
		return Token{Kind: KeywordKind, Keyword: keyword, Offset: begin}
	}
	return Token{Kind: Identifier, Value: strings.ToLower(text), Offset: begin}
}

func (l *Lexer) scanNumber() (Token, error) {
	begin := l.offset
	for !l.atEnd() && isDigit(l.peek()) {
		l.offset++
	}

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

func (l *Lexer) scanString() (Token, error) {
	begin := l.offset
	l.advance()

	relativeEnd := strings.IndexByte(l.input[l.offset:], '\'')
	if relativeEnd < 0 {
		l.offset = len(l.input)
		return Token{}, fmt.Errorf("lexer: unterminated string literal at %s", DescribePosition(begin))
	}

	valueStart := l.offset
	l.offset += relativeEnd + 1
	return Token{Kind: String, Value: l.input[valueStart : l.offset-1], Offset: begin}, nil
}

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
