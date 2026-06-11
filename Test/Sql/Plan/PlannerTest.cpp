#include "Sql/Plan/Planner.h"
#include "Sql/Parser/Parser.h"


#include <cstdint>
#include <utility>

#include <catch2/catch_test_macros.hpp>

namespace spacedb
{
    TEST_CASE("planner builds normalized create table plan")
    {
        Parser parser("CREATE TABLE users ("
                      "id INT NOT NULL,"
                      "name STRING,"
                      "age INT DEFAULT 18,"
                      "active BOOL DEFAULT TRUE"
                      ");");

        auto statement = parser.Parse();
        REQUIRE(statement.ok());

        auto plan = Planner::Build(std::move(*statement));
        REQUIRE(plan.ok());
        REQUIRE(std::holds_alternative<CreateTableNode>(plan->node));

        const auto& table = std::get<CreateTableNode>(plan->node).table;

        CHECK(table.name == "users");
        REQUIRE(table.columns.size() == 4);

        CHECK_FALSE(table.columns[0].nullable);
        CHECK_FALSE(table.columns[0].defaultValue.has_value());

        CHECK(table.columns[1].nullable);
        REQUIRE(table.columns[1].defaultValue.has_value());
        CHECK(table.columns[1].defaultValue->IsNull());

        const auto* age = table.columns[2].defaultValue->GetIf<std::int64_t>();
        REQUIRE(age != nullptr);
        CHECK(*age == 18);

        const auto* active = table.columns[3].defaultValue->GetIf<bool>();
        REQUIRE(active != nullptr);
        CHECK(*active);
    }
} // namespace spacedb