# 生产回退 Runbook

回退只在 Go/No-Go 负责人批准后执行。本项目不自动连接生产资源，不执行数据库向下迁移，也不把新系统事实直接反写旧库。

## 触发条件

- 迁移、对象或财务核对出现不可解释差异；
- 管理/Reader 核心旅程、正文读取、支付或 Worker 持续失败；
- critical 告警触发，或 5xx、p95、连接、资源水位持续超过批准阈值；
- 到达阶段停止点，或早于最晚回退时间 15 分钟仍无法完成当前验收；
- TLS/DNS、可信代理或外部依赖行为与批准方案不符。

## 立即止损与封存

1. 记录触发时间、commit、镜像 digest、请求/trace ID、迁移 checkpoint、流量比例和指标快照。
2. 按 `maintenance-mode.md` 让新入口返回 503 并停止 Server/Web/Reader；确认外置 Worker 和迁移命令也停止。
3. 保持新 PostgreSQL/MinIO 不变。用仓库外 `0700` 目录创建切换后完整事实快照，而不是选择性猜测受影响表：

```bash
umask 077
export MOONBOOK_ROLLBACK_DIR=/secure/rollback/moonbook/$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$MOONBOOK_ROLLBACK_DIR"
compose exec -T postgres sh -ec \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-acl' \
  > "$MOONBOOK_ROLLBACK_DIR/post-cutover.dump"
compose run --rm --no-deps -v "$MOONBOOK_ROLLBACK_DIR:/evidence" \
  --entrypoint /bin/sh minio-init -ec '
    mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
    mc mirror --preserve "local/$MINIO_BUCKET" /evidence/minio
  '
{
  shasum -a 256 "$MOONBOOK_ROLLBACK_DIR/post-cutover.dump"
  find "$MOONBOOK_ROLLBACK_DIR/minio" -type f -print0 | sort -z | \
    xargs -0 shasum -a 256
} > "$MOONBOOK_ROLLBACK_DIR/SHA256SUMS"
```

同时导出切换时间范围内订单、钱包流水、支付回调、权益、Reader 账号和任务的脱敏汇总，由数据负责人登记。完整 dump/对象快照是取证底座，汇总用于人工分流；两者都不得放进 Git。

## 选择回退路径

- 数据库迁移未执行：恢复上一个应用镜像 digest，验证后恢复旧入口。
- 迁移已执行且发布清单证明旧应用向后兼容：只回退应用镜像，重复健康、业务、对象和财务核对。
- 迁移不兼容或数据已写入：不得把旧应用连接新数据库。保持新系统维护，按 `backup-restore.md` 把切换前协调备份恢复到新的受控环境，再恢复旧入口。

恢复旧入口/TLS/DNS 只能由授权人员执行。先走内部入口验证旧系统登录、书库、章节、钱包/订单只读查询和支付回调替身，再逐步恢复流量。旧系统恢复期间，真实支付回调按批准的暂停、渠道重投或人工审核队列处理；禁止双写。

## 切换后事实处置

对封存事实逐项分类为“可由原渠道幂等重投”“需人工补偿”“仅保留审计”。任何订单、钱包、权益或回调写入旧系统前，必须有单独的数据变更方案、双人复核、备份和财务验收；本 Runbook 不授权自动重放。新 PostgreSQL、MinIO、日志和备份保留到事件复盘批准清理为止。

## 完成标准

- 旧入口健康，关键业务、支付替身和监控通过，错误率恢复到批准阈值；
- 新系统切换后 dump、对象、哈希和脱敏差异清单完整；
- 支付/Worker 处置责任人明确，财务没有未登记差异；
- 报告记录实际 RTO/RPO、触发原因、时间线、数据差异、外部变更和下一次 Go/No-Go 修复项。
