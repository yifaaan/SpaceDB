#include "Sql/Plan/Plan.h"

#include <memory>

#include "Sql/Executor/Executor.h"

namespace spacedb
{
    absl::StatusOr<ResultSet> Plan::Execute(ISqlTransaction& txn)
    {
        // 根据节点类型构造执行器，再交给事务执行。
        std::unique_ptr<Executor> executor = Executor::Build(std::move(node));
        return executor->Execute(txn);
    }
} // namespace spacedb
