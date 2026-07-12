package storage

import "errors"

var ErrNotImplemented = errors.New("storage: operation not implemented")

// MVCC 底层多版本并发控制存储
type MVCC struct {
}

func NewMVCC() *MVCC {
	return &MVCC{}
}

func (m *MVCC) Begin() (*MVCCTransaction, error) {
	return &MVCCTransaction{}, nil
}

type MVCCTransaction struct{}
