# V2 Phase 6 Web/API 接入实施记录

**日期**：2026-05-14

## 目标

为 V2 流动性策略增加独立 Web/API 接入层，在保留 V1 主面板和交易链路的基础上，提供 V2 pipeline 的只读观测入口。

## 实施范围

### API 防腐层

- 在 `internal/api/server.go` 新增 `V2PipelineStatus` 接口，仅暴露 `Status() map[string]interface{}`。
- 在 `Server` 中增加 `v2Pipeline` 引用，通过 `SetV2Pipeline()` 注入，避免 API 层直接依赖 V2 决策层具体类型。
- 新增 V2 路由：
  - `GET /api/v2/status`：返回 V2 pipeline 总体状态。
  - `GET /api/v2/liquidity?symbol=BTC-USDT`：返回指定交易对的 V2 流动性策略摘要。
  - `GET /v2.html`：返回 V2 独立面板页面。

### Pipeline 注入

- 在 `cmd/trader/main.go` 为 `v2PipelineHandle` 增加 `Status()` 方法。
- V2 pipeline 启动成功后调用 `apiServer.SetV2Pipeline(v2Pipeline)`，使 Web/API 层可读取 pipeline 运行态。
- `--mode=v1` 且未启用 V2 时，V2 API 返回 `enabled=false`，保证观测端口稳定。

### V2 Web 面板

- 新增 `web/v2.html`，提供独立 V2 流动性策略面板。
- 新增 `web/static/js/v2-app.js`，每 5 秒轮询 `/api/v2/status`。
- 面板展示内容：
  - pipeline 启用状态、运行模式、运行状态、只读风控状态。
  - 基础资金、利润子弹池、模拟订单数、模拟盈亏。
  - 各交易对状态机状态。

### 测试与质量修复

- 在 `internal/api/server_test.go` 增加 V2 status API 测试：
  - pipeline 未注入时返回 `enabled=false`。
  - pipeline 注入后返回状态机和 `enabled=true`。
- 修复 `internal/v2/execution/risk_guard.go` 中 `go vet` 发现的自赋值问题。

## 文件变更

| 文件 | 说明 |
|------|------|
| `internal/api/server.go` | 增加 V2 pipeline 注入接口、V2 API 和面板路由 |
| `cmd/trader/main.go` | 向 API server 注入 V2 pipeline handle |
| `web/v2.html` | 新增 V2 独立 Web 面板 |
| `web/static/js/v2-app.js` | 新增 V2 面板前端轮询逻辑 |
| `internal/api/server_test.go` | 新增 V2 status API 测试 |
| `internal/v2/execution/risk_guard.go` | 清理 vet 报告的自赋值代码 |

## 验证结果

```bash
# 运行 API 与 V2 相关测试
go test ./internal/api ./internal/v2/...

# 全量构建
go build ./...

# 静态检查
go vet ./...
```

以上命令均已通过。

## 当前状态

- V2 Web/API 已具备只读观测能力。
- V1 主面板和原有 API 路由保持兼容。
- V2 面板数据来自 `FullPipeline.Status()`，当前覆盖 pipeline、风控、资金池、模拟盘和状态机摘要。

## 后续建议

- 在 V2 pipeline 中补充更细粒度的因子输出，让面板展示流动性墙、真空区、OBI、Delta Ratio 等核心因子。
- 为 `/api/v2/liquidity` 增加结构化响应类型，减少前端对 map key 的依赖。
- 在模拟盘灰度阶段补充信号快照详情页，用于复盘 missed signal 和 shadow signal。
