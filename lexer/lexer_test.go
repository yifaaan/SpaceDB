package lexer

import (
	"math"
	"slices"
	"strings"
	"testing"
)

func TestLexerTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
		kinds []Kind
	}{
		{"select", "select * from users;", []Kind{KeywordKind, Asterisk, KeywordKind, Identifier, Semicolon, EndOfInput}},
		{"symbols", "(),;*+-/", []Kind{OpenParen, CloseParen, Comma, Semicolon, Asterisk, Plus, Minus, Slash, EndOfInput}},
		{"whitespace", " \t\r\nSELECT", []Kind{KeywordKind, EndOfInput}},
		{"number then identifier", "12abc", []Kind{Number, Identifier, EndOfInput}},
		{"cross join", "SELECT * FROM t1 CROSS JOIN t2;", []Kind{KeywordKind, Asterisk, KeywordKind, Identifier, KeywordKind, KeywordKind, Identifier, Semicolon, EndOfInput}},
		{"outer join", "t1 LEFT JOIN t2 ON a = b", []Kind{Identifier, KeywordKind, KeywordKind, Identifier, KeywordKind, Identifier, Equal, Identifier, EndOfInput}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := Lex(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			kinds := make([]Kind, len(tokens))
			for i, token := range tokens {
				kinds[i] = token.Kind
			}
			if !slices.Equal(kinds, tt.kinds) {
				t.Fatalf("token kinds = %v, want %v", kinds, tt.kinds)
			}
		})
	}
}

func TestLexerPayloadsAndOffsets(t *testing.T) {
	tokens, err := Lex("CREATE\n  users 42 3.14 'MiXeD_123'")
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].Keyword != KeywordCreate || tokens[1].Value != "users" || tokens[1].Offset != 9 {
		t.Fatalf("unexpected keyword/identifier: %#v", tokens[:2])
	}
	if value, ok := tokens[2].Value.(int64); !ok || value != 42 {
		t.Fatalf("integer payload = %#v", tokens[2].Value)
	}
	if value, ok := tokens[3].Value.(float64); !ok || math.Abs(value-3.14) > 1e-9 {
		t.Fatalf("float payload = %#v", tokens[3].Value)
	}
	if tokens[4].Value != "MiXeD_123" {
		t.Fatalf("string payload = %#v", tokens[4].Value)
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"@", "unexpected character"},
		{"10.", "digits required after decimal point"},
		{"1.2.3", "unexpected character"},
		{"99999999999999999999999999", "integer literal out of range"},
		{"'unfinished", "unterminated string literal"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := Lex(tt.input)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
