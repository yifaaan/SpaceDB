#include "../Engine/Engine.h"

#include <utility>

#include <absl/status/status.h>

#include "../Parser/Parser.h"
#include "../Plan/Planner.h"

namespace spacedb
{
    Session::Session(ISqlEngine& engine) : engine_(engine)
    {
    }

    absl::StatusOr<ResultSet> Session::Execute(std::string_view sql)
    {
        // 第一步：词法/语法解析 → AST
        absl::StatusOr<Statement> statement = Parser(sql).Parse();
        if (!statement.ok())
        {
            return statement.status();
        }

        // 第二步：规划 → 计划树
        absl::StatusOr<Plan> plan = Planner::Build(std::move(*statement));
        if (!plan.ok())
        {
            return plan.status();
        }

        // 第三步：开启事务
        absl::StatusOr<std::unique_ptr<ISqlTransaction>> txn = engine_.Begin();
        if (!txn.ok())
        {
            return txn.status();
        }

        // 第四步：在事务中执行计划
        absl::StatusOr<ResultSet> result = plan->Execute(**txn);
        if (!result.ok())
        {
            // 第五步：失败 → 回滚，向上传播原始错误。
            // 回滚自身失败的细节暂不叠加（阶段 8 MVCC 再细化）。
            std::ignore = (*txn)->Rollback();
            return result.status();
        }

        // 第六步：成功 → 提交
        if (absl::Status status = (*txn)->Commit(); !status.ok())
        {
            return status;
        }
        return result;
    }
} // namespace spacedb