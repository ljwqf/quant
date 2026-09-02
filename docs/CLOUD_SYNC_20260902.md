# 云端同步与审查记录（2026-09-02）

## 分支同步

- 云端默认分支：`master`，同步前云端提交：`d2840207eede4c9f7f9d0211655fb0b34ec063d2`。
- 本地旧 `master` 为独立初始历史 `3f41a3c9dfdd028b92e341086ba1f51b815f9b3c`，已在工作区外保存安全引用和 bundle；本分支以云端 `master` 为基线。
- 本次审查分支：`codex/audit-fix-20260902`。

## 审查发现与修复

`internal/v2/decision/ShadowPipeline.Close` 原先只调用 `Flush`，没有关闭 `MissedSignalLogger`。Windows 临时目录清理时会留下 `missed_signals.json` 文件句柄，导致 `TestShadowPipelineProcessesOrderBookAndLogsSignal` 间歇失败。

修复为同时关闭 missed-signal 与 shadow 日志文件，并合并两个关闭错误：

```go
return errors.Join(p.missLogger.Close(), p.shadowLog.Close())
```

## 验证

- `gofmt -w internal/v2/decision/shadow_pipeline.go`：退出码 0
- `go test ./internal/v2/decision -run TestShadowPipelineProcessesOrderBookAndLogsSignal -count=5`：退出码 0
- `go test ./internal/v2/decision`：退出码 0
- `go test ./...`：退出码 0
- `git diff --check`：退出码 0

## 回滚

工作区同步报告中的 `pre-sync-backup/quant` bundle 与 `codex/backup-cloud-sync-20260902/quant-master` 保存原始指针。回滚本地修复可将 `master` 指回该安全引用；远端回滚使用带租约的显式 ref 更新，并先核对当前远端 SHA。
