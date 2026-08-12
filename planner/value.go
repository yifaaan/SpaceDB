package planner

import (
	"fmt"
	"spacedb/parser"
	"spacedb/types"
)

// ValueFromExpression 将 Parser AST 中的常量表达式转化为 types 包中运行时值 Value
//
// Parser 只描述 SQL 语法，types.Value 只描述运行时数据，
// Planner 负责连接这两个层次。
func ValueFromExpression(expression parser.Expression) (types.Value, error) {
	switch expression.Kind {
	case parser.Null:
		return types.Value{Kind: types.ValueNull}, nil

	case parser.BooleanLiteral:
		v := expression.Value.(bool)
		return types.Value{Kind: types.ValueBoolean, Boolean: v}, nil

	case parser.IntegerLiteral:
		v := expression.Value.(int64)
		return types.Value{Kind: types.ValueInteger, Integer: v}, nil

	case parser.FloatLiteral:
		v := expression.Value.(float64)
		return types.Value{Kind: types.ValueFloat, Float: v}, nil

	case parser.StringLiteral:
		v := expression.Value.(string)
		return types.Value{Kind: types.ValueString, String: v}, nil

	default:
		return types.Value{}, fmt.Errorf(
			"planner: unsupported expression kind %d", expression.Kind,
		)
	}
}
