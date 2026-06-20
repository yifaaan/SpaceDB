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

    Plan Planner::BuildInsert(InsertStatement statement)
    {
        std::vector<std::string> columns;

        if (statement.columns.has_value())
        {
            columns = std::move(*statement.columns);
        }

        return Plan{
            .node =
                InsertNode{
                    .tableName = std::move(statement.tableName),
                    .columns = std::move(columns),
                    .values = std::move(statement.values),
                },
        };
    }

    Plan Planner::BuildScan(SelectStatement statement)
    {
        return Plan{
            .node =
                ScanNode{
                    .tableName = std::move(statement.tableName),
                },
        };
    }

    absl::StatusOr<Plan> Planner::Build(Statement statement)
    {
        if (auto create = std::get_if<CreateTableStatement>(&statement))
        {
            return BuildCreateTable(std::move(*create));
        }

        if (auto insert = std::get_if<InsertStatement>(&statement))
        {
            return BuildInsert(std::move(*insert));
        }

        if (auto select = std::get_if<SelectStatement>(&statement))
        {
            return BuildScan(std::move(*select));
        }

        return absl::InternalError("planner: statement variant has no active value");
    }
} // namespace spacedb