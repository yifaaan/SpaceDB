package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// version 是 MVCC 使用的事务版本号
type version uint64

// MVCC 内部 key 使用独立命名空间，避免和当前 SQL 表、行数据的 key 冲突。
const mvccMetadataPrefix = "\x00spacedb\x00"

type mvccKeyKind byte

// mvcc key 的种类
const (
	mvccKeyNextVersion mvccKeyKind = iota
	mvccKeyTxnActive
	mvccKeyTxnWrite
	mvccKeyVersion
)

type mvccKey struct {
	kind    mvccKeyKind
	version version
	rawKey  []byte
}

// encode 将完整的 MVCC key 编码成保持字典序的字节
func (k mvccKey) encode() []byte {
	encoded := mvccKeyKindPrefix(k.kind)

	switch k.kind {
	case mvccKeyNextVersion:
		return encoded
	case mvccKeyTxnActive:
		return binary.BigEndian.AppendUint64(encoded, uint64(k.version))
	case mvccKeyTxnWrite:
		encoded = binary.BigEndian.AppendUint64(encoded, uint64(k.version))
		return appendTerminatedRawKey(encoded, k.rawKey)
	case mvccKeyVersion:
		encoded = appendTerminatedRawKey(encoded, k.rawKey)
		return binary.BigEndian.AppendUint64(encoded, uint64(k.version))
	default:
		panic(fmt.Sprintf("storage: invalid MVCC key kind %d", k.kind))
	}
}

// decodeMvccKey 解码一个完整的 MVCC key。
//
// encoded 来自磁盘，可能损坏，因此所有长度、tag 和转义都必须检查。
func decodeMvccKey(encoded []byte) (mvccKey, error) {
	prefixLength := len(mvccMetadataPrefix)

	if len(encoded) < prefixLength+1 {
		return mvccKey{}, fmt.Errorf("storage: truncated MVCC key: got %d bytes", len(encoded))
	}

	if !bytes.Equal(encoded[:prefixLength], []byte(mvccMetadataPrefix)) {
		return mvccKey{}, fmt.Errorf("storage: invalid MVCC key namespace: %x", encoded)
	}

	kind := mvccKeyKind(encoded[prefixLength])
	payload := encoded[prefixLength+1:]

	switch kind {
	case mvccKeyNextVersion:
		if len(payload) != 0 {
			return mvccKey{}, fmt.Errorf("storage: next-version key has %d trailing bytes", len(payload))
		}

		return nextVersionKey(), nil

	case mvccKeyTxnActive:
		v, err := decodeVersion(payload)
		if err != nil {
			return mvccKey{}, fmt.Errorf("storage: decoding active-transaction key: %w", err)
		}

		return activeTransactionKey(v), nil

	case mvccKeyTxnWrite:
		if len(payload) < 8 {
			return mvccKey{}, fmt.Errorf("storage: truncated transaction-write version")
		}

		v := version(binary.BigEndian.Uint64(payload[:8]))

		rawKey, remaining, err := decodeRawKey(payload[8:])
		if err != nil {
			return mvccKey{}, fmt.Errorf("storage: decoding transaction-write key: %w", err)
		}
		if len(remaining) != 0 {
			return mvccKey{}, fmt.Errorf("storage: transaction-write key has %d trailing bytes", len(remaining))
		}

		return mvccKey{
			kind:    mvccKeyTxnWrite,
			version: v,
			rawKey:  rawKey,
		}, nil

	case mvccKeyVersion:
		rawKey, remaining, err := decodeRawKey(payload)
		if err != nil {
			return mvccKey{}, fmt.Errorf("storage: decoding versioned key: %w", err)
		}

		v, err := decodeVersion(remaining)
		if err != nil {
			return mvccKey{}, fmt.Errorf("storage: decoding versioned-key version: %w", err)
		}

		return mvccKey{
			kind:    mvccKeyVersion,
			version: v,
			rawKey:  rawKey,
		}, nil

	default:
		return mvccKey{}, fmt.Errorf("storage: unknown MVCC key kind %d", kind)
	}
}

// nextVersionKey 同过该 key 获取 DB 的下一个事务序列号
func nextVersionKey() mvccKey {
	return mvccKey{
		kind: mvccKeyNextVersion,
	}
}

// activeTransactionKey 将事务 v 设置为当前活跃事务时需要的 key
func activeTransactionKey(v version) mvccKey {
	return mvccKey{
		kind:    mvccKeyTxnActive,
		version: v,
	}
}

// transactionWriteKey 设置事务 v 的写集合时需要的 key
func transactionWriteKey(v version, rawKey []byte) mvccKey {
	return mvccKey{
		kind:    mvccKeyTxnWrite,
		version: v,
		rawKey:  bytes.Clone(rawKey),
	}
}

// versionedKey 表示用户数据的不同版本的 key，写入用户数据时构造
func versionedKey(rawKey []byte, v version) mvccKey {
	return mvccKey{
		kind:    mvccKeyVersion,
		version: v,
		rawKey:  bytes.Clone(rawKey),
	}
}

// activeTransactionPrefix 返回所有活跃事务 key 的扫描前缀
func activeTransactionPrefix() []byte {
	return mvccKeyKindPrefix(mvccKeyTxnActive)
}

// transactionWritePrefix 返回事务 v 的所有写集合记录共享的前缀
//
// key 格式：
//
//	MVCC namespace | TxnWrite kind | transaction version | raw key
//
// Commit 和 Rollback 都需要扫描该前缀。
func transactionWritePrefix(v version) []byte {
	prefix := mvccKeyKindPrefix(mvccKeyTxnWrite)
	return binary.BigEndian.AppendUint64(prefix, uint64(v))
}

// versionedKeyPrefix 返回原始 key 前缀对应的版本记录扫描前缀。
//
// 这里只转义 rawPrefix，不添加 [0, 0] 终止符，因为它表示前缀，
// 而不是一个完整的 raw key。
func versionedKeyPrefix(rawPrefix []byte) []byte {
	prefix := mvccKeyKindPrefix(mvccKeyVersion)
	return appendEscapedRawKey(prefix, rawPrefix)
}

// mvccKeyKindPrefix 根据 kind 获取对应的前缀
func mvccKeyKindPrefix(kind mvccKeyKind) []byte {
	prefix := make([]byte, 0, len(mvccMetadataPrefix)+1)
	prefix = append(prefix, mvccMetadataPrefix...)
	prefix = append(prefix, byte(kind))
	return prefix
}

// appendEscapedRawKey 转义原始 key 中的 0 字节。
//
//	00 -> 00 ff
//
// 完整 key 最后使用 00 00 作为终止符。
func appendEscapedRawKey(dst, rawKey []byte) []byte {
	for _, b := range rawKey {
		if b == 0 {
			dst = append(dst, 0, 0xff)
			continue
		}

		dst = append(dst, b)
	}

	return dst
}

// appendTerminatedRawKey 编码完整的原始 key
func appendTerminatedRawKey(dst, rawKey []byte) []byte {
	dst = appendEscapedRawKey(dst, rawKey)
	return append(dst, 0, 0)
}

// decodeRawKey 解码一个以 00 00 结束的原始 key。
//
// remaining 是终止符之后尚未消费的内容，例如 Version key 的版本号。
func decodeRawKey(encoded []byte) (rawKey []byte, remaining []byte, err error) {
	rawKey = make([]byte, 0, len(encoded))

	for i := 0; i < len(encoded); {
		if encoded[i] != 0 {
			rawKey = append(rawKey, encoded[i])
			i++
			continue
		}

		if i+1 >= len(encoded) {
			return nil, nil, fmt.Errorf("storage: truncated raw-key escape")
		}

		switch encoded[i+1] {
		case 0:
			return rawKey, encoded[i+2:], nil

		case 0xff:
			rawKey = append(rawKey, 0)
			i += 2

		default:
			return nil, nil, fmt.Errorf("storage: invalid raw-key escape 00 %02x", encoded[i+1])
		}
	}

	return nil, nil, fmt.Errorf("storage: raw key has no terminator")
}

// encodeVersion 编码事务版本号
func encodeVersion(v version) []byte {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, uint64(v))
	return encoded
}

func decodeVersion(encoded []byte) (version, error) {
	if len(encoded) != 8 {
		return 0, fmt.Errorf("storage: invalid version encoding: got %d bytes, want 8", len(encoded))
	}
	return version(binary.BigEndian.Uint64(encoded)), nil
}
