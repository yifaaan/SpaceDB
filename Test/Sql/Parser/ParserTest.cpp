#include "Sql/Parser/Parser.h"

#include <catch2/catch_test_macros.hpp>

namespace spacedb
{
    TEST_CASE("parser parses select statement")
    {
        Parser parser("  SeLeCt * FROM users; ");

        auto result = parser.Parse();

        REQUIRE(result.ok());
        REQUIRE(std::holds_alternative<SelectStatement>(*result));

        const auto& statement = std::get<SelectStatement>(*result);

        CHECK(statement.tableName == "users");
    }

    TEST_CASE("parser requires semicolon")
    {
        Parser parser("SELECT * FROM users");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("parser rejects trailing tokens")
    {
        Parser parser("SELECT * FROM users; SELECT * FROM other;");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("parser rejects non select statement")
    {
        Parser parser("INSERT INTO users VALUES (1);");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }
} // namespace spacedb