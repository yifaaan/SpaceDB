#pragma once

#include "../Parser/Ast.h"
#include "../Types/Value.h"
#include "./Plan.h"

#include <absl/status/statusor.h>

namespace spacedb
{
    class Planner
    {
    public:
        static absl::StatusOr<Plan> Build(Statement statement);

    private:
        static Plan BuildCreateTable(CreateTableStatement statement);
        static Plan BuildInsert(InsertStatement statement);
        static Plan BuildScan(SelectStatement statement);

        static Value BuildValue(Expression expression);
    };
} // namespace spacedb