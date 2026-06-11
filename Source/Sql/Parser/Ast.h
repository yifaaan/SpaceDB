#pragma once

#include <cstdint>
#include <optional>
#include <string>
#include <variant>
#include <vector>

#include "../Types/DataType.h"

namespace spacedb
{

    using Expression = std::variant<std::monostate, bool, std::int64_t, double, std::string>;

    struct Column
    {
        std::string name;
        DataType dataType;
        std::optional<bool> nullable;
        std::optional<Expression> defaultValue;

        friend bool operator==(const Column&, const Column&) = default;
    };

    struct CreateTableStatement
    {
        std::string name;
        std::vector<Column> columns;

        friend bool operator==(const CreateTableStatement&, const CreateTableStatement&) = default;
    };

    struct InsertStatement
    {
        std::string tableName;
        std::optional<std::vector<std::string>> columns;
        std::vector<std::vector<Expression>> values;

        friend bool operator==(const InsertStatement&, const InsertStatement&) = default;
    };

    struct SelectStatement
    {
        std::string tableName;

        friend bool operator==(const SelectStatement&, const SelectStatement&) = default;
    };

    using Statement = std::variant<CreateTableStatement, InsertStatement, SelectStatement>;
} // namespace spacedb