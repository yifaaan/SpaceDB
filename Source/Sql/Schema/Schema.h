#pragma once

#include <optional>
#include <string>
#include <vector>

#include "../Types/DataType.h"
#include "../Types/Value.h"

namespace spacedb::schema
{
    struct Column
    {
        std::string name;
        DataType dataType;
        bool nullable = true;
        std::optional<Value> defaultValue;

        friend bool operator==(const Column&, const Column&) = default;
    };

    struct Table
    {
        std::string name;
        std::vector<Column> columns;

        friend bool operator==(const Table&, const Table&) = default;
    };
} // namespace spacedb::schema