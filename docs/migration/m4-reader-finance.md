# M4 Reader 财务迁移证据

## 范围

`reader-finance` 是 `reader-identity`、`reader-commerce` 之后的独立迁移阶段，覆盖当前旧库以下事实：

1. 钱包汇总、不可变流水和奖励币桶；
2. 购买订单、签到规则/记录、邀请奖励和人工调账；
3. 充值产品/设置、支付渠道、充值订单和支付回调日志。

阶段按表主键游标分批读取，每张表独立保存 checkpoint，支持中断恢复和幂等重跑。旧 bigint 主键原值写入 PostgreSQL，完成每张 identity 表后将序列推进到目标最大 ID。

## 安全边界

- 不读取或迁移 `reader_payment_credential.encrypted_secret`。
- 充值订单仅保留旧渠道 ID、凭据 ID 和商户 PID 的 SHA-256；不保存原始 PID。
- 支付回调来源 IP 仅保存 SHA-256；不保存原始 IP。
- `payload_snapshot` 仅在 JSON 有效时写入，且迁移日志和错误记录不包含 payload。
- 旧表缺失会按确定顺序跳过；存在但字段、枚举或关联无效的记录进入 `migration_errors`。

## 核对门

财务验收调用 `commerce/reconcile.Wallets`，逐读者、逐币种从不可变流水重算：

- 当前余额 = 收入合计 - 支出合计；
- 累计收入 = 收入流水合计；
- 累计支出 = 支出流水合计。

任一差异均视为迁移失败。购买订单、充值订单、回调日志和调账记录还需通过行数、主键范围、关联、枚举和金额分布核对；完整生产副本的最终报告在 M6 生成。

## 可重复验证

准备隔离的只读 MySQL 8.4 和已应用最新迁移的 PostgreSQL 17，使用无生产秘密的 DSN：

```bash
cd server
MOONBOOK_LEGACY_TEST_DSN='<readonly-mysql-dsn>?parseTime=true&charset=utf8mb4' \
MOONBOOK_MIGRATION_TEST_DSN='<isolated-postgres-dsn>' \
CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache \
go test ./internal/platform/legacymigrate \
  -run TestReaderMigrationWithMySQLAndPostgres -count=1 -v
```

2026-08-15 本地隔离验证结果：

- 已有 53 个版本的 PostgreSQL 前向迁移首次 `applied=1`、重复 `applied=0`；独立空库从零首次 `applied=54`、重复 `applied=0`；
- 合成旧库财务记录 19 条，分批大小为 2；
- 有效记录全部落入目标表，无效订单类型和回调状态共 2 条进入结构化错误清单；
- 使用同一 migration name 重跑不增加记录或错误；
- 钱包核对差异为 0；
- MySQL 源 `reader_user` 行数/Token 版本合计及流水行数/金额合计在迁移前后不变；
- 大于 JavaScript 安全整数范围的主键原值保留，目标 identity 序列均推进到迁移最大值之后；
- 商户 PID 和来源 IP 原文未进入目标列。

该结果证明工具与合成边界用例可重复，不替代 M6 对约 8 GB 完整数据副本的容量、耗时和全量零差异演练。
