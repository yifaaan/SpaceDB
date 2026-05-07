#pragma once

#include <cstdint>
#include <optional>
#include <string>
#include <string_view>
#include <variant>

namespace spacedb
{
    enum class Keyword : uint8_t
    {
        CREATE,
        TABLE,
        INT,
        INTEGER,
        BOOLEAN,
        BOOL,
        STRING,
        TEXT,
        VARCHAR,
        FLOAT,
        DOUBLE,
        SELECT,
        FROM,
        INSERT,
        INTO,
        VALUES,
        TRUE,
        FALSE,
        DEFAULT,
        NOT,
        NULL_VALUE,
        PRIMARY,
        KEY,
    };

    std::optional<Keyword> KeywordFromIdentifier(std::string_view identifier);

    enum class TokenKind : uint8_t
    {
        END_OF_INPUT,
        KEYWORD,
        IDENTIFIER,
        STRING,
        NUMBER,
        OPEN_PAREN,
        CLOSE_PAREN,
        COMMA,
        SEMICOLON,
        ASTERISK,
        PLUS,
        MINUS,
        SLASH,
    };

    using TokenPayload = std::variant<std::monostate, Keyword, std::string>;

    struct Token
    {
        TokenKind kind;
        TokenPayload payload;

        friend bool operator==(const Token&, const Token&) = default;
    };
} // namespace spacedb