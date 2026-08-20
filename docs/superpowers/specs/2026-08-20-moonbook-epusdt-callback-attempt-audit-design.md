# Moonbook EPUSDT 回调全尝试审计设计

## 1. 文档状态

- 日期：2026-08-20
- 状态：设计内容已获用户分段确认，书面规格待用户复核
- 适用仓库：`/Users/yve/code/ai-project/moonbook`
- 目标：在不修改冻结 Reader 业务代码的前提下，让每次 EPUSDT 异步回调在进入资金处理前持久化审计，并补齐脱敏查询、告警和失败重试边界

本规格是 M4 交易、支付与运营的独立纵向切片。它只补齐 EPUSDT 回调每次尝试审计，不把审计页扩展为资金补偿工具，也不包含签到/邀请奖励运营页签、财务全域核对或真实付款验收。完成本规格后，M4 仍须单独完成其余退出项。

## 2. 已确认事实

1. EPUSDT 官方约定只有商户回调返回 HTTP 200 且正文为 `ok` 或 `success` 才停止重试；其他响应按 `callback_retry_base_seconds` 指数退避，最多重试 `order_notice_max_retry`，最大间隔 5 分钟。
2. 当前合法成功回调会在同一 PostgreSQL 事务内写钱包流水、首充奖励、订单 `paid` 终态及一条成功日志，但格式错误、未知订单、验签失败、重放和依赖失败不会完整记录每次尝试。
3. 当前支付成功事务已覆盖当前凭据、历史凭据及 legacy 空凭据引用，支持五种晚到订单状态，并通过网关交易号和链上交易哈希防重放。
4. 当前 `reader_payment_callback_logs` 已包含业务标识、payload 哈希、脱敏快照、签名状态、处理结果、失败原因、响应状态、请求时间、来源类型及 legacy 来源引用。
5. 当前管理端已有支付回调日志列表和详情 API，但页面只支持关键词及有限结果筛选，没有详情入口，也不能区分接收中、幂等重投、业务拒绝和基础设施失败。
6. 当前 Server 已提供受保护的 Prometheus 指标端点，可以增加低基数的支付回调指标，不需要引入独立监控端点。
7. 管理前端的 PostgreSQL `bigint`/Go `int64` ID 必须始终作为字符串传递和展示，禁止经过 JavaScript `Number`。

## 3. 范围

### 3.1 本切片包含

- 每次 EPUSDT HTTP 回调独立创建审计记录；
- 审计先于解析、验签、订单查询和资金处理持久化；
- 成功、合法幂等、业务拒绝、鉴权拒绝和可重试故障的终态记录；
- 请求体上限、安全哈希、字段白名单及敏感信息脱敏；
- 审计记录与钱包、奖励和订单资金事务的一致提交；
- 审计失败时的 EPUSDT 可重试响应合同；
- 现有管理查询的筛选、列表字段和脱敏详情抽屉；
- 滞留审计、拒绝、失败、503 和幂等重投的 Prometheus 指标及 Runbook；
- Goose 前向迁移、真实 PostgreSQL 集成测试和冻结 Reader 回归。

### 3.2 本切片不包含

- 管理员从审计页面人工入账、重放、补偿或修改支付事实；
- 自动处理滞留 `received` 记录；
- Redis、消息队列、新 Worker 或异步资金处理；
- 签到奖励和邀请奖励运营审计页签；
- 订单、支付、钱包、奖励、会员和权益全域核对器；
- 真实 EPUSDT 最小金额支付；
- 修改冻结 Reader 业务代码；
- 生产连接、正式迁移、切流或 `git push`。

## 4. 方案与事务边界

采用“先持久化审计、失败即请求网关重试”的同步方案。不会采用先处理资金再补日志，也不会仅依赖结构化应用日志，因为两者都无法证明拒绝尝试和进程异常。

核心边界：

```text
EPUSDT HTTP callback
  -> 读取至多 16 KiB + 1 字节，生成安全摘要
  -> CallbackAuditRepository.Begin：独立提交 received
  -> 解析、查单、选择凭据、校验 PID/签名/快照
       -> 拒绝：FinalizeRejected 条件更新同一审计
       -> 合法：payment transaction
            -> 钱包流水
            -> 首充奖励
            -> 订单 paid
            -> 审计 success 或 idempotent
  -> 返回严格的 HTTP 状态和纯文本正文
```

### 4.1 开始审计

Handler 在接触业务服务前完成以下最小工作：

1. 从 `X-Request-ID` 和 `X-Trace-ID` 取得受控关联值，缺失时由服务端生成；外部值必须只含字母、数字、点、下划线或连字符且不超过 64 字符，不可信值由服务端重新生成；
2. 最多读取 16385 字节，以便判断 16 KiB 上限；
3. 计算捕获字节的 SHA-256、实际捕获字节数和截断标记；
4. 对来源 IP 做 SHA-256，不保存明文；
5. 调用 `CallbackAuditRepository.Begin`，以 `processing_result='received'` 独立提交。

在 Begin 成功前不得解析业务字段、查询订单、加载凭据、验签或进入资金处理。请求体超过 16 KiB 时不解析、不保存 snapshot，审计成功落为 `rejected/PAYLOAD_TOO_LARGE` 后返回 `400 fail`。底层请求读取异常属于可重试故障：使用已捕获字节创建审计并最终化为 `failed/REQUEST_READ_FAILED`，返回 `503 fail`。若连 Begin 也失败，直接返回 `503 fail`，不进入任何业务处理。

### 4.2 拒绝路径

格式错误、未知订单、快照不匹配、重放、未知凭据、PID 错配和签名失败都通过 `FinalizeRejected` 更新 Begin 创建的同一记录。更新条件必须包含审计 ID 和 `processing_result='received'`；受影响行数不是 1 视为最终化失败。

只有拒绝审计最终化成功后才能返回 `400 fail` 或 `401 fail`。最终化失败统一返回 `503 fail`，让 EPUSDT 自动重试。拒绝路径不得创建钱包流水、奖励事实或支付终态。

### 4.3 合法路径

首次合法回调在现有 PostgreSQL 资金事务中完成：

- 锁定并校验订单；
- 写入不可变充值钱包流水；
- 幂等授予首充奖励；
- 把订单转为 `paid`；
- 把当前审计从 `received` 更新为 `success`。

合法重复回调不再复用首次成功日志，而是在新的事务中把本次独立审计更新为 `idempotent`。只有订单快照、网关交易号、链上交易哈希和金额等事实与首次成功完全一致时才属于合法幂等；其他已支付订单回调仍按重放或快照不匹配拒绝。

资金事实和当前审计终态必须同一事务提交。任一步失败都回滚本次事务，审计随后尽最大努力从 `received` 最终化为固定失败码；无论该最终化成功与否，HTTP 都返回 `503 fail`，不得把未提交的资金处理报告为成功。

### 4.4 panic 与滞留记录

回调 Handler 在 Begin 成功后设置局部 panic 恢复边界。捕获 panic 后不得输出 panic 值、请求正文或凭据；只记录审计 ID、request ID、trace ID 和固定错误码，尝试将审计最终化为 `failed/PANIC`，并返回 `503 fail`。

进程被强制终止、数据库连接中断等情况仍可能留下 `received`。此状态是中断证据，不代表支付成功，也不得由定时任务自动入账。超过部署配置阈值的记录只进入管理标识和告警，由人工核查订单、钱包流水及链上状态。

## 5. 数据模型与迁移

当前最高迁移为 `00060_epusdt_payment_creation.sql`。实施时新增 `00061_epusdt_callback_attempt_audit.sql`；创建前必须再次检查最高版本，若版本已被占用则使用新的严格递增版本，不得修改已执行迁移。

迁移扩展 `reader_payment_callback_logs`：

- `failure_code varchar(64)`：稳定、非敏感的机器错误码；
- `request_id varchar(64)`：本次 HTTP 请求关联 ID；
- `trace_id varchar(64)`：链路关联 ID；
- `payload_bytes integer`：本次实际捕获并参与哈希的字节数；
- `payload_truncated boolean`：是否因 16 KiB 上限只捕获前 16385 字节；
- `completed_at timestamptz`：审计进入终态的时间。

运行时 EPUSDT 回调的 `processing_result` 固定为：

- `received`：已持久化，处理尚未形成终态；
- `success`：首次合法入账并与资金事实共同提交；
- `idempotent`：与既有成功事实完全一致的合法重投；
- `rejected`：格式、鉴权或业务规则拒绝；
- `failed`：应由 EPUSDT 重试的读取、审计、依赖、事务或 panic 故障。

`response_status` 在 `received` 阶段使用 0，终态时写入实际计划返回的 200、400、401 或 503；`response_body` 只允许 `success` 或 `fail`。`completed_at` 在 `received` 时为空，终态时与结果、失败码和响应字段同时写入。

运行时 `failure_code` 使用以下封闭集合，成功和幂等记录使用空值：

- `PAYLOAD_TOO_LARGE`、`REQUEST_READ_FAILED`、`INVALID_PAYLOAD`；
- `UNKNOWN_ORDER`、`UNKNOWN_CREDENTIAL`、`PID_MISMATCH`、`SIGNATURE_INVALID`；
- `SNAPSHOT_MISMATCH`、`REPLAY_DETECTED`；
- `DEPENDENCY_FAILED`、`TRANSACTION_FAILED`、`PANIC`。

实现需要新增分类时，必须同步更新规格对应的代码枚举、脱敏文案、测试和 Runbook，不能把底层错误文本临时当作失败码。

迁移不得删除、重写或重新分类历史日志。既有 `manual_success`、主动同步及 legacy 结果继续保留；新字段允许历史记录为空或采用无歧义默认值。管理查询必须按 `source_type` 和字段是否为空识别历史记录，不能把缺少 `completed_at` 的非运行时旧记录判为滞留。

为避免无界扫描，迁移应为管理筛选和滞留查询增加与实际 SQL 匹配的索引；不得为低选择性的布尔字段单独创建无效索引。Down 继续采用仓库 forward-only 失败合同。

## 6. 脱敏与快照

任何路径都不得保存或返回：

- EPUSDT Secret；
- 完整签名；
- 原始请求体或请求头；
- 明文 PID；
- 明文来源 IP；
- 未知字段或嵌套对象。

来源 IP 只保存 SHA-256。`payload_snapshot` 仅允许以下扁平字段：

- `order_id`；
- `trade_id`；
- `amount`；
- `actual_amount`；
- `receive_address`；
- `token`；
- `block_transaction_id`；
- `status`。

snapshot 构建必须显式使用白名单，删除 `signature`、`pid` 和所有未知字段，不能通过“复制后删除已知敏感字段”实现。非法 JSON/表单、嵌套值和超限 payload 的 snapshot 必须为空。快照只保存原始文本值，不做浮点转换；数据库 ID 和订单 ID 在 Go 与前端之间仍按字符串处理。

`failure_reason` 只写固定的脱敏运维文案，不拼接解析器错误、数据库错误、panic、请求字段、远程地址或签名。`failure_code` 使用有限枚举，指标不得把自由文本或业务 ID 用作 label。

## 7. 错误、响应与重试合同

回调响应固定为 `text/plain`：

| 场景 | HTTP | 正文 | 审计终态 |
| --- | ---: | --- | --- |
| 首次合法成功 | 200 | `success` | `success` |
| 完全一致的合法幂等重投 | 200 | `success` | `idempotent` |
| 格式错误、未知订单、快照不匹配、重放 | 400 | `fail` | `rejected` |
| 未知凭据、PID 错配、签名失败 | 401 | `fail` | `rejected` |
| 请求读取、审计、依赖、资金事务、最终化或 panic 失败 | 503 | `fail` | `failed`，或无法写入时无终态 |

每次 EPUSDT 重投都创建新的审计记录，不按 payload 哈希、订单号或 request ID 合并。EPUSDT 的重试承担基础设施故障恢复；Server 不实现第二套内部重试队列。

Runbook 必须要求 EPUSDT 生产配置使用非零且受控的 `callback_retry_base_seconds` 和 `order_notice_max_retry`，记录最大 5 分钟间隔，并对重试耗尽建立告警与人工核查流程。仓库测试和本地冒烟不得连接生产 EPUSDT。

## 8. 管理查询与兼容

复用现有菜单、API 和页面：

- `GET /reader/payment/callbackLogs`；
- `GET /reader/payment/callbackLogs/:id`；
- `web/src/view/reader/payment/callbackLogs/index.vue`。

列表扩展以下筛选：

- 关键词：商户订单号或网关交易号；
- 处理结果：`received`、`success`、`idempotent`、`rejected`、`failed`，同时兼容历史结果；
- 签名状态；
- 失败码；
- HTTP 响应状态；
- 回调时间范围。

时间范围使用明确时区的 ISO 8601 参数，后端校验起止顺序，单次查询跨度最多 31 天。分页继续有上限，所有筛选使用参数化 SQL。

列表增加失败码、request ID、完成时间和处理中断标识。详情通过现有详情 API 在抽屉展示：允许的业务快照、payload 哈希、字节数、截断标记、request ID、trace ID、请求时间、创建时间及完成时间。列表和详情都不返回本规格禁止的敏感原文。

签名筛选和展示使用三态：`valid` 对应 `signature_valid=true`；`invalid` 仅对应 `failure_code='SIGNATURE_INVALID'`；其余尚未执行或未能执行验签的记录为 `not_checked`，不得把数据库中的默认 `false` 直接显示为签名无效。

运行时 `source_type='runtime'`、`processing_result='received'` 且超过阈值的记录显示“处理中断”。阈值配置键固定为 `MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES`，默认 5 分钟，必须是 1 至 1440 的整数；历史、人工补单、主动同步和 legacy 记录不套用该判定。

审计页只读，不增加入账、重放、补偿或编辑按钮。详情 API 继续按字符串接收和输出 Long ID；前端使用 `appendLongId`，不得调用 `Number`、`parseInt`、算术运算或 `el-input-number` 处理这些 ID。

## 9. 指标与告警

在现有受保护的 Prometheus registry 中增加低基数指标，至少表达：

- 回调尝试按 `received`、`success`、`idempotent`、`rejected`、`failed` 的累计数；
- 当前超过阈值的滞留 `received` 数量；
- 回调响应 503 数量；
- 合法幂等重投数量。

进程内累计 Counter 用于请求趋势，数据库 collector 提供滞留 Gauge；重启导致 Counter 归零符合 Prometheus 语义。label 只允许固定结果或响应类别，不包含失败原因文本、订单号、交易号、request ID、trace ID、IP 或凭据引用。

告警至少覆盖：

- 存在超过阈值的 `received`；
- `failed` 或 503 在观察窗口持续增长；
- `rejected` 相对基线异常增长；
- EPUSDT 重试耗尽。

本切片负责指标、部署配置合同和 Runbook；完整 Prometheus/Alertmanager 部署仍归 M7 监控范围，不在本规格重复建设。

## 10. 测试与验收

### 10.1 迁移验证

- 空 PostgreSQL 执行到 `00061`；
- 从当前 `00060` 升级；
- 重复执行得到 `applied=0`；
- 迁移前后订单、钱包、流水、奖励和回调日志数量不减少；
- 历史、人工补单和主动同步日志不丢失、不被错误改写；
- 新约束、索引、空值和 forward-only Down 合同符合设计。

### 10.2 Handler 与脱敏测试

- 空请求、合法 JSON、非 JSON 格式、非法 JSON、嵌套值、恰好 16 KiB 和 16 KiB + 1 字节；
- payload 哈希、捕获字节数和截断标记准确；
- 白名单 snapshot 只保留八个允许字段；
- Secret、签名、PID、IP、请求头、未知字段及原始正文不落库、不进 API、不进日志；
- request ID 和 trace ID 的合法复用、非法替换及长度限制；
- Long ID 在后端 JSON 和管理前端中保持字符串精度。

### 10.3 状态与 HTTP 合同测试

- 每次请求独立创建 `received`；
- 首次成功转为 `success`；
- 完全一致的重复回调转为 `idempotent`；
- 格式、未知订单、快照不匹配和重放转为 `rejected` 并返回 `400 fail`；
- 凭据、PID 和签名问题转为 `rejected` 并返回 `401 fail`；
- 可重试故障返回 `503 fail`；
- 终态记录不能再次更新；
- 只有 `200 success` 满足本地替身的停止重试判定。

### 10.4 事务与故障注入

- Begin 失败时不解析、不查单、不进入资金处理；
- 拒绝最终化失败时返回 503；
- 钱包、奖励、订单或审计成功更新任一步失败时整笔资金事务回滚；
- 请求读取、数据库依赖及资金事务失败形成固定失败码；
- panic 被局部捕获，不造成入账、半完成订单或敏感日志；
- EPUSDT 替身收到非 200 或非成功正文后重投，并产生新的审计记录；
- 进程中断留下的 `received` 只触发标识和告警，不自动入账。

### 10.5 管理端与指标验收

- 新增筛选、列表字段和详情抽屉可用；
- 历史空字段展示正常，不误报处理中断；
- 页面不存在入账、重放或补偿入口；
- 敏感原文无法从列表或详情 API 获取；
- 指标能区分滞留、拒绝、失败、503 和幂等重投；
- Runbook 包含告警含义、排查步骤、EPUSDT 重试配置和重试耗尽处理。

### 10.6 最终回归

- Payment、Recharge、Wallet、InviteReward 和管理回调日志定向测试；
- 完整 Commerce/Reader 真实 PostgreSQL、Redis、MinIO 回归；
- `go test -race`、`go vet`、模块依赖静态门和秘密扫描；
- 冻结 Reader 接口兼容测试、536 项测试和生产构建；
- 管理前端 ESLint、生产构建和浏览器验收；
- Compose、配置渲染合同及 Shell 语法检查；
- 隔离 HTTP 替身验证回调重试，不连接生产 EPUSDT，不发起真实支付。

## 11. 完成条件与证据

只有以下条件全部满足，才能把 M4 的“回调每次尝试完整审计”标记为完成：

1. 每次 EPUSDT 回调在业务处理前独立持久化；
2. 成功、幂等、拒绝和失败路径均有脱敏终态证据；
3. 资金事实与成功审计保持同一事务；
4. 审计故障不会进入资金处理，且响应能触发 EPUSDT 重试；
5. 管理查询、指标、告警说明和 Runbook 可用；
6. 迁移、故障注入、完整回归及浏览器验收通过。

实施完成后至少更新：

- `docs/verification/m4-exit-audit.md`；
- `docs/inventory/admin-feature-matrix.md`；
- `docs/progress/refactor-status.md`；
- 与 EPUSDT 回调告警和排障对应的 Runbook。

受控真实 EPUSDT 最小金额支付、两个运营审计页签和财务全域核对仍是独立 M4 退出项，不能用本切片的本地证据替代。
