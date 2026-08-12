package engine

import (
	"bytes"
	"spacedb/types"
	"testing"
)

func TestTableAndRowNamespacesAreSeparated(t *testing.T) {
	table, err := tableKey("users")
	if err != nil {
		t.Fatal(err)
	}

	row, err := rowKey("users", types.Value{
		Kind:    types.ValueInteger,
		Integer: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if table[0] != keyNamespaceTable {
		t.Fatalf("table namespace = %x", table[0])
	}
	if row[0] != keyNamespaceRow {
		t.Fatalf("row namespace = %x", row[0])
	}

	// 元数据和行使用不同命名空间，不会相互覆盖
	if bytes.HasPrefix(row, table) {
		t.Fatalf("row key %x unexpectedly has table prefix %x", row, table)
	}
}

func TestRowPrefixSeparatesSimilarTableNames(t *testing.T) {
	userPrefix, err := rowPrefixKey("user")
	if err != nil {
		t.Fatal(err)
	}

	usersRow, err := rowKey("users", types.Value{
		Kind:    types.ValueInteger,
		Integer: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	if bytes.HasPrefix(usersRow, userPrefix) {
		t.Fatalf("users row unexpectedly matched user prefix")
	}
}

func TestIntegerRowKeysFollowNumericOrder(t *testing.T) {
	two, err := rowKey("users", types.Value{
		Kind:    types.ValueInteger,
		Integer: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	ten, err := rowKey("users", types.Value{
		Kind:    types.ValueInteger,
		Integer: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 使用大端序和符号位翻转后，2 会正确排在 10 前面。
	if bytes.Compare(two, ten) >= 0 {
		t.Fatalf("key for 2 must sort before key for 10")
	}
}
