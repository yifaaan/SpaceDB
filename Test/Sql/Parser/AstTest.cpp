#include "Sql/Parser/Ast.h"

#include <catch2/catch_test_macros.hpp>
#include <variant>

namespace spacedb
{
    TEST_CASE("AST stores literal expressions")
    {
        Expression nullValue = std::monostate();
        Expression booleanValue = true;
        Expression integerValue = int64_t{42};
        Expression floatValue = 3.14;
        Expression stringValue = std::string{"hello"};

        CHECK(std::holds_alternative<std::monostate>(nullValue));
        CHECK(std::get<bool>(booleanValue));
        CHECK(std::get<std::int64_t>(integerValue) == 42);
        CHECK(std::get<double>(floatValue) == 3.14);
        CHECK(std::get<std::string>(stringValue) == "hello");
    }

    TEST_CASE("AST compares create table statements structurally")
    {
        const CreateTableStatement expected{
            .name = "users",
            .columns =
                {
                    Column{
                        .name = "id",
                        .dataType = DataType::INTEGER,
                        .nullable = false,
                        .defaultValue = std::nullopt,
                    },
                    Column{
                        .name = "name",
                        .dataType = DataType::STRING,
                        .nullable = true,
                        .defaultValue =
                            Expression{
                                std::string{"guest"},
                            },
                    },
                },
        };

        Statement statement = expected;

        REQUIRE(std::holds_alternative<CreateTableStatement>(statement));
        CHECK(std::get<CreateTableStatement>(statement) == expected);
    }

    TEST_CASE("AST represents insert and select statements")
    {
        const InsertStatement insert{
            .tableName = "users",
            .columns = std::vector<std::string>{"id", "name"},
            .values =
                {
                    {
                        Expression{std::int64_t{1}},
                        Expression{std::string{"alice"}},
                    },
                },
        };

        const SelectStatement select{
            .tableName = "users",
        };

        Statement insert_statement = insert;
        Statement select_statement = select;

        CHECK(std::get<InsertStatement>(insert_statement) == insert);
        CHECK(std::get<SelectStatement>(select_statement) == select);
    }
} // namespace spacedb