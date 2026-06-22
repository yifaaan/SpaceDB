#pragma once

#include <memory>
#include <optional>
#include <string>
#include <unordered_map>
#include <utility>
#include <vector>

#include "Sql/Engine/Engine.h"
#include "Sql/Engine/Transaction.h"

#include <absl/status/status.h>
#include <absl/status/statusor.h>

namespace spacedb
{
    // 假存储：所有会话共享的表与行数据。
    // 模拟"引擎内的持久状态"，让多条 SQL 之间能互相看到结果。
    // 阶段 5/6 由真实的内存 KV 引擎替换。
    class FakeStore
    {
    public:
        std::unordered_map<std::string, schema::Table> tables;
        std::unordered_map<std::string, std::vector<Row>> rows;
    };

    // 假事务：操作直接作用在共享的 FakeStore 上。
    // 没有真实事务的版本号、写集与冲突检测（阶段 8 MVCC 才引入），
    // 所以 Commit/Rollback 目前只是成功返回。
    class FakeTransaction : public ISqlTransaction
    {
    public:
        explicit FakeTransaction(std::shared_ptr<FakeStore> store) : store_(std::move(store))
        {
        }

        absl::Status Commit() override
        {
            return absl::OkStatus();
        }

        absl::Status Rollback() override
        {
            return absl::OkStatus();
        }

        absl::Status CreateRow(const std::string& tableName, Row row) override
        {
            if (!store_->tables.contains(tableName))
            {
                // 防呆：写不存在的表是调用方 bug，直接报错
                return absl::NotFoundError("table " + tableName + " does not exist");
            }
            store_->rows[tableName].push_back(std::move(row));
            return absl::OkStatus();
        }

        absl::StatusOr<std::vector<Row>> ScanTable(const std::string& tableName) override
        {
            if (!store_->tables.contains(tableName))
            {
                return absl::NotFoundError("table " + tableName + " does not exist");
            }
            return store_->rows[tableName];
        }

        absl::Status CreateTable(schema::Table table) override
        {
            if (store_->tables.contains(table.name))
            {
                return absl::AlreadyExistsError("table " + table.name + " already exists");
            }
            store_->tables[table.name] = std::move(table);
            return absl::OkStatus();
        }

        absl::StatusOr<std::optional<schema::Table>> GetTable(const std::string& tableName) override
        {
            auto it = store_->tables.find(tableName);
            if (it == store_->tables.end())
            {
                return std::nullopt;
            }
            return it->second;
        }

    private:
        std::shared_ptr<FakeStore> store_; // 共享：同一引擎的所有事务看到同一份数据
    };

    // 假引擎：每次 Begin 创建共享同一存储的新事务。
    // 这模拟了"一个引擎对应一份数据，多次事务共享它"的真实形态。
    class FakeEngine : public ISqlEngine
    {
    public:
        FakeEngine() : store_(std::make_shared<FakeStore>())
        {
        }

        absl::StatusOr<std::unique_ptr<ISqlTransaction>> Begin() override
        {
            // 注意先显式升型为 unique_ptr<ISqlTransaction>：
            // absl::StatusOr 没有从任意子类指针转换的构造函数
            return std::unique_ptr<ISqlTransaction>(std::make_unique<FakeTransaction>(store_));
        }

    private:
        std::shared_ptr<FakeStore> store_;
    };
} // namespace spacedb
