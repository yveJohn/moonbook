# PostgreSQL 与 MinIO 备份恢复 Runbook

本 Runbook 只描述可在本地或用户明确授权的受控环境执行的操作。默认命令使用当前仓库的 Docker Compose、显式环境文件和仓库外备份目录，不会自动连接生产环境。Redis 只保存缓存、限流和任务协调状态，不保存唯一业务事实，不纳入事实备份。

## 变量与安全边界

先设置绝对路径。备份目录必须位于 Git 工作区之外，并只允许执行账号读取：

```bash
export MOONBOOK_ROOT=/path/to/moonbook
export MOONBOOK_ENV_FILE=/secure/path/moonbook.env
export MOONBOOK_BACKUP_DIR=/secure/backups/moonbook/$(date -u +%Y%m%dT%H%M%SZ)
umask 077
mkdir -p "$MOONBOOK_BACKUP_DIR/postgres" "$MOONBOOK_BACKUP_DIR/minio"

compose() {
  docker compose \
    --project-directory "$MOONBOOK_ROOT" \
    --env-file "$MOONBOOK_ENV_FILE" \
    -f "$MOONBOOK_ROOT/compose.yaml" "$@"
}

compose config --quiet
compose ps
```

不要在命令行、报告、Shell trace 或 Git 文件中展开密码、DSN、Token。运行期间禁止启用 `set -x`。

## 协调停写

PostgreSQL dump 和 MinIO mirror 只有在所有 Moonbook 写入方均停止后才构成协调备份。生产执行前还必须确认支付渠道在停机期间会可靠重试回调，或已有受控回放和人工核对方案；未确认时禁止开始正式备份。

Compose 单机部署按以下顺序阻断入口并优雅停止应用写入：

```bash
compose stop gateway
compose stop reader-ui web server
compose ps
```

验收条件：`gateway`、`reader-ui`、`web`、`server` 均为 stopped；PostgreSQL 和 MinIO 保持 healthy；不存在单独部署的 Worker 或迁移进程。记录 `git rev-parse HEAD`、镜像 digest、迁移版本、PostgreSQL/MinIO 版本和停写时间。

## 创建备份

PostgreSQL 凭据在容器内展开，dump 写入宿主受控目录：

```bash
compose exec -T postgres sh -ec \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-acl' \
  > "$MOONBOOK_BACKUP_DIR/postgres/moonbook.dump"
```

使用 Compose 已固定的 `minio-init` 客户端镜像，并显式把宿主备份目录挂载为 `/backup`：

```bash
compose run --rm --no-deps \
  -v "$MOONBOOK_BACKUP_DIR/minio:/backup" \
  --entrypoint /bin/sh minio-init -ec '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    mc mirror --preserve "local/$MINIO_BUCKET" /backup
    mc du --recursive "local/$MINIO_BUCKET"
  '
```

校验清单只扫描数据目录，不会把 `SHA256SUMS` 自身纳入哈希：

```bash
find "$MOONBOOK_BACKUP_DIR/postgres" "$MOONBOOK_BACKUP_DIR/minio" \
  -type f -print0 | sort -z | xargs -0 shasum -a 256 \
  > "$MOONBOOK_BACKUP_DIR/SHA256SUMS"
shasum -a 256 -c "$MOONBOOK_BACKUP_DIR/SHA256SUMS"

compose exec -T postgres pg_restore --list \
  < "$MOONBOOK_BACKUP_DIR/postgres/moonbook.dump" >/dev/null
du -sh "$MOONBOOK_BACKUP_DIR"
```

备份验收至少包括：dump 清单可读、所有文件哈希通过、MinIO 对象数量与字节数已记录、数据库业务汇总已记录、备份目录权限正确。任一步失败都必须保持停写并按 No-Go 处理，不能用不完整备份继续切换。

## 隔离环境恢复

恢复环境必须使用单独的环境文件，其中 `COMPOSE_PROJECT_NAME` 以 `moonbook_verify_` 开头，宿主端口不得与日常环境冲突。恢复前确认该项目名没有复用现有容器或卷：

```bash
export MOONBOOK_RESTORE_ENV_FILE=/secure/path/moonbook-restore.env

restore_compose() {
  docker compose \
    --project-directory "$MOONBOOK_ROOT" \
    --env-file "$MOONBOOK_RESTORE_ENV_FILE" \
    -f "$MOONBOOK_ROOT/compose.yaml" "$@"
}

restore_compose config --quiet
restore_compose up -d postgres redis minio minio-init
restore_compose ps
```

不要在恢复前启动 `migrate` 或 `server`。全新的 PostgreSQL 数据库为空，因此恢复不使用 `--clean`：

```bash
restore_compose exec -T postgres sh -ec \
  'pg_restore --exit-on-error --no-owner --no-acl -U "$POSTGRES_USER" -d "$POSTGRES_DB"' \
  < "$MOONBOOK_BACKUP_DIR/postgres/moonbook.dump"

restore_compose run --rm --no-deps \
  -v "$MOONBOOK_BACKUP_DIR/minio:/backup:ro" \
  --entrypoint /bin/sh minio-init -ec '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    mc mirror --overwrite /backup "local/$MINIO_BUCKET"
    mc du --recursive "local/$MINIO_BUCKET"
  '
```

恢复后使用一次性容器检查迁移，不能对已经退出的 `migrate` 服务执行 `exec`：

```bash
restore_compose run --rm migrate
restore_compose exec -T postgres sh -ec '
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
    "SELECT version_id, is_applied FROM moonbook_schema_version ORDER BY id DESC LIMIT 5"
'
restore_compose run --rm --no-deps server moonbook-finance-reconcile

MOONBOOK_ENV_FILE="$MOONBOOK_RESTORE_ENV_FILE" \
  "$MOONBOOK_ROOT/scripts/verify-infrastructure.sh"
```

迁移输出必须为 `applied=0`，财务核对必须为 `mismatches=0`。随后启动完整应用栈，执行管理登录、Reader 登录、书库、章节正文、订单和对象读取冒烟，并对比恢复前后的表行数、订单/流水/权益汇总、对象数量、总字节数和抽样 SHA-256。任何无法解释的差异都判定恢复失败。

## 清理与恢复服务

仅在确认恢复项目名和卷均属于本次演练后，才清理隔离环境：

```bash
restore_compose down --volumes --remove-orphans
```

备份目录不得随演练环境删除。正式环境只有在备份验收通过且切换负责人批准后，才按切换 Runbook 恢复服务；普通本地备份可以执行：

```bash
compose up -d server web reader-ui gateway
compose ps
```

## 结果记录

报告必须保留：开始/结束时间、Git commit、镜像 digest、迁移版本、基础设施版本、停写方式、备份字节数、恢复耗时、对象数量/字节数、校验清单结果、数据库与财务核对摘要、冒烟结果、RPO/RTO 和结论。真实生产备份、恢复或切换仍需用户单独授权。
