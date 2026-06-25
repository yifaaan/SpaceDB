package types

// DataType 数据库支持的列类型
type DataType uint8

const (
	Boolean DataType = iota
	Integer
	Float
	String
)
