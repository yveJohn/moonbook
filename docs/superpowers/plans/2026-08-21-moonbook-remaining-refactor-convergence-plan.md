# Moonbook 剩余重构收敛实施计划

**目标：** 按 R1 至 R5 完成 Moonbook M4-M7 的全部本地剩余工作，在完整副本演练、统一验证和最终证据通过后达到生产切换就绪，或仅剩必须由用户授权的真实第三方、商业授权和生产操作验收。

**架构：** 保持 Go 模块化单体、GVA 管理端、冻结 Reader、PostgreSQL、Redis、MinIO 和根 Docker Compose。业务任务继续由各模块状态机管理，Platform 只提供只读任务观测、指标、迁移和统一运维能力。所有结构变更使用前向 Goose 迁移，所有外部 Secret 由环境变量或 Secret 引用注入。

**技术栈：** Go 1.24、Gin、Vue 3、Element Plus、PostgreSQL 17、Redis 7、MinIO、Docker Compose、Prometheus、Alertmanager、Playwright、Go race/vet、Node test、ESLint、Gitleaks、Trivy、Syft。

**批准规格：** `docs/superpowers/specs/2026-08-21-moonbook-remaining-refactor-convergence-design.md`

---

## 执行规则

- [ ] 只修改 `/Users/yve/code/ai-project/moonbook`，冻结旧项目默认只读，冻结 `reader-ui` 业务源码不得改动。
- [ ] 不连接或修改生产，不执行真实支付、生产迁移、DNS/反向代理切流、生产写入或 `git push`。
- [ ] 每个任务先补能证明缺口的失败测试或审计证据，再实现完整行为；不得删除、跳过或放宽既有测试。
- [ ] PostgreSQL 迁移只新增更高版本；创建前检查实际最高版本，已执行迁移不得修改、改名或删除。
- [ ] 所有 JavaScript Long ID 作为字符串处理，禁止 `Number`、`parseInt`、算术和 `el-input-number`。
- [ ] 真实集成测试只使用本项目专用 PostgreSQL、Redis、MinIO、网络和命名卷；外部 HTTP 替身只监听回环地址。
- [ ] 完整副本只在隔离目录和项目专用容器中恢复，源 MySQL 保持 `read_only=1`、`super_read_only=1`。
- [ ] Secret、Cookie、Token、连接串、对象键和完整敏感正文不得进入 Git、日志、管理响应、测试快照和报告。
- [ ] 每个逻辑单元验证后检查全部 tracked/untracked 修改，按功能使用简体中文提交，提交后执行 `git status`。
- [ ] 任一必需门失败时保留失败证据并修复，不使用 `continue-on-error` 或局部通过代替工作包退出。

## Task 1：完成平台任务监控迁移和领域边界

**文件：**

- Modify: `server/internal/platform/migrate/migrations/00067_platform_job_monitor_admin.sql`
- Modify or remove if not executed: `server/internal/platform/migrate/migrations/00068_platform_job_monitor_menu_fix.sql`
- Modify: `server/internal/platform/migrate/migrate_test.go`
- Modify: `server/internal/platform/migrate/migrate_integration_test.go`
- Create/Modify: `server/internal/platform/jobmonitor/types.go`
- Create/Modify: `server/internal/platform/jobmonitor/repository.go`
- Create/Modify: `server/internal/platform/jobmonitor/service.go`
- Create/Modify: `server/internal/platform/jobmonitor/service_test.go`
- Create: `server/internal/platform/jobmonitor/repository_integration_test.go`

- [x] **Step 1：确认迁移状态。** 检查本项目全部开发/验收库的 `moonbook_schema_version`，确认 `00067`、`00068` 是否执行；未执行时合并为一个无冲突迁移，已执行时保留并新增更高版本修复，禁止静默改写历史。
- [x] **Step 2：增加迁移失败测试。** 固定菜单、API、Casbin、唯一 ID、序列推进、forward-only Down、空库和升级语义；模拟 ID 已占用时不得把平台任务权限关联到其他菜单。
- [x] **Step 3：固定查询合同。** 列表按 module、jobType、status、leaseOwner、from、to 筛选，稳定按 `updated_at,id` 排序；详情返回 attempts，所有 ID 为字符串。
- [x] **Step 4：实现 SQL 仓储。** 使用参数化 SQL、受控排序和明确分页上限；错误消息通过封闭敏感词和长度规则脱敏，不返回 payload/result JSON。
- [x] **Step 5：真实 PostgreSQL 验证。** 覆盖五种状态、空租约、过期租约、大 ID、多 attempt、时间边界、分页稳定性、缺失任务和敏感错误文本。
- [x] **Step 6：质量门。** 运行 `go test ./internal/platform/jobmonitor ./internal/platform/migrate -v`、`go vet` 和迁移空库/升级/重跑验证。
- [x] **Step 7：提交。** 提交信息：`完善平台任务监控后端`。

## Task 2：完成平台任务管理 API 和页面

**文件：**

- Modify: `server/internal/platform/jobmonitor/http.go`
- Modify: `server/initialize/router_biz.go`
- Create: `server/internal/platform/jobmonitor/http_integration_test.go`
- Create/Modify: `web/src/api/platform.js`
- Create/Modify: `web/src/view/platform/jobs/index.vue`
- Modify: `web/src/pathInfo.json`
- Modify: `web/test/pathMap.test.js`
- Create: `web/test/platformJobs.test.js`
- Modify: `server/cmd/moonbook-browser-fixture/main.go`
- Modify: `server/cmd/moonbook-browser-fixture/main_test.go`

- [x] **Step 1：增加 HTTP 失败测试。** 固定 JWT、Casbin、非法分页/状态/时间、列表/详情响应、404 和 Long ID 字符串；证明无任何写路由。
- [x] **Step 2：完成 Handler。** 复用管理响应和 GVA 私有路由，错误对外脱敏；列表和详情不接受自由排序或业务状态变更。
- [x] **Step 3：完善页面。** 使用紧凑筛选栏、状态显示、任务详情和 attempt 表；时间筛选使用 Element Plus 日期时间控件并转 RFC3339，不要求管理员手写时间格式。
- [x] **Step 4：前端合同测试。** 证明页面只读、Long ID 无数字转换、错误详情不使用 `v-html`、移动端筛选和表格不产生页面级横向溢出。
- [x] **Step 5：真实管理员验收。** 在隔离 Compose 中完成登录、动态菜单、列表筛选、详情、权限拒绝、桌面 `1440x1000` 和移动 `390x844` 浏览器旅程；控制台 0 error/0 warning。
- [x] **Step 6：验证。** 运行后端 HTTP 集成、`pnpm run test:moonbook`、定向 ESLint、生产构建和供应链检查。
- [x] **Step 7：提交。** 提交信息：`新增平台任务监控页面`。

## Task 3：收敛 GVA 管理基座范围和真实验收

**文件：**

- Create: `docs/verification/m7-gva-foundation-final.md`
- Modify: `docs/inventory/admin-feature-matrix.md`
- Modify: `docs/verification/m7-gva-foundation-audit.md`
- Create: `server/internal/platform/adminbootstrap/foundation_http_integration_test.go`
- Create: `web/test/managementFoundation.test.js`
- Modify: `server/cmd/moonbook-browser-fixture/main.go`

- [x] **Step 1：冻结能力审计。** 对在线状态、通知公告、通用 OSS 和 SMTP 分别检查旧库表行数、旧 Controller/页面调用、当前配置和业务引用，形成可复核的实现或移除结论。
- [x] **Step 2：补管理 HTTP 验收。** 覆盖登录、强制改密、注销、会话撤销、RBAC 越权、角色授权、部门、岗位、字典、参数、登录日志、操作日志、定时任务和数据权限。
- [x] **Step 3：补系统配置安全回归。** 真实管理员请求只能读取白名单和 Secret 配置状态，写接口不存在或拒绝；响应与日志不含注入的测试 Secret。
- [x] **Step 4：浏览器旅程。** 覆盖动态菜单、页面切换、刷新、keep-alive、平台任务、系统配置、日志和定时任务；夹具创建和清理必须可重复。
- [x] **Step 5：更新矩阵。** 所有“基线已盘点”“待确认”和已经过时的缺口改为直接证据或批准移除记录。
- [x] **Step 6：提交。** 提交信息：`完成管理基座最终验收`。

## Task 4：将论坛 Cookie 改为 Secret 引用

**文件：**

- Create: `server/internal/platform/migrate/migrations/00069_forum_cookie_secret_reference.sql`（若最高版本变化则顺延）
- Modify: `server/internal/platform/migrate/migrate_test.go`
- Modify: `server/internal/modules/novel/crawlsource/types.go`
- Modify: `server/internal/modules/novel/crawlsource/service.go`
- Modify: `server/internal/modules/novel/crawlsource/repository.go`
- Modify: `server/internal/modules/novel/crawlsource/repository_integration_test.go`
- Create: `server/internal/modules/novel/crawlsource/secrets.go`
- Create: `server/internal/modules/novel/crawlsource/secrets_test.go`
- Modify: `server/internal/modules/novel/importtask/worker.go`
- Modify: `server/internal/modules/novel/candidate/worker.go`
- Modify: `.env.example`
- Modify: `compose.yaml`
- Modify: `server/config/config_contract_test.go`

- [x] **Step 1：固定 Secret 合同。** 来源只保存安全引用和 `cookieConfigured`，创建/更新不接受或返回 Cookie 明文；引用格式使用受控环境变量名，不允许路径、模板或任意表达式。
- [x] **Step 2：前向迁移。** 增加 `cookie_secret_ref` 并拒绝新明文；历史 `cookie_text` 不复制到 Git 或报告，迁移后清空明文字段前必须生成仅含来源 ID 哈希和处置状态的审计。
- [x] **Step 3：运行时解析。** Worker 在执行请求前从进程环境解析引用；缺失、空值和超限值使用稳定错误码并禁止任务发出请求。
- [x] **Step 4：管理兼容。** 页面仅编辑引用和显示已配置状态，不显示环境值；Long ID 保持字符串。
- [x] **Step 5：真实验证。** 覆盖空库/升级/重跑、明文清理、环境 Secret 注入、未配置失败、日志脱敏和论坛 HTTP 替身实际收到 Cookie。
- [x] **Step 6：提交。** 提交信息：`改用Secret引用配置论坛Cookie`。

## Task 5：补论坛连接检查和内容 Worker 重启恢复

**文件：**

- Modify: `server/internal/modules/novel/crawlsource/http.go`
- Modify: `server/internal/modules/novel/crawlsource/service.go`
- Create: `server/internal/modules/novel/crawlsource/check.go`
- Create: `server/internal/modules/novel/crawlsource/check_test.go`
- Modify: `server/initialize/import_worker.go`
- Create: `server/initialize/import_worker_integration_test.go`
- Modify: `server/internal/modules/novel/importtask/worker_integration_test.go`
- Modify: `server/internal/modules/novel/txtimport/worker_integration_test.go`
- Modify: `server/internal/modules/novel/chapterclean/worker_integration_test.go`
- Create: `server/internal/modules/novel/chaptersummary/worker_integration_test.go`
- Create: `server/internal/modules/novel/bookprofile/worker_integration_test.go`
- Modify: `web/src/api/novel/crawlSources.js`
- Modify: `web/src/view/novel/crawlSources/index.vue`

- [ ] **Step 1：连接检查合同。** 只访问来源配置允许的 HTTP/HTTPS 目标，限制端口、DNS/私网、重定向、响应体和超时；使用 Secret 引用但响应仅返回状态、耗时和稳定错误码。
- [ ] **Step 2：本地 HTTP 替身。** 覆盖成功、认证失败、重定向越界、超时、过大响应、DNS/连接错误和 Cookie 脱敏。
- [ ] **Step 3：生命周期故障矩阵。** 同一测试进程执行 Start→任务领取→Stop→租约过期→Start，覆盖论坛发现/导入、TXT、清洗、摘要、画像；证明配置重载最多一个 Worker 集合。
- [ ] **Step 4：结果唯一性。** 每类任务重启和重复执行后章节、对象、清洗结果、摘要、画像及 attempt 数量符合既有幂等约束。
- [ ] **Step 5：真实依赖验证。** 使用项目专用 PostgreSQL、Redis、MinIO 和回环 HTTP/OpenAI 替身完成整体恢复场景。
- [ ] **Step 6：提交。** 提交信息：`完善内容任务连接检查与重启恢复`。

## Task 6：扩展全域迁移核对器

**文件：**

- Modify: `server/internal/platform/legacyaudit/*`
- Modify: `server/cmd/moonbook-migration-audit/main.go`
- Create: `server/internal/platform/legacyaudit/object_audit.go`
- Create: `server/internal/platform/legacyaudit/object_audit_integration_test.go`
- Create: `server/internal/platform/legacyaudit/content_audit_integration_test.go`
- Modify: `server/internal/modules/commerce/reconcile/*`
- Modify: `docs/migration/rehearsal.md`
- Modify: `docs/migration/mapping.md`

- [ ] **Step 1：建立覆盖清单测试。** 每个注册 migration stage 必须有行数、主键、关联和业务核对项；测试在增加新 stage 而未增加核对时失败。
- [ ] **Step 2：补内容生产核对。** 覆盖论坛来源/板块/候选/任务/日志、TXT、AI 配置/模型、清洗、摘要、画像和合并血缘。
- [ ] **Step 3：补对象核对。** 从 PostgreSQL 业务引用流式读取对象，逐对象 `Stat/Get` 校验存在性、kind、owner、字节和 SHA-256；报告聚合和最多 100 个哈希指纹，不输出对象键或正文。
- [ ] **Step 4：扩展全域财务核对。** 保持钱包、流水、订单、支付、回调、会员和权益零差异门，异常退出非零。
- [ ] **Step 5：真实双库与 MinIO 测试。** 注入行数、关联、财务、缺对象、错大小和错哈希故障，证明每类差异可被稳定分类且无静默通过。
- [ ] **Step 6：提交。** 提交信息：`完善全域迁移与对象核对`。

## Task 7：脚本化完整副本迁移演练

**文件：**

- Create: `scripts/verify-m6-full-copy.sh`
- Create: `scripts/lib/full-copy-common.sh`
- Modify: `scripts/inventory-legacy-mysql.sh`
- Modify: `docs/migration/rehearsal.md`
- Create: `docs/verification/m6-full-copy-rehearsal.md`
- Modify: `docs/runbooks/production-cutover.md`

- [ ] **Step 1：只读预检。** 脚本要求明确备份绝对路径、期望 SHA-256、隔离 Compose 项目名和确认标记；拒绝工作区根目录、冻结仓库目录和非本项目容器。
- [ ] **Step 2：可恢复恢复流程。** 创建隔离 MySQL、PostgreSQL、Redis、MinIO 和网络；恢复后立即启用 `read_only`、`super_read_only` 并记录源行数哈希。
- [ ] **Step 3：空目标迁移。** 从版本 0 执行结构迁移和所有业务 stage，保存每 stage 行数、错误、耗时、吞吐及资源采样。
- [ ] **Step 4：全域核对。** 运行 Task 6 核对器、财务核对、对象数量/字节/哈希和 Reader 契约；任一差异退出非零。
- [ ] **Step 5：幂等重跑。** 重新运行结构和业务迁移，证明结构 `applied=0`、业务结果不增长、检查点稳定和源库未改变。
- [ ] **Step 6：完成约 8GB 演练。** 使用已定位候选生成固定报告，记录总耗时、各阶段耗时、峰值资源、错误处置、12 小时余量和清理结果。
- [ ] **Step 7：提交。** 先提交脚本和模板：`新增完整数据副本迁移演练`；真实报告另提交：`记录完整数据副本迁移结果`。

## Task 8：增加 Compose 监控栈

**文件：**

- Modify: `compose.yaml`
- Create: `deploy/monitoring/prometheus.yml`
- Create: `deploy/monitoring/alerts/*.yml`
- Create: `deploy/monitoring/alertmanager.yml`
- Create: `deploy/monitoring/blackbox.yml`
- Create: `deploy/monitoring/webhook/*`
- Modify: `.env.example`
- Create: `scripts/verify-monitoring.sh`
- Create: `docs/runbooks/monitoring.md`
- Create: `docs/runbooks/incidents/*.md`

- [ ] **Step 1：固定镜像。** Prometheus、Alertmanager、postgres_exporter、redis_exporter、cAdvisor 和 Blackbox Exporter 使用固定 tag/digest；监控 profile 默认不启动。
- [ ] **Step 2：最小权限采集。** PostgreSQL exporter 使用只读监控账号，Redis 和 MinIO 使用受限采集配置；Prometheus Bearer Token 通过只读 Secret 文件或环境渲染，不写入仓库。
- [ ] **Step 3：实现规则。** 覆盖 API/Reader、PostgreSQL、Redis、MinIO、支付、任务、迁移、对象和容器资源；每条告警带 severity、runbook 和稳定低基数标签。
- [ ] **Step 4：Alertmanager。** 提供本地 UI 和通用 Webhook；生产地址由环境变量注入，默认本地 sink 不外发。
- [ ] **Step 5：自动验证。** 使用 `promtool`/`amtool` 校验配置，通过故障注入证明关键告警进入 firing、通知送达 sink、恢复后 resolved。
- [ ] **Step 6：提交。** 提交信息：`新增生产监控与告警栈`。

## Task 9：关闭可修复漏洞并生成供应链证据

**文件：**

- Modify: `server/go.mod`, `server/go.sum`
- Modify: `web/package.json`, `web/pnpm-lock.yaml`
- Modify only with explicit frozen-Reader compatibility evidence: `reader-ui/package.json`, `reader-ui/package-lock.json`
- Modify: Dockerfiles under `server`, `web`, `deploy/compose`
- Create: `scripts/scan-supply-chain.sh`
- Create: `docs/verification/m7-sbom-licenses.md`
- Modify: `docs/verification/m7-security-supply-chain-audit.md`

- [ ] **Step 1：固定扫描工具。** 固定 govulncheck、Trivy、Syft 和许可证扫描版本/digest，扫描源码锁文件、最终镜像和 SBOM。
- [ ] **Step 2：升级基础镜像和依赖。** 优先关闭已有修复版本的 high/critical；每次升级按 Server/Web/Reader 独立提交和回归，不跨模块混合。
- [ ] **Step 3：冻结 Reader 决策。** 只有契约、536 项测试、SSR 和关键旅程均通过才允许依赖升级；无法兼容时保留冻结源码并形成具体缓解和用户批准项。
- [ ] **Step 4：安全行为门。** 验证 RBAC、暴力登录、限流、安全响应头、敏感日志、Secret 注入和最小权限凭据。
- [ ] **Step 5：SBOM 和许可证。** 为三个最终镜像输出 SPDX/CycloneDX 摘要和许可证结论；GVA BSL 1.1 商业授权明确列为外部证据。
- [ ] **Step 6：提交。** 按实际逻辑分别使用 `升级后端安全依赖`、`升级管理端安全依赖`、`更新基础镜像安全版本`、`补充供应链验收证据`。

## Task 10：建立 M0-M7 统一验证入口

**文件：**

- Modify: `Makefile`
- Create: `scripts/verify-all.sh`
- Create: `scripts/verify-management-e2e.sh`
- Create: `scripts/verify-migrations.sh`
- Create: `scripts/verify-security.sh`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/verification/m7-ci-coverage-audit.md`

- [ ] **Step 1：定义分层门。** `make verify` 汇总 quality、management、reader、integration、migration、e2e、compose、monitoring 和 security；本地可按阶段运行，最终汇总不能遗漏。
- [ ] **Step 2：固定产物。** 每个阶段输出结构化摘要、工具版本、commit、开始/结束时间和退出状态，敏感日志只保留脱敏摘要。
- [ ] **Step 3：CI 分 job。** 真实依赖使用独立 Compose 项目；完整副本和真实外部验收不进入普通 PR CI，但对应脚本合同、合成数据和报告校验必须进入。
- [ ] **Step 4：失败传播。** 任一必需 job 失败时最终门失败，不使用 `continue-on-error`；报告上传不掩盖退出码。
- [ ] **Step 5：本地和 CI 等价验证。** 在当前固定 commit 运行所有不需要外部凭据的入口并记录耗时。
- [ ] **Step 6：提交。** 提交信息：`建立全量重构统一验收入口`。

## Task 11：完善生产部署、故障、切换和回退 Runbook

**文件：**

- Modify: `docs/runbooks/deploy-compose.md`
- Modify: `docs/runbooks/upgrade-compose.md`
- Modify: `docs/runbooks/backup-restore.md`
- Modify: `docs/runbooks/production-cutover.md`
- Modify: `docs/runbooks/rollback.md`
- Create: `docs/runbooks/maintenance-mode.md`
- Create: `docs/runbooks/external-validation.md`
- Modify: `compose.yaml`
- Modify: `deploy/compose/gateway.conf`

- [ ] **Step 1：部署合同。** 固定 commit/tag/digest、配置版本、TLS 终止、可信代理、Secret 完整性、日志轮转、资源限制和磁盘水位。
- [ ] **Step 2：协调停写。** 提供可验证的维护模式或网关阻断流程，明确支付回调缓存/重投、Worker 停止、PostgreSQL 与 MinIO 协调快照顺序。
- [ ] **Step 3：切换时限。** 把 12 小时拆为备份、预检、迁移、核对、冒烟、观察和回退余量；使用 Task 7 实测数据计算最晚回退时间。
- [ ] **Step 4：故障处理。** 每个监控告警链接到 PostgreSQL、Redis、MinIO、Gateway、Reader、Worker、支付、迁移和对象故障手册。
- [ ] **Step 5：外部清单。** 对真实支付、AI、论坛/代理、SMTP、GVA 商业授权和生产切流记录责任人、输入、命令、通过标准、风险和证据位置。
- [ ] **Step 6：独立复现。** 在隔离 Compose 中由脚本按文档完成部署、升级、备份恢复和应用镜像回退。
- [ ] **Step 7：提交。** 提交信息：`完善生产切换与故障处理手册`。

## Task 12：收敛状态文档和功能矩阵

**文件：**

- Modify: `docs/progress/refactor-status.md`
- Modify: `docs/inventory/admin-feature-matrix.md`
- Modify: `docs/migration/mapping.md`
- Modify: `docs/verification/m4-exit-audit.md`
- Modify: `docs/verification/m5-m6-content-migration-audit.md`
- Modify: `docs/verification/m7-readiness-audit.md`
- Modify: this plan

- [ ] **Step 1：删除陈旧事实。** 修正已完成运营页签、财务全域核对、内容迁移 stage、自动发现调度和模块边界整改仍被写成未完成的问题。
- [ ] **Step 2：逐项链接证据。** 每个矩阵能力链接代码、自动测试、运行报告、提交或批准移除记录。
- [ ] **Step 3：更新 R1-R4 状态。** 只有直接证据通过才勾选；真实第三方和商业授权进入外部清单，不混为代码缺口。
- [ ] **Step 4：提交。** 提交信息：`更新剩余重构验收状态`。

## Task 13：执行固定提交最终验收

**文件：**

- Modify: `docs/verification/final-report.md`
- Create: `docs/verification/artifacts/README.md`
- Modify: `docs/progress/refactor-status.md`
- Modify: this plan

- [ ] **Step 1：选择候选 commit。** 所有 R1-R4 实现和证据已提交，工作区除明确忽略内容外干净。
- [ ] **Step 2：运行统一门。** 执行 `make verify` 及完整副本演练报告校验，记录工具版本、commit、命令摘要和结果。
- [ ] **Step 3：复核完成判据。** 对目标文档每个交付物逐项检查直接证据，不用局部测试支持全局结论。
- [ ] **Step 4：检查外部项。** 仅允许真实支付/AI/论坛/代理/SMTP、GVA 商业授权和生产权限留待用户；每项必须具备精确验收步骤。
- [ ] **Step 5：最终报告。** 删除模板占位，记录 M0-M7、完整副本、财务、对象、性能、安全、恢复、提交和服务重启要求。
- [ ] **Step 6：最终提交。** 提交信息：`完成Moonbook生产切换就绪验收`。
- [ ] **Step 7：最终状态。** `git status` 干净，未 push，未执行任何生产动作；此时才声明“生产切换就绪”。

## 服务影响

计划文档本身不影响服务，无需重启。

实施期间的累计影响如下：

- R1、R2、R3 后端代码：重建并重启 `server`；
- 新增 PostgreSQL 迁移：先运行一次性 `migrate`，再重启依赖新结构的 `server`；
- 管理页面：重建并重启 `web`；
- 监控 profile：创建或重建 Prometheus、Alertmanager、exporter、cAdvisor、Blackbox 和本地 Webhook；
- Compose、Gateway、镜像或资源配置：重建对应服务；
- 冻结 Reader 只有依赖安全整改实际发生时才重建 `reader-ui`；
- PostgreSQL、Redis、MinIO 的业务数据默认不因应用提交而重建或清空。
