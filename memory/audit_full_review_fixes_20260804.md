# 全目录审查修复记录 2026-08-04

> 背景：Codex 对 D:\Project\Go_project 全目录项目审查（排除 ENScan_GO / ICP_Query / RustScan / xquant-learning），quant 侧确认并修复两类问题。
> 上级报告：`D:\Project\Go_project\.audit-20260803\REPORT.md`

## 1. Web 面板 XSS 修复（web/static/js/app.js）

- 问题：多处渲染路径把服务端/LLM 数据原样拼入 innerHTML（escapeHtml 仅部分使用），且存在
  `onclick="fn('${serverData}')"` 内联事件与未编码的 URL 路径参数 → 存储/反射型 XSS。
- 覆盖范围（22 处替换）：持仓/订单/信号表格、手动订单、手动持仓、限时单、条件单、
  再平衡事件、LLM 历史与 renderLLMResult（summary/recommendation）、5 处 `${error.message}`、
  showGlobalLoading、closePosition/cancelManualOrder 的 URL 参数。
- 修法：全部动态插值 escapeHtml()；内联 onclick 改 data-* 属性 + addEventListener
  （data-position-action / data-manual-action / data-manual-position / data-timed-action /
  data-cond-action / data-llm-toggle）；路径参数 encodeURIComponent。
- 验证：node --check 通过；残留扫描仅剩字面量三元表达式。

## 2. 墙钟依赖测试修复（internal/risk + internal/execution）

- 问题：风险引擎硬编码结算熔断窗口（00:55-01:05 / 07:55-08:05 / 15:55-16:05 / 23:55-00:05），
  用 time.Now() 判定；测试未注入时钟 → 每天 23:55-00:05 窗口内 12 个测试必失败
  （2026-08-04 00:05 现场复现：risk 3 个 + execution 9 个，报"市场已关闭"）。
- 修法：
  - `risk.NewEngine` 新增 `WithClock(now func() time.Time)` 选项；`lastResetTime` 改为
    在 opts 应用后取 `e.nowFunc()`（生产默认行为不变）。
  - 新增 `internal/risk/risk_testclock_test.go`、`internal/execution/execution_testclock_test.go`，
    固定时钟 = 2026-01-01 12:00 UTC（远离所有熔断窗口）。
  - risk 三个测试文件 33 处、execution 三个测试文件 26 处构造点全部注入。
- 验证：固定时钟拨到 23:57 → 测试确定性失败；拨回正午 → 通过（证明注入生效且结果与运行时刻无关）；
  `go test ./...` 全量 16 包 0 失败。

## 状态

- 2026-07-23 审计闭合记录中"go test ./... 当前仍失败，项目继续冻结"的状态已解除：
  截至 2026-08-04 全量测试稳定通过（含时间窗口内外的双向验证）。
- 历史审计 P0 中涉及 quant 的其余发现（configs 密钥、app.js 早期行号）核实为误报或已被后续迭代修复。
