# Moonbook 重构最终验证报告

验收日期：2026-08-21

## 最终结论

固定代码候选 `a1c193821ea0bf7f7551d72741b39a2aa2fccc40` 的 `make verify` 九阶段全部通过，证明当前仓库的本地代码、合成迁移合同、单机 Compose、浏览器旅程、监控配置和供应链技术门已经闭环。

生产结论仍为 **No-Go**。旧库完整副本只完成 3,834,607,260 字节恢复和只读预检，16 个 TXT 导入任务缺少原文件及受控清单，业务迁移、全量对象/财务核对、完整耗时和回退余量均未执行。真实 EPUSDT、AI、论坛/代理、SMTP、生产 TLS/DNS/流量权限、GVA 商业授权和第三方许可证处置也没有生产证据。本报告不授权生产写入、迁移、DNS 或流量切换。

## 固定候选验收

- commit：`a1c193821ea0bf7f7551d72741b39a2aa2fccc40`
- UTC：`2026-08-20T23:55:32Z` 至 `2026-08-20T23:57:49Z`
- 结果：quality、management、reader、integration、migration、e2e、compose、monitoring、security 共九阶段全部通过
- 结构化摘要与工具版本：`docs/verification/artifacts/README.md`
- macOS 后端门按既定规避路径使用 `CGO_ENABLED=0`；Linux CI 继续强制 race，不以本机结果替代 CI race

## M0-M7 结论

| 领域 | 结论 | 直接证据 |
| --- | --- | --- |
| M0 基线与盘点 | 通过 | `docs/baselines/`、`docs/inventory/admin-feature-matrix.md` |
| M1 工程与基础设施 | 通过 | 空库迁移、完整 Compose、健康/指标、管理员引导、优雅停机及秘密扫描进入 compose 门 |
| M2 小说与对象存储 | 通过 | 小说迁移、PostgreSQL/MinIO 集成及对象完整性测试进入 integration/migration 门 |
| M3 Reader 兼容 | 通过 | 冻结树、536 个基线用例、SSR/SEO、生产构建、真实依赖与浏览器旅程通过 |
| M4 交易与运营 | 本地通过，外部待验收 | `m4-exit-audit.md`；真实 EPUSDT 最小金额支付未执行 |
| M5 内容生产与长任务 | 本地通过，外部待验收 | `m5-m6-content-migration-audit.md`；真实论坛、代理和 AI 未执行 |
| M6 全量迁移 | No-Go | 核对器和故障注入通过；`m6-full-copy-rehearsal.md` 因 16 个 TXT 原文件缺失在目标写入前停止 |
| M7 运维与自动门 | 本地通过，生产 No-Go | 九阶段全门、监控、安全、备份/升级/维护/回退合同通过；外部授权和 M6 完整副本未关闭 |

## 专项验收

- 财务：支付状态机、每次回调审计、签到/邀请运营页和 `commerce/reconcile.Full` 已通过本地测试与故障注入；2026-09-01 又补齐 Reader 邀请奖励事实读写闭环和确定性运行期历史补偿，真实支付仍按外部清单执行。
- 对象与迁移：`all` Stage、checkpoint、迁移错误、源目标行数/主键/关联、MinIO 字节与 SHA-256、Full 财务任一差异都会非零退出。完整副本没有运行这些业务阶段，因此不得用合成故障注入替代全量结果。
- 性能：空库短时基线零失败，只用于回归；没有覆盖完整数据、鉴权、正文、支付、Worker 或持续负载，不能作为生产容量证明。
- 安全：三镜像、SBOM 和锁文件在固定口径下无已有修复版本的 High/Critical，秘密与恶意构建依赖门通过。GVA BSL 1.1 Production Use 授权、Server `NOASSERTION` 和第三方许可证履约仍阻断生产。
- 恢复与切换：隔离备份恢复、升级、应用回退、503 维护模式、可信代理和生产 Compose 合同通过；真实数据恢复、TLS、DNS 和切流未执行。

## 未关闭项

1. 取得 16 个 TXT 原文件，建立仓库外清单，从空目标重跑完整副本迁移、幂等、对象、财务、资源和 12 小时窗口核对。
2. 按 `docs/runbooks/external-validation.md` 完成真实 EPUSDT、AI、论坛/代理、SMTP、TLS/可信代理和 DNS/流量验收。
3. 提供 GVA 商业授权，并完成 `NOASSERTION`、GPL/LGPL notice/source-offer 等第三方许可证书面处置。
4. 由变更负责人批准生产容量、备份、值班、最晚回退时间和切流窗口。

## 生产动作与服务影响

本轮没有执行生产迁移、真实支付、TLS/DNS、流量切换、`git push` 或其他生产动作。Task 13 只更新文档，不影响运行服务，无需重启。未来实际发布仍须按 Runbook 运行一次性 `migrate`，重建并替换 `server`、`web`、`reader-ui` 和 `gateway`；监控配置变化时重建 monitoring profile。PostgreSQL、Redis 和 MinIO 不因本报告重启或重建。

## 2026-09-01 邀请奖励专项增补

本增补不改变上述固定候选的历史九阶段结论，也不授权生产操作。专项修复新增迁移 `00076`，恢复 `POST /reader/me/invite/code` 的邀请人奖励规则、事实累计和最近 20 条明细，并让新注册奖励在同一 PostgreSQL 事务内写奖励事实与钱包流水。旧 MySQL 奖励事实仍由 `reader-finance` 迁移；`00076` 只补建能够由既有运行期不可变流水确定证明的缺失事实，不修改余额或补发资金。

专项验证通过 quality、migration、真实 PostgreSQL/Redis/MinIO integration、RegistrationReward/Reader Invite race、Reader 45 个文件 542 项测试、树外 SSR/SEO 6 项及生产构建。Reader 总门的冻结树基线已从过期的 `26743db` 推进到仓库中已批准并部署的四位小数修复 `5227a99`；本次未修改 Reader 业务文件。隔离 Playwright 旅程确认邀请人“已邀请 1”“累计金币 13”及“邀请注册奖励 +13金币”弹窗，临时事实和数据库已清理。

发布影响仅涉及 `moonbook-server` 和 PostgreSQL schema：部署时先执行迁移 `00076`，再重建/重启 Server。`moonbook-web`、冻结 Reader UI、Redis、MinIO 和 Gateway 无代码变更，不需要因本修复重启。正式生产迁移仍需单独授权。
