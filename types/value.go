package types

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
	String  string
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
