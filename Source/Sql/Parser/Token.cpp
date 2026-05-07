#include "Sql/Parser/Token.h"

#include <array>
#include <ranges>
#include <string_view>
#include <utility>

#include <absl/strings/match.h>

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
} // namespace spacedb