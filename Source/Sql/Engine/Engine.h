#pragma once

#include <memory>
#include <string_view>

#include "../Executor/ResultSet.h"
#include "Transaction.h"

#include <absl/status/statusor.h>

namespace spacedb
{
    // 抽象引擎：负责开启事务
    class ISqlEngine
    {
    public:
        virtual ~ISqlEngine() = default;

        virtual absl::StatusOr<std::unique_ptr<ISqlTransaction>> Begin() = 0;
    };

    // 客户端会话：一条 SQL 字符串 → 解析 → 规划 → 事务 → 结果。
    class Session
    {
    public:
        explicit Session(ISqlEngine& engine);

        // 执行客户端 SQL 语句；执行失败自动回滚
        absl::StatusOr<ResultSet> Execute(std::string_view sql);

    private:
        ISqlEngine& engine_;
    };
} // namespace spacedb
