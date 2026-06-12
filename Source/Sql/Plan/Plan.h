#pragma once

#include <variant>

#include "../Parser/Ast.h"
#include "../Schema/Schema.h"

namespace spacedb
{
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

    using PlanNode = std::variant<CreateTableNode, InsertNode>;

    struct Plan
    {
        PlanNode node;

        friend bool operator==(const Plan&, const Plan&) = default;
    };
} // namespace spacedb