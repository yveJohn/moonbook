# Moonbook GM Pay v2 创建响应兼容与诊断设计

## 1. 文档状态

- 日期：2026-08-28
- 状态：设计已获用户批准
- 范围：GM Pay v2 创建响应校验、充值订单失败诊断、管理订单详情
- 影响服务：`server`、管理前端 `web`

## 2. 已确认问题

生产环境使用 GM Pay v2.0.0，镜像源码修订为
`619cecb11d6bbd1dcdd1b698ee08b37fc722d3e3`。GM Pay 创建订单时会把
`currency` 和 `token` 规范化为大写，并在成功响应中返回 `USD` 和 `USDT`。

Moonbook 当前创建客户端只接受严格小写 `usd` 和 `usdt`。GM Pay 已成功创建
订单、分配交易号和收款地址后，Moonbook 因大小写不一致将响应归类为
`RESPONSE_MISMATCH`，不保存任何网关支付字段，订单进入 `gateway_unknown`，
Reader 因 `receiveAddress` 为空而无法生成二维码。

现有诊断还有两个缺口：

1. 所有成功响应字段校验失败都收敛为同一个 `RESPONSE_MISMATCH`，不能定位字段；
2. `gateway_unknown` 自动过期时，原始创建失败码会被 `ORDER_EXPIRED` 覆盖，
   管理订单详情也没有返回或展示失败码和失败原因。

## 3. 方案选择

采用“有限大小写兼容、其余财务字段保持严格、字段级脱敏诊断”的方案。

未采用以下方案：

- 仅使用大小写不敏感比较：能修复当前订单，但不能改善后续协议故障定位；
- 统一规范化或放宽所有响应字段：会削弱订单号、金额、状态、时间和 URL 的
  财务一致性校验，不符合支付安全边界。

## 4. 创建响应校验

`currency` 和 `token` 使用大小写不敏感的精确语义比较：

- `currency` 仅接受任意大小写组合的 `USD`；
- `token` 仅接受任意大小写组合的 `USDT`。

以下字段继续执行现有严格校验，不扩大兼容范围：

- `trade_id` 非空、无首尾空白且长度合法；
- `order_id` 与本地订单号完全一致；
- `amount` 与本地基础定价数值一致；
- `actual_amount` 为正且精度合法；
- `receive_address` 非空、无首尾空白且长度合法；
- `status` 必须为等待支付状态 `1`；
- `expiration_time` 必须是未来的秒级 Unix 时间戳；
- `payment_url` 必须是合法的绝对 HTTP/HTTPS URL。

Moonbook 只把经过完整校验的响应写入充值订单。不得因能够提取收款地址而部分
接受不可信响应。

## 5. 字段级失败诊断

创建客户端为响应字段校验失败返回稳定、脱敏的失败码。至少包括：

- `RESPONSE_TRADE_ID_MISMATCH`
- `RESPONSE_ORDER_ID_MISMATCH`
- `RESPONSE_AMOUNT_MISMATCH`
- `RESPONSE_CURRENCY_MISMATCH`
- `RESPONSE_ACTUAL_AMOUNT_MISMATCH`
- `RESPONSE_ADDRESS_MISMATCH`
- `RESPONSE_TOKEN_MISMATCH`
- `RESPONSE_STATUS_MISMATCH`
- `RESPONSE_EXPIRATION_MISMATCH`
- `RESPONSE_PAYMENT_URL_MISMATCH`

响应 envelope 或 `data` 不是合法 JSON/对象时继续使用现有
`INVALID_RESPONSE`。失败摘要只能说明哪个字段不符合协议，不得包含原始响应体、
完整 URL 查询参数、签名、PID、Secret、Cookie、Token 或网络堆栈。

字段级失败仍属于结果不确定，因为 GM Pay 可能已经创建订单；本地订单继续进入
`gateway_unknown`，不得自动重试创建请求。

## 6. 过期与管理端可见性

`gateway_unknown` 到达安全释放窗口后仍转为 `expired` 并释放活动订单限制，但若
订单已有创建失败码和失败原因，过期更新必须保留它们。没有既有诊断的订单才写入
`ORDER_EXPIRED` 和固定过期原因。

管理充值订单查询和详情响应新增现有数据库字段：

- `failureCode`
- `failureMessage`

管理前端在订单详情中只读展示这两个字段。列表、筛选、人工补单和主动同步行为
不因本次修复改变。冻结的 `reader-ui` 不修改；Reader 兼容响应已经包含失败字段。

## 7. 测试与验证

后端测试至少覆盖：

- GM Pay v2 返回大写 `USD/USDT` 时创建成功并保存交易号、实际金额、收款地址、
  支付链接和过期时间；
- 合法的大小写组合均被接受，其他币种或 Token 被拒绝；
- 每个关键字段不匹配时产生对应的稳定失败码并进入 `gateway_unknown`；
- 失败摘要不包含原始响应和敏感请求字段；
- `gateway_unknown` 过期后保留原始失败诊断；
- 没有原始失败诊断的过期订单仍记录 `ORDER_EXPIRED`；
- 管理订单 API 返回失败码和失败原因；
- Long ID 继续作为 JSON 字符串输出。

管理前端测试至少覆盖订单详情展示失败码和失败原因。验证执行相关 Go 单元测试、
真实 PostgreSQL 集成测试、管理前端测试与生产构建，并执行格式和静态检查。

## 8. 发布与既有订单

本次不新增数据库迁移。发布需要更新并重启 `server` 和管理前端 `web`；
`reader-ui`、EPUSDT、PostgreSQL、Redis 和 MinIO 无需重启。

已在 GM Pay 过期且本地没有保存网关字段的历史订单不得恢复为可支付状态，也不得
自动补单。修复发布后使用新的未付款测试订单验证二维码、金额、地址和过期时间，
不得向已过期地址付款。
