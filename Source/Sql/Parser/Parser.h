#pragma once

#include <optional>
#include <string>
#include <string_view>

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
        absl::Status EnsureLookahead();

        absl::StatusOr<const Token*> Peek();
        absl::StatusOr<Token> Next();

        absl::Status Expect(TokenKind expected);
        absl::Status ExpectKeyword(Keyword expected);

        absl::StatusOr<std::string> ExpectIdentifier();

        Lexer lexer_;
        std::optional<Token> lookahead_;
    };
} // namespace spacedb