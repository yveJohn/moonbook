# Docker Compose 升级 Runbook

本 Runbook 用于单机 Docker Compose 部署的受控升级。命令默认作用于显式环境文件中的项目，不连接生产环境；正式执行前仍需用户单独授权。k3s 部署不在当前目标范围内，但发布清单中的 commit、镜像 digest、配置版本、迁移边界和验收项应继续沿用。

## 发布清单

每次发布必须先在仓库外保存不可变清单，至少包含：

- 发布 commit 和上一稳定 commit；
- `MOONBOOK_SERVER_IMAGE`、`MOONBOOK_WEB_IMAGE`、`MOONBOOK_READER_IMAGE` 的不可变 tag 和 digest；
- PostgreSQL、Redis、MinIO、MinIO Client、Nginx 镜像 digest；
- 配置版本及新增、删除、改名的环境变量；
- 迁移起始版本、目标版本和向后兼容结论；
- 备份目录、备份校验结果和恢复演练报告；
- 负责人、开始时间、最长执行时间、停止条件和最晚回退时间。

生产清单禁止使用 `local`、`latest` 或可覆盖 tag。先设置绝对路径并固定 Compose 调用：

```bash
export MOONBOOK_ROOT=/path/to/moonbook
export MOONBOOK_ENV_FILE=/secure/path/moonbook.env

compose() {
  docker compose \
    --project-directory "$MOONBOOK_ROOT" \
    --env-file "$MOONBOOK_ENV_FILE" \
    -f "$MOONBOOK_ROOT/compose.yaml" "$@"
}

compose config --quiet
compose config --images
compose ps
```

不得启用 shell `set -x`，不得在命令、日志或报告中打印 Secret。升级前通过部署系统确认 PostgreSQL、Redis、MinIO、JWT、指标、支付和已启用 AI 供应商需要的 Secret 均已注入；`.env.example` 的本地占位值不得用于生产。

## 升级前门禁

1. 工作目录必须处于发布清单记录的 commit，Compose 和环境文件解析成功。
2. 当前所有基础设施和应用容器健康，迁移表不存在失败或未解释版本。
3. 固定镜像已拉取并用 digest 核对，旧镜像仍保留在宿主机或可信镜像仓库。
4. 已按 `backup-restore.md` 协调停写并完成 PostgreSQL/MinIO 备份，哈希、dump 清单和对象统计全部通过。
5. 已确认停机期间支付回调的重试、回放或人工核对方案。
6. 已审查目标迁移是否允许旧应用读取新结构。没有明确证据时，默认不允许迁移后直接回退旧应用。

任一门禁失败即 No-Go，不执行迁移。固定镜像可在停写前拉取：

```bash
compose pull postgres redis minio minio-init web reader-ui gateway
docker pull '<发布清单中的 server 镜像@digest>'
compose config --images
```

## 升级顺序

入口和写入方应已由备份流程停止。再次确认状态后，仅用目标 Server 镜像运行一次性迁移：

```bash
compose ps
compose run --rm --no-deps migrate
```

记录迁移输出和 `moonbook_schema_version`：

```bash
compose exec -T postgres sh -ec '
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
    "SELECT version_id, is_applied, tstamp FROM moonbook_schema_version ORDER BY id DESC LIMIT 10"
'
```

迁移失败时保持入口停止，不得启动新旧应用混合提供服务，也不得执行向下迁移。迁移成功后按依赖顺序替换应用层：

```bash
compose up -d --no-build --force-recreate server web reader-ui gateway
compose ps
```

`server`、`web`、`reader-ui` 和 `gateway` 必须全部 healthy；`postgres`、`redis`、`minio` 数据卷不得重建。使用 `compose images` 核对运行容器的 tag 和 image ID 与发布清单一致。

## 冒烟与观察

先验证无需认证的基础链路：

```bash
curl -fsS 'https://admin.example.com/gateway-health'
curl -fsS 'https://admin.example.com/api/health/ready'
curl -fsS 'https://reader.example.com/health'
```

随后使用受控测试账号验证管理登录、Reader 登录、书库、章节正文、订单查询和对象读取；有非空财务数据时执行：

```bash
compose run --rm --no-deps server moonbook-finance-reconcile
```

财务输出必须为 `mismatches=0`。观察期持续检查 HTTP 错误率和延迟、数据库连接、Redis/MinIO 错误、任务积压与失租、支付创建/回调和磁盘水位。任一关键旅程失败、财务不一致、对象校验失败、持续 5xx 或依赖不健康均停止放量并进入回退判断。

## 回退边界

数据库迁移尚未执行时，可把环境文件中的三个应用镜像恢复为旧 digest，再执行应用层重建。

数据库迁移已经执行后，禁止运行 down migration 或手工反向修改结构。只有同时满足以下条件，才允许“数据库不回退、仅回退应用镜像”：

- 发布清单明确证明新迁移对旧应用向后兼容；
- 回退版本已在该迁移版本上完成自动化或演练验证；
- 回退不会写出新旧版本语义冲突的数据；
- 支付、任务 Worker 和对象引用不存在跨版本状态机不兼容。

满足条件时，恢复旧应用镜像 digest 并重建应用层，随后重复全部健康、业务和财务验证：

```bash
compose up -d --no-build --force-recreate server web reader-ui gateway
compose images
compose ps
```

不满足条件时保持入口停止，按 `backup-restore.md` 将升级前 PostgreSQL 与 MinIO 协调备份恢复到新的受控环境，再按 `rollback.md` 执行回退。不得把旧应用直接连接到未经兼容证明的新数据库。

## 结果记录

报告必须记录开始/结束时间、commit、全部镜像 digest、配置版本、迁移前后版本和输出、备份位置及哈希、容器替换耗时、健康与认证冒烟、财务核对、观察指标、回退决定、RTO/RPO 和清理结果。正式环境的镜像拉取、迁移、重启、回退和流量操作仍需单独授权。
