#include "../Executor/Executor.h"

#include <cstddef>
#include <memory>
#include <type_traits>
#include <unordered_map>
#include <utility>
#include <variant>

#include <absl/container/flat_hash_map.h>
#include <absl/status/status.h>

namespace spacedb
{
    namespace
    {
        // 常量表达式 → Value。
        Value ValueFromExpression(Expression expression)
        {
            return std::visit(
                [](auto value) -> Value
                {
                    using Type = std::decay_t<decltype(value)>;

                    if constexpr (std::is_same_v<Type, std::monostate>)
                    {
                        return Value::Null(); // 字面量 NULL → 空值
                    }
                    else
                    {
                        return Value{std::move(value)};
                    }
                },
                std::move(expression));
        }

        // 列对齐：值不够的尾部列用默认值补齐。
        // 例：INSERT INTO t VALUES (1, 2) 写入四列表 → 后两列填默认值
        absl::StatusOr<Row> PadRow(const schema::Table& table, Row row)
        {
            if (row.size() > table.columns.size())
            {
                return absl::InvalidArgumentError("row has more values than columns");
            }

            for (std::size_t i = row.size(); i < table.columns.size(); ++i)
            {
                const auto& column = table.columns[i];
                if (column.defaultValue.has_value())
                {
                    row.push_back(*column.defaultValue);
                }
                else
                {
                    return absl::InternalError("No default value for column " + column.name);
                }
            }
            return row;
        }

        // 列对齐：按清单把值放到对应列，其余列填默认值。
        // 例：INSERT INTO t (d, c) VALUES (1, 2) → 按 schema 顺序重排。
        absl::StatusOr<Row> MakeRow(const schema::Table& table, const std::vector<std::string>& columns, Row values)
        {
            // 清单列数必须和值个数一致
            if (columns.size() != values.size())
            {
                return absl::InternalError("columns and values num mismatch");
            }

            // 第一步：列名 → 值
            absl::flat_hash_map<std::string, Value> inputs;
            for (std::size_t i = 0; i < columns.size(); ++i)
            {
                inputs[columns[i]] = std::move(values[i]);
            }

            // 第二步：按 schema 列顺序输出
            Row result;
            result.reserve(table.columns.size());
            for (const auto& column : table.columns)
            {
                auto it = inputs.find(column.name);
                if (it != inputs.end())
                {
                    result.push_back(std::move(it->second));
                }
                else if (column.defaultValue.has_value())
                {
                    result.push_back(*column.defaultValue);
                }
                else
                {
                    return absl::InternalError("No value given for the column " + column.name);
                }
            }
            return result;
        }
    } // namespace

    std::unique_ptr<Executor> Executor::Build(PlanNode node)
    {
        return std::visit(
            [](auto&& plan) -> std::unique_ptr<Executor>
            {
                using Type = std::decay_t<decltype(plan)>;

                if constexpr (std::is_same_v<Type, CreateTableNode>)
                {
                    return std::make_unique<CreateTableExecutor>(std::move(plan.table));
                }
                else if constexpr (std::is_same_v<Type, InsertNode>)
                {
                    return std::make_unique<InsertExecutor>(std::move(plan.tableName), std::move(plan.columns), std::move(plan.values));
                }
                else if constexpr (std::is_same_v<Type, ScanNode>)
                {
                    return std::make_unique<ScanExecutor>(std::move(plan.tableName));
                }
                else
                {
                    return nullptr; // 不可达：PlanNode 只含以上三种
                }
            },
            std::move(node));
    }

    absl::StatusOr<ResultSet> CreateTableExecutor::Execute(ISqlTransaction& txn)
    {
        // 结果需要表名，先拷贝一份，再把表定义移交给事务
        const std::string tableName = table_.name;

        if (absl::Status status = txn.CreateTable(std::move(table_)); !status.ok())
        {
            return status;
        }

        return CreateTableResult{.tableName = tableName};
    }

    absl::StatusOr<ResultSet> InsertExecutor::Execute(ISqlTransaction& txn)
    {
        // 第一步：先取出表信息（表不存在 → 直接失败）
        absl::StatusOr<schema::Table> table = txn.MustGetTable(tableName_);
        if (!table.ok())
        {
            return table.status();
        }

        // 第二步：逐行处理。一行 = 表达式求值 → 列对齐 → 写入事务
        std::size_t count = 0;
        for (auto& exprs : values_)
        {
            // 2a. 常量表达式 → Value
            Row row;
            row.reserve(exprs.size());
            for (auto& expr : exprs)
            {
                row.push_back(ValueFromExpression(std::move(expr)));
            }

            // 2b. 列对齐：没写列清单 → 尾部默认值补齐；写了 → 按 schema 重排
            absl::StatusOr<Row> insertRow = columns_.empty() ? PadRow(*table, std::move(row)) : MakeRow(*table, columns_, std::move(row));
            if (!insertRow.ok())
            {
                return insertRow.status();
            }

            // 2c. 写入事务；写失败立即返回（由 Session 统一回滚）
            if (absl::Status status = txn.CreateRow(tableName_, std::move(*insertRow)); !status.ok())
            {
                return status;
            }
            ++count;
        }

        return InsertResult{.count = count};
    }

    absl::StatusOr<ResultSet> ScanExecutor::Execute(ISqlTransaction& txn)
    {
        // 第一步：表必须存在
        absl::StatusOr<schema::Table> table = txn.MustGetTable(tableName_);
        if (!table.ok())
        {
            return table.status();
        }

        // 第二步：读取全部行
        absl::StatusOr<std::vector<Row>> rows = txn.ScanTable(tableName_);
        if (!rows.ok())
        {
            return rows.status();
        }

        // 第三步：输出列名（按 schema 顺序）与行
        std::vector<std::string> columns;
        columns.reserve(table->columns.size());
        for (const auto& column : table->columns)
        {
            columns.push_back(column.name);
        }

        return RowsResult{.columns = std::move(columns), .rows = std::move(*rows)};
    }
} // namespace spacedb