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

    TEST_CASE("parser parses insert statement")
    {
        Parser parser("INSERT INTO users "
                      "(id, name) "
                      "VALUES (1, 'alice'), (2, 'bob');");

        auto result = parser.Parse();

        REQUIRE(result.ok());
        REQUIRE(std::holds_alternative<InsertStatement>(*result));

        const auto& statement = std::get<InsertStatement>(*result);

        CHECK(statement.tableName == "users");

        REQUIRE(statement.columns.has_value());
        CHECK(*statement.columns == std::vector<std::string>{"id", "name"});

        REQUIRE(statement.values.size() == 2);
        REQUIRE(statement.values[0].size() == 2);
        REQUIRE(statement.values[1].size() == 2);

        CHECK(std::get<std::int64_t>(statement.values[0][0]) == 1);
        CHECK(std::get<std::string>(statement.values[0][1]) == "alice");

        CHECK(std::get<std::int64_t>(statement.values[1][0]) == 2);
        CHECK(std::get<std::string>(statement.values[1][1]) == "bob");
    }

    TEST_CASE("parser rejects empty insert row")
    {
        Parser parser("INSERT INTO users VALUES ();");

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

    TEST_CASE("parser parses column constraints and defaults")
    {
        Parser parser("CREATE TABLE users ("
                      "id INT NOT NULL,"
                      "name STRING NULL,"
                      "age INT DEFAULT 18,"
                      "score FLOAT DEFAULT 3.14,"
                      "active BOOL DEFAULT TRUE,"
                      "note TEXT DEFAULT 'guest',"
                      "missing STRING DEFAULT NULL"
                      ");");

        auto result = parser.Parse();

        REQUIRE(result.ok());

        const auto& statement = std::get<CreateTableStatement>(*result);

        REQUIRE(statement.columns.size() == 7);

        REQUIRE(statement.columns[0].nullable.has_value());
        CHECK(*statement.columns[0].nullable == false);

        REQUIRE(statement.columns[1].nullable.has_value());
        CHECK(*statement.columns[1].nullable == true);

        REQUIRE(statement.columns[2].defaultValue.has_value());
        CHECK(std::get<std::int64_t>(*statement.columns[2].defaultValue) == 18);

        REQUIRE(statement.columns[3].defaultValue.has_value());
        CHECK(std::get<double>(*statement.columns[3].defaultValue) == 3.14);

        REQUIRE(statement.columns[4].defaultValue.has_value());
        CHECK(std::get<bool>(*statement.columns[4].defaultValue) == true);

        REQUIRE(statement.columns[5].defaultValue.has_value());
        CHECK(std::get<std::string>(*statement.columns[5].defaultValue) == "guest");

        REQUIRE(statement.columns[6].defaultValue.has_value());
        CHECK(std::holds_alternative<std::monostate>(*statement.columns[6].defaultValue));
    }

    TEST_CASE("parser rejects missing default expression")
    {
        Parser parser("CREATE TABLE users (age INT DEFAULT);");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("parser error messages include offset")
    {
        Parser parser("SELECT * FROM users");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().message() == "parser: expected ';', got end of input at offset 19");
    }

    TEST_CASE("parser describes unexpected token in error")
    {
        Parser parser("SELECT * FROM 123;");

        auto result = parser.Parse();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().message() == "parser: expected identifier, got number literal 123 at offset 14");
    }
} // namespace spacedb