#include "Sql/Parser/Lexer.h"

#include <charconv>
#include <string>
#include <system_error>
#include <utility>

#include "Sql/Parser/Token.h"
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

    bool Lexer::IsDigit(char value)
    {
        return value >= '0' && value <= '9';
    }

    Token Lexer::ScanIdentifier()
    {
        const std::size_t beginOffset = offset_;

        while (!AtEnd() && IsIdentifierPart(Peek()))
        {
            Advance();
        }

        std::string value(input_.substr(beginOffset, offset_ - beginOffset));
        absl::AsciiStrToLower(&value);

        const auto keyword = KeywordFromIdentifier(value);

        if (keyword.has_value())
        {
            return Token{
                .kind = TokenKind::KEYWORD,
                .payload = TokenPayload{keyword.value()},
                .offset = beginOffset,
            };
        }

        return Token{
            .kind = TokenKind::IDENTIFIER,
            .payload = TokenPayload{std::move(value)},
            .offset = beginOffset,
        };
    }

    absl::StatusOr<Token> Lexer::ScanNumber()
    {
        size_t beginOffset = offset_;

        while (!AtEnd() && IsDigit(Peek()))
        {
            Advance();
        }

        bool isFloat = !AtEnd() && Peek() == '.';

        if (isFloat)
        {
            Advance();

            // SQL 不允许 "10." 这类小数点后无数字的字面量
            if (AtEnd() || !IsDigit(Peek()))
            {
                return absl::InvalidArgumentError(absl::StrCat("lexer: digits required after decimal point at ", DescribePosition(beginOffset)));
            }

            while (!AtEnd() && IsDigit(Peek()))
            {
                Advance();
            }
        }

        const std::string_view text = input_.substr(beginOffset, offset_ - beginOffset);

        if (isFloat)
        {
            double value = 0.0;

            const auto [end, error] = std::from_chars(text.data(), text.data() + text.size(), value);

            if (error != std::errc{} || end != text.data() + text.size())
            {
                return absl::InvalidArgumentError(absl::StrCat("lexer: invalid number literal '", text, "' at ", DescribePosition(beginOffset)));
            }

            return Token{
                .kind = TokenKind::NUMBER,
                .payload = TokenPayload{value},
                .offset = beginOffset,
            };
        }

        int64_t value = 0;

        const auto [end, error] = std::from_chars(text.data(), text.data() + text.size(), value);

        if (error == std::errc::result_out_of_range)
        {
            return absl::InvalidArgumentError(absl::StrCat("lexer: integer literal out of range at ", DescribePosition(beginOffset)));
        }

        if (error != std::errc{} || end != text.data() + text.size())
        {
            return absl::InvalidArgumentError(absl::StrCat("lexer: invalid integer literal '", text, "' at ", DescribePosition(beginOffset)));
        }

        return Token{
            .kind = TokenKind::NUMBER,
            .payload = TokenPayload{value},
            .offset = beginOffset,
        };
    }

    absl::StatusOr<Token> Lexer::ScanString()
    {
        // Next() 只有在当前字符为单引号时才调用本函数。
        // 先消费起始单引号
        const std::size_t beginOffset = offset_;

        Advance();

        std::string value;
        while (!AtEnd())
        {
            char current = Advance();
            if (current == '\'')
            {
                return Token{
                    .kind = TokenKind::STRING,
                    .payload = TokenPayload{std::move(value)},
                    .offset = beginOffset,
                };
            }
            value.push_back(current);
        }
        return absl::InvalidArgumentError(absl::StrCat("lexer: unterminated string literal at ", DescribePosition(beginOffset)));
    }

    absl::StatusOr<Token> Lexer::ScanSymbol()
    {
        size_t beginOffset = offset_;

        TokenKind kind;

        switch (Peek())
        {
        case '(':
            kind = TokenKind::OPEN_PAREN;
            break;
        case ')':
            kind = TokenKind::CLOSE_PAREN;
            break;
        case ',':
            kind = TokenKind::COMMA;
            break;
        case ';':
            kind = TokenKind::SEMICOLON;
            break;
        case '*':
            kind = TokenKind::ASTERISK;
            break;
        case '+':
            kind = TokenKind::PLUS;
            break;
        case '-':
            kind = TokenKind::MINUS;
            break;
        case '/':
            kind = TokenKind::SLASH;
            break;
        default:
            return absl::InvalidArgumentError(
                absl::StrCat("lexer: unexpected character '", std::string(1, Peek()), "' at ", DescribePosition(beginOffset)));
        }

        Advance();

        return Token{
            .kind = kind,
            .payload = std::monostate{},
            .offset = beginOffset,
        };
    }

    absl::StatusOr<std::vector<Token>> Lexer::TokenizeAll()
    {
        std::vector<Token> tokens;

        while (true)
        {
            auto token = Next();

            if (!token.ok())
            {
                return token.status();
            }

            bool isEnd = token->kind == TokenKind::END_OF_INPUT;
            tokens.push_back(std::move(*token));

            if (isEnd)
            {
                break;
            }
        }

        return tokens;
    }

    absl::StatusOr<Token> Lexer::Next()
    {
        SkipWhitespace();

        if (AtEnd())
        {
            return Token{
                .kind = TokenKind::END_OF_INPUT,
                .payload = std::monostate{},
                .offset = offset_,
            };
        }

        if (Peek() == '\'')
        {
            return ScanString();
        }

        if (IsDigit(Peek()))
        {
            return ScanNumber();
        }

        if (IsIdentifierStart(Peek()))
        {
            return ScanIdentifier();
        }

        return ScanSymbol();
    }
} // namespace spacedb
