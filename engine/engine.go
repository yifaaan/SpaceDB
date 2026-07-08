package engine

import (
	"errors"
	"fmt"
	"spacedb/executor"
	"spacedb/parser"
	"spacedb/planner"
)

// Engine 是 SQL 引擎依赖的接口
//
// 具体实现可以是内存引擎、KV 引擎或分布式引擎,
// Engine 不关心 SQL 解析和执行细节，只负责开启事务
type Engine interface {
	Begin() (executor.Transaction, error)
}

// Session 表示一个客户端会话
//
// Session 持有 Engine，但不直接持有事务,
// 每次 Execute 都创建一个新的事务
type Session struct {
	engine Engine
}

func NewSession(engine Engine) *Session {
	return &Session{engine: engine}
}

// Execute 执行一条 SQL 语句
func (s *Session) Execute(sql string) (executor.ResultSet, error) {
	// 解析 AST
	stmt, err := parser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("engine: parsing SQL: %w", err)
	}

	// 构建执行计划节点
	plan, err := planner.Build(stmt)
	if err != nil {
		return nil, fmt.Errorf("engine: building plan: %w", err)
	}

	// 每个节点都有一个执行器对应
	exec, err := executor.Build(plan.Node)
	if err != nil {
		return nil, fmt.Errorf("engine: building executor: %w", err)
	}

	// 从 存储引擎获取 txn
	txn, err := s.engine.Begin()
	if err != nil {
		return nil, fmt.Errorf("engine: beginning transaction: %w", err)
	}

	// 将 txn 交给执行器执行
	result, err := exec.Execute(txn)
	if err != nil {
		if rollbackErr := txn.Rollback(); rollbackErr != nil {
			return nil, errors.Join(
				fmt.Errorf("engine: executing statement: %w", err),
				fmt.Errorf("engine: rolling back transaction: %w", rollbackErr),
			)
		}
		return nil, fmt.Errorf("engine: executing statement: %w", err)
	}

	if err := txn.Commit(); err != nil {
		return nil, fmt.Errorf("engine: committing transaction: %w", err)
	}

	return result, nil
}
