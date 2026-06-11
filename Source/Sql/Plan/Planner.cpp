#include "Sql/Plan/Planner.h"

#include <optional>
#include <type_traits>
#include <utility>
#include <vector>

#include <absl/status/status.h>

namespace spacedb
{
    Value Planner::BuildValue(Expression expression)
    {
        return std::visit(
            [](auto value) -> Value
            {
                using Type = std::decay_t<decltype(value)>;

                if constexpr (std::is_same_v<Type, std::monostate>)
                {
                    return Value::Null();
                }
                else
                {
                    return Value{std::move(value)};
                }
            },
            std::move(expression));
    }

    Plan Planner::BuildCreateTable(CreateTableStatement statement)
    {
        std::vector<schema::Column> columns;
        columns.reserve(statement.columns.size());

        for (auto& source : statement.columns)
        {
            const bool nullable = source.nullable.value_or(true);
            std::optional<Value> defaultValue;

            if (source.defaultValue.has_value())
            {
                defaultValue = BuildValue(std::move(*source.defaultValue));
            }
            else if (nullable)
            {
                defaultValue = Value::Null();
            }

            columns.push_back(schema::Column{
                .name = std::move(source.name),
                .dataType = source.dataType,
                .nullable = nullable,
                .defaultValue = std::move(defaultValue),
            });
        }

        return Plan{
            .node =
                CreateTableNode{
                    .table =
                        schema::Table{
                            .name = std::move(statement.name),
                            .columns = std::move(columns),
                        },
                },
        };
    }

    absl::StatusOr<Plan> Planner::Build(Statement statement)
    {
        if (auto create = std::get_if<CreateTableStatement>(&statement))
        {
            return BuildCreateTable(std::move(*create));
        }

        return absl::UnimplementedError("planner: statement is not supported yet");
    }
} // namespace spacedb