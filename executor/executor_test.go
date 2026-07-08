package executor

import (
	"errors"
	"testing"

	"spacedb/planner"
)

func TestBuildExecutor(t *testing.T) {
	tests := []struct {
		name string
		node planner.Node
		want any
	}{
		{
			name: "create table",
			node: planner.CreateTableNode{},
			want: CreateTableExecutor{},
		},
		{
			name: "insert",
			node: planner.InsertNode{},
			want: InsertExecutor{},
		},
		{
			name: "scan",
			node: planner.ScanNode{},
			want: ScanExecutor{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Build(tt.node)
			if err != nil {
				t.Fatal(err)
			}

			switch tt.want.(type) {
			case CreateTableExecutor:
				if _, ok := got.(CreateTableExecutor); !ok {
					t.Fatalf("executor = %T", got)
				}
			case InsertExecutor:
				if _, ok := got.(InsertExecutor); !ok {
					t.Fatalf("executor = %T", got)
				}
			case ScanExecutor:
				if _, ok := got.(ScanExecutor); !ok {
					t.Fatalf("executor = %T", got)
				}
			}
		})
	}
}

func TestExecuteReturnsNotImplemented(t *testing.T) {
	executor := ScanExecutor{TableName: "users"}

	_, err := executor.Execute(nil)
	if !errors.Is(err, ErrNotImplemented) {
		t.Fatalf("error = %v, want ErrNotImplemented", err)
	}
}
