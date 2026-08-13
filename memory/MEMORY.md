> **Claude Code 项目记忆同步说明**
> 
> 本项目的完整 Claude Code 会话记忆同时保存在 C 盘客户端目录：
> 
> ```
> C:\Users\ljw\.claude\projects\D--Project-Go-project-quant\memory
> ```

# Memory Index

- [Cloud Servers](cloud_servers.md) — RackNerd (海外) and Tencent Cloud (国内) deployment details and SSH access
- [OKX Proxy](okx_proxy.md) — Why China servers need proxy_url for OKX API access
- [Session Status 2026-04-15](session_status_2026_04_15.md) — Work completed and remaining items as of last session
- [Session Status 2026-04-17](session_status_2026_04_17.md) — Contract quantity integer fix, deploy pending
- [Session Status 2026-04-18](session_status_2026_04_18.md) — 3 core fixes verified, deploy complete, docs written
- [Session Status 2026-04-19](session_status_2026_04_19.md) — Web panel PnL/uptime fix, simulation mode PnL calculation fix
- [Session Status 2026-04-20](session_status_2026_04_20.md) — 3 optimizations + nil-panic fix, deployed to Tencent Cloud
- [Session Status 2026-04-21](session_status_2026_04_21.md) — CRITICAL + HIGH issues from production viability assessment, all fixed and deployed
- [Session Status 2026-04-22](session_status_2026_04_22.md) — Web panel regression fix: server.go nil panic, app.js fake data removal, handleSystemStatus broken WS fields
- [Session Status 2026-04-23](session_status_2026_04_23.md) — LiquidityHuntEngine concurrent map panic fix + Web panel zero-value date fix, deployed
- [Session Status 2026-04-26](session_status_2026_04_26.md) — V2 execution pipeline completed (SignalRouter, MakerFirstExecutor, ProfitPool), KlineStream + proxy wiring
- [Syslog Backup](syslog_backup.md) — Tencent Cloud rsyslog remote log backup config (TCP/UDP port 8514) + Windows UDP sending gotchas
- [Audit Remediation 2026-07-08](audit_remediation_session_20260708.md) — Codex + Claude Code 双重审核: quant 错误处理修复（server json.MarshalIndent + client WriteFile），全部 build 通过
- [Audit Closure 2026-07-23](audit_closure_20260723.md) — 全量审计闭合复核：DOM XSS 已修复并通过 JS 语法检查，`go test ./...` 当前仍失败，项目继续冻结
- [Full-Review Fixes 2026-08-04](audit_full_review_fixes_20260804.md) — Web 面板 XSS 全面修复（22 处）+ 墙钟依赖测试修复（WithClock 注入）；全量测试恢复稳定通过，冻结状态解除
- [Audit Commit & Push 2026-08-05](audit_commit_push_20260805.md) — 遗留代码提交（534f768/a21d29e）+ 分支推送 ljwqf/quant（gh 切换）；C 类部署暂缓（quant-v2 为活跃项目）
