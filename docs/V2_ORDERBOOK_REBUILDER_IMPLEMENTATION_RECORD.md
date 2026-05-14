# V2 OrderBook 重建实施记录

**日期**：2026-05-14

## 目标

为 V2 ingestion 层补齐 OrderBook snapshot/增量重建能力，提升流动性因子计算前的数据完整性与可信度。

## 实施范围

### OrderBook 重建器

- 新增 `OrderBookRebuilder`，按 symbol 维护本地 bids/asks 状态。
- 支持 snapshot 初始化与重新同步。
- 支持增量更新档位。
- 数量小于等于 0 的档位会从本地簿中移除。
- 输出 bids 降序、asks 升序，并按配置 depth 裁剪。

### 序列与 Checksum 校验

- `OrderBookEvent` 增加 `Snapshot` 字段，用于区分快照和增量事件。
- 增量事件校验 sequence 连续性，发现 gap 时重置重建状态并丢弃该事件。
- 基于前 25 档交错拼接价格与数量，计算 CRC32 checksum。
- 当上游提供 checksum 且校验失败时，重置重建状态并丢弃该事件。

### Ingestion 接入

- `IngestionManager` 为每个 symbol 创建一个 `OrderBookRebuilder`。
- 下游 V2 pipeline 接收重建与校验后的 OrderBook 事件。
- V1 exchange adapter 转换来的 OrderBook 事件标记为 snapshot，匹配当前 V1 OKX `books5` 推送语义。

## 文件变更

| 文件 | 说明 |
|------|------|
| `internal/v2/ingestion/orderbook_rebuilder.go` | 新增 OrderBook 重建器、sequence 校验和 checksum 计算 |
| `internal/v2/ingestion/orderbook_rebuilder_test.go` | 新增 snapshot、增量、删除、sequence gap、checksum 测试 |
| `internal/v2/ingestion/ingestion.go` | 接入每个 symbol 的 OrderBook 重建器 |
| `internal/v2/events/events.go` | 为 OrderBookEvent 增加 Snapshot 字段 |
| `internal/v2/adapter/exchange_adapter.go` | V1 转换事件标记为 snapshot |

## 验证结果

```bash
# 运行 V2 包测试
go test ./internal/v2/...

# 运行 API 与 V2 相关测试
go test ./internal/api ./internal/v2/...

# 全量构建
go build ./...

# 静态检查
go vet ./...
```

以上命令均已通过。

## 当前状态

- V2 ingestion 已具备 OrderBook snapshot/增量重建、sequence gap 检测和 checksum 校验能力。
- 当前 V1 适配路径以 snapshot 方式输入 V2，后续接入原生增量行情源时可直接使用 `Snapshot=false` 的事件路径。
