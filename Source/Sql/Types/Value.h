#pragma once

#include <cstdint>
#include <string>
#include <utility>
#include <variant>

namespace spacedb
{
    class Value
    {
    public:
        using Storage = std::variant<std::monostate, bool, std::int64_t, double, std::string>;

        Value() = default;
        explicit Value(bool value) : data_(value)
        {
        }
        explicit Value(std::int64_t value) : data_(value)
        {
        }
        explicit Value(double value) : data_(value)
        {
        }
        explicit Value(std::string value) : data_(std::move(value))
        {
        }

        static Value Null()
        {
            return Value{};
        }

        bool IsNull() const
        {
            return std::holds_alternative<std::monostate>(data_);
        }

        template <typename T> const T* GetIf() const
        {
            return std::get_if<T>(&data_);
        }

        friend bool operator==(const Value&, const Value&) = default;

    private:
        Storage data_;
    };
} // namespace spacedb