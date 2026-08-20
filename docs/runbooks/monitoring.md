# Moonbook 监控与告警 Runbook

## 启动边界

监控栈属于根 `compose.yaml` 的可选 `monitoring` profile，默认不会启动。镜像使用固定版本；Prometheus、Alertmanager 和 Webhook 只绑定 `127.0.0.1`，生产环境通过受控反向代理、VPN 或运维隧道访问，不直接暴露公网。

启动前必须在仓库外创建指标 Token 文件，权限设为 `0600`，文件内容与 `MOONBOOK_METRICS_TOKEN` 完全一致。`.env` 还必须设置独立的 `POSTGRES_EXPORTER_PASSWORD`；初始化服务只创建/轮换固定账号 `moonbook_monitor` 并授予 PostgreSQL 内建只读角色 `pg_monitor`。Redis exporter 使用现有 Redis 认证；MinIO 指标仅在 Compose backend 和宿主回环网络可达。

```bash
mkdir -p .local
printf '%s' "$MOONBOOK_METRICS_TOKEN" > .local/moonbook-metrics-token
chmod 600 .local/moonbook-metrics-token
docker compose --profile monitoring up -d --build
```

本地入口：Prometheus `http://127.0.0.1:19090`，Alertmanager `http://127.0.0.1:19093`，Webhook 状态 `http://127.0.0.1:19094/events`。端口可由环境变量覆盖。

## 通知

Alertmanager 总是把通知发送到本地 `monitoring-webhook`，默认只保留聚合计数，不记录标签、业务 ID、正文或 Secret。生产转发地址和可选 Bearer Token 分别通过 `MONITORING_WEBHOOK_FORWARD_URL`、`MONITORING_WEBHOOK_FORWARD_TOKEN` 注入；转发返回非 2xx 时本地 sink 返回 502，使 Alertmanager 保留失败状态并重试。

## 验证

```bash
scripts/verify-monitoring.sh --config /absolute/path/to/.env
scripts/verify-monitoring.sh --inject /absolute/path/to/.env
```

`--config` 使用自动删除的工具容器运行镜像内 `promtool` 和 `amtool`，不创建 Compose 网络或数据卷。`--inject` 会启动本项目栈、暂时停止 Reader SSR，等待 `MoonbookEndpointDown` firing 通知到达 sink，再恢复 Reader 并等待 resolved；脚本用 trap 保证异常退出时也尝试恢复 Reader。

## 分级与静默

- `critical`：公开入口或核心依赖不可用、支付滞留、任务租约过期，立即通知值班人员；五分钟内确认，十五分钟内决定降级或回退。
- `warning`：容量、延迟、失败任务或错误率趋势，需要当班内处理并创建记录。
- 只允许按 `alertname`、`service` 和明确维护窗口静默，禁止使用空 matcher 或长期全局静默。
- 关闭告警前必须确认指标恢复、对应 Runbook 验证完成且 resolved 已送达；不能仅删除规则或停止 Prometheus。

Prometheus 默认保留 15 天，可用 `PROMETHEUS_RETENTION_TIME` 调整。调整前核对卷容量；告警历史不是审计记录，重要事件结论必须写入事件报告。
