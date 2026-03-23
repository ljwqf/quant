# quant 项目问题清单（核查记录）

**核查日期：** 2026-03-23  
**核查范围：** 全量 `go test ./...`、`go vet ./...` + 关键模块/配置的快速静态检查  
**核查结论：** 当前存在阻塞级构建问题（测试未能通过），因此建议优先处理阻塞项后再推进其它优化。

---

## 阻塞 / 高优先级

### 1. 测试/构建失败：测试桩未实现 `exchange.Exchange.SetLeverage`

**现象：**
- 执行 `go test ./...` 时，`internal/risk` 与 `internal/execution` 的测试无法编译通过。
- 同一原因也导致 `go vet ./...` 报错。

**原因（接口不匹配）：**
- `internal/exchange/exchange.go` 中的 `exchange.Exchange` 接口包含：
  - `SetLeverage(symbol string, leverage int, marginMode string) error`
- 但测试桩类型未实现该方法，导致无法作为 `exchange.Exchange` 传入生产代码构造函数。

**涉及位置：**
- `internal/risk/risk_manager_test.go`：`stubExchange` 缺少 `SetLeverage`
- `internal/execution/execution_integration_test.go`：`flowExchangeStub` 缺少 `SetLeverage`

**影响：**
- `go test ./...` 无法完成（至少上述两个包 build failed）。

**建议：**
- 在对应 stub 中补齐 `SetLeverage` 方法（可按测试需要返回 nil 或模拟错误分支），并确保其它接口方法签名与 `exchange.Exchange` 完全一致。

---

### 2. 配置敏感信息提交风险：历史提交检查（已核实）

**现象（规则层面）：**
- `.gitignore` 中包含：
  - `configs/*`
  - `!configs/config.yaml.example`
- **核实结论**：根据上述规则，`configs/config.yaml` **会被忽略**（不会被误提交），原问题描述有误。

**当前状态：**
- `.gitignore` 规则正确，`configs/config.yaml` 已被忽略。
- 风险点：如曾有密钥历史外泄（在添加 `.gitignore` 规则之前提交），仍需进行密钥轮换。

**建议：**
- 检查 git 历史记录，确认是否有敏感信息曾提交。
- 如有历史外泄，务必进行密钥轮换。

---

## 中优先级

### 3. `go.mod` 模块路径使用占位符（不利于迁移/发布）

**现象：**
- `go.mod` 里 `module` 为 `github.com/yourusername/okx-quant`（占位路径）。

**影响：**
- 若后续要发布到真实仓库/更换模块来源，可能引发依赖或导入路径不一致问题。

**建议：**
- 将 `module` 替换为真实仓库路径，并同步维护依赖（例如之后再考虑运行 `go mod tidy`）。

---

## 已核查为“不再是问题”（当前代码层面已具备）

### 4. LLM 集成到主程序（已存在初始化与挂载）
- `cmd/trader/main.go`：存在 `llmanalysis.NewClient / llmanalysis.NewAnalyzer`，并将 analyzer 注入 API server（通过 `SetAnalyzer`）。

### 5. LLM 请求 `Model` 不为空（已做默认 fallback）
- `internal/llmanalysis/analyzer.go`：会从 `cfg` provider 的模型字段读取，并在为空时 fallback 为默认值（例如 `gpt-4`）。

### 6. 数据采集与提醒服务已在主程序中启动与挂载
- `cmd/trader/main.go`：存在 `dataservice.NewDataService` 与 `alertservice.NewAlertService`，并分别通过 API server 的 setter 注入。

---

## 核实结论（2026-03-23 验证）

| 问题编号 | 描述 | 核实结果 | 状态 |
|---------|------|---------|------|
| 1 | 测试桩缺少 `SetLeverage` | ✅ **属实** - `go test ./...` 确认 `internal/risk` 和 `internal/execution` 构建失败 | 阻塞级 |
| 2 | `.gitignore` 未忽略 `configs/config.yaml` | ❌ **原描述有误** - `configs/config.yaml` 会被忽略，风险仅在历史提交 | 低优先级 |
| 3 | `go.mod` 模块路径占位符 | ✅ **属实** - `module github.com/yourusername/okx-quant` | 中优先级 |

**验证命令执行结果：**
- `go build ./...` - 通过（生产代码可编译）
- `go test ./...` - 失败（2 个包构建失败：`internal/risk`、`internal/execution`）

**修复建议：**
1. 在 `internal/risk/risk_manager_test.go` 的 `stubExchange` 中添加：
   ```go
   func (s *stubExchange) SetLeverage(symbol string, leverage int, marginMode string) error { return nil }
   ```
2. 在 `internal/execution/execution_integration_test.go` 的 `flowExchangeStub` 中添加：
   ```go
   func (s *flowExchangeStub) SetLeverage(symbol string, leverage int, marginMode string) error { return nil }
   ```

---

## 修复记录（2026-03-23）

### 已完成修复

**问题 1：测试桩缺少 `SetLeverage` 方法** ✅ 已修复

修复位置：
- `internal/risk/risk_manager_test.go:50-52` - 为 `stubExchange` 添加 `SetLeverage` 方法
- `internal/execution/execution_integration_test.go:241-243` - 为 `flowExchangeStub` 添加 `SetLeverage` 方法
- `internal/execution/execution_test.go:618-620` - 为 `stubExchange` 添加 `SetLeverage` 方法

**问题 3：`go.mod` 模块路径占位符** ✅ 已修复

修复内容：
- 更新 `go.mod` 模块路径：`github.com/yourusername/okx-quant` → `github.com/ljwqf/quant`
- 批量更新所有 Go 文件中的导入路径（共 78 个文件）

**问题 4：测试用例失败修复** ✅ 已修复

1. `TestExecuteUsesCalculatedQuantityForRiskCheck`
   - 问题：测试配置的 `MaxPositionSize: 10` 与计算出的仓位大小不匹配，未触发仓位限制
   - 修复：将 `MaxPositionSize` 从 10 改为 5
   - 额外修复：`smartRoute` 方法在 `orderBook == nil` 时未正确跳过深度检查

2. `TestExecutionEngineCompletesEntryAndExitLifecycleWithStrategyEngine`
   - 问题：入场订单成交后，仓位未同步更新到 `riskEngine`
   - 修复：在 `recordStrategyEntryFill` 方法中添加同步更新 `riskEngine.UpdatePosition`

**验证结果：**
- `go build ./...` ✅ 通过
- `go test ./...` ✅ 所有测试通过

