#pragma once

#include <variant>

#include "../Executor/ResultSet.h"
#include "../Parser/Ast.h"
#include "../Schema/Schema.h"

#include <absl/status/statusor.h>

namespace spacedb
{
    class ISqlTransaction;

    struct CreateTableNode
    {
        schema::Table table;

        friend bool operator==(const CreateTableNode&, const CreateTableNode&) = default;
    };

    struct InsertNode
    {
        std::string tableName;
        std::vector<std::string> columns;
        std::vector<std::vector<Expression>> values;

        friend bool operator==(const InsertNode&, const InsertNode&) = default;
    };

    struct ScanNode
    {
        std::string tableName;

        friend bool operator==(const ScanNode&, const ScanNode&) = default;
    };

    using PlanNode = std::variant<CreateTableNode, InsertNode, ScanNode>;

    struct Plan
    {
        PlanNode node;

        // 在事务上执行计划，返回结果集。
        // 内部通过 Executor 工厂分派到具体执行器
        absl::StatusOr<ResultSet> Execute(ISqlTransaction& txn);

        friend bool operator==(const Plan&, const Plan&) = default;
    };
} // namespace spacedb