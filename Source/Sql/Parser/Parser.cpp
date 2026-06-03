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

    absl::StatusOr<Statement> Parser::ParseStatement()
    {
        auto token = Peek();

        if (!token.ok())
        {
            return token.status();
        }

        auto current = token.value();

        if (current->kind == TokenKind::END_OF_INPUT)
        {
            return absl::InvalidArgumentError("parser: expected statement, got end of input");
        }

        if (current->kind != TokenKind::KEYWORD)
        {
            return absl::InvalidArgumentError("parser: statement must begin with a keyword");
        }

        auto keyword = std::get<Keyword>(current->payload);

        switch (keyword)
        {
        case Keyword::SELECT:
            return ParseSelect();
        case Keyword::CREATE:
            return ParseCreateTable();
        default:
            return absl::InvalidArgumentError("parser: unsupported statement");
        }
    }

    absl::StatusOr<Statement> Parser::ParseSelect()
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

        return Statement{
            SelectStatement{
                .tableName = std::move(table_name.value()),
            },
        };
    }

    absl::StatusOr<Statement> Parser::ParseCreateTable()
    {
        auto status = ExpectKeyword(Keyword::CREATE);

        if (!status.ok())
        {
            return status;
        }

        status = ExpectKeyword(Keyword::TABLE);

        if (!status.ok())
        {
            return status;
        }

        auto table_name = ExpectIdentifier();

        if (!table_name.ok())
        {
            return table_name.status();
        }

        status = Expect(TokenKind::OPEN_PAREN);

        if (!status.ok())
        {
            return status;
        }

        std::vector<Column> columns;
        while (true)
        {
            auto column = ParseColumn();

            if (!column.ok())
            {
                return column.status();
            }

            columns.push_back(std::move(column.value()));

            auto next = Peek();

            if (!next.ok())
            {
                return next.status();
            }

            if ((*next)->kind != TokenKind::COMMA)
            {
                break;
            }

            status = Expect(TokenKind::COMMA);

            if (!status.ok())
            {
                return status;
            }
        }

        status = Expect(TokenKind::CLOSE_PAREN);
        if (!status.ok())
        {
            return status;
        }

        return Statement{
            CreateTableStatement{
                .name = std::move(table_name.value()),
                .columns = std::move(columns),
            },
        };
    }

    absl::StatusOr<Column> Parser::ParseColumn()
    {
        auto name = ExpectIdentifier();

        if (!name.ok())
        {
            return name.status();
        }

        auto data_type = ParseDataType();

        if (!data_type.ok())
        {
            return data_type.status();
        }

        return Column{
            .name = std::move(name.value()),
            .dataType = data_type.value(),
            .nullable = std::nullopt,
            .defaultValue = std::nullopt,
        };
    }

    absl::StatusOr<DataType> Parser::ParseDataType()
    {
        auto token = Next();

        if (!token.ok())
        {
            return token.status();
        }

        if (token->kind != TokenKind::KEYWORD)
        {
            return absl::InvalidArgumentError("parser: expected column data type");
        }

        switch (std::get<Keyword>(token->payload))
        {
        case Keyword::INT:
        case Keyword::INTEGER:
            return DataType::INTEGER;

        case Keyword::BOOLEAN:
        case Keyword::BOOL:
            return DataType::BOOLEAN;

        case Keyword::FLOAT:
        case Keyword::DOUBLE:
            return DataType::FLOAT;

        case Keyword::STRING:
        case Keyword::TEXT:
        case Keyword::VARCHAR:
            return DataType::STRING;

        default:
            return absl::InvalidArgumentError("parser: unexpected column data type");
        }
    }

    absl::StatusOr<Statement> Parser::Parse()
    {
        auto statement = ParseStatement();

        if (!statement.ok())
        {
            return statement.status();
        }

        auto status = Expect(TokenKind::SEMICOLON);

        if (!status.ok())
        {
            return status;
        }

        status = Expect(TokenKind::END_OF_INPUT);

        if (!status.ok())
        {
            return status;
        }

        return std::move(statement.value());
    }
} // namespace spacedb
