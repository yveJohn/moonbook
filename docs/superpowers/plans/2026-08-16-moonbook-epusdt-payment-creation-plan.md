# Moonbook EPUSDT 支付创建闭环实施计划

**目标：** 为 Go Commerce 模块补齐 EPUSDT 同步创建、请求幂等、订单替换、失败分类、过期释放和历史凭据验签基础，保持冻结 Reader 业务代码与接口契约不变。

**架构：** `recharge.Service` 编排 PostgreSQL 短事务和事务外网关调用，`recharge.Repository` 只负责锁与状态事实，`epusdt.Client` 只负责签名、HTTP 和协议解析，`epusdt.CredentialProvider` 只从环境 Secret 解析当前及历史凭据。`initialize` 是唯一组合根并管理过期 Worker 生命周期。

**技术栈：** Go、Gin、PostgreSQL 17、现有 Goose Runner、HMAC-SHA256、`net/http`、真实 PostgreSQL 集成测试、本地 HTTP 替身、Docker Compose、冻结 React Reader 契约。

**批准规格：** `docs/superpowers/specs/2026-08-16-moonbook-epusdt-payment-creation-design.md`

---

## 执行规则

- [ ] 只修改活跃仓库，不修改冻结仓库或 `reader-ui` 业务代码。
- [ ] 不连接生产、不使用真实支付凭据、不执行真实支付、正式迁移、切流或 `git push`。
- [ ] 每个任务先写失败测试并确认失败原因，再做最小完整实现；不得删除、跳过或放宽既有测试。
- [ ] PostgreSQL 迁移只新增，不修改、改名或删除 `00001` 至 `00059`；创建前必须重新检查最高版本。
- [ ] Go 测试使用本机 JDK 无关；Go 命令在 `server` 下执行，使用项目现有 Go 工具链和隔离 `GOCACHE`。
- [ ] 真实集成测试只使用项目专用 PostgreSQL，不使用 SQLite；本地 HTTP 替身只监听回环地址。
- [ ] Secret 不进入源码、数据库、日志、快照或测试报告；`.env.example` 只含空值或明确非秘密示例。
- [ ] Long ID 继续在 Reader JSON 和管理前端按字符串处理。
- [ ] 每个逻辑单元验证后检查全部 tracked/untracked 修改，使用简体中文独立提交。
- [ ] 本切片完成不等于 M4 完成，回调全尝试审计、运营审计页签、全域核对和真实支付外部验收继续保留。

## Task 1：新增支付创建迁移合同

**文件：**

- Create: `server/internal/platform/migrate/migrations/00060_epusdt_payment_creation.sql`
- Modify: `server/internal/platform/migrate/migrate_test.go`
- Modify: `server/internal/platform/migrate/migrate_integration_test.go`

- [x] **Step 1：确认版本。** 检查迁移目录最高版本仍为 `00059`；若已被并发工作占用，按真实最高版本顺延，禁止重复或倒序。
- [x] **Step 2：编写失败合同测试。** 要求迁移新增 `credential_ref`、`merchant_pid_snapshot`、`active_reader_id`、三类部分唯一索引、历史活动订单确定性收敛和 forward-only Down。
- [x] **Step 3：验证预期失败。** 运行迁移清单/合同测试，失败原因只能是 `00060` 尚不存在或合同未实现。
- [x] **Step 4：实现前向迁移。** 不删除事实；重复网关交易号或链上哈希必须显式失败；每个读者仅保留最新活动订单，其余转 `superseded` 并记录稳定原因。
- [x] **Step 5：真实 PostgreSQL 验证。** 覆盖空库、从 `00059` 带多活动订单升级、重复执行 `applied=0`、索引拒绝重复、关键订单/钱包/流水/权益行数不减少。
- [x] **Step 6：提交。** 提交信息：`新增EPUSDT支付创建迁移`。

## Task 2：建立环境凭据与配置合同

**文件：**

- Create: `server/internal/modules/commerce/epusdt/config.go`
- Create: `server/internal/modules/commerce/epusdt/credential.go`
- Create: `server/internal/modules/commerce/epusdt/config_test.go`
- Modify: `.env.example`
- Modify: `compose.yaml`
- Modify: `server/config/config_contract_test.go`

- [x] **Step 1：编写失败测试。** 覆盖禁用通道无需支付配置、启用时必填项、当前凭据、历史 JSON、重复/空标识、URL、超时、安全窗口和错误脱敏。
- [x] **Step 2：实现 CredentialProvider。** 当前凭据可创建和验签；历史凭据只可按 `credential_ref` 验签；订单引用为空时仅兼容当前凭据。不得输出 PID、Secret 或原始 JSON。
- [x] **Step 3：实现配置解析。** URL 必须为绝对 HTTP/HTTPS 且无用户信息/片段；超时和安全窗口有明确上下限，总超时不得小于连接超时。
- [x] **Step 4：更新环境合同。** Compose Server/Migrate/Bootstrap 使用同一环境锚点透传变量；示例文件不写秘密，历史 JSON 默认 `[]`。
- [x] **Step 5：验证。** 运行定向 Go 测试、配置渲染测试、`docker compose config`、Shell 语法和秘密扫描。
- [x] **Step 6：提交。** 提交信息：`增加EPUSDT环境凭据配置`。

## Task 3：实现 EPUSDT 协议客户端

**文件：**

- Create: `server/internal/modules/commerce/epusdt/signer.go`
- Create: `server/internal/modules/commerce/epusdt/client.go`
- Create: `server/internal/modules/commerce/epusdt/types.go`
- Create: `server/internal/modules/commerce/epusdt/signer_test.go`
- Create: `server/internal/modules/commerce/epusdt/client_test.go`

- [x] **Step 1：签名测试先失败。** 固定旧实现向量、ASCII 字段排序、空值/签名排除、`1.00` 尾零、Unicode 名称和常量时间验签。
- [x] **Step 2：HTTP 替身测试先失败。** 固定表单字段、Content-Type、禁止重试、同源重定向、跨主机/HTTPS 降级拒绝及有限响应体。
- [x] **Step 3：响应与错误测试先失败。** 覆盖成功、明确业务拒绝、非 2xx、连接/读取失败、超时、非法/过大 JSON，以及订单号、金额、币种、Token、状态、地址、实际金额、过期时间和支付 URL 不匹配。
- [x] **Step 4：实现精确协议。** 金额使用字符串与 `math/big`，禁止浮点；请求固定 `usd/usdt/tron` 和两位基础金额；响应实际金额最多八位。
- [x] **Step 5：实现稳定错误分类。** 明确未创建为 definite，其他发出请求后的不可证明结果为 uncertain；错误只保留稳定代码和脱敏摘要。
- [x] **Step 6：验证并提交。** 运行 `go test ./internal/modules/commerce/epusdt -v`、race 和 vet；提交信息：`实现EPUSDT支付协议客户端`。

## Task 4：重构订单开始事务与幂等替换

**文件：**

- Modify: `server/internal/modules/commerce/recharge/types.go`
- Modify: `server/internal/modules/commerce/recharge/repository.go`
- Modify: `server/internal/modules/commerce/recharge/service.go`
- Modify: `server/internal/modules/commerce/provider/recharge.go`
- Modify: `server/internal/modules/commerce/provider/reader_compat_test.go`
- Create: `server/internal/modules/commerce/recharge/service_test.go`
- Modify: `server/internal/modules/commerce/recharge/repository_integration_test.go`
- Modify: `server/initialize/router_biz.go`

- [x] **Step 1：应用服务测试先失败。** Repository 替身固定“已存在同请求”“返回活动未知订单”“新建订单”三种结果；只有新建结果允许调用网关。
- [x] **Step 2：真实事务测试先失败。** 覆盖 Reader advisory lock、相同请求幂等、不同请求替换 `pending`、`creating/gateway_unknown` 不替换、到期活动订单释放和唯一冲突重读。
- [x] **Step 3：实现 Start。** 报价、通道与配置验证在创建前完成；替换旧单和插入 `creating` 同事务；订单号使用 `RC{id}` 且不超过 32 字符。
- [x] **Step 4：实现 Service 编排。** 事务外调用 Client，成功/失败由 Repository 在新事务按订单锁提交；完成事务失败绝不重新调用网关。
- [x] **Step 5：保持兼容 DTO。** `credential_ref`、PID 和内部错误不进入 Reader；Long ID、空值、八状态和日期格式不变。
- [x] **Step 6：组合根注入。** `initialize` 是唯一构造配置、凭据、Client、Repository 和 Service 的位置。
- [x] **Step 7：验证并提交。** 运行 Recharge、Provider、Reader Commerce 兼容测试及真实 PostgreSQL 集成；提交信息：`补齐充值订单创建状态机`。

## Task 5：补齐并发创建与网关完成原子性

**文件：**

- Modify: `server/internal/modules/commerce/recharge/repository_integration_test.go`
- Create: `server/internal/modules/commerce/recharge/create_gateway_integration_test.go`
- Modify: `server/internal/modules/reader/commercecompat/http_contract_test.go`

- [x] **Step 1：八路并发测试。** 同一 `reader_id + request_id` 八路创建只产生一条订单并只调用一次本地网关替身。
- [x] **Step 2：不同请求并发测试。** 最终最多一条活动订单；旧 `pending` 被替换，`creating/gateway_unknown` 不能被覆盖；不得把其他金额订单作为同请求结果。
- [x] **Step 3：完成失败测试。** 网关已成功而数据库完成事务失败时，响应为可关联错误，订单保持可同步状态且网关调用次数仍为一。
- [x] **Step 4：协议快照测试。** 成功订单完整保存交易号、实际金额、地址、支付 URL、网关状态、过期时间、凭据引用和 PID 快照。
- [x] **Step 5：Reader 合同测试。** 创建成功、确定失败和结果未知返回冻结 JSON；内部字段和秘密不泄漏。
- [x] **Step 6：验证并提交。** 提交信息：`验证EPUSDT并发创建幂等`。

## Task 6：接入订单过期生命周期

**文件：**

- Create: `server/internal/modules/commerce/recharge/expiry.go`
- Create: `server/internal/modules/commerce/recharge/expiry_test.go`
- Create: `server/internal/modules/commerce/recharge/expiry_integration_test.go`
- Create: `server/initialize/recharge_expiry.go`
- Modify: `server/initialize/reload.go`
- Modify: `server/core/server.go`

- [x] **Step 1：失败测试。** 覆盖启动即扫描、有限批量、周期运行、Context 停止、重复运行幂等和错误后继续下一周期。
- [x] **Step 2：状态测试。** `pending` 按网关过期时间、无交易号 `gateway_unknown` 和陈旧 `creating` 按安全窗口转 `expired` 并释放活动读者；未到期及 `paid/create_failed` 不变。
- [x] **Step 3：实现惰性兜底。** Create/Get 在读者锁内先过期该读者到期活动单，周期任务延迟不阻塞新订单。
- [x] **Step 4：实现生命周期。** 启动和配置重载前停止旧 Worker，启动新 Worker；优雅停机等待退出，不泄漏 goroutine。
- [x] **Step 5：真实 PostgreSQL 验证。** 周期和惰性路径均证明订单释放且新订单可创建。
- [x] **Step 6：提交。** 提交信息：`增加充值订单过期任务`。

## Task 7：按订单凭据验证晚到回调

**文件：**

- Modify: `server/internal/modules/commerce/payment/types.go`
- Modify: `server/internal/modules/commerce/payment/service.go`
- Modify: `server/internal/modules/commerce/payment/http.go`
- Modify: `server/internal/modules/commerce/payment/repository.go`
- Modify: `server/internal/modules/commerce/payment/repository_integration_test.go`
- Modify: `server/initialize/router_biz.go`

- [ ] **Step 1：凭据解析失败测试。** 新订单必须按 `credential_ref` 和 PID 快照选择 Secret；历史订单空引用使用当前凭据；未知或 PID 不匹配拒绝且不入账。
- [ ] **Step 2：晚到状态测试。** 合法回调允许 `pending/gateway_unknown/superseded/expired/callback_exception -> paid`，仍拒绝 `create_failed`；重复回调幂等。
- [ ] **Step 3：资金事务测试。** 每种允许状态只产生一条充值流水、一条首充奖励事实和一个 `paid` 终态，失败全部回滚。
- [ ] **Step 4：防重放测试。** 网关交易号、链上交易哈希和已支付订单不同哈希均不能重复入账。
- [ ] **Step 5：实现并验证。** 回调仍不记录 Secret/完整签名；本任务只补凭据选择和状态允许面，全尝试审计留在下一切片。
- [ ] **Step 6：提交。** 提交信息：`支持EPUSDT历史凭据回调`。

## Task 8：全量回归与验收证据

**文件：**

- Modify: `docs/verification/m4-exit-audit.md`
- Modify: `docs/inventory/admin-feature-matrix.md`
- Modify: `docs/progress/refactor-status.md`
- Modify: `docs/verification/final-report.md`
- Modify: this plan

- [ ] **Step 1：迁移验证。** 空库、`00059 -> 00060`、重复执行和关键行数保护全部通过。
- [ ] **Step 2：后端质量门。** 定向测试、完整 Commerce/Reader 模块测试、race、vet、模块依赖静态门通过。
- [ ] **Step 3：冻结 Reader 门。** 源树差异仍为零，536 个测试、生产构建和充值契约通过。
- [ ] **Step 4：Compose 门。** 配置合同、Compose 模型、Shell 语法、隔离栈创建/查询/管理可见性和秘密扫描通过。
- [ ] **Step 5：记录固定证据。** 写入 commit、工具版本、命令、迁移结果、网关替身场景、并发次数和剩余外部项。
- [ ] **Step 6：更新结论。** 只关闭支付创建、替换、失败和过期缺口；回调全尝试审计、运营页签、全域核对与真实支付继续未完成，M4 保持实施中。
- [ ] **Step 7：最终检查。** 检查全部 tracked/untracked、忽略产物、秘密、`git diff --check` 和暂存区。
- [ ] **Step 8：提交。** 提交信息：`补充EPUSDT支付创建验收证据`。
- [ ] **Step 9：状态确认。** `git status` 干净且未 push，继续下一 M4 退出项。

## 服务影响

计划文档本身不影响服务，无需重启。实施完成后需要执行新增 PostgreSQL 前向迁移并重建、替换 `moonbook-server`；Commerce 充值、支付回调配置和过期任务随 Server 重启生效。管理前端、冻结 Reader、Redis 和 MinIO 不因本切片单独重启或迁移。
