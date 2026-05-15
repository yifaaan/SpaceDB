#pragma once

#include <cstddef>
#include <string_view>

#include "Sql/Parser/Token.h"

#include <absl/status/statusor.h>

namespace spacedb
{
    class Lexer
    {
    public:
        explicit Lexer(std::string_view input);

        absl::StatusOr<Token> Next();

    private:
        bool AtEnd() const;
        char Peek() const;
        char Advance();

        void SkipWhitespace();

        Token ScanIdentifier();
        Token ScanNumber();
        absl::StatusOr<Token> ScanString();
        absl::StatusOr<Token> ScanSymbol();

        static bool IsWhitespace(char value);
        static bool IsIdentifierStart(char value);
        static bool IsIdentifierPart(char value);
        static bool IsDigit(char value);

        std::string_view input_;
        std::size_t offset_ = 0;
    };
} // namespace spacedb