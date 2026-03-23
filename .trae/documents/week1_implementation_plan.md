# Week 1 实施计划 - 数据库与基础架构

## \[ ] Task 1: 添加 SQLite 依赖

* **Priority**: P0

* **Depends On**: None

* **Description**:

  * 在 `go.mod` 中添加 SQLite 驱动依赖

  * 使用 modernc.org/sqlite（纯Go实现，无需CGO）

* **Success Criteria**:

  * 依赖成功添加到 go.mod

  * `go mod tidy` 执行成功

* **Test Requirements**:

  * `programmatic` TR-1.1: 执行 `go mod tidy` 无错误

  * `programmatic` TR-1.2: 依赖项正确显示在 go.mod 中

***

## \[ ] Task 2: 更新配置管理 (config.go)

* **Priority**: P0

* **Depends On**: None

* **Description**:

  * 添加 `DatabaseConfig` 结构体

  * 添加 `ManualTradingConfig` 结构体

  * 添加 `LLMConfig` 结构体（支持多提供商）

  * 添加 `DataCollectorConfig` 结构体

  * 添加 `AlertConfig` 结构体

  * 更新 `Config` 主结构体包含新配置

  * 添加配置验证逻辑

* **Success Criteria**:

  * 新配置结构体正确定义

  * 配置验证逻辑完整

  * 与现有配置结构兼容

* **Test Requirements**:

  * `programmatic` TR-2.1: `go build` 编译成功

  * `programmatic` TR-2.2: 配置结构体字段可正确序列化

  * `human-judgement` TR-2.3: 代码风格与现有代码一致

***

## \[ ] Task 3: 更新 config.yaml 配置文件

* **Priority**: P0

* **Depends On**: Task 2

* **Description**:

  * 添加 `database` 配置段

  * 添加 `manual_trading` 配置段

  * 添加 `llm` 配置段（含 openai、claude、qwen 三个提供商）

  * 添加 `data_collector` 配置段

  * 添加 `alert` 配置段

* **Success Criteria**:

  * 配置文件包含所有新配置项

  * YAML 格式正确

  * 有合理的默认值

* **Test Requirements**:

  * `programmatic` TR-3.1: YAML 文件语法验证通过

  * `human-judgement` TR-3.2: 配置结构清晰合理

***

## \[ ] Task 4: 创建数据存储模块基础结构

* **Priority**: P0

* **Depends On**: Task 1

* **Description**:

  * 创建 `internal/storage/` 目录

  * 创建 `internal/storage/repository/` 目录

  * 创建 `internal/storage/database.go` - 数据库管理主文件

  * 创建 `internal/storage/migrations.go` - 数据库迁移脚本

* **Success Criteria**:

  * 目录结构创建成功

  * 基础文件创建成功

* **Test Requirements**:

  * `programmatic` TR-4.1: 目录和文件存在

  * `human-judgement` TR-4.2: 代码结构合理

***

## \[ ] Task 5: 实现数据库管理 (database.go)

* **Priority**: P0

* **Depends On**: Task 4

* **Description**:

  * 定义 `Database` 结构体

  * 实现 `NewDatabase(cfg *config.DatabaseConfig) (*Database, error)` 构造函数

  * 实现 `Open()` 方法 - 打开数据库连接

  * 实现 `Close()` 方法 - 关闭数据库连接

  * 实现 `Migrate()` 方法 - 执行数据库迁移

  * 实现连接池配置

* **Success Criteria**:

  * 数据库能够成功打开和关闭

  * 连接池配置合理

* **Test Requirements**:

  * `programmatic` TR-5.1: 能够成功创建 SQLite 数据库文件

  * `programmatic` TR-5.2: 能够正常打开和关闭连接

  * `programmatic` TR-5.3: `go test` 通过（创建基础测试）

***

## \[ ] Task 6: 实现数据库迁移 (migrations.go)

* **Priority**: P0

* **Depends On**: Task 5

* **Description**:

  * 定义迁移版本列表

  * 实现迁移 001: 创建 `manual_trades` 表

  * 实现迁移 002: 创建 `ai_analyses` 表

  * 实现迁移 003: 创建 `news_events` 表

  * 实现迁移 004: 创建 `economic_events` 表

  * 实现迁移 005: 创建 `alert_records` 表

  * 实现迁移执行逻辑

  * 实现迁移版本管理

* **Success Criteria**:

  * 所有表成功创建

  * 索引正确创建

  * 迁移版本管理正常工作

* **Test Requirements**:

  * `programmatic` TR-6.1: 执行迁移后所有表存在

  * `programmatic` TR-6.2: 表结构符合 spec 文档定义

  * `programmatic` TR-6.3: 索引正确创建

***

## \[ ] Task 7: 定义数据模型

* **Priority**: P0

* **Depends On**: Task 6

* **Description**:

  * 创建 `internal/storage/models.go`

  * 定义 `ManualTrade` 结构体

  * 定义 `AIAnalysis` 结构体

  * 定义 `NewsEvent` 结构体

  * 定义 `EconomicEvent` 结构体

  * 定义 `AlertRecord` 结构体

  * 实现必要的辅助方法

* **Success Criteria**:

  * 所有模型结构体定义完整

  * 字段类型与数据库表匹配

* **Test Requirements**:

  * `programmatic` TR-7.1: `go build` 编译成功

  * `human-judgement` TR-7.2: 模型定义清晰，注释完整

***

## \[ ] Task 8: 实现手动交易数据访问层 (manual\_trade\_repo.go)

* **Priority**: P0

* **Depends On**: Task 7

* **Description**:

  * 创建 `internal/storage/repository/manual_trade_repo.go`

  * 定义 `ManualTradeRepository` 接口

  * 实现 `Create(trade *models.ManualTrade) error`

  * 实现 `GetByID(id int64) (*models.ManualTrade, error)`

  * 实现 `GetByOrderID(orderID string) (*models.ManualTrade, error)`

  * 实现 `List(symbol string, limit, offset int) ([]*models.ManualTrade, error)`

  * 实现 `Update(trade *models.ManualTrade) error`

  * 实现 `Delete(id int64) error`

* **Success Criteria**:

  * CRUD操作完整实现

  * 错误处理完善

* **Test Requirements**:

  * `programmatic` TR-8.1: 所有CRUD操作正常工作

  * `programmatic` TR-8.2: 单元测试覆盖率 ≥ 80%

***

## \[ ] Task 9: 实现 AI 分析数据访问层 (ai\_analysis\_repo.go)

* **Priority**: P0

* **Depends On**: Task 7

* **Description**:

  * 创建 `internal/storage/repository/ai_analysis_repo.go`

  * 定义 `AIAnalysisRepository` 接口

  * 实现 `Create(analysis *models.AIAnalysis) error`

  * 实现 `GetByID(id int64) (*models.AIAnalysis, error)`

  * 实现 `ListBySymbol(symbol string, limit, offset int) ([]*models.AIAnalysis, error)`

  * 实现 `ListByType(analysisType string, limit, offset int) ([]*models.AIAnalysis, error)`

  * 实现 `GetLatestBySymbolAndType(symbol, analysisType string) (*models.AIAnalysis, error)`

* **Success Criteria**:

  * 所有操作完整实现

  * 查询方法按需求实现

* **Test Requirements**:

  * `programmatic` TR-9.1: 所有方法正常工作

  * `programmatic` TR-9.2: 单元测试覆盖率 ≥ 80%

***

## \[ ] Task 10: 实现新闻事件数据访问层 (news\_event\_repo.go)

* **Priority**: P1

* **Depends On**: Task 7

* **Description**:

  * 创建 `internal/storage/repository/news_event_repo.go`

  * 定义 `NewsEventRepository` 接口

  * 实现 `Create(event *models.NewsEvent) error`

  * 实现 `GetByID(id int64) (*models.NewsEvent, error)`

  * 实现 `GetByExternalID(externalID string) (*models.NewsEvent, error)`

  * 实现 `List(limit, offset int) ([]*models.NewsEvent, error)`

  * 实现 `ListByImportance(minImportance int, limit, offset int) ([]*models.NewsEvent, error)`

  * 实现 `ListByTimeRange(start, end time.Time, limit, offset int) ([]*models.NewsEvent, error)`

  * 实现 `Upsert(event *models.NewsEvent) error`

* **Success Criteria**:

  * 所有操作完整实现

  * Upsert方法正确处理重复

* **Test Requirements**:

  * `programmatic` TR-10.1: 所有方法正常工作

  * `programmatic` TR-10.2: 单元测试覆盖率 ≥ 70%

***

## \[ ] Task 11: 实现经济事件和提醒记录数据访问层

* **Priority**: P1

* **Depends On**: Task 7

* **Description**:

  * 创建 `internal/storage/repository/economic_event_repo.go` - 经济事件Repository

  * 创建 `internal/storage/repository/alert_record_repo.go` - 提醒记录Repository

  * 实现基本CRUD操作

* **Success Criteria**:

  * 两个Repository都实现完成

* **Test Requirements**:

  * `programmatic` TR-11.1: 所有方法正常工作

  * `programmatic` TR-11.2: 单元测试覆盖率 ≥ 70%

***

## \[ ] Task 12: 创建手动交易模块基础结构

* **Priority**: P0

* **Depends On**: Task 8

* **Description**:

  * 创建 `internal/manualtrading/` 目录

  * 创建 `internal/manualtrading/manager.go` - 管理器主入口

  * 创建 `internal/manualtrading/order_manager.go` - 订单管理

  * 创建 `internal/manualtrading/position_manager.go` - 持仓管理

  * 创建 `internal/manualtrading/market_data.go` - 市场数据

  * 定义基础接口和结构体

* **Success Criteria**:

  * 目录和文件创建成功

  * 基础接口定义完整

* **Test Requirements**:

  * `programmatic` TR-12.1: `go build` 编译成功

  * `human-judgement` TR-12.2: 代码结构合理

***

## \[ ] Task 13: 集成数据库到主程序 (main.go)

* **Priority**: P0

* **Depends On**: Task 5, Task 12

* **Description**:

  * 在 `cmd/trader/main.go` 中初始化数据库

  * 执行数据库迁移

  * 初始化手动交易管理器

  * 确保优雅关闭时关闭数据库连接

* **Success Criteria**:

  * 数据库成功初始化

  * 迁移成功执行

  * 系统正常启动和关闭

* **Test Requirements**:

  * `programmatic` TR-13.1: 程序能够正常启动

  * `programmatic` TR-13.2: 数据库文件被创建

  * `programmatic` TR-13.3: 优雅关闭正常工作

***

## \[ ] Task 14: 集成测试和验证

* **Priority**: P0

* **Depends On**: All previous tasks

* **Description**:

  * 运行完整编译测试

  * 运行所有单元测试

  * 手动测试系统启动

  * 验证数据库功能

* **Success Criteria**:

  * 所有测试通过

  * 系统正常运行

* **Test Requirements**:

  * `programmatic` TR-14.1: `go build ./...` 无错误

  * `programmatic` TR-14.2: `go test ./...` 所有测试通过

  * `programmatic` TR-14.3: 系统启动后数据库表完整创建

