package planner

import (
	"testing"

	"spacedb/parser"
	"spacedb/types"
)

func TestValueFromExpression(t *testing.T) {
	tests := []struct {
		name       string
		expression parser.Expression
		want       types.Value
	}{
		{"null", parser.NullExpression(), types.Value{Kind: types.ValueNull}},
		{"bool", parser.Expression{Kind: parser.BooleanLiteral, Value: true},
			types.Value{Kind: types.ValueBoolean, Boolean: true}},
		{"integer", parser.Expression{Kind: parser.IntegerLiteral, Value: int64(4)},
			types.Value{Kind: types.ValueInteger, Integer: 4}},
		{"float", parser.Expression{Kind: parser.FloatLiteral, Value: 3.14},
			types.Value{Kind: types.ValueFloat, Float: 3.14}},
		{"string", parser.Expression{Kind: parser.StringLiteral, Value: "alice"},
			types.Value{Kind: types.ValueString, String: "alice"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValueFromExpression(tt.expression)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("value = %#v, want %#v", got, tt.want)
			}
		})
	}
}
