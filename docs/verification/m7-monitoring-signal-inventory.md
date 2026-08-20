# M7 监控信号与缺口清单

检查日期：2026-08-21

## 结论

根 `compose.yaml` 已实现默认关闭的 `monitoring` profile，包含 Prometheus、Alertmanager、PostgreSQL/Redis exporter、MinIO 原生指标、cAdvisor、Blackbox Exporter 和本地通用 Webhook sink。21 条规则覆盖应用、依赖、容器、支付、任务、迁移、对象和财务；真实隔离栈已证明 8 个 target 全部 `up=1`，Reader 故障 firing 和恢复 resolved 均送达 sink。完整证据见 `docs/verification/m7-monitoring-alerting.md`。

## 已有信号

| 信号 | 当前能力 | 限制 |
| --- | --- | --- |
| `moonbook_http_requests_total` | 按 method、Gin 路由模板和 HTTP status 计数；路由标签为低基数 | 只覆盖 Go Server，Reader 兼容业务大量使用 HTTP 200，不能只按 status 判断业务失败 |
| `moonbook_http_request_duration_seconds` | 按 method、路由模板记录默认 Prometheus 延迟桶 | 没有区分业务结果、依赖耗时或外部调用阶段 |
| `moonbook_http_requests_in_flight` | 当前 Go Server 在途请求数 | 没有连接池、队列等待或容量上限对照 |
| `moonbook_platform_jobs` | 按 module、job_type、status 汇总持久化任务 | 没有最老任务年龄、重试速率、处理耗时或失败原因维度 |
| `moonbook_platform_job_overdue_leases` | 统计租约已过期的 running 任务 | 只给总量，无法区分模块和任务类型 |
| `moonbook_platform_job_metrics_collection_success` | 标记任务指标 SQL 采集是否成功 | 不能替代 PostgreSQL 自身可用性和连接池监控 |
| Go/process collectors | Go runtime、进程 CPU、内存、FD 等标准指标 | 仅 Server 进程，不覆盖 Reader SSR、Nginx、数据库、Redis、MinIO 或宿主机 |
| `/health/live` | 证明 Go HTTP 进程可响应 | 不检查依赖，也没有持续采集和历史 |
| `/health/ready` | 两秒内并发检查 PostgreSQL、Redis、MinIO Bucket 和迁移版本 | 仅返回当前 ok/unavailable；没有每项延迟、错误计数、趋势或告警状态 |
| Compose healthcheck | 覆盖 PostgreSQL、Redis、MinIO、Server、Web、Reader SSR 和 Gateway | Docker health 状态当前没有进入 Prometheus/Alertmanager |

`/metrics` 只在指标配置启用时注册；路由已注册但 Token 为空时 handler 返回 503。Server 在 Compose 中只 `expose` 到内部 backend 网络，但 Gateway 的 `/api/`、`/dev-api/` 和 `/prod-api/` 会转发任意 Server 路径，因此部署设计还需验证外部入口不会成为指标抓取主路径，并保留 Bearer 保护。

## 已关闭与后续增强

| 目标 | 当前缺口 |
| --- | --- |
| PostgreSQL | 已有只读 `pg_monitor` exporter 和连接告警；慢查询基线、复制告警按生产拓扑后续配置 |
| Redis | 已有认证 exporter、可用性和内存告警；命中率/淘汰趋势可在稳定基线后收紧 |
| MinIO | 已有内部网络原生指标、缺失指标和离线盘告警；生产容量阈值需结合实际卷配额 |
| Gateway/Web/Reader SSR | Gateway、Reader、Server readiness 已有 Blackbox；Nginx/Node 内部延迟仍可后续细化 |
| 容器与主机 | cAdvisor 已覆盖容器资源；宿主磁盘和 Docker 重启事件需部署环境 exporter 补充 |
| 业务可靠性 | 支付、任务、迁移错误、异常对象和缓存式全域财务核对已有指标与告警；备份新鲜度归 Task 12 |
| Worker | 当前覆盖状态、失败、积压和过期租约；最老任务年龄与分类型耗时仍是增强项 |
| 日志关联 | 已有 request/trace/task 标识，但没有日志聚合或从告警跳转到对应日志证据的路径 |

## 已确认的实现边界

- 监控是根 Compose 的可选 `monitoring` profile，默认业务栈不强制启动可视化组件；
- 首期通知出口为 Alertmanager UI 与通用 Webhook，不配置 SMTP；
- 所有监控组件保持 Compose 可运行，并避免依赖宿主机固定业务路径，为后续 k3s 迁移保留 ConfigMap/Secret/PVC 对应边界；
- Prometheus 抓取凭据、Webhook URL/Token 和管理密码只能由环境变量或 Secret 注入；
- 监控不得直接修改业务数据，也不得把 Redis 当作告警或任务状态的唯一事实源。

## 运行入口

配置、Secret、告警分级、静默和故障处置见 `docs/runbooks/monitoring.md`。生产通知目标仍必须通过环境变量注入，仓库不保存 URL、Token 或凭据。
