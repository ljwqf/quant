---
name: session status 2026-04-17 updated
description: Work completed and current blockers as of 2026-04-17 evening session
type: project
originSessionId: 0b2ce4c0-01c4-4825-ab95-2e8ec22abd14
---
## 今日工作 (2026-04-17 晚)

### 1. 部署最新二进制到 RackNerd
- 构建命令：`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o trader ./cmd/trader/`
- 上传路径：本地 → scp → 腾讯云(132.232.231.41) → scp → RackNerd(23.95.165.223)
- 跳板方式：腾讯云上保存了 SSH key 在 `/tmp/racknerd_key`
- MD5: `e957a09f87a3ddf90290f3c9666ef519`（已验证一致）

### 2. OKX 51010 错误根因诊断
**问题**: 所有 SWAP 合约下单返回 `51010 - You can't complete this request under your current account mode`

**诊断结果**:
- 账户等级: `acctLv=1` (Simple Trading Mode / 简单交易模式)
- 该模式不支持 `tdMode=cross` 或 `isolated` 参数
- `tdMode` 参数不能为空（报错 50014）
- API 无法升级账户等级（报错 51070）

**解决方案**: 用户需要手动在 OKX 网页/APP 上升级账户模式
1. 登录 OKX → 交易 → 模拟盘
2. 将账户从"简单交易模式"升级为"单币种保证金模式"或"多币种保证金模式"
3. 升级后代码中的 `tdMode=cross` 即可正常工作

### 3. 代码修复（已提交）
- `internal/exchange/okx/order.go`: SWAP 合约使用 `tdMode=cross`（之前错误地用了 `cash`）
- `internal/exchange/okx/order.go`: cross/isolated 模式需要发送 lever 参数
- `internal/execution/execution.go`: 模拟盘跳过订单簿深度检查
- `internal/risk/risk_engine.go`: 模拟盘流动性不足改为警告通过
- 提交 hash: `b587890`

### 4. 已解决的历史问题
- allocationAmount: 0 → 已修复（回退到 TotalEquity）
- 合约数量取整 → 已修复（math.Round + int64 + 最小 1 张保护）
- 现货流动性不足 → 已修复（模拟盘模式下通过）
- RackNerd 部署路径 → 已打通（通过腾讯云跳板）
- getPositions 405 错误 → 之前已修复（POST → GET）

### 5. 待完成
- **[阻塞]** OKX 账户升级到保证金模式（需用户在 OKX UI 手动操作）
- 升级后重新部署，验证 TestBuySellStrategy 完整买卖循环
- 验证现货下单（BTC-USDT cash 模式也需要账户支持）

## 服务器信息
| 服务器 | IP | SSH 端口 | 用途 |
|--------|-----|---------|------|
| RackNerd | 23.95.165.223 | 内网 22 | 交易程序主运行 |
| 腾讯云 | 132.232.231.41 | 22 | 部署跳板 |

SSH key: `C:\Users\ljw\.ssh\racknerd_key`
腾讯云上 key 路径: `/tmp/racknerd_key`

## API 凭证（模拟盘）
- API Key: `f37e34fd-d351-4fe9-8a9e-60badd32ef07`
- Secret: `E547F1D0D6E7E5EDA27A60E1DD10E7DC`
- Passphrase: `Ljwnb666@okb`
