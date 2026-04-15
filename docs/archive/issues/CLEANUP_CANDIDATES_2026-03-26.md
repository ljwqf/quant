# Cleanup Candidates (2026-03-26)

> 目的：梳理“可能无用”文件/文档，供人工决策是否删除。
> 说明：本清单不自动删除任何文件。

## A. 高置信可删（构建/测试产物）

1. 根目录覆盖率产物（一次性文件）
- `coverage_execution`
- `coverage_risk`
- `coverage_strategy`
- `coverage_execution.out`
- `coverage_risk.out`
- `coverage_risk_after.out`
- `coverage_strategy.out`
- `coverage_strategy_after.out`

备注：这些是测试输出，通常不应入库；当前仓库只跟踪了 `coverage`，其余多为未跟踪临时文件。

2. 根目录可执行文件（本地编译产物）
- `trader.exe`
- `trader_new.exe`

备注：`.gitignore` 已忽略 `*.exe`，建议清理本地产物避免误用旧二进制。

3. 空目录（如不作为约定位）
- `docs/`
- `scripts/`

备注：目前为空，可删除；若未来要保留结构，建议放 `.gitkeep` 并在 README 说明用途。

## B. 建议归档/下沉（非代码运行必需）

1. 审计/过程文档（仅管理用途）
- `VIBE_CONTROL_GUARDIAN_REPORT.md`
- `ISSUES.md`
- `CLEANUP_CANDIDATES_2026-03-26.md`（本文件）

备注：建议迁移到 `docs/audit/` 或 `docs/reports/`，减少根目录噪音。

2. 规范文档
- `spec/SPEC.md`
- `spec/TASKS.md`
- `spec/checklist.md`

备注：内容有效，建议保留；其中已注明合并来源（旧文档已删除或待删除），无需再拆分维护。

## C. 待确认后再删（可能涉及发布流程）

1. 已跟踪的二进制/发布包
- `okx-quant`（根目录二进制）
- `release/okx-quant-1.0.0/`（完整历史发布包，当前仍被 git 跟踪）

备注：从仓库清洁角度倾向删除或迁移到 Release Asset/制品库；但是否保留取决于团队发布策略。

2. 本地未跟踪发布目录
- `release/linux/`
- `release/windows/`
- `release/README.txt`

备注：更像本地打包输出（未跟踪）。若只是临时构建，建议本地清理并在 `.gitignore` 增加规则（例如 `release/linux/`, `release/windows/`）。

3. 本地 IDE 个性化配置
- `.claude/settings.local.json`

备注：通常属于个人环境文件，建议忽略而非入库。

## D. 已在工作区中标记删除（可继续确认提交）

以下文件在 `git status` 中已显示删除（D）：
- `CURRENT_PROJECT_ISSUE_LIST_2026-03-23.md`
- `PROJECT_AUDIT_REPORT.md`
- `PROJECT_ISSUE_REPORT.md`
- `WORK_RECORD.md`
- `spec/function_checklist.md`
- `spec/manual_trading_module_spec.md`
- `spec/manual_trading_tasks.md`
- `spec/plan.md`
- `spec/task_list.md`

备注：若已由 `spec/SPEC.md`、`spec/TASKS.md`、`spec/checklist.md` 承接内容，可继续保持删除并提交。

## E. 建议的后续动作（可选）

1. 删除 A 类本地产物并保留 B/C 类。
2. 确认是否清理 `okx-quant` 与 `release/okx-quant-1.0.0/` 两个已跟踪制品。
3. 补充 `.gitignore`：
- `coverage*`
- `release/linux/`
- `release/windows/`
- `.claude/settings.local.json`
