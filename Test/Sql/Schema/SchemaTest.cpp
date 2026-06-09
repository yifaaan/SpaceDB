#include "Sql/Schema/Schema.h"

#include <cstdint>

#include <catch2/catch_test_macros.hpp>

namespace spacedb
{
    TEST_CASE("schema represents normalized columns")
    {
        const schema::Table table{
            .name = "users",
            .columns =
                {
                    {
                        .name = "id",
                        .dataType = DataType::INTEGER,
                        .nullable = false,
                        .defaultValue = std::nullopt,
                    },
                    {
                        .name = "age",
                        .dataType = DataType::INTEGER,
                        .nullable = true,
                        .defaultValue = Value{std::int64_t{18}},
                    },
                    {
                        .name = "note",
                        .dataType = DataType::STRING,
                        .nullable = true,
                        .defaultValue = Value::Null(),
                    },
                },
        };

        REQUIRE(table.columns.size() == 3);

        CHECK_FALSE(table.columns[0].nullable);
        CHECK_FALSE(table.columns[0].defaultValue.has_value());

        const auto* age = table.columns[1].defaultValue->GetIf<std::int64_t>();
        REQUIRE(age != nullptr);
        CHECK(*age == 18);

        REQUIRE(table.columns[2].defaultValue.has_value());
        CHECK(table.columns[2].defaultValue->IsNull());
    }
} // namespace spacedb