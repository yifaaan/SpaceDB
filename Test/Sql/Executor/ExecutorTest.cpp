#include "Sql/Executor/Executor.h"

#include <cstdint>
#include <memory>
#include <string>

#include <catch2/catch_test_macros.hpp>

#include "../Engine/FakeTransaction.h"

namespace spacedb
{
namespace
{
    // 两列表：id 无默认值，name 默认 "unknown"。
    // 这样能同时测试"有默认补齐"和"无默认报错"两条路径。
    schema::Table MakeUserTable()
    {
        return schema::Table{
            .name = "users",
            .columns =
                {
                    schema::Column{
                        .name = "id",
                        .dataType = DataType::INTEGER,
                        .nullable = false,
                        .defaultValue = std::nullopt,
                    },
                    schema::Column{
                        .name = "name",
                        .dataType = DataType::STRING,
                        .nullable = true,
                        .defaultValue = Value{std::string{"unknown"}},
                    },
                },
        };
    }
} // namespace

    TEST_CASE("create table executor registers table")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};

        CreateTableExecutor executor{MakeUserTable()};

        const ResultSet result = executor.Execute(txn).value();

        // 返回表名
        CHECK(std::get<CreateTableResult>(result).tableName == "users");

        // 表定义已写入事务，可被读回
        const auto table = txn.GetTable("users").value();
        REQUIRE(table.has_value());
        CHECK(table->name == "users");
        CHECK(table->columns.size() == 2);
    }

    TEST_CASE("insert executor writes full rows")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};
        REQUIRE(txn.CreateTable(MakeUserTable()).ok());

        InsertExecutor executor{"users", {},
                                 {{std::int64_t{1}, std::string{"alice"}}}};

        const ResultSet result = executor.Execute(txn).value();

        CHECK(std::get<InsertResult>(result).count == 1);

        const auto rows = txn.ScanTable("users").value();
        REQUIRE(rows.size() == 1);
        REQUIRE(rows[0].size() == 2);
        CHECK(*rows[0][0].GetIf<std::int64_t>() == 1);
        CHECK(*rows[0][1].GetIf<std::string>() == "alice");
    }

    TEST_CASE("insert executor pads missing trailing columns with defaults")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};
        REQUIRE(txn.CreateTable(MakeUserTable()).ok());

        // 只给一列的值：name 应被默认值 "unknown" 补齐（PadRow 路径）
        InsertExecutor executor{"users", {}, {{std::int64_t{1}}}};

        const ResultSet result = executor.Execute(txn).value();
        CHECK(std::get<InsertResult>(result).count == 1);

        const auto rows = txn.ScanTable("users").value();
        REQUIRE(rows[0].size() == 2);
        CHECK(*rows[0][0].GetIf<std::int64_t>() == 1);
        CHECK(*rows[0][1].GetIf<std::string>() == "unknown");
    }

    TEST_CASE("insert executor rearranges values by column list")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};
        REQUIRE(txn.CreateTable(MakeUserTable()).ok());

        // 列清单与 schema 顺序不同：id 是第二列的值（MakeRow 路径）
        InsertExecutor executor{"users", {"name", "id"},
                                {{std::string{"bob"}, std::int64_t{2}}}};

        const ResultSet result = executor.Execute(txn).value();
        CHECK(std::get<InsertResult>(result).count == 1);

        const auto rows = txn.ScanTable("users").value();
        REQUIRE(rows[0].size() == 2);
        CHECK(*rows[0][0].GetIf<std::int64_t>() == 2);
        CHECK(*rows[0][1].GetIf<std::string>() == "bob");
    }

    TEST_CASE("insert executor reports missing table")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};

        InsertExecutor executor{"missing", {}, {{std::int64_t{1}}}};

        CHECK(!executor.Execute(txn).ok());
    }

    TEST_CASE("insert executor reports column count mismatch")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};
        REQUIRE(txn.CreateTable(MakeUserTable()).ok());

        // 列清单 2 个、值 1 个 → mismatch
        InsertExecutor executor{"users", {"id", "name"}, {{std::int64_t{1}}}};

        CHECK(!executor.Execute(txn).ok());
    }

    TEST_CASE("insert executor reports missing default for referenced column")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};
        REQUIRE(txn.CreateTable(MakeUserTable()).ok());

        // id 没有默认值，只给 name → "No value given for the column id"
        InsertExecutor executor{"users", {"name"}, {{std::string{"bob"}}}};

        CHECK(!executor.Execute(txn).ok());
    }

    TEST_CASE("scan executor returns schema columns and stored rows")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};
        REQUIRE(txn.CreateTable(MakeUserTable()).ok());

        InsertExecutor{"users", {},
                       {{std::int64_t{1}, std::string{"alice"}},
                        {std::int64_t{2}, std::string{"bob"}}}}
            .Execute(txn)
            .value();

        ScanExecutor executor{"users"};

        const ResultSet result = executor.Execute(txn).value();

        const auto& rows = std::get<RowsResult>(result);
        CHECK(rows.columns == std::vector<std::string>{"id", "name"});
        REQUIRE(rows.rows.size() == 2);
        CHECK(*rows.rows[1][0].GetIf<std::int64_t>() == 2);
    }

    TEST_CASE("plan execute dispatches through executor factory")
    {
        FakeTransaction txn{std::make_shared<FakeStore>()};

        // CreateTable 计划 → 工厂 → CreateTableExecutor
        Plan createPlan{.node = CreateTableNode{.table = MakeUserTable()}};
        CHECK(std::get<CreateTableResult>(createPlan.Execute(txn).value()).tableName == "users");

        // Insert 计划 → InsertExecutor
        Plan insertPlan{.node = InsertNode{.tableName = "users", .columns = {}, .values = {{std::int64_t{1}}}}};
        CHECK(std::get<InsertResult>(insertPlan.Execute(txn).value()).count == 1);

        // Scan 计划 → ScanExecutor
        Plan scanPlan{.node = ScanNode{.tableName = "users"}};
        CHECK(std::get<RowsResult>(scanPlan.Execute(txn).value()).rows.size() == 1);
    }
} // namespace spacedb
