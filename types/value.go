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
