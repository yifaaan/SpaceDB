#pragma once

#include <optional>
#include <string>
#include <vector>

#include "../Schema/Schema.h"
#include "../Types/Value.h"

#include <absl/status/status.h>
#include <absl/status/statusor.h>

namespace spacedb
{
    // 抽象事务：执行器只能通过它访问存储。
    class ISqlTransaction
    {
    public:
        virtual ~ISqlTransaction() = default;

        // 提交事务
        virtual absl::Status Commit() = 0;
        // 回滚事务
        virtual absl::Status Rollback() = 0;

        // DML：写入一行
        virtual absl::Status CreateRow(const std::string& tableName, Row row) = 0;
        // DML：扫描整张表，返回全部行
        virtual absl::StatusOr<std::vector<Row>> ScanTable(const std::string& tableName) = 0;

        // DDL：注册表定义
        virtual absl::Status CreateTable(schema::Table table) = 0;
        // DDL：读取表定义，不存在时返回 nullopt
        virtual absl::StatusOr<std::optional<schema::Table>> GetTable(const std::string& tableName) = 0;

        absl::StatusOr<schema::Table> MustGetTable(const std::string& tableName)
        {
            absl::StatusOr<std::optional<schema::Table>> table = GetTable(tableName);
            if (!table.ok())
            {
                return table.status();
            }
            if (!table->has_value())
            {
                return absl::NotFoundError("table " + tableName + " does not exist");
            }
            return std::move(**table);
        }
    };
} // namespace spacedb