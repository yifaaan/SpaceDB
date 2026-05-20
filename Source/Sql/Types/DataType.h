#pragma once

#include <cstdint>

namespace spacedb
{
    // 列类型
    enum class DataType : uint8_t
    {
        BOOLEAN,
        INTEGER,
        FLOAT,
        STRING,
    };
} // namespace spacedb