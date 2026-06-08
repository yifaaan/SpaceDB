#pragma once

#include <cstddef>
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
    std::string_view KeywordName(Keyword keyword);

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

    std::string_view TokenKindName(TokenKind kind);

    std::string DescribePosition(std::size_t offset);

    // 数字字面量在词法阶段直接解析为 int64_t / double,不再携带原始文本
    using TokenPayload = std::variant<std::monostate, Keyword, std::string, std::int64_t, double>;

    struct Token
    {
        TokenKind kind;
        TokenPayload payload;
        std::size_t offset = 0;

        friend bool operator==(const Token&, const Token&) = default;
    };

    // 将 token 渲染为可读文本,用于错误消息,如 "identifier 'users'"、"number literal 42"
    std::string DescribeToken(const Token& token);
} // namespace spacedb
