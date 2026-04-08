# 性能测试和基准测试指南

## 概述

本文档描述了量化交易系统的性能测试方法、基准测试和压力测试。

## 文件结构

```
tests/
├── benchmark_test.go          # 基准测试
├── stress_test.go             # 压力测试
├── performance_monitor.go     # 性能监控工具
└── test_helper.go             # 测试辅助
```

## 运行测试

### 运行基准测试

```bash
# 运行所有基准测试
go test -bench=. -benchmem ./tests

# 运行特定的基准测试
go test -bench=BenchmarkIndicator -benchmem ./tests

# 运行基准测试并生成CPU profile
go test -bench=. -benchmem -cpuprofile=cpu.prof ./tests

# 运行基准测试并生成内存profile
go test -bench=. -benchmem -memprofile=mem.prof ./tests
```

### 运行压力测试

```bash
# 运行所有压力测试
go test -v -run=TestStress ./tests

# 运行特定的压力测试
go test -v -run=TestStressIndicatorCalculations ./tests

# 跳过短时间测试（默认）
go test -v -run=TestStress ./tests

# 运行包括长时间压力测试
go test -v -run=TestStress -short=false ./tests
```

### 分析 Profile

```bash
# 分析CPU profile
go tool pprof cpu.prof

# 分析内存profile
go tool pprof mem.prof

# 在浏览器中查看profile
go tool pprof -http=:8080 cpu.prof
```

## 基准测试说明

### 技术指标基准测试

| 测试名称 | 描述 |
|---------|------|
| BenchmarkIndicatorMACD | MACD指标计算性能 |
| BenchmarkIndicatorRSI | RSI指标计算性能 |
| BenchmarkIndicatorBollingerBands | 布林带指标计算性能 |
| BenchmarkIndicatorATR | ATR指标计算性能 |
| BenchmarkIndicatorADX | ADX指标计算性能 |

### 风险引擎基准测试

| 测试名称 | 描述 |
|---------|------|
| BenchmarkRiskEngineCheck | 风险检查性能 |
| BenchmarkRiskEngineConcurrentChecks | 并发风险检查性能 |

### 策略基准测试

| 测试名称 | 描述 |
|---------|------|
| BenchmarkTrendFollowingGenerateSignals | 趋势跟踪策略信号生成 |
| BenchmarkMeanReversionGenerateSignals | 均值回归策略信号生成 |
| BenchmarkVolatilityBreakoutGenerateSignals | 波动率突破策略信号生成 |

### 并发基准测试

| 测试名称 | 描述 |
|---------|------|
| BenchmarkParallelIndicatorCalculations | 并行指标计算 |
| BenchmarkRiskEngineConcurrentChecks | 并发风险检查 |

## 压力测试说明

### 指标计算压力测试

- **TestStressIndicatorCalculations**: 100个并发goroutine，每个运行1000次指标计算
- 测试吞吐量和错误率

### 风险引擎压力测试

- **TestStressRiskEngine**: 50个并发goroutine，每个运行500次风险检查
- 使用多个交易对模拟真实场景

### 信号生成压力测试

- **TestStressConcurrentSignalGeneration**: 多个策略并发生成信号
- 测试总信号生成能力

### 内存压力测试

- **TestStressMemoryUsage**: 测试长时间运行后的内存增长
- 默认跳过，使用 `-short=false` 运行

### 超时压力测试

- **TestStressWithTimeout**: 30秒超时的持续压力测试
- 测试系统稳定性

## 性能监控工具

### PerformanceMetrics

用于跟踪操作的性能指标。

```go
metrics := tests.NewPerformanceMetrics()

// 开始计时
metrics.StartOperation("my-operation")

// 执行操作...

// 结束计时
metrics.EndOperation("my-operation")

// 或者直接记录
metrics.RecordOperation("my-operation", duration)

// 获取统计
count, avg, total := metrics.GetStats("my-operation")

// 打印报告
metrics.PrintReport()
```

### LatencyHistogram

用于统计延迟分布。

```go
histogram := tests.NewLatencyHistogram()

// 记录延迟
histogram.Record(duration)

// 获取统计
count, avg, p50, p95, p99 := histogram.GetStats()

// 打印报告
histogram.PrintReport()
```

### ThroughputMonitor

用于监控吞吐量。

```go
monitor := tests.NewThroughputMonitor(time.Second, 10)

// 记录操作
monitor.Record(1)

// 获取吞吐量
throughput := monitor.GetThroughput()

// 打印报告
monitor.PrintReport()
```

## 性能目标

### 技术指标计算

| 指标 | 目标 | 说明 |
|-----|------|------|
| MACD (1000数据点) | < 1ms | 12, 26, 9参数 |
| RSI (1000数据点) | < 0.5ms | 14周期 |
| 布林带 (1000数据点) | < 0.5ms | 20周期, 2标准差 |

### 风险检查

| 操作 | 目标 |
|-----|------|
| 单次风险检查 | < 0.1ms |
| 并发风险检查 (50并发) | < 0.5ms/次 |

### 信号生成

| 策略 | 目标 (1000数据点) |
|-----|-------------------|
| 趋势跟踪 | < 5ms |
| 均值回归 | < 3ms |
| 波动率突破 | < 4ms |

### 系统吞吐量

| 场景 | 目标 |
|-----|------|
| 指标计算 | > 10,000 ops/sec |
| 风险检查 | > 50,000 ops/sec |
| 信号生成 | > 2,000 signals/sec |

## 性能优化建议

### 已实现的优化

1. **缓存机制** - Redis和本地缓存
2. **模拟模式API限制** - 最多5次请求
3. **并发优化** - 使用sync.Pool等

### 建议的优化方向

1. **技术指标计算**
   - 使用SIMD指令优化
   - 预计算常用值
   - 增量计算

2. **数据处理**
   - 使用缓冲池减少分配
   - 批量处理而非单次处理
   - 异步处理非关键路径

3. **内存使用**
   - 对象复用
   - 减少临时对象
   - 使用更紧凑的数据结构

## 持续监控

建议在生产环境中监控以下指标：

1. **API响应时间** - P50, P95, P99
2. **吞吐量** - 每秒请求数
3. **错误率** - 失败请求比例
4. **资源使用** - CPU, 内存, 磁盘IO
5. **GC统计** - 垃圾回收频率和时间

## 常见问题

### Q: 基准测试结果不稳定怎么办？

A: 
- 使用 `-count=5` 运行多次取平均
- 在安静的系统上运行测试
- 关闭不必要的后台进程
- 使用更长的测试时间

### Q: 如何识别性能瓶颈？

A:
- 使用pprof分析CPU和内存profile
- 查看trace profile了解执行流程
- 使用benchstat比较不同版本的性能

### Q: 压力测试应该运行多久？

A:
- 快速验证: 10-30秒
- 标准测试: 1-5分钟
- 长时间稳定性: 30分钟以上
