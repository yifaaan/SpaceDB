#pragma once

#include <cstddef>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

#include "Sql/Parser/Ast.h"
#include "Sql/Parser/Lexer.h"

#include <absl/status/status.h>
#include <absl/status/statusor.h>

namespace spacedb
{
    class Parser
    {
    public:
        explicit Parser(std::string_view input);

        absl::StatusOr<Statement> Parse();

    private:
        absl::StatusOr<Statement> ParseStatement();

        absl::StatusOr<Statement> ParseSelect();
        absl::StatusOr<Statement> ParseCreateTable();
        absl::StatusOr<Statement> ParseInsert();

        absl::StatusOr<Column> ParseColumn();
        absl::StatusOr<DataType> ParseDataType();
        absl::StatusOr<Expression> ParseExpression();

        const Token& Peek() const;
        Token Next();

        absl::Status Expect(TokenKind expected);
        absl::Status ExpectKeyword(Keyword expected);

        absl::StatusOr<std::string> ExpectIdentifier();

        Lexer lexer_;
        std::vector<Token> tokens_;
        std::size_t index_ = 0;
    };
} // namespace spacedb
