#include "Sql/Executor/ResultSet.h"

#include <cstdint>
#include <string>

#include <catch2/catch_test_macros.hpp>

namespace spacedb
{
    TEST_CASE("result set represents command results")
    {
        const ResultSet create = CreateTableResult{.tableName = "users"};

        const ResultSet insert = InsertResult{.count = 2};

        CHECK(std::get<CreateTableResult>(create).tableName == "users");

        CHECK(std::get<InsertResult>(insert).count == 2);
    }

    TEST_CASE("result set represents query rows")
    {
        const ResultSet result = RowsResult{
            .columns = {"id", "name"},
            .rows =
                {
                    Row{
                        Value{std::int64_t{1}},
                        Value{std::string{"alice"}},
                    },
                },
        };

        const auto& rows = std::get<RowsResult>(result);

        CHECK(rows.columns == std::vector<std::string>{"id", "name"});

        REQUIRE(rows.rows.size() == 1);
        REQUIRE(rows.rows[0].size() == 2);

        const auto* id = rows.rows[0][0].GetIf<std::int64_t>();
        const auto* name = rows.rows[0][1].GetIf<std::string>();

        REQUIRE(id != nullptr);
        REQUIRE(name != nullptr);

        CHECK(*id == 1);
        CHECK(*name == "alice");
    }
} // namespace spacedb