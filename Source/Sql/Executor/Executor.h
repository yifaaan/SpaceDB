#pragma once

#include <memory>
#include <string>
#include <utility>
#include <variant>
#include <vector>

#include "../Engine/Transaction.h"
#include "../Plan/Plan.h"
#include "ResultSet.h"

#include <absl/status/statusor.h>

namespace spacedb
{
    // 执行器：把 Plan 节点变成对事务的具体调用。
    // 每个节点类型一个执行器类
    class Executor
    {
    public:
        virtual ~Executor() = default;

        // 在给定事务上执行，返回结果集（CreateTable/Insert/Scan 三种）
        virtual absl::StatusOr<ResultSet> Execute(ISqlTransaction& txn) = 0;

        // 工厂：按节点类型构造对应的执行器
        static std::unique_ptr<Executor> Build(PlanNode node);
    };

    // 创建表执行器：把表定义交给事务持久化，返回表名
    class CreateTableExecutor : public Executor
    {
    public:
        explicit CreateTableExecutor(schema::Table table) : table_(std::move(table))
        {
        }

        absl::StatusOr<ResultSet> Execute(ISqlTransaction& txn) override;

    private:
        schema::Table table_; // 来自 CreateTableNode 的表定义
    };

    // 插入行执行器：常量表达式求值 → 列对齐 → 逐行写入
    class InsertExecutor : public Executor
    {
    public:
        InsertExecutor(std::string tableName, std::vector<std::string> columns, std::vector<std::vector<Expression>> values)
            : tableName_(std::move(tableName)), columns_(std::move(columns)), values_(std::move(values))
        {
        }

        absl::StatusOr<ResultSet> Execute(ISqlTransaction& txn) override;

    private:
        std::string tableName_;
        std::vector<std::string> columns_;            // 空 = 未指定列清单
        std::vector<std::vector<Expression>> values_; // 每行一组常量表达式
    };

    // 扫描表执行器：读全部行，配上 schema 列名
    class ScanExecutor : public Executor
    {
    public:
        explicit ScanExecutor(std::string tableName) : tableName_(std::move(tableName))
        {
        }

        absl::StatusOr<ResultSet> Execute(ISqlTransaction& txn) override;

    private:
        std::string tableName_;
    };
} // namespace spacedb