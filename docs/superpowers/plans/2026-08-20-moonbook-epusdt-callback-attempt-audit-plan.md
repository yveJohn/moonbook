# Moonbook EPUSDT 回调全尝试审计实施计划

**目标：** 让每次 EPUSDT HTTP 回调在业务处理前独立持久化脱敏审计，保证首次入账、合法幂等、拒绝和可重试故障都有可关联终态，并补齐管理查询、Prometheus 指标和运维证据。

**架构：** `payment.Handler` 只负责有限读取、安全摘要、开始审计、协议解析和 HTTP 映射；`payment.Service` 负责订单凭据、PID、签名和业务错误分类；`payment.SQLRepository` 负责订单锁、资金事实及审计终态事务。`payment.AttemptObserver` 是低耦合指标接口，由 Platform Metrics 实现；管理端继续使用现有回调日志 API 和页面。

**技术栈：** Go、Gin、PostgreSQL 17、现有 Goose Runner、SHA-256、Gin RequestMeta、Prometheus client、Vue 3、Element Plus、真实 PostgreSQL/Redis/MinIO 集成测试、本地 HTTP 重试替身、Docker Compose、Playwright。

**批准规格：** `docs/superpowers/specs/2026-08-20-moonbook-epusdt-callback-attempt-audit-design.md`

---

## 执行规则

- [ ] 只修改活跃仓库，不修改冻结仓库或 `reader-ui` 业务代码。
- [ ] 不连接生产、不使用真实支付凭据、不执行真实支付、正式迁移、切流或 `git push`。
- [ ] 每个任务先写失败测试并确认失败原因，再做最小完整实现；不得删除、跳过或放宽既有测试。
- [ ] PostgreSQL 迁移只新增，不修改、改名或删除 `00001` 至 `00060`；创建前重新检查实际最高版本。
- [ ] 集成测试只使用项目专用 PostgreSQL、Redis 和 MinIO；HTTP 替身只监听回环地址。
- [ ] 原始请求体、请求头、完整签名、PID、IP、Secret 和自由文本底层错误不得进入数据库、管理 API、日志、指标或报告。
- [ ] 每个 EPUSDT 重投必须创建独立审计；不得用 Redis、进程缓存或 payload 哈希合并尝试。
- [ ] 钱包流水、首充奖励、订单 `paid` 和首次成功审计必须同一 PostgreSQL 事务提交。
- [ ] Long ID 在管理 API 和 Vue 中继续按字符串处理，禁止 `Number`、`parseInt`、算术或 `el-input-number`。
- [ ] 每个逻辑单元验证后检查全部 tracked/untracked 修改，使用简体中文独立提交。
- [ ] 本切片完成只关闭 M4 的回调全尝试审计项；两个运营页签、财务全域核对和真实最小金额支付仍保持未完成。

## Task 1：新增回调审计迁移与配置合同

**文件：**

- Create: `server/internal/platform/migrate/migrations/00061_epusdt_callback_attempt_audit.sql`
- Modify: `server/internal/platform/migrate/migrate_test.go`
- Modify: `server/internal/platform/migrate/migrate_integration_test.go`
- Modify: `.env.example`
- Modify: `compose.yaml`
- Modify: `server/config/config_contract_test.go`

- [x] **Step 1：确认迁移版本。** 检查最高版本仍为 `00060`；若已被其他工作占用，按真实最高版本顺延，不得重复、倒序或改写已执行迁移。
- [x] **Step 2：编写失败合同测试。** 要求新增 `failure_code/request_id/trace_id/payload_bytes/payload_truncated/completed_at`，字段类型、空值策略、实际查询索引和 forward-only Down 均符合规格。
- [x] **Step 3：验证预期失败。** 运行迁移清单和合同测试，失败原因只能是新迁移或新配置尚不存在。
- [x] **Step 4：实现前向迁移。** 不删除、不重写、不重新分类历史 `manual_success`、主动同步和 legacy 日志；运行时新字段允许历史兼容空值。
- [x] **Step 5：增加滞留配置。** 注入 `MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES=5`，解析范围固定 1 至 1440；示例和 Compose 不包含秘密。
- [x] **Step 6：真实 PostgreSQL 验证。** 覆盖空库、`00060 -> 00061`、重复执行 `applied=0`、历史日志保持及订单/钱包/流水/奖励/权益行数不减少。
- [x] **Step 7：提交。** 提交信息：`新增EPUSDT回调审计迁移`。

## Task 2：建立审计领域模型与持久化边界

**文件：**

- Modify: `server/internal/modules/commerce/payment/types.go`
- Create: `server/internal/modules/commerce/payment/audit.go`
- Create: `server/internal/modules/commerce/payment/audit_test.go`
- Modify: `server/internal/modules/commerce/payment/repository.go`
- Modify: `server/internal/modules/commerce/payment/repository_integration_test.go`

- [x] **Step 1：编写失败模型测试。** 固定五种结果、封闭失败码、响应状态/正文、snapshot 八字段白名单、16 KiB 上限和 SHA-256 行为。
- [x] **Step 2：定义窄接口。** `AuditRepository.Begin` 独立提交 `received`；拒绝/失败最终化使用审计 ID 与 `processing_result='received'` 条件更新；资金处理接收审计 ID，不暴露 HTTP 或 Gin 类型。
- [x] **Step 3：实现 Begin。** 只写安全元数据、payload 哈希/捕获字节、截断标记、IP 哈希及 RequestMeta 关联 ID；`response_status=0`、`completed_at=NULL`，不得业务解析。
- [x] **Step 4：实现条件最终化。** 受影响行数必须为 1；终态不得覆盖；固定失败码映射固定脱敏文案，禁止拼接 SQL、解析器或网络错误。
- [x] **Step 5：真实 PostgreSQL 验证。** 每次 Begin 产生独立行，重复 payload 不合并；终态不可二次更新；历史/manual/sync 记录继续可读。
- [x] **Step 6：验证并提交。** 运行 payment 单元测试和真实 PostgreSQL 定向测试；提交信息：`建立支付回调审计持久化`。

## Task 3：把审计终态纳入支付事务

**文件：**

- Modify: `server/internal/modules/commerce/payment/service.go`
- Modify: `server/internal/modules/commerce/payment/service_test.go`
- Modify: `server/internal/modules/commerce/payment/repository.go`
- Modify: `server/internal/modules/commerce/payment/repository_integration_test.go`
- Modify: `server/internal/modules/commerce/payment/late_callback_integration_test.go`
- Modify: `server/internal/modules/commerce/invitereward/repository_integration_test.go`

- [x] **Step 1：编写错误分类失败测试。** 区分未知订单、未知凭据、PID 错配、签名失败、快照不匹配、重放、依赖失败和事务失败；业务拒绝与可重试故障不得混淆。
- [x] **Step 2：重构 Service。** 按订单凭据选择 Secret，返回稳定分类结果；未知订单只在 `sql.ErrNoRows` 时拒绝，数据库故障必须进入 503 路径。
- [x] **Step 3：首次成功同事务。** 钱包流水、首充奖励、订单 `paid` 和当前审计 `success/200/success/completed_at` 同成同败，审计更新条件仍要求 `received`。
- [x] **Step 4：合法幂等独立终态。** 已支付订单只有所有支付快照完全一致才把本次新审计转为 `idempotent`；不同交易号、链上哈希、金额、地址或 Token 均拒绝。
- [x] **Step 5：故障矩阵。** 注入账号锁、钱包、奖励、订单和审计成功更新故障，证明没有半入账或错误成功响应；回滚后审计保留 `received` 供失败最终化。
- [x] **Step 6：晚到和并发回归。** 五种允许晚到状态、重复回调和跨订单重放仍只产生一套资金事实；并发回调最多一次首次成功，其余只能合法幂等或拒绝。
- [x] **Step 7：验证并提交。** 运行 Payment/Wallet/InviteReward 定向测试、race 和真实 PostgreSQL 集成；提交信息：`实现支付回调审计事务终态`。

## Task 4：实现有限读取、HTTP 合同与重试故障路径

**文件：**

- Modify: `server/internal/modules/commerce/payment/http.go`
- Modify: `server/internal/modules/commerce/payment/http_test.go`
- Create: `server/internal/modules/commerce/payment/http_integration_test.go`
- Modify: `server/initialize/router_biz.go`
- Modify: `server/initialize/router.go`

- [x] **Step 1：编写有限读取失败测试。** 覆盖空请求、合法 JSON、非 JSON、嵌套值、恰好 16 KiB、16 KiB + 1 字节、底层读取错误和 payload 哈希/字节/截断结果。
- [x] **Step 2：复用 RequestMeta。** 从 logger Context 取得已校验的 request ID、trace ID 和 Client IP；测试路由必须显式安装 `middleware.RequestMeta()`，业务 Handler 不再自行信任原始头。
- [x] **Step 3：先 Begin 后解析。** Begin 失败直接 `503 fail`；超限、格式和业务拒绝只有最终化成功后返回 400/401；请求读取、依赖、事务或最终化失败返回 `503 fail`。
- [x] **Step 4：局部 panic 恢复。** Begin 后 panic 最终化为 `failed/PANIC`，结构化日志只含审计/request/trace ID 和固定码；最终化失败仍返回 503。
- [x] **Step 5：严格响应合同。** Content-Type 为纯文本；首次成功和完全一致幂等返回 `200 success`，其余按规格返回 `400/401/503 fail`。
- [x] **Step 6：本地重试替身。** 用回环 HTTP 客户端模拟 EPUSDT：仅在 `200 success` 停止，非 200 或非成功正文创建下一次独立审计；不连接真实支付服务。
- [x] **Step 7：组合根注入。** `initialize` 构造审计配置、Repository、Handler 和可选观察器；不引入 Redis、消息队列或 Worker。
- [x] **Step 8：验证并提交。** 运行 HTTP 单元/真实 PostgreSQL 集成、路由合同和 `go vet`；提交信息：`完善EPUSDT回调失败重试`。

## Task 5：扩展管理查询与脱敏详情

**文件：**

- Modify: `server/internal/modules/commerce/adminrechargeorder/types.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/callback.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/service.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/http.go`
- Modify: `server/internal/modules/commerce/adminrechargeorder/repository_integration_test.go`
- Modify: `web/src/api/reader/paymentCallbackLogs.js`
- Modify: `web/src/view/reader/payment/callbackLogs/index.vue`
- Create: `web/test/paymentCallbackLogs.test.js`
- Modify: `server/cmd/moonbook-browser-fixture/main.go`
- Modify: `server/cmd/moonbook-browser-fixture/main_test.go`

- [x] **Step 1：编写查询失败测试。** 固定关键词、五种运行时结果、历史结果、签名三态、失败码、HTTP 状态、明确时区、起止顺序、31 天跨度、稳定分页和参数化 SQL。
- [x] **Step 2：扩展安全 DTO。** 列表/详情输出新审计字段和白名单 snapshot；不得返回原始请求、签名、PID、IP、请求头或自由底层错误。
- [x] **Step 3：实现处理中断判定。** 仅 `source_type=runtime + received + 超过阈值` 为中断；manual/sync/legacy 和历史空字段不得误报。
- [x] **Step 4：实现签名三态。** `signature_valid=true` 为 valid，`SIGNATURE_INVALID` 为 invalid，其余为 not_checked；禁止把默认 false 全部显示为无效。
- [x] **Step 5：完善 Vue 页面。** 增加筛选、失败码/request ID/完成状态列和详情抽屉；保持紧凑管理布局、移动端不溢出，不提供入账、重放、补偿或编辑入口。
- [x] **Step 6：Long ID 测试。** API 与页面使用字符串和 `appendLongId`，以超过 `Number.MAX_SAFE_INTEGER` 的 ID 证明无精度损失。
- [x] **Step 7：浏览器夹具与验收。** 只创建可追踪测试行，桌面和移动视口验证筛选、详情、脱敏和中断标识；完成后精确清理夹具。
- [x] **Step 8：验证并提交。** 运行管理端真实 PostgreSQL 测试、Node 测试、ESLint、生产构建和 Playwright；提交信息：`完善支付回调审计管理页`。

## Task 6：增加回调指标、配置门和 Runbook

**文件：**

- Create: `server/internal/platform/metrics/payment_callbacks.go`
- Create: `server/internal/platform/metrics/payment_callbacks_test.go`
- Modify: `server/internal/platform/metrics/metrics.go`
- Modify: `server/internal/platform/metrics/metrics_test.go`
- Modify: `server/initialize/router.go`
- Modify: `server/initialize/router_biz.go`
- Create or Modify: `docs/runbooks/epusdt-callback.md`

- [x] **Step 1：编写指标失败测试。** 固定尝试结果 Counter、503 Counter、幂等重投 Counter、数据库滞留 Gauge 和采集成功 Gauge；label 只能是固定结果/响应类别。
- [x] **Step 2：实现低耦合 Observer。** payment 包定义窄接口，Platform Metrics 实现；指标关闭时使用 nil/no-op，不让 payment 依赖 Platform 实现。
- [x] **Step 3：实现数据库 collector。** 2 秒超时查询超过配置阈值的 runtime `received`；查询失败只暴露采集失败，不泄漏 SQL、ID 或错误文本 label。
- [x] **Step 4：验证低基数。** 指标正文不得包含订单号、交易号、request/trace ID、IP、凭据引用、失败文案或 payload 哈希。
- [x] **Step 5：编写 Runbook。** 记录滞留、rejected、failed、503、幂等增长和重试耗尽的含义、查询步骤、禁止自动入账边界及人工核查清单。
- [x] **Step 6：固定 EPUSDT 重试要求。** 明确 `callback_retry_base_seconds`、`order_notice_max_retry` 必须非零受控，最大间隔 5 分钟；仅 `200 ok/success` 停止重试。
- [x] **Step 7：验证并提交。** 运行指标单元/集成、配置合同、Compose 展开、Shell 语法和秘密扫描；提交信息：`增加EPUSDT回调审计指标`。

## Task 7：全量回归与 M4 证据收口

**文件：**

- Modify: `docs/verification/m4-exit-audit.md`
- Modify: `docs/inventory/admin-feature-matrix.md`
- Modify: `docs/progress/refactor-status.md`
- Modify: `docs/verification/final-report.md`
- Modify: this plan

- [ ] **Step 1：迁移门。** 空库、`00060 -> 00061`、重复执行、历史兼容和关键财务行数保护全部通过。
- [ ] **Step 2：后端质量门。** Payment/Recharge/Wallet/InviteReward/AdminRechargeOrder 定向测试、完整 Commerce/Reader、race、vet、模块边界和 SQL 所有权通过。
- [ ] **Step 3：真实依赖门。** 使用项目专用 PostgreSQL、Redis、MinIO 完成资金事务、回调重试、指标和管理查询回归。
- [ ] **Step 4：冻结 Reader 门。** 冻结树差异为零，536 项测试、树外 SSR/SEO、充值合同和生产构建通过。
- [ ] **Step 5：管理前端门。** Node 测试、ESLint、生产构建、桌面与移动浏览器验收通过，产物不含敏感 payload。
- [ ] **Step 6：Compose 与安全门。** 配置合同、Compose 模型、Shell、模块边界、秘密扫描和临时凭据/数据清理通过。
- [ ] **Step 7：更新证据。** 记录固定 commit、工具版本、命令摘要、迁移结果、故障矩阵、重试次数、指标样例和浏览器验收；不伪造真实付款结果。
- [ ] **Step 8：更新 M4 结论。** 只勾选“每次回调尝试完整审计”；两个运营页签、全域核对和受控真实支付继续为 No-Go。
- [ ] **Step 9：最终检查与提交。** 检查全部 tracked/untracked、敏感信息、`git diff --check` 和暂存区；提交信息：`补充EPUSDT回调审计验收证据`。
- [ ] **Step 10：状态确认。** `git status` 干净且未 push，继续下一个最靠前的 M4 退出项。

## 服务影响

计划文档本身不影响服务，无需重启。实施完成后需要执行新增 PostgreSQL 前向迁移，重建并替换 `moonbook-server` 与 `moonbook-web`；回调 Handler、管理 API、指标和滞留配置随 Server 重启生效，管理页面随 Web 替换生效。冻结 Reader、Redis 和 MinIO 不因本切片单独重启或迁移。
