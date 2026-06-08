#include "Sql/Parser/Token.h"

#include <array>
#include <ranges>
#include <string_view>
#include <utility>

#include <absl/strings/match.h>
#include <absl/strings/str_cat.h>

namespace spacedb
{
    std::optional<Keyword> KeywordFromIdentifier(std::string_view identifier)
    {
        static constexpr std::array<std::pair<std::string_view, Keyword>, 23> keywords = {{
            {"CREATE", Keyword::CREATE},   {"TABLE", Keyword::TABLE},     {"INT", Keyword::INT},         {"INTEGER", Keyword::INTEGER},
            {"BOOLEAN", Keyword::BOOLEAN}, {"BOOL", Keyword::BOOL},       {"STRING", Keyword::STRING},   {"TEXT", Keyword::TEXT},
            {"VARCHAR", Keyword::VARCHAR}, {"FLOAT", Keyword::FLOAT},     {"DOUBLE", Keyword::DOUBLE},   {"SELECT", Keyword::SELECT},
            {"FROM", Keyword::FROM},       {"INSERT", Keyword::INSERT},   {"INTO", Keyword::INTO},       {"VALUES", Keyword::VALUES},
            {"TRUE", Keyword::TRUE},       {"FALSE", Keyword::FALSE},     {"DEFAULT", Keyword::DEFAULT}, {"NOT", Keyword::NOT},
            {"NULL", Keyword::NULL_VALUE}, {"PRIMARY", Keyword::PRIMARY}, {"KEY", Keyword::KEY},
        }};

        const auto found = std::ranges::find_if(keywords, [identifier](const auto& item) { return absl::EqualsIgnoreCase(item.first, identifier); });

        if (found == keywords.end())
        {
            return std::nullopt;
        }

        return found->second;
    }

    std::string_view KeywordName(Keyword keyword)
    {
        switch (keyword)
        {
        case Keyword::CREATE:
            return "CREATE";
        case Keyword::TABLE:
            return "TABLE";
        case Keyword::INT:
            return "INT";
        case Keyword::INTEGER:
            return "INTEGER";
        case Keyword::BOOLEAN:
            return "BOOLEAN";
        case Keyword::BOOL:
            return "BOOL";
        case Keyword::STRING:
            return "STRING";
        case Keyword::TEXT:
            return "TEXT";
        case Keyword::VARCHAR:
            return "VARCHAR";
        case Keyword::FLOAT:
            return "FLOAT";
        case Keyword::DOUBLE:
            return "DOUBLE";
        case Keyword::SELECT:
            return "SELECT";
        case Keyword::FROM:
            return "FROM";
        case Keyword::INSERT:
            return "INSERT";
        case Keyword::INTO:
            return "INTO";
        case Keyword::VALUES:
            return "VALUES";
        case Keyword::TRUE:
            return "TRUE";
        case Keyword::FALSE:
            return "FALSE";
        case Keyword::DEFAULT:
            return "DEFAULT";
        case Keyword::NOT:
            return "NOT";
        case Keyword::NULL_VALUE:
            return "NULL";
        case Keyword::PRIMARY:
            return "PRIMARY";
        case Keyword::KEY:
            return "KEY";
        }
    }

    std::string_view TokenKindName(TokenKind kind)
    {
        switch (kind)
        {
        case TokenKind::END_OF_INPUT:
            return "end of input";
        case TokenKind::KEYWORD:
            return "keyword";
        case TokenKind::IDENTIFIER:
            return "identifier";
        case TokenKind::STRING:
            return "string literal";
        case TokenKind::NUMBER:
            return "number literal";
        case TokenKind::OPEN_PAREN:
            return "'('";
        case TokenKind::CLOSE_PAREN:
            return "')'";
        case TokenKind::COMMA:
            return "','";
        case TokenKind::SEMICOLON:
            return "';'";
        case TokenKind::ASTERISK:
            return "'*'";
        case TokenKind::PLUS:
            return "'+'";
        case TokenKind::MINUS:
            return "'-'";
        case TokenKind::SLASH:
            return "'/'";
        }
    }

    std::string DescribePosition(std::size_t offset)
    {
        return absl::StrCat("offset ", offset);
    }

    std::string DescribeToken(const Token& token)
    {
        switch (token.kind)
        {
        case TokenKind::KEYWORD:
            return absl::StrCat("keyword '", KeywordName(std::get<Keyword>(token.payload)), "'");
        case TokenKind::IDENTIFIER:
            return absl::StrCat("identifier '", std::get<std::string>(token.payload), "'");
        case TokenKind::STRING:
            return absl::StrCat("string literal '", std::get<std::string>(token.payload), "'");
        case TokenKind::NUMBER:
            if (const auto* integer = std::get_if<std::int64_t>(&token.payload))
            {
                return absl::StrCat("number literal ", *integer);
            }
            return absl::StrCat("number literal ", std::get<double>(token.payload));
        default:
            return std::string(TokenKindName(token.kind));
        }
    }
} // namespace spacedb
