# 旧 MySQL 全量迁移演练记录

## 当前结论

迁移工具的 Reader 身份与商业数据已在隔离 MySQL 8.4 和真实 PostgreSQL 上完成小数据夹具演练。该结果只证明字段映射、只读门、分批检查点、错误清单、幂等和序列推进可执行，不代表约 8GB 生产数据副本的容量、完整性或 12 小时窗口验收通过。

## 2026-08-15 Reader 小夹具演练

- 源：临时 MySQL 8.4，独立数据库和只读账号，`read_only=ON`，连接字符集 `utf8mb4`。
- 目标：`moonbook_browser` 隔离 Compose PostgreSQL。
- 批量大小：2。
- Stage：`reader-identity`、`reader-commerce`。
- 结果：identity 处理 3 行/错误 1 行，commerce 处理 13 行/错误 2 行；两个 checkpoint 均 `done=true`。
- 重跑：同一 migration name 再次执行不新增目标事实或错误。
- 大 ID：覆盖 `9007199254740993` 以上 ID，并验证 PostgreSQL 参数为 `bigint`、6 个 identity 序列推进。
- 脱敏：坏密码摘要只生成 `INVALID_PASSWORD_HASH`，错误消息不包含摘要内容。
- 源不变：`reader_user` 行数及 `token_version` 汇总值执行前后相同。

执行入口：

```bash
MOONBOOK_LEGACY_TEST_DSN='<readonly-mysql-dsn>?charset=utf8mb4&parseTime=true&loc=UTC' \
MOONBOOK_MIGRATION_TEST_DSN='<isolated-postgres-dsn>' \
CGO_ENABLED=0 GOCACHE=/tmp/moonbook-gocache \
go test -count=1 -run '^TestReaderMigrationWithMySQLAndPostgres$' -v ./internal/platform/legacymigrate
```

## 8GB 副本待验

冻结旧仓库已定位到 `exports/moonbook_admin_20260715_175649.sql.gz` 候选：压缩大小 1,065,163,488 字节；结合 gzip ISIZE 模值和用户提供的约 8 GB 信息，单成员候选原始大小约为 8,129,574,556 字节。本次没有读取、校验或导入 SQL，完整元数据和安全门见 `docs/verification/m6-data-copy-discovery.md`。

完整演练仍需先关闭内容生产 stage、对象转换和全域核对器缺口，再在受控隔离环境执行。验收必须记录源表容量/行数、各阶段耗时和吞吐、峰值磁盘与连接、目标行数/主键/关联/枚举、财务零差异、MinIO 对象数量/字节/哈希、全量重跑结果，以及是否稳定低于 12 小时并保留回退余量。在取得这些证据前，M6 不得关闭。

## 可重复的只读核对命令

内容迁移 stage 完成后，在同一隔离源库和目标库执行：

```bash
cd server
MOONBOOK_LEGACY_MYSQL_DSN='受控只读 MySQL DSN' \
MOONBOOK_DATABASE_DSN='隔离 PostgreSQL DSN' \
MOONBOOK_MINIO_ENDPOINT='隔离 MinIO 地址' \
MINIO_ROOT_USER='通过 Secret 注入' \
MINIO_ROOT_PASSWORD='通过 Secret 注入' \
MINIO_BUCKET='隔离内容桶' \
go run ./cmd/moonbook-migration-audit moonbook-v1 > /tmp/moonbook-migration-audit.json
```

命令只读访问 MySQL、PostgreSQL 和 MinIO，输出脱敏 JSON。它校验 `all` 的全部 stage checkpoint、迁移错误、源/目标行数与最大 ID、legacy 来源键、内容生产关联、钱包/流水/订单/支付/回调/会员/权益，以及业务引用对象的 kind/owner、Stat 字节/哈希和实际流式内容 SHA-256。报告只保留聚合和最多 100 个哈希指纹，不输出 DSN、对象键、业务 ID、正文或 Secret；任一差异退出非零。
