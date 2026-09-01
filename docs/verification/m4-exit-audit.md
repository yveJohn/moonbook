# M4 交易、支付与运营退出审计

审计日期：2026-08-20
固定代码基线：`009391c`

## 结论

M4 的本地代码与自动化退出条件已经满足。Reader 发起 EPUSDT 交易的创建、替换、失败分类、过期释放、并发幂等、历史凭据回调及每次 HTTP 回调尝试审计已闭环；签到/邀请奖励运营页签完成桌面/移动验收；订单、支付、钱包、奖励、会员和权益 Full 核对器在合成边界数据上通过并能检出断链。唯一未完成项是需要用户授权和真实凭据的 EPUSDT 最小金额支付，它按 `docs/runbooks/external-validation.md` 管理，不再记为代码缺口。

本轮没有使用真实支付凭据，没有执行付款、生产迁移、生产连接或流量切换。本地 HTTP 替身只能证明协议与应用状态机，不替代真实 EPUSDT 外部验收。

## 已证明能力

- 钱包余额由不可变流水维护，人工调账具备请求幂等、事务锁和余额不足保护。
- 签到奖励重复请求只产生一条奖励流水；邀请注册奖励和首充奖励均先形成奖励事实，再以事实 ID 发放邀请人奖励并具备业务幂等键。
- 书籍购买并发测试证明八个并发请求只产生一个订单、两条扣款流水和一个权益事实。
- 会员人工发放按请求 ID 幂等，限时和永久发放可独立叠加。
- 模拟充值、人工补单和 EPUSDT 入账共用首充奖励幂等事实。
- 管理端可查询充值订单与回调日志，并可执行受控主动同步和人工补单。
- 每次运行时 EPUSDT HTTP 回调都会在业务处理前建立独立脱敏审计，成功、合法幂等、拒绝、可重试失败及 panic 均有明确终态或滞留告警。
- `reader-finance` 在隔离 MySQL 8.4 和 PostgreSQL 17 上验证当前财务表迁移、幂等重跑、旧 ID 保留、秘密脱敏和钱包流水零差异。

## 支付创建闭环

前向迁移 `00060_epusdt_payment_creation.sql` 和 Commerce 运行路径已补齐冻结 Reader 依赖的状态：

`creating`、`pending`、`gateway_unknown`、`create_failed`、`superseded`、`expired`、`callback_exception`、`paid`。

已固定的实现和验证证据包括：

- 订单先以 `creating` 持久化，再在事务外调用 EPUSDT；成功响应严格校验订单号、金额、币种、Token、状态、过期时间、地址和支付 URL。
- 确定拒绝进入 `create_failed`，超时、网络错误、重定向拒绝和不可信响应进入 `gateway_unknown`，Reader 只收到稳定脱敏错误。
- 同一读者同一请求 ID 并发 16 次只调用一次网关并返回同一订单；新活动订单确定性把旧活动订单转为 `superseded`。
- 过期 Worker 使用租约、批次和 `FOR UPDATE SKIP LOCKED` 把 `pending` 及到达释放窗口的 `gateway_unknown` 转为 `expired`，并释放活动订单唯一槽位。
- 创建请求和订单快照保存 `credential_ref` 与 PID 快照；当前、历史和 legacy 空引用订单均可验签，未知引用或 PID 错配拒绝入账。
- `pending/gateway_unknown/superseded/expired/callback_exception` 的晚到有效回调可转 `paid`；`creating/create_failed` 禁止入账。
- 网关交易号与链上交易哈希跨订单防重放；已支付重复回调继续校验金额、地址和交易快照。

隔离 `moonbook_browser` 栈使用当前 Server 镜像和严格 HMAC 本地替身完成真实 HTTP 冒烟：临时 Reader 创建订单 `9007199254743784` 后查询返回同一订单，状态 `pending`，`gateway_trade_id=stub-RC9007199254743784`、收款地址和 `credential_ref=task8-local` 已持久化，管理订单数据源可见。测试账号、订单、邀请码和搜索投影随后清理为 0，支付通道恢复关闭，临时凭据从容器移除。

## 回调全尝试审计

前向迁移 `00061_epusdt_callback_attempt_audit.sql` 将运行时回调审计固定为“先建记录、后做资金处理”。每次 EPUSDT HTTP 重投都会获得独立日志 ID，不按 payload 哈希、订单号或 Request ID 合并；成功入账、合法幂等、格式拒绝、签名拒绝、未知订单、快照不匹配、跨订单重放、依赖故障和 panic 均覆盖。跨订单复用已占用交易标识稳定分类为 `REPLAY_DETECTED`，金额、地址等当前订单快照校验仍保留。

成功回调在钱包入账、首充奖励、订单 `paid` 转换和审计成功终态之间保持同一 PostgreSQL 事务。拒绝路径只有在审计最终化成功后才返回确定拒绝；建审计或最终化失败、依赖故障和 panic 返回 `503 fail` 触发 EPUSDT 重试，且不会留下充值流水、奖励事实或支付终态。隔离 HTTP 替身连续调用 2 次，第一次产生独立 `failed` 审计并收到可重试响应，第二次产生独立 `success` 审计并收到 `200 success`；资金事实仅写入一次。

管理 API 和页面只返回白名单字段与业务快照。Payload 仅保留 SHA-256、字节数和截断标志，不返回原始 body、完整签名、PID、请求头或来源 IP；页面没有重放、补偿或入账操作。Prometheus 提供尝试结果、失败码、503、审计持久化失败和滞留 `received` 指标，标签不包含订单号、交易号、Request ID 或 Trace ID。运行时 `received` 超过 `MOONBOOK_EPUSDT_CALLBACK_STALE_MINUTES` 后显示“处理中断”。

## 回归证据

在固定代码基线 `2658a44` 上完成：

- `00060` 空库、`00059 -> 00060`、重复执行 `applied=0`、关键财务事实行数不减少、活动订单收敛及重复交易标识阻断迁移验证。
- EPUSDT 客户端、配置、创建仓储、16 路并发、失败分类、替换、过期 Worker、当前/历史凭据回调、防重放及五种晚到状态测试。
- 五种晚到状态均只产生一条充值流水、一条首充奖励事实、一条奖励流水和一个 `paid` 终态。
- Payment、InviteReward、AdminRechargeOrder 真实 PostgreSQL 集成，以及 Commerce/Reader 全量真实 PostgreSQL/Redis/MinIO 回归。
- `make verify-m3` 从头通过：冻结 Reader tree `ffbe7bb4c56792e94e160de86cc71bce0a6e0e5e`、536 个冻结测试、6 个树外 SSR/SEO 测试和 Reader 生产构建。
- 模块依赖、SQL/表所有权、配置合同、Compose 展开、全部 Shell 语法、`go vet` 和 Gitleaks 通过；秘密扫描约 29.87 MB，结果为 `no leaks found`。
- `00060 -> 00061`、空库 61 个迁移、重复执行 `applied=0`、历史日志兼容和关键财务事实行数保护通过。
- Payment 全包真实 PostgreSQL 集成通过；Commerce/Reader 全包、模块依赖、SQL 所有权和 `go vet` 通过，官方 `golang:1.24.2-bookworm` 容器内同范围 `go test -race` 通过。
- 全新隔离数据库执行完整 `make verify-m3`：PostgreSQL、Redis、MinIO 均就绪，冻结 Reader 536 项、树外 SSR/SEO 6 项及 Reader 生产构建通过。共享验收库存在历史夹具污染，未删除或改写，未用于替代该隔离结果。
- `pnpm run verify:management` 通过 29 项测试、ESLint、生产构建和供应链检查；仅保留已知 Vite 配置及 `arcdash` BigInt target 警告。
- 本地隔离 Compose 管理端在 `1440x1000` 与 `390x844` 视口打开钱包页面，三页签“钱包概览/签到记录/邀请奖励”可切换；移动端 `document.documentElement.scrollWidth=390`，无横向溢出。临时高位 ID `9007199254743784`、`9007199254743785` 在页面中保持文本字符串显示；邀请奖励筛选控件可见，页面没有补签、补发、删除、重算或导出按钮。截图保存在 `output/playwright/m4-wallet-desktop.png`、`m4-checkins-desktop.png`、`m4-rewards-desktop.png`。
- `adminoperations` 集成测试使用真实 PostgreSQL 验证高位 ID、日期/阶段/状态筛选、稳定分页以及搜索投影缺失时事实记录仍保留；无 `MOONBOOK_READER_TEST_DSN` 时安全跳过，隔离 Compose 数据库本轮实际执行并清理夹具。
- Playwright 使用本轮隔离管理员与夹具，在 `1440x1000` 和 `390x844` 验证“处理中断”“无效”“未校验”、Long ID 文本、详情字段白名单及无布局重叠；回调前缀、管理员、书籍、对象元数据、读者和邀请码最终均为 0，MinIO 删除 2 个夹具对象。
- 2026-09-01 邀请奖励专项修复在迁移 `00076` 的隔离 PostgreSQL 上通过 16 路注册奖励并发、事实/流水冲突、配置禁用与 0 金额、事务回滚、最近 20 条/全量累计/排序/Long 精度合同及完整 Reader/Commerce 真实依赖门。Playwright 真实注册邀请人和被邀请人后，邀请人页面显示“已邀请 1”“累计金币 13”，奖励弹窗显示“邀请注册奖励 +13金币”与发放时间；截图为 `output/playwright/invite-reward-detail.png`。临时账号、奖励事实、流水、对象和隔离数据库最终均为 0 或已删除。

验收工具版本：Go `1.25.12 darwin/arm64`、Node `24.14.1`、npm `11.11.0`、Docker Client/Server `29.4.0`、Compose `5.1.2`、PostgreSQL `17.6`、Redis `7.4.5`、MinIO `RELEASE.2025-07-23T15-54-02Z`。Server 容器按项目 Dockerfile 使用 Go `1.24.2-alpine3.21` 构建。

## 财务核对范围

`commerce/reconcile.Full` 现在在保留 `Wallets` 逐读者/逐币种核对的基础上，检查已支付充值订单与充值流水、已支付消费订单的扣款拆分、订单来源的会员/权益事实、失败/退款订单的有效权益、成功支付回调与订单终态/流水关联，并统计会员授予和权益事实数量。`moonbook-finance-reconcile` 输出可审计的字符串 key/expected/actual 差异，发现差异返回非零退出码。

M4 已提供可执行核对，覆盖：

- 每个已支付充值订单恰好关联一条充值流水，金额和读者一致；
- 链上交易哈希、请求 ID 和首充奖励业务键全局唯一；
- 每个已支付消费订单的扣款拆分等于订单价格；
- 每个成功购买或发放与相应会员/权益事实一致；
- 退款、失败、关闭、失效状态不会保留不应存在的有效权益；
- 回调成功事实、订单终态和钱包入账不存在孤立记录。

完整约 8 GB 副本的行数、主键、金额分布和耗时报告仍归 M6；M4 核对器本身及合成边界数据验证已完成。

## 管理能力证据

旧管理 API 定义的 `GET /reader/checkin/list` 和 `GET /reader/inviteReward/list` 已映射为钱包页面内“签到记录”“邀请奖励”只读页签，对外提供 `GET /reader/checkins`、`GET /reader/inviteRewards`（迁移 `00062`）。M4 本地能力无剩余缺口，真实支付属于外部放行项。

## Compose 与秘密边界

根 `.env.example`、`compose.yaml` 和 Server 配置合同已覆盖 PID、Secret、当前凭据引用、历史验签凭据 JSON、创建/通知/跳转/健康/同步 URL 和超时窗口。Secret 只经环境注入，配置合同禁止真实值进入模板或 Compose 模型，Gitleaks 本轮无发现。

隔离冒烟使用一次性本地凭据，结束后已从容器环境移除；仓库未写入该凭据。

## M4 退出门

1. `[x]` 真实网关创建适配器的本地替身实现与失败状态测试；
2. `[x]` 每次回调尝试均有脱敏、可关联、可检索的审计记录；
3. `[x]` 创建、成功、失败、重复、重放、晚到和并发状态机测试；
4. `[x]` 签到和邀请奖励记录审计页签通过桌面与移动端 E2E；
5. `[x]` 订单、支付、钱包、奖励、会员和权益通用核对器在隔离 PostgreSQL 合成边界数据上零差异，并能检测充值订单断链差异；完整副本零差异仍待 M6；
6. `[x]` EPUSDT 变量完整注入 Compose，仓库不包含真实凭据；
7. `[ ]` 使用受控真实支付环境完成连通性和最小金额支付验收。
