#include "Sql/Parser/Lexer.h"

#include <string>

#include <catch2/catch_test_macros.hpp>

namespace spacedb
{
    TEST_CASE("lexer scans keywords and identifiers")
    {
        Lexer lexer("  CrEaTe TABLE User_Table");

        auto create = lexer.Next();

        REQUIRE(create.ok());
        CHECK(create->kind == TokenKind::KEYWORD);
        CHECK(std::get<Keyword>(create->payload) == Keyword::CREATE);

        auto table = lexer.Next();

        REQUIRE(table.ok());
        CHECK(table->kind == TokenKind::KEYWORD);
        CHECK(std::get<Keyword>(table->payload) == Keyword::TABLE);

        auto identifier = lexer.Next();

        REQUIRE(identifier.ok());
        CHECK(identifier->kind == TokenKind::IDENTIFIER);
        CHECK(std::get<std::string>(identifier->payload) == "user_table");

        auto eof = lexer.Next();

        REQUIRE(eof.ok());
        CHECK(eof->kind == TokenKind::END_OF_INPUT);
    }

    TEST_CASE("lexer skips all supported whitespace")
    {
        Lexer lexer(" \t\r\nSELECT");

        auto token = lexer.Next();

        REQUIRE(token.ok());
        CHECK(token->kind == TokenKind::KEYWORD);
        CHECK(std::get<Keyword>(token->payload) == Keyword::SELECT);
    }

    TEST_CASE("lexer rejects unsupported character")
    {
        Lexer lexer("@");

        auto result = lexer.Next();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("lexer scans integer and floating point numbers")
    {
        Lexer lexer("0 42 3.14 10.");

        for (const std::string expected : {"0", "42", "3.14", "10."})
        {
            auto token = lexer.Next();

            REQUIRE(token.ok());
            CHECK(token->kind == TokenKind::NUMBER);
            CHECK(std::get<std::string>(token->payload) == expected);
        }

        auto eof = lexer.Next();

        REQUIRE(eof.ok());
        CHECK(eof->kind == TokenKind::END_OF_INPUT);
    }

    TEST_CASE("lexer stops a number before an identifier")
    {
        Lexer lexer("12abc");

        auto number = lexer.Next();
        REQUIRE(number.ok());
        CHECK(number->kind == TokenKind::NUMBER);
        CHECK(std::get<std::string>(number->payload) == "12");

        auto identifier = lexer.Next();
        REQUIRE(identifier.ok());
        CHECK(identifier->kind == TokenKind::IDENTIFIER);
        CHECK(std::get<std::string>(identifier->payload) == "abc");
    }

    TEST_CASE("lexer rejects a second decimal point")
    {
        Lexer lexer("1.2.3");

        auto number = lexer.Next();
        REQUIRE(number.ok());
        CHECK(std::get<std::string>(number->payload) == "1.2");

        auto invalid = lexer.Next();
        REQUIRE_FALSE(invalid.ok());
        CHECK(invalid.status().code() == absl::StatusCode::kInvalidArgument);
    }

    TEST_CASE("lexer scans string literals")
    {
        Lexer lexer("'Hello SQL' '' 'with spaces'");

        for (const std::string expected : {"Hello SQL", "", "with spaces"})
        {
            auto token = lexer.Next();

            REQUIRE(token.ok());
            CHECK(token->kind == TokenKind::STRING);
            CHECK(std::get<std::string>(token->payload) == expected);
        }

        auto eof = lexer.Next();

        REQUIRE(eof.ok());
        CHECK(eof->kind == TokenKind::END_OF_INPUT);
    }

    TEST_CASE("lexer preserves string literal contents")
    {
        Lexer lexer("'MiXeD_123'");

        auto token = lexer.Next();

        REQUIRE(token.ok());
        CHECK(token->kind == TokenKind::STRING);
        CHECK(std::get<std::string>(token->payload) == "MiXeD_123");
    }

    TEST_CASE("lexer rejects unterminated string literal")
    {
        Lexer lexer("'unfinished");

        auto result = lexer.Next();

        REQUIRE_FALSE(result.ok());
        CHECK(result.status().code() == absl::StatusCode::kInvalidArgument);
        CHECK(result.status().message() == "lexer: unterminated string literal");
    }
} // namespace spacedb