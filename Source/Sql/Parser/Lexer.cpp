#include "Sql/Parser/Lexer.h"

#include <string>

#include "absl/status/status.h"
#include "absl/strings/ascii.h"
#include "absl/strings/str_cat.h"

namespace spacedb
{
    Lexer::Lexer(std::string_view input) : input_(input)
    {
    }

    bool Lexer::AtEnd() const
    {
        return offset_ >= input_.size();
    }

    char Lexer::Peek() const
    {
        return AtEnd() ? '\0' : input_[offset_];
    }

    char Lexer::Advance()
    {
        return input_[offset_++];
    }

    bool Lexer::IsWhitespace(char value)
    {
        return value == ' ' || value == '\t' || value == '\r' || value == '\n' || value == '\f' || value == '\v';
    }

    bool Lexer::IsIdentifierStart(char value)
    {
        return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') || value == '_';
    }

    bool Lexer::IsIdentifierPart(char value)
    {
        return IsIdentifierStart(value) || (value >= '0' && value <= '9');
    }

    void Lexer::SkipWhitespace()
    {
        while (!AtEnd() && IsWhitespace(Peek()))
        {
            Advance();
        }
    }

    Token Lexer::ScanIdentifier()
    {
        std::size_t begin = offset_;

        while (offset_ < input_.size() && IsIdentifierPart(input_[offset_]))
        {
            ++offset_;
        }

        std::string value(input_.substr(begin, offset_ - begin));
        absl::AsciiStrToLower(&value);

        const auto keyword = KeywordFromIdentifier(value);

        if (keyword.has_value())
        {
            return Token{
                .kind = TokenKind::KEYWORD,
                .payload = TokenPayload{keyword.value()},
            };
        }

        return Token{
            .kind = TokenKind::IDENTIFIER,
            .payload = TokenPayload{std::move(value)},
        };
    }

    absl::StatusOr<Token> Lexer::Next()
    {
        SkipWhitespace();

        if (AtEnd())
        {
            return Token{
                .kind = TokenKind::END_OF_INPUT,
                .payload = std::monostate{},
            };
        }

        if (IsIdentifierStart(Peek()))
        {
            return ScanIdentifier();
        }

        return absl::InvalidArgumentError(
            absl::StrCat("lexer: unexpected character '", std::string(1, Peek()), "'"));
    }
} // namespace spacedb
