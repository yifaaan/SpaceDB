#include "Sql/Parser/Parser.h"

#include <utility>

#include <absl/strings/str_cat.h>

namespace spacedb
{
    Parser::Parser(std::string_view input) : lexer_(input)
    {
    }

    absl::Status Parser::EnsureLookahead()
    {
        if (lookahead_.has_value())
        {
            return absl::OkStatus();
        }

        auto token = lexer_.Next();

        if (!token.ok())
        {
            return token.status();
        }

        lookahead_ = *token;
        return absl::OkStatus();
    }

    absl::StatusOr<const Token*> Parser::Peek()
    {
        auto status = EnsureLookahead();

        if (!status.ok())
        {
            return status;
        }

        return &lookahead_.value();
    }

    absl::StatusOr<Token> Parser::Next()
    {
        auto token = Peek();

        if (!token.ok())
        {
            return token.status();
        }

        Token result = std::move(lookahead_.value());
        lookahead_.reset();

        return result;
    }

    absl::Status Parser::Expect(TokenKind expected)
    {
        auto token = Next();

        if (!token.ok())
        {
            return token.status();
        }

        if (token->kind != expected)
        {
            return absl::InvalidArgumentError(
                absl::StrCat("parser: unexpected token kind, expected ", static_cast<int>(expected), ", got ", static_cast<int>(token->kind)));
        }

        return absl::OkStatus();
    }

    absl::Status Parser::ExpectKeyword(Keyword expected)
    {
        auto token = Next();

        if (!token.ok())
        {
            return token.status();
        }

        if (token->kind != TokenKind::KEYWORD)
        {
            return absl::InvalidArgumentError("parser: expected keyword");
        }

        if (std::get<Keyword>(token->payload) != expected)
        {
            return absl::InvalidArgumentError("parser: unexpected keyword");
        }

        return absl::OkStatus();
    }

    absl::StatusOr<std::string> Parser::ExpectIdentifier()
    {
        auto token = Next();

        if (!token.ok())
        {
            return token.status();
        }

        if (token->kind != TokenKind::IDENTIFIER)
        {
            return absl::InvalidArgumentError("parser: expected identifier");
        }

        return std::get<std::string>(token->payload);
    }

    absl::StatusOr<Statement> Parser::Parse()
    {
        auto status = ExpectKeyword(Keyword::SELECT);

        if (!status.ok())
        {
            return status;
        }

        status = Expect(TokenKind::ASTERISK);

        if (!status.ok())
        {
            return status;
        }

        status = ExpectKeyword(Keyword::FROM);

        if (!status.ok())
        {
            return status;
        }

        auto table_name = ExpectIdentifier();

        if (!table_name.ok())
        {
            return table_name.status();
        }

        status = Expect(TokenKind::SEMICOLON);

        if (!status.ok())
        {
            return status;
        }

        status = Expect(TokenKind::END_OF_INPUT);

        if (!status.ok())
        {
            return status;
        }

        return Statement{
            SelectStatement{
                .tableName = std::move(table_name.value()),
            },
        };
    }
} // namespace spacedb