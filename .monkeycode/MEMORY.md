# 用户指令记忆

本文件记录了用户的指令、偏好和教导，用于在未来的交互中提供参考。

## 格式

### 用户指令条目
用户指令条目应遵循以下格式：

[用户指令摘要]
- Date: [YYYY-MM-DD]
- Context: [提及的场景或时间]
- Instructions:
  - [用户教导或指示的内容，逐行描述]

### 项目知识条目
Agent 在任务执行过程中发现的条目应遵循以下格式：

[项目知识摘要]
- Date: [YYYY-MM-DD]
- Context: Agent 在执行 [具体任务描述] 时发现
- Category: [代码结构|代码模式|代码生成|构建方法|测试方法|依赖关系|环境配置]
- Instructions:
  - [具体的知识点，逐行描述]

## 去重策略
- 添加新条目前，检查是否存在相似或相同的指令
- 若发现重复，跳过新条目或与已有条目合并
- 合并时，更新上下文或日期信息
- 这有助于避免冗余条目，保持记忆文件整洁

## 条目

[评估任务不生产代码]
- Date: 2026-05-07
- Context: 用户请求评估当前项目以及后续实施方案
- Instructions:
  - 当用户明确要求“不需要生产代码”时，仅做项目评估、方案设计和实施建议，不修改业务代码。

[项目当前处于 V1 到 V2 五层架构重构阶段]
- Date: 2026-05-07
- Context: Agent 在执行当前项目评估和实施方案梳理时发现
- Category: 代码结构
- Instructions:
  - 项目是 Go 编写的 OKX 量化交易系统，V1 已完成核心交易、风控、Web 面板、监控、通知、回测等能力。
  - 当前主要技术方向是按 `docs/V2_ARCHITECTURE_PLAN.md` 推进 V2 五层架构：ingestion、computation、decision、execution、monitor。
  - V2 目录当前尚未落地，实施应优先新增 `internal/v2/` 骨架、事件类型、接口和 V1 防腐适配器，再逐层接入。

[项目构建与测试命令]
- Date: 2026-05-07
- Context: Agent 在执行当前项目评估和实施方案梳理时发现
- Category: 构建方法
- Instructions:
  - 本地构建入口为 `go build ./cmd/trader` 或 Makefile 的 `make build`，主程序入口在 `cmd/trader/main.go`。
  - 测试命令为 `go test ./... -v`，CI 使用 `go test -race -coverprofile=coverage.out ./...`、`go vet ./...`、`gofmt` 检查和 golangci-lint。
  - Makefile 的 `deps` 会执行 `go mod tidy`，在只评估项目时不需要运行。
