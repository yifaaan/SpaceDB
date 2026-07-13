package storage

// Engine 最底层 KV 存储的接口。
//
// key 和 value 都使用字节切片
type Engine interface {
	Set(key, value []byte) error

	Get(key []byte) ([]byte, error)

	Delete(key []byte) error
}
