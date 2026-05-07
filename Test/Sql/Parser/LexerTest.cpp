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
} // namespace spacedb