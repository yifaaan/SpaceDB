#pragma once

#include <cstddef>
#include <string>
#include <variant>
#include <vector>

#include "../Types/Value.h"

namespace spacedb
{
    struct CreateTableResult
    {
        std::string tableName;

        friend bool operator==(const CreateTableResult&, const CreateTableResult&) = default;
    };

    struct InsertResult
    {
        size_t count = 0;

        friend bool operator==(const InsertResult&, const InsertResult&) = default;
    };

    struct RowsResult
    {
        std::vector<std::string> columns;
        std::vector<Row> rows;

        friend bool operator==(const RowsResult&, const RowsResult&) = default;
    };

    using ResultSet = std::variant<CreateTableResult, InsertResult, RowsResult>;
} // namespace spacedb