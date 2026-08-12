package types

import (
	"fmt"
	"strconv"
	"strings"
)

// ValueKind 表示数据库运行时值的具体类型
type ValueKind uint8

const (
	ValueNull ValueKind = iota
	ValueBoolean
	ValueInteger
	ValueFloat
	ValueString
)

// Value 是执行计划和执行器使用的值
type Value struct {
	Kind    ValueKind
	Boolean bool
	Integer int64
	Float   float64
	Str     string
}

// DataType 返回 Value 对应的 SQL 数据类型
//
// 第二个返回值表示这个 Value 是否具有具体类型
func (v Value) DataType() (DataType, bool) {
	switch v.Kind {
	case ValueBoolean:
		return Boolean, true
	case ValueInteger:
		return Integer, true
	case ValueFloat:
		return Float, true
	case ValueString:
		return String, true
	case ValueNull:
		return 0, false
	default:
		// 未知 ValueKind 视为无效值。
		return 0, false
	}
}

func (v Value) Compare(other Value) (int, error) {
	if v.Kind == ValueNull && other.Kind == ValueNull {
		return 0, nil
	}

	// NULL 小于任何 非 NULL
	if v.Kind == ValueNull {
		return -1, nil
	}
	if other.Kind == ValueNull {
		return 1, nil
	}

	switch {
	case v.Kind == ValueBoolean && other.Kind == ValueBoolean:
		if v.Boolean == other.Boolean {
			return 0, nil
		}
		if !v.Boolean {
			return -1, nil
		}
		return 1, nil
	case v.Kind == ValueInteger && other.Kind == ValueInteger:
		if v.Integer < other.Integer {
			return -1, nil
		}
		if v.Integer > other.Integer {
			return 1, nil
		}
		return 0, nil

	case v.Kind == ValueFloat && other.Kind == ValueFloat:
		if v.Float < other.Float {
			return -1, nil
		}
		if v.Float > other.Float {
			return 1, nil
		}
		return 0, nil

	case v.Kind == ValueInteger && other.Kind == ValueFloat:
		left := float64(v.Integer)
		if left < other.Float {
			return -1, nil
		}
		if left > other.Float {
			return 1, nil
		}
		return 0, nil

	case v.Kind == ValueFloat && other.Kind == ValueInteger:
		right := float64(other.Integer)
		if v.Float < right {
			return -1, nil
		}
		if v.Float > right {
			return 1, nil
		}
		return 0, nil

	case v.Kind == ValueString && other.Kind == ValueString:
		return strings.Compare(v.Str, other.Str), nil

	default:
		return 0, fmt.Errorf("types: cannot compare value kinds %d and %d", v.Kind, other.Kind)
	}
}

func (v Value) String() string {
	switch v.Kind {
	case ValueNull:
		return "NULL"
	case ValueBoolean:
		return strconv.FormatBool(v.Boolean)
	case ValueInteger:
		return strconv.FormatInt(v.Integer, 10)
	case ValueFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case ValueString:
		return v.Str
	default:
		return fmt.Sprintf("<invalid value kind %d>", v.Kind)
	}
}
