# M7 生产切换就绪审计

审计日期：2026-08-15

## 结论

M7 当前未满足生产切换就绪条件。仓库已经有 Compose 部署、备份恢复、切换和回退文档骨架，也有 M1/M3 及部分业务域的真实验证记录；但最终报告仍是模板，备份恢复 Runbook 存在确定的不可执行步骤，监控告警、性能、安全、升级、完整迁移和恢复演练没有闭环证据。

本审计不执行生产操作，也不把文档存在等同于流程已经演练。

## 已有基础

- 根 `compose.yaml` 提供 PostgreSQL、Redis、MinIO、Go Server、管理前端、冻结 Reader、网关、迁移和管理员引导服务。
- `make verify-m1` 可在独立 Compose 项目验证空库迁移、基础设施、网关、指标鉴权、管理员登录、非 root Server、优雅停机和秘密扫描。
- `make verify-m3` 已有冻结 Reader 差异、契约、真实依赖、SSR、生产构建和浏览器关键旅程证据。
- PostgreSQL 迁移、Reader 小夹具迁移、钱包核对和 MinIO 对象服务已有分域测试。
- `production-cutover.md` 和 `rollback.md` 明确生产动作需要单独授权，并禁止直接反向修改旧库。

这些证据是 M7 输入，不是最终验收报告。

## 最终证据索引缺口

`docs/verification/final-report.md` 仍明确标注：

- 新仓库 commit 待记录；
- M0、M1、M2 待最终审计；
- M4、M5、M6、M7 未完成。

管理功能矩阵还有 GVA 基座和读者行为等多项 `基线已盘点`，并存在支付、运营审计、采集连接、自动发现等明确缺口。M7 不能在矩阵仍有未解释入口或“待补”能力时通过。

最终报告还缺少每项验证的命令、执行时间、commit、环境版本、结果摘要和报告文件定位。当前进度文档可追踪开发过程，但不能替代一次固定 commit 上的最终验收执行。

## 备份恢复 Runbook 缺口

`docs/runbooks/backup-restore.md` 当前存在以下确定问题：

1. MinIO 备份命令在 `minio` 服务内写 `/backup/minio`，但根 Compose 没有为该服务挂载宿主备份目录；备份不会按文档落到 `MOONBOOK_BACKUP_DIR`。
2. Compose 为 MinIO 客户端定义了独立 `minio-init` 镜像，Runbook 却假设 `minio` Server 容器内可直接运行 `mc`，没有经过 Compose 合同或真实演练验证。
3. `SHA256SUMS` 在重定向开始时就会出现在被扫描目录，当前 `find` 可能把清单文件自身纳入哈希，无法形成稳定、可重复验证的清单。
4. `migrate` 是执行后退出的一次性服务，成功完成后不能使用 `docker compose exec -T migrate ...`；恢复后迁移应使用受控的 `docker compose run --rm migrate ...` 路径。
5. 迁移版本查询使用 `goose_db_version`，实际 Moonbook 版本表固定为 `moonbook_schema_version`。
6. 恢复示例只在 shell 中导出 `COMPOSE_PROJECT_NAME`，随后执行的 `verify-infrastructure.sh` 会再次加载根 `.env`，可能覆盖项目名并校验日常项目而非隔离恢复项目。
7. “停止写入或进入维护模式”没有对应的确定命令、入口阻断方式或支付回调处置。仓库也没有维护模式实现，无法证明 PostgreSQL dump 和 MinIO mirror 取得协调一致快照。
8. 没有脚本生成恢复前后表计数、财务汇总、对象数/字节数和抽样哈希报告，也没有任何已提交的恢复演练结果。

在修正并用独立项目演练前，备份恢复只能视为未验证草案。

审计后进展：2026-08-15 已修正上述命令级问题，并以两个 `moonbook_verify_` 隔离项目完成 PostgreSQL/MinIO 本地合成备份恢复，报告见 `docs/verification/m7-backup-restore-rehearsal.md`。该演练没有真实业务数据、支付回调协调停写或完整应用冒烟，因此 M7 备份恢复退出门仍未完成。

## 部署与升级缺口

`deploy-compose.md` 覆盖首次启动和日常启停，但没有完整升级流程。至少缺少：

- 固定发布 commit、镜像 tag/digest 和配置版本的发布清单；
- 升级前备份、镜像拉取或构建、迁移状态检查、一次性迁移、服务替换和冒烟的确定顺序；
- 数据库迁移后应用镜像回退的兼容边界；
- 配置项新增/删除检查和 Secret 完整性门；
- 外部 TLS 终止、可信代理头和证书续期的部署合同；
- 容器日志轮转、磁盘容量、水位和资源限制；
- 升级失败时保留旧镜像及恢复入口的命令。

Compose 当前适合作为本地和单机基础，但尚没有经过升级/降级演练的生产操作证据。k3s 清单仍按批准范围留在本目标之外。

审计后进展：2026-08-15 已新增 `docs/runbooks/upgrade-compose.md`，并在 `3058fee` 到 `a8c209d` 两个固定提交镜像之间完成隔离 Compose 升级和应用镜像回退演练，详见 `docs/verification/m7-compose-upgrade-rehearsal.md`。迁移输出 `applied=0`，数据库版本和持久化夹具不变，升级与回退后的网关、API readiness 和 Reader health 均通过；目标镜像财务核对为 `mismatches=0`。该版本对没有迁移差异，因此只关闭命令级升级和特定兼容回退缺口；生产镜像 digest、真实业务冒烟、含迁移版本兼容性、TLS/可信代理、资源限制和观察阈值仍待完成。

## 切换与回退缺口

现有切换文档给出正确的高层顺序和最晚回退公式，但不足以让另一名工程师独立执行：

- 没有每阶段负责人、输入、精确命令、期望输出、最长耗时和停止条件；
- 没有把 12 小时窗口拆为备份、迁移、核对、冒烟、观察和回退安全余量；
- 没有固定的 Go/No-Go 证据包目录和签字记录；
- 没有错误率、延迟、队列积压、数据库连接或对象读取失败的量化阈值；
- 没有支付回调在停机和回退窗口内的路由、重放和人工核对步骤；
- 回退文档要求导出切换后差异，但没有可执行导出器、核对器或加密保管流程；
- 没有实测 RTO、RPO、回退耗时和由此计算出的最晚回退时间示例。

正式生产切换仍不属于本目标的执行范围，但可复现 Runbook 和演练报告属于 M7 必需交付物。

## 监控与故障处理缺口

应用已经暴露受 Bearer Token 保护的 Prometheus 指标，包括 HTTP 请求、延迟、在途请求、任务状态和过期租约。但仓库没有：

- Prometheus scrape 配置和告警规则；
- PostgreSQL、Redis、MinIO、Nginx 和宿主资源采集配置；
- 错误率、p95/p99 延迟、支付失败、回调拒绝、财务差异、对象校验失败、迁移错误、任务积压和租约过期阈值；
- 告警分级、通知路径、静默/升级策略和责任人；
- 日志持久化、轮转、检索和敏感字段抽查流程；
- 数据库耗尽、Redis 不可用、MinIO 对象缺失、Worker 失租、支付异常和迁移中断的故障处理手册。

`/health` 和 `/metrics` 可用不等于监控告警已经就绪。

## 性能与容量缺口

仓库没有 k6、Vegeta 或等价负载测试场景，也没有 Go Benchmark、API 吞吐、Reader 并发、章节对象读取、支付回调并发或 Worker 吞吐报告。约 8 GB 副本迁移尚未执行，因此也没有：

- 各 stage 行数、吞吐和耗时；
- 峰值 CPU、内存、磁盘、数据库连接和 MinIO 带宽；
- 12 小时窗口余量；
- 恢复、回退和对象核对耗时。

M7 必须基于目标容量建立可重复场景、阈值和报告，不能只用单请求功能测试推断生产性能。

审计后进展：2026-08-15 已在隔离空库 Compose 项目建立四个公开读取入口的本地短时基线，网关 health、API readiness、Reader 书籍列表和 Reader SSR 均为零失败，负载后容器继续 healthy，详见 `docs/verification/m7-performance-baseline.md`。该基线没有鉴权、业务数据、MinIO 正文、支付、Worker、持续负载、资源限额或约 8 GB 数据副本，因此只能作为后续回归起点，不能关闭 M7 性能与容量退出门。

## 安全与合规缺口

已有 Gitleaks、鉴权、CORS、回调验签和部分秘密脱敏证据，但最终安全门仍缺少：

- 冻结 Reader 依赖的 `9 high` 漏洞升级结果或用户批准的缓解记录；
- Go、管理前端、Reader 和容器镜像的依赖漏洞报告；
- 镜像、基础镜像、SBOM 和许可证清单；
- 管理 RBAC、越权、暴力登录、限流、敏感日志和安全响应头的最终测试报告；
- PostgreSQL/MinIO/Redis 最小权限凭据和轮换演练；
- GVA BSL 1.1 生产商业授权证据。

商业授权和真实第三方凭据属于外部 Go/No-Go 项，但必须有明确责任人和证据位置，不能用代码测试替代。

审计后进展：2026-08-15 已执行锁文件、govulncheck 和三个本地镜像的 Trivy 扫描，详见 `docs/verification/m7-security-supply-chain-audit.md`。管理端直接构建依赖 `vite-vue-path-map@1.0.2` 已被标记为 critical 恶意包，且注入逻辑实际进入本地生产产物；当前 Server、Web 和 Reader 镜像也均存在已有修复版本的 high/critical 漏洞。因此当前应用镜像明确为生产 No-Go，安全退出门未关闭。

## CI 与统一验收缺口

当前 CI 执行基础 Go race 测试、管理前端测试/构建、冻结 Reader 测试/构建、Gitleaks 和 M1 Compose 验证。但它没有：

- 执行 `make verify-m3` 或业务集成测试标签；
- 执行完整空库迁移后所有真实 PostgreSQL/Redis/MinIO 业务域测试；
- 执行浏览器 E2E、迁移双库测试、备份恢复或性能测试；
- 上传版本化测试报告、日志摘要和验证制品；
- 依赖/镜像漏洞扫描。

根 `make verify` 当前只依赖 `verify-m1`，不存在覆盖 M0-M7 的统一最终验收入口。最终门需要在一个固定 commit 上执行全部可自动化证据，并保留结构化报告。

审计后进展：2026-08-15 在固定提交 `c88052f` 按 CI 后端包范围复跑格式、模块校验、vet 和普通测试。格式、模块校验及 vet 通过，本地监听相关测试在允许回环监听后通过；但 `TestModuleDependencyRules` 确定发现 15 处跨域实现引用，当前 Linux CI 同样应失败，详见 `docs/verification/m7-backend-quality-baseline.md`。修复必须建立真实契约边界并保持测试原样，不能通过跳过或白名单掩盖违规。

## M7 退出门

M7 只有在以下条件全部满足后才能标记完成：

1. M0-M6 已完成，管理功能矩阵无缺项；
2. 修正并脚本化备份恢复，在独立 Compose 项目完成 PostgreSQL/MinIO 协调恢复演练；
3. 部署升级、监控告警、故障处理、任务恢复、切换和回退 Runbook 可由另一名工程师复现；
4. 单元、race、真实集成、契约、SSR、E2E、性能、安全和迁移报告固定到同一验收 commit；
5. 完整副本迁移、财务/对象核对和 12 小时窗口证据通过；
6. Reader 高危依赖、镜像漏洞、许可证和 GVA 商业授权均有关闭或批准记录；
7. 真实支付、AI、邮件、采集代理的待验项给出精确输入、命令、期望输出和风险；
8. `docs/verification/final-report.md` 不再含待最终审计或未完成项；
9. 工作区干净，全部有效修改按功能提交，且未执行生产切换或 `git push`。
