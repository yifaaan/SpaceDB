#include "Sql/Parser/Parser.h"

#include <utility>

#include <absl/strings/str_cat.h>

namespace spacedb
{
    Parser::Parser(std::string_view input) : lexer_(input)
    {
    }

    const Token& Parser::Peek() const
    {
        // tokens_ 以 END_OF_INPUT 结尾,index_ 永远指向有效 token
        return tokens_[index_];
    }

    Token Parser::Next()
    {
        Token token = tokens_[index_];

        // END_OF_INPUT 是最后一个 token,消费它后不再推进
        if (token.kind != TokenKind::END_OF_INPUT)
        {
            ++index_;
        }

        return token;
    }

    absl::Status Parser::Expect(TokenKind expected)
    {
        const Token token = Next();

        if (token.kind != expected)
        {
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: expected ", TokenKindName(expected), ", got ", DescribeToken(token), " at ", DescribePosition(token.offset)));
        }

        return absl::OkStatus();
    }

    absl::Status Parser::ExpectKeyword(Keyword expected)
    {
        const Token token = Next();

        if (token.kind != TokenKind::KEYWORD)
        {
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: expected keyword '", KeywordName(expected), "', got ", DescribeToken(token), " at ", DescribePosition(token.offset)));
        }

        if (std::get<Keyword>(token.payload) != expected)
        {
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: expected keyword '", KeywordName(expected), "', got ", DescribeToken(token), " at ", DescribePosition(token.offset)));
        }

        return absl::OkStatus();
    }

    absl::StatusOr<std::string> Parser::ExpectIdentifier()
    {
        const Token token = Next();

        if (token.kind != TokenKind::IDENTIFIER)
        {
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: expected identifier, got ", DescribeToken(token), " at ", DescribePosition(token.offset)));
        }

        return std::get<std::string>(std::move(token.payload));
    }

    absl::StatusOr<Statement> Parser::ParseStatement()
    {
        const Token& current = Peek();

        if (current.kind == TokenKind::END_OF_INPUT)
        {
            return absl::InvalidArgumentError(absl::StrCat("parser: expected statement, got end of input at ", DescribePosition(current.offset)));
        }

        if (current.kind != TokenKind::KEYWORD)
        {
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: statement must begin with a keyword, got ", DescribeToken(current), " at ", DescribePosition(current.offset)));
        }

        switch (std::get<Keyword>(current.payload))
        {
        case Keyword::SELECT:
            return ParseSelect();

        case Keyword::CREATE:
            return ParseCreateTable();

        case Keyword::INSERT:
            return ParseInsert();

        default:
            return absl::InvalidArgumentError("parser: unsupported statement");
        }
    }

    absl::StatusOr<Statement> Parser::ParseSelect()
    {
        if (auto status = ExpectKeyword(Keyword::SELECT); !status.ok())
        {
            return status;
        }

        if (auto status = Expect(TokenKind::ASTERISK); !status.ok())
        {
            return status;
        }

        if (auto status = ExpectKeyword(Keyword::FROM); !status.ok())
        {
            return status;
        }

        auto tableName = ExpectIdentifier();

        if (!tableName.ok())
        {
            return tableName.status();
        }

        return Statement{
            SelectStatement{
                .tableName = std::move(tableName.value()),
            },
        };
    }

    absl::StatusOr<Statement> Parser::ParseCreateTable()
    {
        if (auto status = ExpectKeyword(Keyword::CREATE); !status.ok())
        {
            return status;
        }

        if (auto status = ExpectKeyword(Keyword::TABLE); !status.ok())
        {
            return status;
        }

        auto tableName = ExpectIdentifier();

        if (!tableName.ok())
        {
            return tableName.status();
        }

        if (auto status = Expect(TokenKind::OPEN_PAREN); !status.ok())
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

            if (Peek().kind != TokenKind::COMMA)
            {
                break;
            }

            if (auto status = Expect(TokenKind::COMMA); !status.ok())
            {
                return status;
            }
        }

        if (auto status = Expect(TokenKind::CLOSE_PAREN); !status.ok())
        {
            return status;
        }

        return Statement{
            CreateTableStatement{
                .name = std::move(tableName.value()),
                .columns = std::move(columns),
            },
        };
    }

    absl::StatusOr<Statement> Parser::ParseInsert()
    {
        if (auto status = ExpectKeyword(Keyword::INSERT); !status.ok())
        {
            return status;
        }

        if (auto status = ExpectKeyword(Keyword::INTO); !status.ok())
        {
            return status;
        }

        auto tableName = ExpectIdentifier();

        if (!tableName.ok())
        {
            return tableName.status();
        }

        std::optional<std::vector<std::string>> columns;

        // 可选列清单：INSERT INTO users (id, name)
        if (Peek().kind == TokenKind::OPEN_PAREN)
        {
            if (auto status = Expect(TokenKind::OPEN_PAREN); !status.ok())
            {
                return status;
            }

            std::vector<std::string> parsedColumns;

            auto firstColumn = ExpectIdentifier();

            if (!firstColumn.ok())
            {
                return firstColumn.status();
            }

            parsedColumns.push_back(std::move(firstColumn.value()));

            while (true)
            {
                if (Peek().kind == TokenKind::CLOSE_PAREN)
                {
                    break;
                }

                if (auto status = Expect(TokenKind::COMMA); !status.ok())
                {
                    return status;
                }

                auto column = ExpectIdentifier();

                if (!column.ok())
                {
                    return column.status();
                }

                parsedColumns.push_back(std::move(column.value()));
            }

            if (auto status = Expect(TokenKind::CLOSE_PAREN); !status.ok())
            {
                return status;
            }

            columns = std::move(parsedColumns);
        }

        if (auto status = ExpectKeyword(Keyword::VALUES); !status.ok())
        {
            return status;
        }

        std::vector<std::vector<Expression>> values;

        // 一个 INSERT 可以包含多行 VALUES
        while (true)
        {
            if (auto status = Expect(TokenKind::OPEN_PAREN); !status.ok())
            {
                return status;
            }

            std::vector<Expression> row;

            // 空行 VALUES () 不允许
            auto firstExpression = ParseExpression();

            if (!firstExpression.ok())
            {
                return firstExpression.status();
            }

            row.push_back(std::move(firstExpression.value()));

            while (true)
            {
                if (Peek().kind == TokenKind::CLOSE_PAREN)
                {
                    break;
                }

                if (auto status = Expect(TokenKind::COMMA); !status.ok())
                {
                    return status;
                }

                auto expression = ParseExpression();

                if (!expression.ok())
                {
                    return expression.status();
                }

                row.push_back(std::move(expression.value()));
            }

            if (auto status = Expect(TokenKind::CLOSE_PAREN); !status.ok())
            {
                return status;
            }

            values.push_back(std::move(row));

            if (Peek().kind != TokenKind::COMMA)
            {
                break;
            }

            if (auto status = Expect(TokenKind::COMMA); !status.ok())
            {
                return status;
            }
        }

        return Statement{
            InsertStatement{
                .tableName = std::move(tableName.value()),
                .columns = std::move(columns),
                .values = std::move(values),
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

        auto dataType = ParseDataType();

        if (!dataType.ok())
        {
            return dataType.status();
        }

        Column column{
            .name = std::move(name.value()),
            .dataType = dataType.value(),
            .nullable = std::nullopt,
            .defaultValue = std::nullopt,
        };

        while (true)
        {
            const Token& next = Peek();

            if (next.kind != TokenKind::KEYWORD)
            {
                break;
            }

            const Keyword keyword = std::get<Keyword>(next.payload);

            switch (keyword)
            {
            case Keyword::NULL_VALUE:
                if (auto status = ExpectKeyword(Keyword::NULL_VALUE); !status.ok())
                {
                    return status;
                }

                column.nullable = true;
                break;

            case Keyword::NOT:
                if (auto status = ExpectKeyword(Keyword::NOT); !status.ok())
                {
                    return status;
                }

                if (auto status = ExpectKeyword(Keyword::NULL_VALUE); !status.ok())
                {
                    return status;
                }

                column.nullable = false;
                break;

            case Keyword::DEFAULT:
            {
                if (auto status = ExpectKeyword(Keyword::DEFAULT); !status.ok())
                {
                    return status;
                }

                auto expression = ParseExpression();

                if (!expression.ok())
                {
                    return expression.status();
                }

                column.defaultValue = std::move(expression.value());
                break;
            }

            default:
                return absl::InvalidArgumentError(absl::StrCat(
                    "parser: unexpected column constraint '", KeywordName(keyword), "' at ", DescribePosition(next.offset)));
            }
        }

        return column;
    }

    absl::StatusOr<DataType> Parser::ParseDataType()
    {
        const Token token = Next();

        if (token.kind != TokenKind::KEYWORD)
        {
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: expected column data type, got ", DescribeToken(token), " at ", DescribePosition(token.offset)));
        }

        switch (std::get<Keyword>(token.payload))
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
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: unexpected column data type '", KeywordName(std::get<Keyword>(token.payload)), "' at ", DescribePosition(token.offset)));
        }
    }

    absl::StatusOr<Expression> Parser::ParseExpression()
    {
        Token token = Next();

        switch (token.kind)
        {
        case TokenKind::STRING:
            return Expression{
                std::get<std::string>(std::move(token.payload)),
            };

        case TokenKind::NUMBER:
        {
            // 字面量已在词法阶段完成解析和校验,这里只需取出
            if (const auto* value = std::get_if<std::int64_t>(&token.payload))
            {
                return Expression{*value};
            }

            return Expression{std::get<double>(token.payload)};
        }

        case TokenKind::KEYWORD:
        {
            switch (std::get<Keyword>(token.payload))
            {
            case Keyword::TRUE:
                return Expression{true};

            case Keyword::FALSE:
                return Expression{false};

            case Keyword::NULL_VALUE:
                return Expression{std::monostate{}};

            default:
                return absl::InvalidArgumentError(absl::StrCat(
                    "parser: keyword '", KeywordName(std::get<Keyword>(token.payload)), "' is not an expression at ", DescribePosition(token.offset)));
            }
        }

        default:
            return absl::InvalidArgumentError(absl::StrCat(
                "parser: expected constant expression, got ", DescribeToken(token), " at ", DescribePosition(token.offset)));
        }
    }

    absl::StatusOr<Statement> Parser::Parse()
    {
        auto tokenize = lexer_.TokenizeAll();

        if (!tokenize.ok())
        {
            return tokenize.status();
        }

        tokens_ = std::move(*tokenize);
        index_ = 0;

        auto statement = ParseStatement();

        if (!statement.ok())
        {
            return statement.status();
        }

        if (auto status = Expect(TokenKind::SEMICOLON); !status.ok())
        {
            return status;
        }

        if (auto status = Expect(TokenKind::END_OF_INPUT); !status.ok())
        {
            return status;
        }

        return std::move(statement.value());
    }
} // namespace spacedb
