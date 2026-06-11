#pragma once

#include <variant>

#include "../Schema/Schema.h"

namespace spacedb
{
    struct CreateTableNode
    {
        schema::Table table;

        friend bool operator==(const CreateTableNode&, const CreateTableNode&) = default;
    };

    using PlanNode = std::variant<CreateTableNode>;

    struct Plan
    {
        PlanNode node;

        friend bool operator==(const Plan&, const Plan&) = default;
    };
} // namespace spacedb