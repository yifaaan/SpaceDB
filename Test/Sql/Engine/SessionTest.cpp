#include "Sql/Engine/Engine.h"

#include <cstdint>
#include <string>
#include <vector>

#include <catch2/catch_test_macros.hpp>

#include "FakeTransaction.h"

namespace spacedb
{
    TEST_CASE("session runs create insert select end to end")
    {
        FakeEngine engine;
        Session session{engine};

        // 第一条 SQL：建表
        const ResultSet create = session.Execute("CREATE TABLE users (id INTEGER, name STRING);").value();
        CHECK(std::get<CreateTableResult>(create).tableName == "users");

        // 第二条 SQL：插入一行。
        // 每次 Execute 都是新事务，但共享 FakeStore，所以前面的建表可见。
        const ResultSet insert = session.Execute("INSERT INTO users VALUES (1, 'alice');").value();
        CHECK(std::get<InsertResult>(insert).count == 1);

        // 第三条 SQL：查回这一行
        const ResultSet query = session.Execute("SELECT * FROM users;").value();

        const auto& rows = std::get<RowsResult>(query);
        CHECK(rows.columns == std::vector<std::string>{"id", "name"});
        REQUIRE(rows.rows.size() == 1);
        CHECK(*rows.rows[0][0].GetIf<std::int64_t>() == 1);
        CHECK(*rows.rows[0][1].GetIf<std::string>() == "alice");
    }

    TEST_CASE("session reports errors for missing table and bad sql")
    {
        FakeEngine engine;
        Session session{engine};

        // 插不存在的表：MustGetTable 报错 → 回滚 → 返回错误
        CHECK(!session.Execute("INSERT INTO missing VALUES (1);").ok());

        // 查不存在的表：同上
        CHECK(!session.Execute("SELECT * FROM missing;").ok());

        // 语法无法解析（没有分号也走不到执行阶段，这里测的是解析错误）
        CHECK(!session.Execute("BOGUS STUFF;").ok());
    }
} // namespace spacedb
