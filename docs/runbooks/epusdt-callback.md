# EPUSDT 回调审计与故障处置

## 范围与边界

本 Runbook 用于 Moonbook EPUSDT 回调的滞留、拒绝、基础设施失败、HTTP 503、合法幂等重投和网关重试耗尽处置。回调审计、Prometheus 指标和管理页面只提供调查证据，不能单独证明链上到账，也不得触发自动入账、自动补单、自动重放或自动补偿。

生产排查不得记录或传播 EPUSDT Secret、PID、完整签名、原始请求体、请求头、明文来源 IP 或数据库连接串。订单号、交易号、Request ID 和 Trace ID 只能进入受控工单和管理页面，不得成为 Prometheus label。

## 网关重试要求

EPUSDT 生产配置中的 `callback_retry_base_seconds` 和 `order_notice_max_retry` 必须设置为非零、受控的正整数。指数退避的最大间隔必须限制为 5 分钟。每次变更都要记录配置值、变更人、变更时间、回滚值及 EPUSDT 版本，并在非生产环境先验证。

只有 Moonbook 回调同时返回 HTTP `200` 且正文为 `ok` 或 `success` 时，EPUSDT 才能停止重试。任何非 200 响应、空正文、其他正文、超时或连接失败都必须继续按上述策略重试，直至成功或达到 `order_notice_max_retry`。Moonbook 不实现第二套内部回调重试队列。

重试耗尽必须告警并进入人工核查，不得把“重试耗尽”解释为支付失败或未到账。核查完成前不得修改订单、钱包、流水、奖励或权益事实。

## 指标

指标通过受 Bearer Token 保护的 `GET /api/metrics` 抓取：

| 指标 | 含义 |
| --- | --- |
| `moonbook_payment_callback_attempts_total{result="received"}` | 审计开始记录已独立提交的累计次数 |
| `moonbook_payment_callback_attempts_total{result="success"}` | 首次成功入账且审计终态同事务提交的累计次数 |
| `moonbook_payment_callback_attempts_total{result="idempotent"}` | 已支付订单收到完全一致合法重投的累计次数 |
| `moonbook_payment_callback_attempts_total{result="rejected"}` | 格式、凭据、签名、快照或防重放校验拒绝的累计次数 |
| `moonbook_payment_callback_attempts_total{result="failed"}` | 读取、依赖、事务或 panic 等可重试故障终态的累计次数 |
| `moonbook_payment_callback_responses_503_total` | 实际返回 HTTP 503 的累计次数，包含无法开始或无法最终化审计的请求 |
| `moonbook_payment_callback_idempotent_retries_total` | 合法幂等重投累计次数，与 `result="idempotent"` 同步增长 |
| `moonbook_payment_callback_stale_received` | 超过 `MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES` 的 `runtime/received` 当前数量 |
| `moonbook_payment_callback_metrics_collection_success` | 最近一次滞留 SQL 采集是否成功，`1` 成功、`0` 失败 |

Counter 在 Server 重启后归零符合 Prometheus 语义，告警应使用 `rate` 或 `increase`，不能比较不同进程生命周期的绝对值。`collection_success=0` 时滞留数量未知；采集器不会输出旧值或数据库错误文本。

建议至少配置以下告警，窗口和阈值需按生产基线校准：

```promql
moonbook_payment_callback_stale_received > 0
min_over_time(moonbook_payment_callback_metrics_collection_success[5m]) == 0
increase(moonbook_payment_callback_responses_503_total[10m]) > 0
increase(moonbook_payment_callback_attempts_total{result="failed"}[10m]) > 0
```

`rejected` 和幂等重投必须另建相对历史基线的异常增长告警；阈值应由至少一个正常业务周期的数据确定，并经过演练，不能在没有基线时使用任意固定数值。

## 调查步骤

1. 确认 `moonbook_payment_callback_metrics_collection_success` 为 `1`，并记录告警开始时间、Server 发布版本和配置版本。
2. 在管理端“小说管理 / 读者支付 / 回调日志”按明确时区的时间范围查询。优先筛选 `received`、`failed`、`rejected`、失败码和 HTTP 状态；单次范围不超过 31 天。
3. 使用 Request ID、Trace ID、商户订单号和网关交易号关联受控应用日志与订单事实。不得把这些值加入指标 label 或普通聊天记录。
4. 对 `received` 滞留确认 `source_type=runtime`，检查同一次尝试是否缺少 `completed_at`。manual、sync 和 legacy 记录不属于运行时滞留。
5. 对 `rejected` 按固定失败码核查输入合同、当前或历史凭据引用、PID 快照、签名校验和防重放结果。拒绝增长可能是攻击、错误配置或旧凭据轮换问题，不等同于基础设施故障。
6. 对 `failed` 或 503 检查 PostgreSQL 可用性、事务错误率、Server readiness、连接池和最近发布。HTTP 503 可能发生在审计 Begin 之前或最终化失败之后，因此 503 数量可以大于 `failed` 终态数量。
7. 对幂等增长确认每次重投都有独立审计，订单、钱包流水、首充奖励和权益只存在一套事实。增长异常通常表示 EPUSDT 未收到严格的 `200 success`、网络不稳定或网关重试配置异常。
8. 对重试耗尽取得 EPUSDT 侧尝试次数、最后响应状态和时间，再与 Moonbook 审计及链上交易核对。不得请求或保存网关 Secret、完整签名或原始回调报文。

只读 SQL 应通过受控 PostgreSQL 会话执行，时间必须带明确时区：

```sql
SELECT processing_result, failure_code, response_status, count(*)
FROM reader_payment_callback_logs
WHERE source_type = 'runtime'
  AND request_time >= TIMESTAMPTZ '2026-08-20T00:00:00+08:00'
  AND request_time <  TIMESTAMPTZ '2026-08-21T00:00:00+08:00'
GROUP BY processing_result, failure_code, response_status
ORDER BY processing_result, failure_code, response_status;

SELECT count(*) AS stale_received
FROM reader_payment_callback_logs
WHERE source_type = 'runtime'
  AND processing_result = 'received'
  AND request_time < now() - interval '5 minutes';
```

第二条 SQL 的间隔必须替换为当前 `MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES`，并与运行实例配置核对。

## 人工核查清单

- 核对链上交易号、Token、网络、收款地址、订单金额和实际金额；不得用浮点数比较金额。
- 核对订单当前状态、网关交易号和链上交易号是否被其他订单使用。
- 核对钱包余额、不可变流水、首充奖励、会员和权益事实是否同成同败。
- 核对本次尝试的 `processing_result`、`failure_code`、HTTP 状态、Request ID、Trace ID、请求时间和完成时间。
- 对历史凭据回调核对订单持久化的凭据引用与 PID 快照，不使用当前 Secret 猜测历史事实。
- 保存调查结论、证据时间范围和操作人；任何人工补单必须走既有受控入口和幂等 Request ID，并接受独立复核。

以下操作一律禁止：直接修改订单为 `paid`、直接增加钱包余额、删除或改写回调审计、根据 `received/failed/rejected` 自动入账、向生产重放捕获的原始请求，以及绕过受控人工补单接口写数据库。

## 恢复与关闭

修复依赖或配置后，观察至少一个完整告警窗口：采集成功保持 `1`、滞留归零、503 与 `failed` 不再增长，`rejected` 和幂等重投恢复基线。关闭事件前必须确认财务事实一致、临时访问权限已撤销、调查导出物按安全策略归档或销毁，并记录是否发生 EPUSDT 重试耗尽。
