# SpaceDB

SQL 数据库,用于学习数据库核心原理:SQL 解析、执行计划、存储引擎与事务。

## 当前功能

- **完整 SQL 管线**:`lexer → parser → planner → executor → engine → storage`
- **DDL**:`CREATE TABLE`(列类型、NOT NULL、DEFAULT 默认值)
- **DML**:`INSERT`(支持指定列名、自动重排、默认值填充)、`SELECT *`(全表扫描)
- **存储层**:内存 KV 引擎,带读写锁的 MVCC 事务封装
- **Key 编码**:二进制命名空间前缀(`0x01` 表元数据 / `0x02` 行数据 + `0x00` 分隔符),类型化、保序的主键编码(整数/浮点按大小端序排序,`ScanPrefix` 可正确分表扫描)

## 快速开始

```bash
# 构建
go build ./...

# 运行全部测试
go test ./...

# 静态检查
go vet ./...

# 命令行(当前仅解析 SQL 并输出 AST)
go run ./cmd/spacedb "SELECT * FROM users;"
```

## 目录结构

```text
cmd/spacedb/      命令行入口(当前为 Parser 冒烟命令)
lexer/            词法分析:SQL 文本 → Token
parser/           AST 定义与递归下降语法分析
planner/          SQL → 执行计划节点(归一化、类型检查)
executor/         计划节点 → 执行器(创建表 / 插入 / 扫描)
engine/           SQL 引擎门面:Session、KVTransaction、key 编码
storage/          底层存储:Engine 接口、内存引擎、MVCC 事务
schema/           表结构定义
types/            运行时值类型(Value / DataType)
Docs/             架构与学习文档(HTML 版见 Docs/html/)
```

## 架构分层

```text
Session.Execute(SQL)
        │
        ▼
┌─────────────────────────────────────────┐
│  lexer → parser → planner → executor    │   SQL 管线(与存储解耦)
└─────────────────────────────────────────┘
        │ executor.Transaction 接口
        ▼
┌─────────────────────────────────────────┐
│  engine:KVTransaction                   │   key 编码 / 值编解码 / 校验
└─────────────────────────────────────────┘
        │ storage.Engine 接口
        ▼
┌─────────────────────────────────────────┐
│  storage:MVCC(读写锁)                   │   事务层
│  storage:MemoryEngine(map)              │   物理存储
└─────────────────────────────────────────┘
```

设计要点:

- **分层解耦**:executor 只依赖 `executor.Transaction` 接口,不感知存储实现;storage 只提供字节级 KV 能力,不感知 SQL。
- **行数据**:整行以 JSON 序列化后作为 value 存储;key 为二进制编码,保证排序与前缀扫描的正确性。
- **事务**:每个 `Session.Execute` 开启一个新事务,存储层通过读写锁串行化访问(提交/回滚暂为空实现)。

## 当前限制与路线

- 尚未实现:`WHERE` 过滤、聚合、`JOIN`、索引、磁盘持久化、网络服务
- 行键暂用**第一列**充当主键(TODO),主键约束与 `PRIMARY KEY` 声明待实现
- 事务目前仅提供读写锁隔离,多版本可见性与写冲突检测待实现
- 数值主键的保序编码已就绪,可直接支撑未来的范围扫描
