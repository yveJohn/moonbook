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

完整演练仍需用户提供可用的约 8GB 数据库副本，并在受控隔离环境执行。验收必须记录源表容量/行数、各阶段耗时和吞吐、峰值磁盘与连接、目标行数/主键/关联/枚举、财务零差异、MinIO 对象数量/字节/哈希、全量重跑结果，以及是否稳定低于 12 小时并保留回退余量。在取得这些证据前，M6 不得关闭。
