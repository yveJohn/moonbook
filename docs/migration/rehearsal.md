# 旧 MySQL 全量迁移演练记录

## 当前结论

迁移工具的 Reader 身份与商业数据已完成小数据夹具演练，全域核对器也已通过真实 MySQL、PostgreSQL 和 MinIO 故障注入。完整副本演练入口为 `scripts/verify-m6-full-copy.sh`；在 `docs/verification/m6-full-copy-rehearsal.md` 写入真实完成结果前，约 8GB 副本的容量、完整性和 12 小时窗口仍不得视为通过。

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

## 8GB 副本执行入口

冻结旧仓库候选为 `exports/moonbook_admin_20260715_175649.sql.gz`，压缩大小 1,065,163,488 字节，固定 SHA-256 为 `94d86df76780cc85cdec4690265866140b202bb2b5d908826bf37783c9c8cbe3`，完整 `gzip -t` 已通过。候选只读保留在旧仓库；实际恢复字节数必须由演练脚本记录，不能继续使用 gzip ISIZE 推算值替代。

脚本默认只做预检，不启动容器。`--run` 恢复到唯一 `moonbook_verify_m6_*` Compose 项目，冻结源 MySQL，执行结构迁移、全部业务 stage、全域核对和结构/业务幂等重跑，并保存事件、资源、源盘点和脱敏审计报告。容器和卷在运行后保留供核验，只有项目名二次确认完全匹配时 `--cleanup` 才删除隔离资源。

TXT 原文件门在恢复源库后决定：源表不存在或任务数为零时，脚本生成只含空映射的清单；任务数大于零时，必须提供外部只读 `MOONBOOK_FULL_COPY_TXT_MANIFEST` 和 `MOONBOOK_FULL_COPY_TXT_ROOT`。找不到任一原文件时必须 No-Go，禁止伪造或跳过任务。

演练环境文件必须位于仓库外，并为端口、数据库和 Secret 使用本次隔离值。执行方式：

```bash
export MOONBOOK_FULL_COPY_BACKUP=/Users/yve/Documents/moonbook/exports/moonbook_admin_20260715_175649.sql.gz
export MOONBOOK_FULL_COPY_SHA256=94d86df76780cc85cdec4690265866140b202bb2b5d908826bf37783c9c8cbe3
export MOONBOOK_FULL_COPY_PROJECT=moonbook_verify_m6_YYYYMMDD
export MOONBOOK_FULL_COPY_ENV_FILE=/private/tmp/moonbook-m6-rehearsal.env
export MOONBOOK_FULL_COPY_WORK_DIR=/private/tmp/moonbook-m6-full-copy-YYYYMMDD
export MOONBOOK_FULL_COPY_CONFIRM=M6_FULL_COPY_LOCAL_ISOLATED

scripts/verify-m6-full-copy.sh --preflight
scripts/verify-m6-full-copy.sh --run
MOONBOOK_FULL_COPY_CLEANUP_CONFIRM="$MOONBOOK_FULL_COPY_PROJECT" \
  scripts/verify-m6-full-copy.sh --cleanup
```

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
