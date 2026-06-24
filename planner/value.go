package planner

import (
	"fmt"
	"spacedb/parser"
	"spacedb/types"
)

// valueFromExpression 将 Parser AST 中的常量表达式转化为 types 包中运行时值 Value
//
// Parser 只描述 SQL 语法，types.Value 只描述运行时数据，
// Planner 负责连接这两个层次。
func valueFromExpression(expression parser.Expression) (types.Value, error) {
	switch expression.Kind {
	case parser.Null:
		return types.Value{Kind: types.ValueNull}, nil

	case parser.BooleanLiteral:
		v, ok := expression.Value.(bool)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"planner: boolean expression contains %T", expression.Value,
			)
		}
		return types.Value{Kind: types.ValueBoolean, Boolean: v}, nil

	case parser.IntegerLiteral:
		v, ok := expression.Value.(int64)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"planner: integer expression contains %T", expression.Value,
			)
		}
		return types.Value{Kind: types.ValueInteger, Integer: v}, nil

	case parser.FloatLiteral:
		v, ok := expression.Value.(float64)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"planner: float expression contains %T", expression.Value,
			)
		}
		return types.Value{Kind: types.ValueFloat, Float: v}, nil

	case parser.StringLiteral:
		v, ok := expression.Value.(string)
		if !ok {
			return types.Value{}, fmt.Errorf(
				"planner: string expression contains %T", expression.Value,
			)
		}
		return types.Value{Kind: types.ValueString, String: v}, nil

	default:
		return types.Value{}, fmt.Errorf(
			"planner: unsupported expression kind %d", expression.Kind,
		)
	}
}
