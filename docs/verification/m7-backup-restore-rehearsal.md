# M7 本地备份恢复演练报告

演练时间：2026-08-15 00:50-00:55 UTC

## 范围

本次演练验证根 Compose 下 PostgreSQL 自定义格式备份、MinIO 目录镜像、SHA-256 清单、全新隔离卷恢复、迁移重跑、财务核对和基础设施健康检查。源和目标项目均以 `moonbook_verify_` 开头，未连接日常或生产资源。

执行基线为 `3058fee` 加本报告同一提交中的镜像打包修复。基础设施镜像为 PostgreSQL `17.6-alpine`、Redis `7.4.5-alpine`、MinIO `RELEASE.2025-07-23T15-54-02Z` 和 MinIO Client `RELEASE.2025-07-21T05-28-08Z`。

## 隔离资源

- 源项目：`moonbook_verify_backup_source`
- 恢复项目：`moonbook_verify_backup_restore`
- 两个项目分别使用独立 PostgreSQL、Redis、MinIO 命名卷和不冲突的回环端口。
- 临时备份目录：`/private/tmp/moonbook-backup-restore.9wkqYI/backup`，未进入 Git。

## 备份结果

- 源数据库包含 106 张 `public` 表；`moonbook_schema_version` 最高已应用版本为 58，共 59 条版本记录。
- `pg_dump` 生成 PostgreSQL custom-format dump，大小 458279 字节；`pg_restore --list` 成功读取 1106 条 TOC 记录。
- MinIO 测试 Bucket 包含 1 个 43 字节对象 `rehearsal/fixture.txt`。
- 备份总目录约 456 KiB；校验清单包含 dump 与对象两项，`shasum -a 256 -c` 均返回 `OK`。
- 修正后的清单命令只扫描 `postgres/` 和 `minio/`，没有把 `SHA256SUMS` 自身纳入校验。

## 恢复结果

- 目标项目只先启动 PostgreSQL、Redis、MinIO 和建桶任务，没有在 restore 前运行迁移。
- `pg_restore --exit-on-error --no-owner --no-acl` 恢复成功，MinIO `mc mirror --overwrite` 恢复成功。
- 恢复后仍为 106 张 `public` 表、最高版本 58 和 59 条版本记录；一次性迁移输出 `applied=0`。
- 恢复 Bucket 为 1 个对象、43 字节，测试对象内容逐字节读取正确。
- `scripts/verify-infrastructure.sh` 对恢复环境返回 PostgreSQL、Redis、MinIO healthy，Bucket 为 private。
- 首次执行财务核对发现 Server 镜像没有分发已存在的 `moonbook-finance-reconcile` 命令；本提交补齐 Docker 构建和入口分派后重新构建镜像，恢复库核对输出 `checked=0 mismatches=0`。
- 恢复项目随后启动完整 Compose 应用栈，gateway、web、server、reader-ui、PostgreSQL、Redis 和 MinIO 均为 healthy；管理网关 `/gateway-health`、API `/api/health/ready` 和 Reader `/health` 返回成功，readiness 中 migrations、MinIO、PostgreSQL、Redis 均为 `ok`。
- 取证完成后，两个隔离项目的容器、网络和专用命名卷均已删除，临时备份目录已清理；日常 `moonbook` 项目未被操作。

## 结论与限制

本次演练证明修订 Runbook 的容器、挂载、dump、mirror、校验、恢复、迁移和财务命令链在本地隔离 Compose 上可执行。它只使用空业务迁移库和一个无敏感测试对象，没有证明以下生产就绪条件：

- 真实业务数据量下的备份/恢复耗时和 12 小时窗口；
- 应用、Worker 和支付回调的协调停写与回放；
- 非空钱包、订单、支付、权益和章节对象全量核对；
- 恢复后管理登录、Reader 登录、章节正文、订单等认证业务旅程；
- 加密备份、异地保管、保留周期和生产 RPO/RTO。

因此本报告关闭命令级可执行性缺口，但不把 M7 备份恢复退出门标记为完成。完整数据副本恢复演练和应用级验收仍必须补齐。
