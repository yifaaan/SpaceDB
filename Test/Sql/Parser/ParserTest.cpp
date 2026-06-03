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

    TEST_CASE("parser rejects empty input")
    {
        Parser parser("");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("parser rejects select without asterisk")
    {
        Parser parser("SELECT FROM users;");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("parser parses single-column create table")
    {
        Parser parser("CREATE TABLE users (id INT);");

        auto result = parser.Parse();

        REQUIRE(result.ok());
        REQUIRE(std::holds_alternative<CreateTableStatement>(*result));

        const auto& statement = std::get<CreateTableStatement>(*result);

        CHECK(statement.name == "users");
        REQUIRE(statement.columns.size() == 1);

        const auto& column = statement.columns.front();

        CHECK(column.name == "id");
        CHECK(column.dataType == DataType::INTEGER);
        CHECK_FALSE(column.nullable.has_value());
        CHECK_FALSE(column.defaultValue.has_value());
    }

    TEST_CASE("parser accepts data type aliases")
    {
        Parser parser("CREATE TABLE flags (enabled BOOL);");

        auto result = parser.Parse();

        REQUIRE(result.ok());

        const auto& statement = std::get<CreateTableStatement>(*result);

        REQUIRE(statement.columns.size() == 1);
        CHECK(statement.columns.front().dataType == DataType::BOOLEAN);
    }

    TEST_CASE("parser parses multiple columns")
    {
        Parser parser("CREATE TABLE users "
                      "(id INT, name STRING, active BOOL);");

        auto result = parser.Parse();

        REQUIRE(result.ok());
        REQUIRE(std::holds_alternative<CreateTableStatement>(*result));

        const auto& statement = std::get<CreateTableStatement>(*result);

        CHECK(statement.name == "users");
        REQUIRE(statement.columns.size() == 3);

        CHECK(statement.columns[0].name == "id");
        CHECK(statement.columns[0].dataType == DataType::INTEGER);

        CHECK(statement.columns[1].name == "name");
        CHECK(statement.columns[1].dataType == DataType::STRING);

        CHECK(statement.columns[2].name == "active");
        CHECK(statement.columns[2].dataType == DataType::BOOLEAN);
    }

    TEST_CASE("parser rejects trailing comma in columns")
    {
        Parser parser("CREATE TABLE users (id INT,);");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }
} // namespace spacedb