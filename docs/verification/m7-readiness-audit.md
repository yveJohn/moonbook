# M7 生产切换就绪审计

审计更新：2026-08-21

## 当前结论

Moonbook 的本地实现、自动化和单机 Compose 运维合同已收敛，但当前仍是生产 No-Go。唯一阻止完整本地数据演练的输入缺口是 16 个旧 `novel_txt_import_task` 原文件及受控清单；没有这些文件，3,834,607,260 字节源库只完成恢复和只读预检，业务迁移、对象/财务核对、完整耗时与回退时间均不能宣称通过。

真实支付、AI、论坛/代理、SMTP、GVA 商业授权、生产 TLS/DNS 和流量权限是外部验收项，按 `docs/runbooks/external-validation.md` 管理，不再混写为代码缺口。Task 13 已在固定候选 `a1c193821ea0bf7f7551d72741b39a2aa2fccc40` 完成九阶段 `make verify`，全部通过；最终摘要见 `docs/verification/artifacts/README.md`，但该自动门不能替代完整副本和外部生产证据。

## 里程碑状态

| 里程碑 | 本地状态 | 直接证据 | 剩余门 |
| --- | --- | --- | --- |
| M0-M3 | 已完成 | `docs/progress/refactor-status.md`、Reader 536 项基线、真实依赖与浏览器旅程 | 无本地缺口 |
| M4 | 本地完成 | `docs/verification/m4-exit-audit.md`：支付状态机、回调全尝试审计、运营页签、Full 财务核对 | 真实 EPUSDT 最小金额支付 |
| M5 | 本地完成 | `m5-content-worker-connection-and-recovery.md`、`m5-forum-cookie-secret-reference.md`、`m5-m6-content-migration-audit.md` | 真实论坛与 AI 授权环境 |
| M6 | 核对器完成、完整副本阻断 | `m6-global-migration-object-audit.md`、`m6-full-copy-rehearsal.md` | 16 个 TXT 原文件/清单，随后重跑全部 Stage 与核对 |
| M7 | 本地自动门完成、生产 No-Go | 固定候选九阶段全门、GVA、模块边界、监控、安全、CI、备份/升级/维护演练报告 | M6 完整副本；生产外部授权与许可证证据 |

## 已关闭的原审计缺口

### 管理基座与模块边界

- GVA 验收覆盖验证码登录、首次改密、JWT 撤销、RBAC、组织岗位、字典参数、日志、定时任务、只读系统配置和平台任务；桌面/移动浏览器动态菜单和刷新通过，见 `m7-gva-foundation-final.md`。
- Reader、Commerce、Novel 的跨模块实现 import 和跨域 SQL 已清零；注册奖励、点赞、购买和首充奖励保持同库原子事务，见模块边界计划与验证报告。
- 管理功能矩阵不再含未解释“基线已盘点”：保留能力有代码/路由/测试索引，通知公告、通用 OSS、SMTP 插件和在线会话 UI 有批准移除证据。

### 数据迁移与核对

- `all` 的 Stage 注册集合与审计覆盖合同双向锁定；checkpoint、迁移错误、源目标行数/主键、关联、Full 财务、MinIO 字节与实际 SHA-256 任一差异均返回非零。
- 合成 MySQL/PostgreSQL/MinIO 已注入缺行、断关联、缺对象、错大小、错哈希与财务异常并全部检出。
- 完整副本预检在任何目标业务写入前因 TXT 清单缺失停止，这是正确的 No-Go，不是测试失败。恢复实测 64 秒不能替代完整迁移耗时。

### 备份、升级、维护与回退

- PostgreSQL custom dump、MinIO mirror、SHA-256、隔离恢复、`applied=0`、财务 `mismatches=0` 和全栈健康已演练，见 `m7-backup-restore-rehearsal.md`。
- 固定镜像升级与特定兼容版本应用回退已演练，数据库迁移后的回退边界明确，见 `m7-compose-upgrade-rehearsal.md`。
- `scripts/maintenance-mode.sh` 先切 Gateway 503/`Retry-After`，再停止 Server/Web/Reader；PostgreSQL、Redis、MinIO 保持运行。退出时先等待应用 healthy，失败则恢复维护 Gateway。隔离进入/退出验收通过。
- 生产 Compose override 强制核心镜像 digest、CPU/内存/PID 限制、日志轮转、回环端口和显式可信代理；Gateway 重写可伪造转发头。12 小时窗口、停止点、最晚回退公式和切换后事实封存已有 Runbook。

### 监控、性能与安全

- 可选 monitoring profile 的 8 个 target、21 条规则、最小权限 PostgreSQL 用户、Alertmanager 和本地 Webhook 已验收；Reader 故障注入产生 firing/resolved，见 `m7-monitoring-alerting.md`。
- 空库短时性能基线覆盖 Gateway、API readiness、Reader 列表和 SSR，全部零失败。它只作为回归基线，不能外推含 8 GB 数据、鉴权、正文、支付和 Worker 的生产容量。
- Server/Web/Reader 候选镜像与锁文件的可修复 High/Critical 已清零，恶意构建依赖已移除，SBOM/许可证摘要和安全响应头已纳入门禁，见 `m7-security-supply-chain-audit.md`。GVA 商业授权、生产 TLS/HSTS 和 `NOASSERTION` 许可证处置仍需外部证据。

### 统一验收

- 根 `make verify` 已统一 quality、management、reader、integration、migration、e2e、compose、monitoring、security 九阶段；GitHub Actions 同名 job 由最终 `required` 强制全部成功。
- Task 13 已在 Task 12 提交后的固定 commit `a1c1938` 重跑九阶段，结果全部通过；UTC、耗时和工具版本已写入 artifact 索引，没有沿用工作树结果冒充候选制品证据。

## Go/No-Go 判据

固定 commit 九阶段统一门、最终报告和 artifact 索引已完成。本地整体 Go 仍必须满足：提供 16 个 TXT 原文件及清单，并让完整副本全部 Stage、幂等、行数/主键/关联、对象、财务和耗时通过。

生产 Go 还必须满足：真实支付/AI/论坛等已启用外部项通过，GVA 商业授权与许可证处置有书面证据，生产 TLS/DNS/备份/容量/值班责任人和回退窗口获批。任何一项缺失都保持 No-Go，不执行生产写入、迁移、DNS 或流量切换。
