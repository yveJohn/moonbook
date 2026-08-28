# Moonbook EPUSDT 支付创建闭环设计

> 2026-08-28 更新：本文第 4、5 节的 EPUSDT 专属环境变量凭据方案已由 `2026-08-28-moonbook-payment-channel-crud-design.md` 替代。支付配置现由管理后台写入 PostgreSQL，并使用全项目唯一的 `MOONBOOK_APP_MASTER_KEY` 加密；本文其余支付状态机和协议约束继续有效。

## 1. 文档状态

- 日期：2026-08-16
- 状态：设计及书面规格已获用户分段确认
- 适用仓库：`/Users/yve/code/ai-project/moonbook`
- 目标：在不修改冻结 Reader 业务代码的前提下，为 Go Commerce 模块补齐 EPUSDT 同步创建、订单替换、失败状态、过期释放和历史凭据验签基础

本规格是 M4 交易、支付与运营的独立纵向切片。它以冻结仓库的 EPUSDT 设计、Java 参考实现和 Reader 现有状态页为事实来源，但按新仓库的 Go 模块边界、PostgreSQL 前向迁移和环境 Secret 注入规范实现。回调每次尝试审计、两个运营审计页签和财务全域核对器属于后续 M4 切片，本规格不提前将 M4 标记为完成。

## 2. 已确认事实

1. 冻结 Reader 创建充值订单后立即进入订单页，并依赖 `creating`、`pending`、`gateway_unknown`、`create_failed`、`superseded`、`expired`、`callback_exception`、`paid` 八种状态。
2. 订单页使用 `actualAmount` 和 `receiveAddress` 展示 TRC20-USDT 支付信息；`paid`、`superseded`、`expired`、`create_failed` 停止轮询，`creating`、`pending`、`gateway_unknown`、`callback_exception` 继续轮询。
3. 当前 Go `commerce/recharge.SQLRepository.CreateOrder` 只写本地 `pending`，没有调用 EPUSDT，也不会填充网关交易号、实际金额、收款地址或支付链接。
4. 当前回调成功路径已经使用验签、订单锁、钱包不可变流水、订单终态和首充奖励事务，但拒绝尝试审计尚不完整，后续单独整改。
5. 当前 PostgreSQL 表已经包含主要支付字段和八状态约束，但缺少活动读者、凭据引用、PID 快照及网关交易号/链上交易哈希唯一约束。
6. 当前 Compose 没有向 Server 注入已声明的 EPUSDT PID、Secret 和同步 URL，也没有创建、回调及 Reader 跳转配置。
7. 旧实现固定使用 USD 定价、USDT Token、TRON 网络，通过 EPUSDT GMPay `application/x-www-form-urlencoded` 创建交易；基础金额保留两位，网关实际金额最多八位且不得经过浮点数。

## 3. 范围

### 3.1 本切片包含

- 仓库内 EPUSDT 协议客户端、签名器、响应解析和脱敏错误分类；
- `creating` 本地订单、事务外同步网关调用和完成/失败状态提交；
- 请求幂等、同读者活动订单锁、不同请求替换和并发唯一性；
- `gateway_unknown`、`create_failed`、`superseded` 和 `expired` 运行路径；
- 活动订单定时过期与创建/查询时的惰性过期兜底；
- 环境 Secret 注入、历史验签凭据解析和订单协议快照；
- PostgreSQL 前向迁移、配置合同、Compose 变量及无秘密模板；
- 本地 HTTP 替身、真实 PostgreSQL、Reader 契约和应用生命周期测试；
- M4 审计、功能矩阵和进度证据更新。

### 3.2 本切片不包含

- 真实 EPUSDT 或生产支付操作；
- 退款、自动退币、其他链/Token、多支付渠道；
- 管理员直接确认支付、编辑支付字段或伪造 `paid`；
- 修改冻结 Reader 业务代码；
- 回调每次尝试完整审计、签到/邀请奖励审计页签和财务全域核对器；
- 生产连接、正式迁移、切流或 `git push`。

真实受控支付环境的最小金额五方一致验收保留为切换前外部证据，不替代本地实现和自动化测试。

## 4. 方案与边界

采用环境 Secret 注入与同步创建方案：

- 不把 Secret 保存到 PostgreSQL；
- 不照搬旧 Java 的 AES-GCM 数据库凭据表；
- 不把创建请求改为异步 Worker，因为冻结 Reader 需要创建响应中的支付地址；
- 订单保存凭据标识和 PID 快照，Secret Provider 从当前及历史环境 Secret 中按标识解析；
- 协议客户端与订单应用服务分离，HTTP 和签名逻辑不得读写业务表，Repository 不执行网络请求。

组件边界：

```text
Reader Commerce 兼容层
  -> recharge.Service
       -> recharge.Repository：报价、锁、订单和状态事务
       -> epusdt.Client：签名、表单、HTTP、响应解析
       -> epusdt.CredentialProvider：当前/历史环境 Secret
  -> PostgreSQL

payment callback
  -> 按订单 credential_ref 解析历史 Secret
  -> 现有钱包/首充奖励事务
```

## 5. 配置与凭据

### 5.1 当前凭据

保留并注入：

- `MOONBOOK_EPUSDT_PID`
- `MOONBOOK_EPUSDT_SECRET`

新增 `MOONBOOK_EPUSDT_CREDENTIAL_REF`，默认本地值可以是 `primary`。新订单只使用当前凭据，并保存 `credential_ref` 与 `merchant_pid_snapshot`。凭据标识是无敏感值的稳定引用，不得包含 PID 或 Secret。

### 5.2 历史验签凭据

新增 `MOONBOOK_EPUSDT_VERIFY_CREDENTIALS_JSON`，由 Secret 注入，结构固定为对象数组：

```json
[{"ref":"previous-2026-07","pid":"merchant-id","secret":"replace-at-deploy"}]
```

历史集合只允许回调验签，不能创建新订单。当前凭据标识不得与历史标识重复；标识、PID 或 Secret 为空、JSON 非法、重复标识均使支付配置无效并阻止启用运行路径。应用日志只输出凭据标识指纹，不输出 PID、Secret 或原始 JSON。

### 5.3 端点与超时

新增并由 Compose 透传：

- `MOONBOOK_EPUSDT_CREATE_URL`
- `MOONBOOK_EPUSDT_NOTIFY_URL`
- `MOONBOOK_EPUSDT_REDIRECT_URL`
- `MOONBOOK_EPUSDT_CONNECT_TIMEOUT_MS`
- `MOONBOOK_EPUSDT_REQUEST_TIMEOUT_MS`
- `MOONBOOK_EPUSDT_UNKNOWN_RELEASE_MINUTES`

创建 URL、回调 URL和跳转 URL必须是绝对 HTTP/HTTPS URL，不得包含用户信息或片段。生产部署应使用 HTTPS；本地集成测试允许回环 HTTP。回调和跳转 URL不得从请求 `Host`、`Origin` 或代理头临时拼接。连接与总超时必须为有界正整数，总超时不得小于连接超时。未知结果安全窗口默认 15 分钟，并应大于 EPUSDT 侧订单有效期。

EPUSDT 数据库通道未启用时不要求凭据完整；通道启用后，当前凭据、创建 URL、回调 URL和跳转 URL缺失或非法必须拒绝创建，不能退化为本地 `pending`。

## 6. 数据模型与迁移

当前迁移目录最高版本为 `00059`，本切片新增 `00060` Goose 前向迁移。创建文件前仍须重新检查实际最高版本，若并发工作已占用 `00060`，必须使用新的严格递增版本而不能改写已有迁移。迁移为 `reader_recharge_orders` 增加：

- `credential_ref varchar(100)`；
- `merchant_pid_snapshot varchar(255)`；
- `active_reader_id bigint`，引用 `reader_accounts(id)`。

增加允许空值的唯一索引：

- `gateway_trade_id` 非空且非空字符串时唯一；
- `block_transaction_id` 非空且非空字符串时唯一；
- `active_reader_id` 非空时唯一。

新活动状态集合为 `creating`、`pending`、`gateway_unknown`。`active_reader_id` 在活动状态时等于 `reader_id`，进入 `create_failed`、`superseded`、`expired`、`callback_exception` 或 `paid` 时清空。

迁移已有数据时：

1. 不删除或重写财务事实；
2. 每个读者的活动状态订单按 `created_at DESC, id DESC` 只保留最新一条；
3. 其余活动订单转为 `superseded`，设置 `failure_code=ORDER_REPLACED` 和固定脱敏文案；
4. 最新活动订单填充 `active_reader_id=reader_id`；
5. 历史订单凭据引用保持空值，现有回调继续使用当前凭据兼容；新订单必须填充引用和 PID 快照；
6. 建索引前验证不存在非空网关交易号或链上哈希重复；发现重复必须使迁移失败，不得静默清理。

迁移 Down 继续采用仓库的 forward-only 失败合同。空库首次迁移、从当前最高版本升级和重复执行 `applied=0` 都必须在真实 PostgreSQL 验证，关键订单、钱包、流水和权益行数不得减少。

## 7. EPUSDT 协议客户端

### 7.1 创建请求

客户端固定提交：

- `pid`
- `order_id`
- `currency=usd`
- `token=usdt`
- `network=tron`
- `amount`，严格两位十进制文本
- `notify_url`
- `redirect_url`
- `name=钻石充值`
- `signature`

签名规则为：排除 `signature`，排除空值，字段名按字节字典序排列，以 `key=value` 和 `&` 连接，使用 HMAC-SHA256 输出小写十六进制。金额文本和 Unicode 名称必须原样参与签名。

请求使用 `application/x-www-form-urlencoded`。客户端设置连接和总超时、有限响应体、`Accept: application/json`，不自动重试创建请求。重定向只允许相同 scheme 与 host；跨主机、HTTPS 降级或超过限制的重定向视为结果不确定。

### 7.2 创建响应

成功只接受 HTTP 2xx、协议 `status_code=200` 和对象 `data`。必须结构化解析并校验：

- `trade_id` 非空且长度合法；
- `order_id` 与本地订单一致；
- `amount` 与基础定价数值相等；
- `currency=usd`、`token=usdt`；
- `actual_amount` 为正、最多八位小数；
- `receive_address` 非空且长度合法；
- `status=1`；
- `expiration_time` 是未来时间；
- `payment_url` 是合法 HTTP/HTTPS URL。

所有金额使用字符串加 `math/big` 或等价精确十进制实现，不使用 `float32/float64`。订单号使用数据库 ID 派生的 `RC{id}`，必须不超过 EPUSDT 32 字符限制。

### 7.3 错误分类

确定失败，订单转 `create_failed`：

- 已取得结构化 HTTP/协议业务拒绝，且响应明确表明未创建交易。

本地身份、配置、请求或报价在开始事务前无效时不创建订单，继续返回现有兼容错误；它不属于 `create_failed` 状态。

结果不确定，订单转 `gateway_unknown`：

- 连接在可能写出请求后中断；
- 总超时、响应读取失败或过大；
- 重定向被拒绝；
- HTTP/JSON/协议响应无法证明未创建；
- 创建成功响应字段与本地订单不匹配。

错误持久化只保留稳定代码和最多 500 字符的脱敏摘要。不得保存远程响应正文、完整 URL 查询、Secret、签名或网络堆栈。

## 8. 订单创建与替换

### 8.1 开始事务

1. 校验 Reader ID、请求 ID、档位或自定义钻石输入；
2. 读取启用通道和完整运行配置；
3. 在 PostgreSQL 事务中取得 Reader 级 advisory lock；
4. 先按 `(reader_id, request_id)` 查询，相同请求直接返回原订单；
5. 把到期的活动订单转为 `expired` 并释放 `active_reader_id`；
6. 锁定当前活动订单；
7. 当前订单为 `pending` 时转为 `superseded`，释放活动读者并记录 `ORDER_REPLACED`；
8. 当前订单为 `creating` 或 `gateway_unknown` 时直接返回该订单，不调用网关；
9. 根据后端档位或规则计算快照，插入 `creating` 新订单并提交。

同一请求幂等优先于订单替换。替换旧 `pending` 与插入新 `creating` 必须在同一事务。唯一冲突后只允许重新读取同请求或当前活动订单；不能返回其他金额的订单并声称本次请求成功，也不能生成第二个网关订单。

### 8.2 网关调用与完成

只有本次事务实际创建 `creating` 订单的调用方可在事务外调用网关。成功响应在新事务中按订单 ID 加锁，只有仍为 `creating` 才写入：

- 网关交易号；
- 实际金额；
- 收款地址；
- 支付链接；
- 网关状态；
- 网关过期时间；
- `status=pending`。

确定失败写 `create_failed` 并释放活动读者；结果不确定写 `gateway_unknown` 并继续占用活动读者。状态提交失败不能重新调用网关，应返回可关联错误并由查询/人工同步恢复。

Reader 创建接口在网关成功时返回 `pending` 支付信息；网关失败时返回已持久化的 `create_failed` 或 `gateway_unknown` 订单，使冻结 Reader 进入已有状态页。只有开始事务前的参数、身份、报价、通道或配置错误使用现有兼容错误响应。

## 9. 过期与晚到回调

状态转换固定为：

```text
creating -> pending | create_failed | gateway_unknown
pending -> superseded | expired | callback_exception | paid
gateway_unknown -> pending | expired | callback_exception | paid
superseded -> expired | callback_exception | paid
callback_exception -> paid
expired -> paid
```

`paid` 和 `create_failed` 不发生普通业务转换。`superseded`、`expired` 是 Reader 页面终态，但不是资金绝对终态；验签及订单快照校验成功的晚到回调仍按原订单幂等入账。

过期服务：

- 应用启动后执行一次；
- 运行期按固定短周期批量扫描；
- 使用数据库条件更新和有限批量，不依赖进程内唯一状态；
- `pending` 使用网关返回的 `expire_time`；
- 没有交易号的 `gateway_unknown` 使用 `created_at + unknown release window`；
- `creating` 超过同一安全窗口按结果不确定处理并释放为 `expired`；
- 创建和查询入口先执行单读者惰性过期，避免周期任务延迟阻塞新订单。

主动同步发现网关已支付但本站没有合法回调时只能转为 `callback_exception` 并告警，不能直接加钻石。`paid` 仍只能由合法回调的单一 PostgreSQL 事务完成钱包流水、首充奖励和订单终态。

## 10. API 与兼容

冻结路径和请求/响应合同不变：

- `POST /reader/me/recharge/orders`
- `GET /reader/me/recharge/orders/:id`

Long ID 继续输出 JSON 字符串。`actualAmount`、`receiveAddress`、`paymentUrl`、`expiresAt` 和失败字段保持原空值与时间格式。Reader 兼容层不得泄漏 EPUSDT 原始响应、内部错误、凭据标识或 PID。

冻结 Reader 业务代码不修改。现有状态页和轮询测试是本切片的回归门；如内部状态扩展，必须在兼容 DTO 前收敛为已冻结八状态之一。

## 11. 测试与验证

### 11.1 单元和协议测试

- 签名字典序、空字段排除、两位金额、Unicode 名称及固定官方/旧实现向量；
- 表单 Content-Type、字段集合和禁止自动重试；
- 精确金额、最多八位小数及非法科学计数/范围；
- 同主机重定向与跨主机/HTTPS 降级拒绝；
- 配置缺失、历史凭据 JSON、重复标识和日志脱敏；
- 所有响应字段不匹配和错误分类。

### 11.2 本地 HTTP 替身

覆盖：

- 成功创建；
- 明确业务拒绝；
- 连接失败、超时和断连；
- 非 2xx、过大响应、非法 JSON 和缺少 `data`；
- 订单号、基础金额、币种、Token、状态、地址、实际金额和过期时间不匹配；
- 同一请求重复或并发只调用一次网关。

替身只监听本地回环地址，不使用真实 PID、Secret 或支付网络。

### 11.3 真实 PostgreSQL

- 空库迁移、当前版本升级和重跑 `applied=0`；
- 迁移前后关键业务行数不减少；
- 相同请求幂等、不同请求替换旧 `pending`；
- `creating`、`gateway_unknown` 不被不同请求替换；
- 八路并发只存在一个活动订单且网关只调用一次；
- 唯一网关交易号、链上交易哈希和活动读者约束；
- 网关完成事务失败不重新下单；
- 周期与惰性过期释放；
- `superseded`、`expired` 的晚到合法回调只产生一条充值流水和一条首充奖励事实。

### 11.4 回归门

- Recharge/Payment/InviteReward 定向测试及完整 Commerce/Reader 模块测试；
- `go test -race`、`go vet` 和模块依赖静态门；
- 冻结 Reader 充值 API 契约、现有 536 个测试及生产构建；
- 配置渲染合同、Compose 模型、Shell 语法和秘密扫描；
- 隔离 Compose 栈 Reader 创建/查询和管理订单可见性冒烟。

不使用真实支付凭据，不以真实支付环境缺失跳过本地失败路径。

## 12. 提交与证据

实施按逻辑拆分为：

1. 设计规格；
2. 前向迁移、配置与凭据 Provider；
3. EPUSDT 协议客户端；
4. 订单状态机、替换和过期；
5. 真实集成、兼容回归与验收证据。

至少更新：

- `docs/verification/m4-exit-audit.md`；
- `docs/inventory/admin-feature-matrix.md`；
- `docs/progress/refactor-status.md`；
- `docs/verification/final-report.md` 中的 M4 状态说明。

证据必须明确真实 EPUSDT 最小金额验收未执行，M4 在回调审计、运营审计页签和全域核对器完成前仍为实施中。

## 13. 验收条件

本切片只有在以下条件全部满足后才完成：

- 启用通道不会再退化为仅本地 `pending`；
- EPUSDT 成功、确定失败和结果不确定都有真实运行路径；
- 同请求幂等及并发创建只调用一次网关；
- 不同请求正确替换旧 `pending`，未知创建结果不会被覆盖；
- 活动订单约束、过期释放和晚到回调资金保护有真实 PostgreSQL 证据；
- 网关字段、金额和状态全部按快照校验；
- 当前与历史 Secret 只由环境 Secret 注入，仓库和数据库无 Secret；
- Compose 完整透传配置，配置合同与秘密扫描通过；
- 冻结 Reader 业务代码无差异，充值契约和现有测试通过；
- 所有有效修改按功能使用简体中文提交，工作区干净且未 push；
- 剩余 M4 和真实支付外部验收项继续明确记录。

## 14. 服务影响

设计文档本身不影响服务，无需重启。实施完成后影响 `moonbook-server` 的 Commerce 充值、支付回调配置和过期任务；部署时需要执行新增 PostgreSQL 迁移并重建、替换 `moonbook-server`。管理前端与冻结 Reader 代码不因本切片修改，无需单独重建；PostgreSQL 需要前向迁移，Redis 和 MinIO 无需迁移或重启。
