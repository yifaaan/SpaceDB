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

    TEST_CASE("planner builds insert plan")
    {
        Parser parser("INSERT INTO users (id, name) "
                      "VALUES (1, 'alice'), (2, 'bob');");

        auto statement = parser.Parse();
        REQUIRE(statement.ok());

        auto plan = Planner::Build(std::move(*statement));
        REQUIRE(plan.ok());
        REQUIRE(std::holds_alternative<InsertNode>(plan->node));

        const auto& insert = std::get<InsertNode>(plan->node);

        CHECK(insert.tableName == "users");
        CHECK(insert.columns == std::vector<std::string>{"id", "name"});

        REQUIRE(insert.values.size() == 2);
        CHECK(std::get<std::int64_t>(insert.values[0][0]) == 1);
        CHECK(std::get<std::string>(insert.values[0][1]) == "alice");
        CHECK(std::get<std::int64_t>(insert.values[1][0]) == 2);
        CHECK(std::get<std::string>(insert.values[1][1]) == "bob");
    }

    TEST_CASE("planner normalizes omitted insert columns")
    {
        Parser parser("INSERT INTO users VALUES (1);");

        auto statement = parser.Parse();
        REQUIRE(statement.ok());

        auto plan = Planner::Build(std::move(*statement));
        REQUIRE(plan.ok());

        const auto& insert = std::get<InsertNode>(plan->node);

        CHECK(insert.columns.empty());
    }
} // namespace spacedb