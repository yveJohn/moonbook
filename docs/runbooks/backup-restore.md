# PostgreSQL 与 MinIO 备份恢复 Runbook

本 Runbook 只描述可在本地或用户明确授权的受控环境执行的操作。默认命令使用当前项目的 Docker Compose 和独立备份目录；不会自动连接生产环境。Redis 只保存缓存、限流和任务协调状态，不保存唯一业务事实，不纳入事实备份。

## 备份前检查

1. 确认目标 Compose 项目和备份目录是本项目专用资源：

```bash
export MOONBOOK_BACKUP_DIR="$(pwd)/var/backups/$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$MOONBOOK_BACKUP_DIR/postgres" "$MOONBOOK_BACKUP_DIR/minio"
docker compose ps
```

2. 确认 API 已停止写入或进入维护模式，并记录 `git rev-parse HEAD`、迁移版本、PostgreSQL/MinIO 版本和备份开始时间。不要把 DSN、密码或 Token 写入报告。

## 创建备份

在已由环境变量或 Secret 注入 Compose 凭据的 shell 中执行：

```bash
docker compose exec -T postgres pg_dump \
  -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  --format=custom --no-owner --no-acl \
  > "$MOONBOOK_BACKUP_DIR/postgres/moonbook.dump"

docker compose exec -T minio sh -lc \
  'mc alias set local http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null && mc mirror --preserve local/"$MINIO_BUCKET" /backup/minio'
```

推荐使用一次性 MinIO 备份容器将 `/backup` 映射到 `MOONBOOK_BACKUP_DIR/minio`；不要把备份文件写入 Git 工作区。生成校验清单并记录大小：

```bash
find "$MOONBOOK_BACKUP_DIR" -type f -print0 | sort -z | xargs -0 shasum -a 256 \
  > "$MOONBOOK_BACKUP_DIR/SHA256SUMS"
du -sh "$MOONBOOK_BACKUP_DIR"
```

备份验收至少包括：`pg_restore --list moonbook.dump` 可读、MinIO 对象数量/字节数记录、`SHA256SUMS` 校验通过、备份目录权限仅允许执行迁移的账号读取。

## 隔离环境恢复演练

恢复必须使用独立 Compose 项目、独立 PostgreSQL 卷和独立 MinIO Bucket。禁止覆盖默认开发卷或任何生产资源。

```bash
export COMPOSE_PROJECT_NAME=moonbook_restore_verify
docker compose up -d postgres redis minio minio-init migrate
docker compose exec -T postgres pg_restore \
  -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  --clean --if-exists --no-owner --no-acl \
  < "$MOONBOOK_BACKUP_DIR/postgres/moonbook.dump"
```

将对象备份复制到一次性 MinIO 客户端容器后执行 `mc mirror --overwrite /backup/minio local/$MINIO_BUCKET`。恢复完成后运行：

```bash
docker compose exec -T migrate moonbook-migrate up
docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 1;"
docker compose run --rm server moonbook-finance-reconcile
./scripts/verify-infrastructure.sh
```

再执行 Reader 登录、书库、章节正文、管理登录和对象读取冒烟；对比恢复前记录的表行数、订单/流水/权益汇总、对象数量、总字节数和抽样 SHA-256。任何差异必须记录为恢复失败并销毁隔离资源，不得修改备份文件后重试。

## 结果记录

保留开始/结束时间、版本、备份字节数、恢复耗时、对象数量/字节数、核对 SQL 输出摘要、冒烟 URL 和结论。真实生产恢复仍需用户单独授权；本项目不会代为执行。
