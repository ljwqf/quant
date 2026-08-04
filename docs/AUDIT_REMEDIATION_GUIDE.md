# 整改修复指南 — quant

> 基于 `qa-security-audit` 审计结果整理，并记录 2026-07-23 / 2026-07-25 闭合修复。
>
> **2026-07-25 状态更新**：项目已解除冻结。`go test ./...` 全部通过。
>
> **2026-08-04 状态更新**：全目录审查发现 7/25 的「全部通过」存在隐藏墙钟前提——风控结算熔断窗口（23:55-00:05 等 4 个窗口，time.Now 判定）会使 12 个测试在窗口内必然失败。已修复：新增 risk.WithClock 选项并在 risk（33 处）/execution（26 处）测试构造点注入固定时钟；同日另完成 app.js 其余 22 处未转义 innerHTML/内联 onclick 的 XSS 修复。修复后 go test ./... 16/16 包通过，且经窗口内/外双向验证与运行时刻无关。详见 memory/audit_full_review_fixes_20260804.md。
> - TLS InsecureSkipVerify：实盘模式下强制启用 TLS 验证，仅模拟盘允许跳过。
> - Weak RNG：`bayesian_allocator.go` 已从 `math/rand` 迁移到 `crypto/rand` 种子的 ChaCha8。
> - 3 个失败测试已修复：`TestHandleMarketData_Candle`（handler key 大小写）、`TestRebalanceEntry*`（再平衡信号数量尊重策略推荐值）、`TestCircuitBreakerStateTransitions`（时序稳定）。

## 审计结论摘要

> 2026-07-23 复核更新：`web/static/js/app.js` 策略列表和策略参数面板的 DOM XSS 已修复；本文件下方列出的 logbackup 错误丢弃项也已在当前代码中修复。

- 当前开放 P0：无。
- 当前开放 P1：无。
- 项目状态：**已解除冻结**，`go test ./...` 全部通过（2026-07-25 验证）。
- 已修复 P0：`web/static/js/app.js` 策略列表、策略参数面板、策略操作 URL、运行时通知消息不再直接插入未转义服务端字段。
- 已修复 P0（2026-08-04 补充）：app.js 持仓/订单/信号/手动订单/手动持仓/限时单/条件单/再平衡事件/LLM 历史与结果渲染、错误提示等其余 22 处插值点全部改为 escapeHtml + data 属性事件绑定；closePosition/cancelManualOrder URL 参数加 encodeURIComponent。
- 旧 P1 logbackup 错误丢弃项：当前已修复，保留原描述仅作历史背景。
- `logbackup/cmd/server/main.go:100` ticker goroutine 仍按误报处理。

---

## P0 已修复项

### 1. `web/static/js/app.js:642,790` — DOM XSS

**问题**：策略列表与策略参数面板使用模板字符串拼接外部/服务端返回字段后赋给 `innerHTML`。典型字段包括策略名、最近信号、参数名、参数值、参数描述、错误消息。

**状态**：已修复。

**修复内容**：
- 策略名称、最近信号、交易次数、参数名、参数值、参数描述、错误消息统一经 `escapeHtml()` 后进入 `innerHTML`。
- 策略启动/停止和配置按钮移除内联 `onclick="...${s.name}..."` 字符串拼接，改为 `data-*` 属性加事件监听。
- 策略名进入 API URL 前使用 `encodeURIComponent()`。
- `showRuntimeNotice()` 对消息内容做 HTML 转义，并限制 toast type 到 `success/error/warning/info`。

**验证**：
- `node --check web/static/js/app.js`：通过。

**剩余项目级风险**：
- `go test ./...` 仍存在历史失败项，项目继续按冻结状态处理；本次 JS 修复不能代表全量 Go 测试通过。

---

## P1 建议

> 以下为历史增量审计项。2026-07-22 复核确认当前代码已处理这些错误返回值；不应继续作为开放 P1 统计。

### 1. `logbackup/cmd/server/main.go:82` — 处理 `os.MkdirAll` 错误

**问题**：第 82 行 `os.MkdirAll(*logDir, 0755)` 丢弃了返回值。如果目录创建失败，后续日志写入会静默失败。

**修复方案**：

```go
if err := os.MkdirAll(*logDir, 0755); err != nil {
    log.Fatalf("Failed to create log directory %s: %v", *logDir, err)
}
```

### 2. `logbackup/cmd/server/main.go:107/111` — 处理 `json.MarshalIndent` / `os.WriteFile` 错误

**问题**：stats 文件写入时 `data, _ := json.MarshalIndent(...)` 丢弃错误。如果序列化失败，`data` 为 nil，后续 `os.WriteFile` 写入空文件且无提示。同时 goroutine 内 `return` 会导致 stats 循环永久停止。

**修复方案：**

```go
data, err := json.MarshalIndent(stats, "", "  ")
if err != nil {
    log.Printf("[SERVER] Failed to marshal stats: %v", err)
    continue  // 跳过本次，保持 goroutine 存活
}
if err := os.WriteFile(*statsFile, data, 0644); err != nil {
    log.Printf("[SERVER] Failed to write stats: %v", err)
}
```

shutdown 路径改为：

```go
data, err := json.MarshalIndent(stats, "", "  ")
if err != nil {
    log.Printf("[SERVER] Failed to marshal stats on shutdown: %v", err)
} else {
    if err := os.WriteFile(*statsFile, data, 0644); err != nil {
        log.Printf("[SERVER] Failed to write stats on shutdown: %v", err)
    }
}
```

### 3. `logbackup/cmd/client/main.go:99/140` — 处理 `os.MkdirAll`/`os.WriteFile` 错误

**问题**：测试文件生成时丢弃错误。

**修复方案**：
- 检查并记录 `os.MkdirAll("./test_logs", 0755)` 返回值。
- 检查并记录 `os.WriteFile(name, []byte(header+content), 0644)` 返回值。

### 4. 误报项（无需修复）

| 位置 | 误报原因 |
|---|---|
| `logbackup/cmd/server/main.go:100` | 长生命周期 ticker goroutine，用于定期写 stats，是预期行为 |

---

## 总结

| 优先级 | 数量 | 性质 |
|---|---|---|
| P0 | 0 | 已验证开放项为 0；DOM XSS 已修复 |
| P1 | 0 | logbackup 错误处理项已修复 |
| 误报 | 1 | goroutine 场景误报 |
| 已修复 | 7 | DOM XSS；server MkdirAll/WriteFile；client MkdirAll/WriteFile |

---

## 已完成的修复

以下修复已在主分支（`D:\Project\Go_project\quant\logbackup`）中完成：

1. `logbackup/cmd/server/main.go:82` — `os.MkdirAll` 增加错误检查，失败时 `log.Fatalf`
2. `logbackup/cmd/server/main.go:108` — `json.MarshalIndent` 错误捕获，ticker 路径用 `continue` 保持 goroutine 存活
3. `logbackup/cmd/server/main.go:118` — `json.MarshalIndent` 错误捕获，shutdown 路径用 `else` 只在成功时写文件
4. `logbackup/cmd/server/main.go:109/122` — `os.WriteFile` 增加错误检查，失败时记录日志
5. `logbackup/cmd/client/main.go:99` — `os.MkdirAll` 增加错误检查，失败时 `log.Fatalf`
6. `logbackup/cmd/client/main.go:142` — `os.WriteFile` 增加错误检查，失败时记录日志

**验证结果**：`go build ./cmd/server/... ./cmd/client/...` 通过。

---

*本指南由 qa-security-audit 审计结果生成，仅供整改参考。*
